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

	"github.com/MalPr0/vapora/internal/chat"
	"github.com/MalPr0/vapora/pkg/upnp"
)

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
	command := "serve"
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
	case "diag":
		return runDiag(args)
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
  vapora probe                 discover the gateway and show its external IP
  vapora serve [-port 40404]   map a port via UPnP and host a TCP chat on it
  vapora connect <host:port>   join a chat hosted by someone else

punch flags:
  -port       local UDP port, 0 lets the OS choose
  -timeout    how long to keep punching before giving up (default 3m)
  -keepalive  how often to refresh the NAT binding while waiting (default 25s)
  -insecure   drop the invite secret and run the session unauthenticated

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

	server := chat.NewServer(os.Stdout)
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

	fmt.Printf("chat server listening on port %d, type to broadcast, ctrl+c to quit\n", opts.port)
	go chat.Pump(os.Stdin, broadcastWriter{server: server})

	return server.Serve(ctx, listener)
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

	address := flags.Arg(0)
	fmt.Printf("connecting to %s\n", address)
	return chat.Dial(ctx, address, os.Stdin, os.Stdout)
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
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

type broadcastWriter struct {
	server *chat.Server
}

func (w broadcastWriter) Write(payload []byte) (int, error) {
	w.server.Broadcast(fmt.Sprintf("<host> %s", strings.TrimRight(string(payload), "\n")), nil)
	return len(payload), nil
}
