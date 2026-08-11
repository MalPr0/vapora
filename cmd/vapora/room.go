package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

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
	timeout   time.Duration
	endpoint  *net.UDPAddr
	announced bool
}

func runRoom(args []string) error {
	flags := flag.NewFlagSet("room", flag.ContinueOnError)
	localPort := flags.Int("port", 0, "local UDP port, 0 lets the OS choose")
	timeout := flags.Duration("timeout", 3*time.Minute, "how long to keep trying before giving up")
	keepalive := flags.Duration("keepalive", stun.DefaultKeepalive, "how often to refresh the NAT binding")
	advertise := flags.String("advertise", "", "put this address on invites instead of the one STUN reports")
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
		Mux:      mux,
		Output:   os.Stdout,
	})
	if err != nil {
		return err
	}

	open := &room{conn: conn, mux: mux, room: party, watcher: watcher, joining: joining, timeout: *timeout}
	party.Observe(open)

	go mux.Run(ctx)
	go watcher.Run(ctx, conn)
	go open.announce(ctx, *advertise)

	if joining != nil {
		fmt.Printf("\njoining the room at %s\n", joining.Endpoint)
		if err := party.Join(ctx, *joining, *timeout); err != nil {
			return err
		}
		fmt.Printf("you are in. You are %s\n", party.Me().Name)
	} else {
		fmt.Println("\nwaiting for somebody to join...")
	}

	go open.watchMembers(ctx)
	return open.readInput(ctx, cancel)
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
func (r *room) announce(ctx context.Context, advertise string) {
	if advertise != "" {
		if endpoint, err := net.ResolveUDPAddr("udp4", advertise); err == nil {
			r.show(endpoint)
			return
		}
	}

	r.watcher.OnChange(func(_, current *net.UDPAddr) {
		fmt.Printf("\n-- your address changed, the invite you shared is dead. Send this one:\n")
		r.show(current)
	})

	endpoint, err := r.watcher.Wait(ctx, endpointTimeout)
	if err != nil {
		fmt.Println("\nno STUN server answered, so there is no address to put on an invite.")
		return
	}
	r.show(endpoint)
}

func (r *room) show(endpoint *net.UDPAddr) {
	r.endpoint = endpoint
	fmt.Printf("\ninvite anyone with this, it is a runnable command:\n\n    %s\n\n",
		r.room.Invite(endpoint).Command(roomCommand))
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
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		line := scanner.Text()

		switch {
		case isExit(line):
			r.room.Goodbye()
			quit()
			return nil
		case strings.TrimSpace(line) == "!who":
			r.listMembers()
		case strings.TrimSpace(line) == "!invite":
			if r.endpoint == nil {
				fmt.Println("-- no address to put on an invite yet")
				continue
			}
			r.show(r.endpoint)
		case line != "":
			r.room.Broadcast(line)
		}
	}

	<-ctx.Done()
	return nil
}

func (r *room) listMembers() {
	members := r.room.Members()
	fmt.Printf("-- you are %s, with %d other(s):\n", r.room.Me().Name, len(members))
	for _, member := range members {
		fmt.Printf("   %-22s %s %s\n", member.Name, member.Health.Link, member.Addr)
	}
}
