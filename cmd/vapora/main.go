package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MalPr0/vapora/internal/tcpchat"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/upnp"
)

// version is stamped at build time with -ldflags "-X main.version=...". Two
// peers running different builds have to be able to say so out loud, because a
// wire format mismatch otherwise just looks like a peer that never answers.
var version = "dev"

const (
	discoveryTimeout = 3 * time.Second
	cleanupTimeout   = 10 * time.Second
	mappingLabel     = "vapora"
)

type options struct {
	port     int
	protocol string
	lease    time.Duration
	skipUPnP bool
	upstream string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "help"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}

	switch command {
	case "probe":
		return runProbe(args)
	case "serve":
		return runServe(args)
	case "connect":
		return runConnect(args)
	case "nat":
		return runNAT(args)
	case "punch":
		return runPunch(args)
	case "room":
		return runRoom(args)
	case "diag":
		return runDiag(args)
	case "version", "-v", "--version":
		fmt.Printf("vapora %s\n", version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func printUsage() {
	fmt.Print(`vapora - open a direct channel between two peers from a single link

  vapora nat                   classify the NAT and tell whether punching works
  vapora diag                  find out which of your routers filters, and how
  vapora punch                 print an invite and wait for a friend to join
  vapora punch <host:port/key> join with the invite a friend sent you
  vapora room                  open a room for several people and print an invite
  vapora room <invite>         join a room with the invite somebody sent you
  vapora probe                 discover the gateway and show its external IP
  vapora serve [-port 40404]      map a port via UPnP and host an authenticated chat
  vapora connect <host:port/key>  join a chat with the invite its host printed
  vapora version               print the build version

punch flags:
  -port       local UDP port, 0 lets the OS choose
  -timeout    how long to keep punching before giving up (default 3m)
  -keepalive  how often to refresh the NAT binding while waiting (default 25s)
  -plain      skip the full screen UI and use plain lines

room flags:
  -port        local UDP port, 0 lets the OS choose
  -timeout     how long to keep trying before giving up (default 3m)
  -keepalive   how often to refresh the NAT binding (default 25s)
  -plain       skip the full screen UI and use plain lines
  -standalone  stay open with nobody else here. Off by default, so a room
               cannot outlive the conversation in it
  -discover    also find each other through the BitTorrent DHT. Publishes
               your address on a public network
  commands:    !who lists who is present, !invite prints a fresh invite,
               !exit leaves

nat flags:
  -port      measure this UDP port instead of one the OS picks, which is
             what a firewall rule opening one specific port calls for
  -pair      combine with the profile the other side reported
  -room      comma separated profiles of everyone who will be in a room

diag flags:
  -only      run just one part: pcp or filter
  -gateway   probe this address instead of searching
  -upstream  address of the upstream router when behind double NAT
  -port      local UDP port for the subject socket
  -external  external port to ask the IGD for

probe flags:
  -gateway   query this router directly instead of searching by multicast

serve flags:
  -port      external and internal TCP port to map (default 40404)
  -protocol  TCP or UDP (default TCP)
  -lease     mapping lease duration, 0 for permanent (default 1h)
  -upstream  address of the upstream router when behind double NAT
  -no-upnp   skip the mapping and only run the chat server
`)
}

func runProbe(args []string) error {
	flags := flag.NewFlagSet("probe", flag.ContinueOnError)
	host := flags.String("gateway", "", "query this router directly instead of searching by multicast")
	if err := flags.Parse(args); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	gateway, err := discover(ctx, *host)
	if err != nil {
		return err
	}

	externalIP, err := gateway.ExternalIP(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("external IP: %s\n", externalIP)
	return nil
}

func runServe(args []string) error {
	opts, err := parseServeOptions(args)
	if err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	// The mapped port is reachable from the internet, so joining takes the
	// session key exactly like the punch channel does.
	secret, err := punch.NewSecret()
	if err != nil {
		return err
	}

	server := tcpchat.NewServer(os.Stdout, secret)
	listener, err := server.Listen(ctx, fmt.Sprintf(":%d", opts.port))
	if err != nil {
		return err
	}

	if !opts.skipUPnP {
		release, err := openPort(ctx, opts)
		if err != nil {
			return err
		}
		defer release()
	}

	fmt.Printf("\nchat listening on port %d. Send this to your friend:\n\n    vapora connect <your-ip>:%d/%s\n\n",
		opts.port, opts.port, secret)
	fmt.Println("type to talk, ctrl+c to quit")

	return server.Serve(ctx, listener, os.Stdin)
}

func runConnect(args []string) error {
	flags := flag.NewFlagSet("connect", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("connect requires a single host:port argument")
	}

	ctx, cancel := signalContext()
	defer cancel()

	invite, err := punch.ParseInvite(flags.Arg(0))
	if err != nil {
		return err
	}
	if len(invite.Secret) == 0 {
		return errors.New("that invite carries no secret, and this chat does not accept unauthenticated peers")
	}

	fmt.Printf("connecting to %s\n", invite.Endpoint)
	return tcpchat.Dial(ctx, invite.Endpoint.String(), invite.Secret, os.Stdin, os.Stdout)
}

func parseServeOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	opts := options{}
	flags.IntVar(&opts.port, "port", 40404, "external and internal port to map")
	flags.StringVar(&opts.protocol, "protocol", "TCP", "TCP or UDP")
	flags.DurationVar(&opts.lease, "lease", time.Hour, "mapping lease duration, 0 for permanent")
	flags.BoolVar(&opts.skipUPnP, "no-upnp", false, "skip the port mapping")
	flags.StringVar(&opts.upstream, "upstream", "", "address of the upstream router when behind double NAT")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	opts.protocol = strings.ToUpper(opts.protocol)
	if opts.protocol != "TCP" && opts.protocol != "UDP" {
		return options{}, fmt.Errorf("invalid protocol %q, expected TCP or UDP", opts.protocol)
	}
	if opts.port < 1 || opts.port > 65535 {
		return options{}, fmt.Errorf("invalid port %d", opts.port)
	}
	return opts, nil
}

// openPort maps the port on every NAT hop and returns the cleanup function that
// removes them again, so a finished session does not leave the routers exposed.
func openPort(ctx context.Context, opts options) (func(), error) {
	fmt.Println("searching for an UPnP gateway...")

	chain, err := upnp.OpenChain(ctx, upnp.ChainOptions{
		Protocol:         opts.protocol,
		Port:             opts.port,
		Description:      mappingLabel,
		Lease:            opts.lease,
		DiscoveryTimeout: discoveryTimeout,
		Upstream:         opts.upstream,
	})
	release := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := chain.Close(cleanupCtx); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove every port mapping: %v\n", err)
			return
		}
		fmt.Printf("port mappings %s/%d removed\n", opts.protocol, opts.port)
	}

	if err != nil {
		release()
		if upnp.IsConflict(err) {
			return nil, fmt.Errorf("port %d is already mapped by another device, pick another one with -port: %w", opts.port, err)
		}
		return nil, err
	}

	printChain(chain)
	return release, nil
}

