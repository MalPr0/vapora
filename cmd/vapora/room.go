package main

import (
	"bufio"
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

	mu     sync.Mutex
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

	open := &room{conn: conn, mux: mux, room: party, watcher: watcher, joining: joining, timeout: *timeout, advertise: *advertise}

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

	if r.joining != nil {
		fmt.Printf("\njoining the room at %s\n", r.joining.Endpoint)
		if err := r.room.Join(ctx, *r.joining, r.timeout); err != nil {
			return err
		}
		fmt.Printf("you are in. You are %s\n", r.room.Me().Name)
	} else {
		fmt.Println("\nwaiting for somebody to join...")
	}

	go r.watchMembers(ctx)
	return r.readInput(ctx, quit)
}

// connect is the same sequence for the full screen front end, reported through
// the view instead of to standard output.
func (r *room) connect(ctx context.Context, chat *tui.Chat) error {
	chat.SetStatus("looking up your public endpoint", 0.05)
	go r.announce(ctx, r.advertise, chat)

	if r.joining == nil {
		// The invite is only on the waiting screen, and it is the whole point of
		// hosting: going straight to an empty conversation leaves the host with
		// nothing to send anyone.
		chat.SetStatus("waiting for somebody to join", 0.2)
		if err := r.waitForCompany(ctx); err != nil {
			return err
		}
		r.mu.Lock()
		r.joined = true
		r.mu.Unlock()
		return nil
	}

	chat.SetStatus("joining the room", 0.2)
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
		case trimmed(line) == "!who":
			r.listMembers()
		case trimmed(line) == "!invite":
			if r.endpoint == nil {
				fmt.Println("-- no address to put on an invite yet")
				continue
			}
			r.show(r.endpoint, nil)
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

func trimmed(line string) string { return strings.TrimSpace(line) }
