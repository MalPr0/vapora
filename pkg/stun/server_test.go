package stun

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// moving is an address the test changes while the server is reading it, which
// is the point: an address that moves under a running watcher is the whole
// scenario. It needs a lock for the same reason.
type moving struct {
	mu   sync.Mutex
	addr *net.UDPAddr
}

func (m *moving) get() *net.UDPAddr {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addr
}

func (m *moving) set(addr *net.UDPAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addr = addr
}

// A STUN server, small enough to be obviously right, so the client can be
// tested against answers this side chooses — including the ones a real server
// would never send and a stranger on a shared port might.
type server struct {
	conn *net.UDPConn
	// sees is what this server claims to observe, which is how a test makes
	// two servers disagree and produce a symmetric NAT.
	sees func(from *net.UDPAddr) *net.UDPAddr
	// other is the OTHER-ADDRESS this server advertises, which is what the
	// filtering probes need in order to ask for a reply from elsewhere.
	other *net.UDPAddr
	// mute drops requests instead of answering, for testing timeouts.
	mute bool
}

func listen(t *testing.T) *net.UDPConn {
	t.Helper()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func serve(t *testing.T, ctx context.Context, srv *server) string {
	t.Helper()

	srv.conn = listen(t)
	if srv.sees == nil {
		srv.sees = func(from *net.UDPAddr) *net.UDPAddr { return from }
	}

	go func() {
		buffer := make([]byte, 2048)
		for {
			read, from, err := srv.conn.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			if srv.mute || read < headerSize || ctx.Err() != nil {
				continue
			}
			var transaction [12]byte
			copy(transaction[:], buffer[8:20])
			_, _ = srv.conn.WriteToUDP(binding(transaction, srv.sees(from), srv.other), from)
		}
	}()

	return srv.conn.LocalAddr().String()
}

// binding builds a success response carrying an XOR-MAPPED-ADDRESS, and an
// OTHER-ADDRESS when one is given.
func binding(transaction [12]byte, mapped, other *net.UDPAddr) []byte {
	body := xorMapped(transaction, mapped)
	if other != nil {
		body = append(body, addressAttribute(attrOtherAddress, other)...)
	}

	response := make([]byte, headerSize, headerSize+len(body))
	binary.BigEndian.PutUint16(response[0:2], 0x0101) // binding success
	binary.BigEndian.PutUint16(response[2:4], uint16(len(body)))
	binary.BigEndian.PutUint32(response[4:8], magicCookie)
	copy(response[8:20], transaction[:])
	return append(response, body...)
}

func xorMapped(transaction [12]byte, addr *net.UDPAddr) []byte {
	value := make([]byte, 8)
	value[1] = 0x01 // IPv4
	binary.BigEndian.PutUint16(value[2:4], uint16(addr.Port)^uint16(magicCookie>>16))

	var cookie [4]byte
	binary.BigEndian.PutUint32(cookie[:], magicCookie)
	for i, octet := range addr.IP.To4() {
		value[4+i] = octet ^ cookie[i]
	}

	attribute := make([]byte, 4, 4+len(value))
	binary.BigEndian.PutUint16(attribute[0:2], attrXORMappedAddress)
	binary.BigEndian.PutUint16(attribute[2:4], uint16(len(value)))
	return append(attribute, value...)
}

func addressAttribute(kind uint16, addr *net.UDPAddr) []byte {
	value := make([]byte, 8)
	value[1] = 0x01
	binary.BigEndian.PutUint16(value[2:4], uint16(addr.Port))
	copy(value[4:8], addr.IP.To4())

	attribute := make([]byte, 4, 4+len(value))
	binary.BigEndian.PutUint16(attribute[0:2], kind)
	binary.BigEndian.PutUint16(attribute[2:4], uint16(len(value)))
	return append(attribute, value...)
}

// A watcher only ever writes: something else owns the read loop and hands it
// what arrives. That is what lets one socket carry STUN and a conversation at
// the same time, and it means a test has to play the part of that reader.
func relay(ctx context.Context, conn *net.UDPConn, watcher *Watcher) {
	buffer := make([]byte, 2048)
	for ctx.Err() == nil {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		read, from, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}
		watcher.Handle(append([]byte(nil), buffer[:read]...), from)
	}
}

func TestQueryReadsTheAddressAServerSees(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seen := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}
	address := serve(t, ctx, &server{
		sees: func(*net.UDPAddr) *net.UDPAddr { return seen },
	})

	conn := listen(t)
	mapped, err := Query(ctx, conn, address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.String() != seen.String() {
		t.Fatalf("read %s, want %s", mapped, seen)
	}
}

// The XOR in XOR-MAPPED-ADDRESS exists because middleboxes used to rewrite
// anything that looked like an address in a packet. Reading it wrong would
// produce a plausible but incorrect address, which is the worst kind.
func TestTheAddressIsUnmaskedCorrectly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, want := range []*net.UDPAddr{
		{IP: net.IPv4(203, 0, 113, 7), Port: 41001},
		{IP: net.IPv4(198, 51, 100, 1), Port: 1},
		{IP: net.IPv4(192, 0, 2, 255), Port: 65535},
	} {
		address := serve(t, ctx, &server{
			sees: func(*net.UDPAddr) *net.UDPAddr { return want },
		})

		conn := listen(t)
		mapped, err := Query(ctx, conn, address, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if mapped.String() != want.String() {
			t.Fatalf("read %s, want %s", mapped, want)
		}
	}
}