func printChain(chain *upnp.Chain) {
	for i, hop := range chain.Hops {
		fmt.Printf("hop %d: %s (%s)\n  %s %s:%d -> %s:%d\n",
			i+1, hop.Gateway.FriendlyName, hop.Gateway.ControlURL,
			chain.Protocol, hop.ExternalIP, chain.Port, hop.InternalHost, chain.Port)
	}

	address, public := chain.PublicAddress()
	if !public {
		fmt.Printf("\nwarning: %s is not reachable from the internet\n", address)
		if chain.Reason != "" {
			fmt.Printf("reason: %s\n", chain.Reason)
		}
		fmt.Println("your friend will not get through until the outermost router forwards the port")
		return
	}

	if chain.Reason != "" {
		fmt.Printf("\nnote: %s\n", chain.Reason)
	}
	fmt.Printf("\nshare this with your friend:\n  vapora connect %s\n  (or: nc %s)\n\n",
		address, strings.Replace(address, ":", " ", 1))
}

func discover(ctx context.Context, host string) (*upnp.Gateway, error) {
	fmt.Println("searching for an UPnP gateway...")

	var gateway *upnp.Gateway
	var err error
	if host != "" {
		gateway, err = upnp.DiscoverAt(ctx, host, discoveryTimeout)
	} else {
		gateway, err = upnp.Discover(ctx, discoveryTimeout)
	}
	if err != nil {
		return nil, err
	}

	fmt.Printf("gateway: %s\n  service: %s\n  control: %s\n  local address: %s\n",
		gateway.FriendlyName, gateway.ServiceType, gateway.ControlURL, gateway.LocalAddress)
	return gateway, nil
}

func formatLease(lease time.Duration) string {
	if lease == 0 {
		return "permanent"
	}
	return lease.String()
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		// Hand the signal back to the runtime once the first one has been
		// delivered, so a second ctrl+c kills the process outright. The first
		// interrupt asks the session to leave properly; if anything is wedged,
		// the second must not have to ask anybody's permission.
		stop()
	}()

	return ctx, stop
}
