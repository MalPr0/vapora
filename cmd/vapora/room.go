package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/MalPr0/vapora/internal/tui"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
	"github.com/MalPr0/vapora/pkg/text"
)

const roomCommand = "vapora room"

// room is everything a session of more than two needs, assembled before the
// front end takes over.
type room struct {
	conn      *net.UDPConn
	mux       *punch.Mux
	room      *punch.Room
	watcher   *stun.Watcher
	joining   *punch.RoomInvite
	advertise string
	timeout   time.Duration
	endpoint  *net.UDPAddr

	meeting *punch.Rendezvous

	mu     sync.Mutex
	tried  map[string]bool
	shared string
	failed string
	joined bool
}

func runRoom(args []string) error {
	flags := flag.NewFlagSet("room", flag.ContinueOnError)
	localPort := flags.Int("port", 0, "local UDP port, 0 lets the OS choose")
	timeout := flags.Duration("timeout", 3*time.Minute, "how long to keep trying before giving up")
	keepalive := flags.Duration("keepalive", stun.DefaultKeepalive, "how often to refresh the NAT binding")
	advertise := flags.String("advertise", "", "put this address on invites instead of the one STUN reports")
	plain := flags.Bool("plain", false, "skip the full screen UI and use plain lines")
	discover := flags.Bool("discover", false,
		"also find each other through the BitTorrent DHT. Publishes your address on a public network")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: *localPort})
	if err != nil {
		return fmt.Errorf("cannot open local UDP socket: %w", err)
	}
	defer conn.Close()

	identity, err := punch.NewIdentity()
	if err != nil {
		return err
	}

	secret, joining, err := resolveRoom(flags.Args())
	if err != nil {
		return err
	}

	mux := punch.NewMux(conn)
	watcher := stun.NewWatcher(stun.DefaultServers, *keepalive)
	mux.Fallback(punch.SinkFunc(watcher.Handle))

	party, err := punch.NewRoom(punch.RoomOptions{
		Identity: identity,
		Secret:   secret,
		Local:    punch.LocalAddr(conn.LocalAddr().(*net.UDPAddr).Port),
		Mux:      mux,
		Output:   os.Stdout,
	})
	if err != nil {
		return err
	}

	open := &room{conn: conn, mux: mux, room: party, watcher: watcher, joining: joining,
		timeout: *timeout, advertise: *advertise}

	if *discover {
		meeting, err := punch.NewRendezvous(mux, secret, conn.LocalAddr().(*net.UDPAddr).Port)
		if err != nil {
			return err
		}
		// The room greets first and only claims what opens under its key, so a
		// DHT reply falls through to here rather than being swallowed.
		mux.Fallback(punch.SinkFunc(meeting.Deliver))
		open.meeting = meeting
	}

	go mux.Run(ctx)
	go watcher.Run(ctx, conn)

	if !*plain && tui.IsTerminal(os.Stdin) {
		if err := runRoomUI(ctx, open); err == nil || !errors.Is(err, errNoTerminal) {
			return err
		}
		// The terminal refused raw mode, so fall through to plain lines rather
		// than leaving the user with nothing.
	}
	return open.runPlain(ctx, cancel)
}

// runPlain is the line based front end: what a pipe, a CI job or a terminal
// that refuses raw mode gets.
func (r *room) runPlain(ctx context.Context, quit context.CancelFunc) error {
	r.room.Observe(r)
	go r.announce(ctx, r.advertise, nil)
	go r.meet(ctx, func(line string) { fmt.Println("--", line) })

	if r.joining != nil {
		fmt.Printf("\njoining the room at %s\n", r.joining.Endpoint)
		time.AfterFunc(pasteHint, func() {
			r.mu.Lock()
			endpoint, joined := r.endpoint, r.joined
			r.mu.Unlock()
			if endpoint != nil && !joined {
				fmt.Printf("-- if it stalls, send this back for them to paste:\n\n    %s\n\n", endpoint)
			}
		})
		if err := r.room.Join(ctx, *r.joining, r.timeout); err != nil {
			return err
		}
		r.mu.Lock()
		r.joined = true
		r.mu.Unlock()
		fmt.Printf("you are in. You are %s\n", r.room.Me().Name)
	} else {
		r.mu.Lock()
		r.joined = true
		r.mu.Unlock()
		fmt.Println("\nwaiting for somebody to join...")
		time.AfterFunc(pasteHint, func() {
			if len(r.room.Members()) == 0 {
				fmt.Println("-- still nobody. If they cannot get in, paste the address their side printed.")
			}
		})
	}

	go r.watchMembers(ctx)
	return r.readInput(ctx, quit)
}