// A server that never answers must not hang the caller forever.
func TestASilentServerTimesOut(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	address := serve(t, ctx, &server{mute: true})
	conn := listen(t)

	started := time.Now()
	if _, err := Query(ctx, conn, address, 300*time.Millisecond); err == nil {
		t.Fatal("a silent server was read as an answer")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("the timeout took %s to give up", elapsed)
	}
}

// Servers that agree mean the address is the same wherever it is sent, which
// is the cone case. Servers that disagree mean it is not.
func TestMappingIsClassifiedFromWhetherServersAgree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	same := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}
	agreeing := []string{
		serve(t, ctx, &server{sees: func(*net.UDPAddr) *net.UDPAddr { return same }}),
		serve(t, ctx, &server{sees: func(*net.UDPAddr) *net.UDPAddr { return same }}),
	}

	conn := listen(t)
	report, err := ProbeWith(ctx, conn, agreeing, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mapping != MappingEndpointIndependent {
		t.Fatalf("agreeing servers gave %s", report.Mapping)
	}

	// Now two that report different ports for the same socket.
	first := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}
	second := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 51002}
	disagreeing := []string{
		serve(t, ctx, &server{sees: func(*net.UDPAddr) *net.UDPAddr { return first }}),
		serve(t, ctx, &server{sees: func(*net.UDPAddr) *net.UDPAddr { return second }}),
	}

	other := listen(t)
	report, err = ProbeWith(ctx, other, disagreeing, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mapping != MappingAddressDependent {
		t.Fatalf("disagreeing servers gave %s, want address dependent", report.Mapping)
	}
}

// A server that failed is not a server that disagreed. Keeping the failures
// is what stops one unreachable server from being read as a symmetric NAT.
func TestAFailedServerIsNotADisagreement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	same := &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}
	servers := []string{
		serve(t, ctx, &server{sees: func(*net.UDPAddr) *net.UDPAddr { return same }}),
		serve(t, ctx, &server{mute: true}),
	}

	conn := listen(t)
	report, err := ProbeWith(ctx, conn, servers, 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if report.Mapping == MappingAddressDependent {
		t.Fatal("a server that never answered was counted as disagreeing")
	}

	var failed int
	for _, observation := range report.Observations {
		if observation.Err != nil {
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("%d observations failed, want the one muted server", failed)
	}
}

// Both classifications name the behaviour in the RFC's vocabulary and in the
// one people say out loud, because readers arrive knowing one or the other.
func TestClassificationsReadInBothVocabularies(t *testing.T) {
	cases := map[string][]string{
		Mapping(MappingEndpointIndependent).String():         {"endpoint-independent", "cone"},
		Mapping(MappingAddressDependent).String():            {"address-dependent", "symmetric"},
		Filtering(FilteringEndpointIndependent).String():     {"endpoint-independent"},
		Filtering(FilteringAddressAndPortDependent).String(): {"port"},
		Filtering(FilteringAddressDependent).String():        {"address"},
	}

	for got, wants := range cases {
		for _, want := range wants {
			if !contains(got, want) {
				t.Fatalf("%q does not mention %q", got, want)
			}
		}
	}

	if Mapping(MappingUnknown).String() == "" || Filtering(FilteringUnknown).String() == "" {
		t.Fatal("an unmeasured classification renders as nothing at all")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// The watcher is what notices an address changing, which is the difference
// between an invite that works and one that quietly stopped.
func TestTheWatcherReportsAnAddressAndThenAChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	current := &moving{addr: &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}}
	address := serve(t, ctx, &server{
		sees: func(*net.UDPAddr) *net.UDPAddr { return current.get() },
	})

	conn := listen(t)
	watcher := NewWatcher([]string{address}, 100*time.Millisecond)

	changes := make(chan *net.UDPAddr, 4)
	watcher.OnChange(func(_, now *net.UDPAddr) { changes <- now })
	go watcher.Run(ctx, conn)
	go relay(ctx, conn, watcher)

	first, err := watcher.Wait(ctx, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if first.String() != current.get().String() {
		t.Fatalf("first answer was %s", first)
	}
	if watcher.Endpoint().String() != first.String() {
		t.Fatal("Endpoint disagrees with what Wait returned")
	}

	// The address moves, the way it does on a router that rebooted.
	current.set(&net.UDPAddr{IP: net.IPv4(203, 0, 113, 9), Port: 51002})

	select {
	case moved := <-changes:
		if moved.String() != current.get().String() {
			t.Fatalf("reported a move to %s", moved)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the address changed and nobody was told")
	}
}

// Nothing useful can happen before the first answer, so Wait must not return
// an address that does not exist yet.
func TestWaitGivesUpRatherThanInventingAnAddress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	address := serve(t, ctx, &server{mute: true})
	conn := listen(t)

	watcher := NewWatcher([]string{address}, 50*time.Millisecond)
	go watcher.Run(ctx, conn)
	go relay(ctx, conn, watcher)

	if endpoint, err := watcher.Wait(ctx, 400*time.Millisecond); err == nil {
		t.Fatalf("waiting on silence produced %s", endpoint)
	}
	if watcher.Endpoint() != nil {
		t.Fatal("an endpoint appeared without any server answering")
	}
}