// connect is the same sequence for the full screen front end, reported through
// the view instead of to standard output.
func (r *room) connect(ctx context.Context, chat *tui.Chat) error {
	chat.SetStatus("looking up your public endpoint", 0.05)
	go r.announce(ctx, r.advertise, chat)
	go r.meet(ctx, chat.System)

	if r.joining == nil {
		// The invite is only on the waiting screen, and it is the whole point of
		// hosting: going straight to an empty conversation leaves the host with
		// nothing to send anyone.
		chat.SetStatus("waiting for somebody to join", 0.2)
		go r.offerPasteHint(ctx, chat)
		if err := r.waitForCompany(ctx); err != nil {
			return err
		}
		r.mu.Lock()
		r.joined = true
		r.mu.Unlock()
		return nil
	}

	chat.SetStatus("joining the room", 0.2)
	go r.offerWayBack(ctx, chat)
	if err := r.room.Join(ctx, *r.joining, r.timeout); err != nil {
		return err
	}

	r.mu.Lock()
	r.joined = true
	r.mu.Unlock()
	return nil
}

// resolveRoom decides between opening a room and joining one: an invite as
// argument means joining, its absence means minting a room nobody is in yet.
func resolveRoom(args []string) (punch.Secret, *punch.RoomInvite, error) {
	if len(args) > 0 {
		invite, err := punch.ParseRoomInvite(args[0])
		if err != nil {
			return nil, nil, err
		}
		return invite.Secret, &invite, nil
	}

	secret, err := punch.NewSecret()
	if err != nil {
		return nil, nil, err
	}
	return secret, nil, nil
}

// announce prints an invite as soon as this side knows its own address, and
// again whenever that address moves out from under it.
func (r *room) announce(ctx context.Context, advertise string, chat *tui.Chat) {
	if advertise != "" {
		if endpoint, err := net.ResolveUDPAddr("udp4", advertise); err == nil {
			r.show(endpoint, chat)
			return
		}
	}

	r.watcher.OnChange(func(_, current *net.UDPAddr) {
		if chat != nil {
			chat.System("your address changed, the invite you shared is dead")
		} else {
			fmt.Print("\n-- your address changed, the invite you shared is dead. Send this one:\n")
		}
		r.show(current, chat)
	})

	endpoint, err := r.watcher.Wait(ctx, endpointTimeout)
	if err != nil {
		if chat != nil {
			chat.SetInvite("no STUN server answered, there is no address to share")
			return
		}
		fmt.Println("\nno STUN server answered, so there is no address to put on an invite.")
		return
	}
	r.show(endpoint, chat)
}

func (r *room) show(endpoint *net.UDPAddr, chat *tui.Chat) {
	invite := r.room.Invite(endpoint).Command(roomCommand)

	r.mu.Lock()
	r.endpoint = endpoint
	r.shared = invite
	r.mu.Unlock()

	if chat != nil {
		chat.SetInvite(invite)
		return
	}
	fmt.Printf("\ninvite anyone with this, it is a runnable command:\n\n    %s\n\n", invite)
}

// offerPasteHint tells the host what to do when nobody arrives. Between two
// networks that both refuse a first packet from a stranger, waiting alone never
// works however long you wait, and nothing on screen would say so.
func (r *room) offerPasteHint(ctx context.Context, chat *tui.Chat) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(pasteHint):
	}
	if len(r.room.Members()) > 0 {
		return
	}
	chat.SetStatus("still nobody. If they cannot get in, paste the address their side printed", 0.4)
}

// offerWayBack gives the newcomer something to send back when their hello is
// being dropped at the other end. Their own address is all the waiting side
// needs: it carries no secret and grants nothing.
func (r *room) offerWayBack(ctx context.Context, chat *tui.Chat) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(pasteHint):
	}

	r.mu.Lock()
	endpoint, joined := r.endpoint, r.joined
	r.mu.Unlock()

	if endpoint == nil || joined {
		return
	}
	chat.SetInvite("if it stalls, send this back for them to paste: " + endpoint.String())
}

// meet runs the DHT side of finding each other: it publishes this address under
// the secret and punches towards whatever else is published there.
//
// Both sides do exactly this, which is what makes it work without either of
// them knowing an address first — and it is also the standoff fix, since both
// end up sending at the same time.
func (r *room) meet(ctx context.Context, say func(string)) {
	if r.meeting == nil {
		return
	}

	err := r.meeting.Publish(ctx, func(peers []*net.UDPAddr) {
		for _, peer := range r.worthTrying(peers) {
			say("found an address on the DHT: " + peer.String())
			r.room.Reach(ctx, peer)
		}
	})
	if err != nil && ctx.Err() == nil {
		say("the DHT is not reachable from here: " + err.Error())
	}
}

// worthTrying filters what the DHT handed back. Nothing there is trustworthy —
// there are nodes that answer every key with whatever addresses they like — so
// this bounds how much traffic a lie can cost: each address is punched at once,
// and only a few of them, because every one of them is a stranger who never
// asked to hear from us.
func (r *room) worthTrying(peers []*net.UDPAddr) []*net.UDPAddr {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tried == nil {
		r.tried = map[string]bool{}
	}

	var fresh []*net.UDPAddr
	for _, peer := range peers {
		if len(fresh) == maxDHTPeers || r.tried[peer.String()] {
			continue
		}
		r.tried[peer.String()] = true
		fresh = append(fresh, peer)
	}
	return fresh
}

// maxDHTPeers is how many unverified addresses are worth a packet per round.
const maxDHTPeers = 4

// waitForCompany holds the host on the waiting screen until the first member
// authenticates. Membership is a poll like everything else here: nothing
// arrives to announce that somebody is about to arrive.
func (r *room) waitForCompany(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(r.timeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("nobody joined within %s", r.timeout)
		case <-ticker.C:
			if len(r.room.Members()) > 0 {
				return nil
			}
		}
	}
}

// reach takes what the other side sent back and starts punching towards it.
// Accepts a whole room invite or a bare host:port, because somebody who cannot
// get in will paste whichever of the two they were handed.
func (r *room) reach(ctx context.Context, line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	endpoint := endpointIn(line)
	if endpoint == nil {
		return "", false
	}

	r.room.Reach(ctx, endpoint)
	return endpoint.String(), true
}

// endpointIn digs an address out of whatever was pasted. Only the address is
// used: a room is joined by producing a hello under its own secret, so an
// address on its own grants nothing.
func endpointIn(line string) *net.UDPAddr {
	if invite, err := punch.ParseRoomInvite(line); err == nil {
		return invite.Endpoint
	}
	for _, field := range strings.Fields(line) {
		field = strings.TrimSuffix(field, ",")
		if endpoint, err := net.ResolveUDPAddr("udp4", field); err == nil && endpoint.Port != 0 {
			return endpoint
		}
	}
	return nil
}

// note records why a session ended, and report writes it to the real screen
// once the full screen view has handed it back.
func (r *room) note(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = reason
}

func (r *room) report() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.joined {
		fmt.Println("\nthe room is closed.")
		return
	}
	fmt.Println("\nyou never got into a room.")
	if r.failed != "" {
		fmt.Printf("  %s\n", r.failed)
	}
	if r.shared != "" {
		fmt.Printf("  the invite this side was offering:\n    %s\n", r.shared)
	}
}

// Message and Typing satisfy the room observer. Every line names who said it:
// a conversation with more than two people has no default speaker.
func (r *room) Message(from punch.Member, payload string) {
	fmt.Printf("<%s> %s\n", from.Name, text.Safe(payload))
}

func (r *room) Typing(punch.Member, bool) {}

// watchMembers reports arrivals and departures by diffing the roster, which is
// polled like everything else about a path.
func (r *room) watchMembers(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	known := map[punch.PublicKey]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			present := map[punch.PublicKey]string{}
			for _, member := range r.room.Members() {
				present[member.Key] = member.Name
				if _, seen := known[member.Key]; !seen {
					fmt.Printf("-- %s joined\n", member.Name)
				}
			}
			for key, name := range known {
				if _, still := present[key]; !still {
					fmt.Printf("-- %s left\n", name)
				}
			}
			known = present
		}
	}
}

func (r *room) readInput(ctx context.Context, quit context.CancelFunc) error {
	lines := readLines(ctx)

	for {
		var line string
		select {
		case <-ctx.Done():
			return nil
		case typed, open := <-lines:
			if !open {
				// Standard input ended, which a pipe does immediately. The
				// session is still live, so wait for it rather than exiting.
				<-ctx.Done()
				return nil
			}
			line = typed
		}

		switch {
		case isExit(line):
			r.room.Goodbye()
			quit()
			return nil
		case trimmed(line) == "!who":
			r.listMembers()
		case trimmed(line) == "!invite":
			if r.endpoint == nil {
				fmt.Println("-- no address to put on an invite yet")
				continue
			}
			r.show(r.endpoint, nil)
		case r.pastePlain(ctx, line):
		case line != "":
			r.room.Broadcast(line)
		}
	}
}

// pastePlain is the line based half of the paste back: an address the other
// side printed, given to a room that nobody has reached yet.
func (r *room) pastePlain(ctx context.Context, line string) bool {
	if strings.HasPrefix(strings.TrimSpace(line), "!") || len(r.room.Members()) > 0 {
		return false
	}

	where, ok := r.reach(ctx, line)
	if !ok {
		return false
	}
	fmt.Printf("-- punching towards %s, they have to be running too\n", where)
	return true
}

func (r *room) listMembers() {
	members := r.room.Members()
	fmt.Printf("-- you are %s, with %d other(s):\n", r.room.Me().Name, len(members))
	for _, member := range members {
		fmt.Printf("   %-22s %s %s\n", member.Name, member.Health.Link, member.Addr)
	}
}

func trimmed(line string) string { return strings.TrimSpace(line) }
