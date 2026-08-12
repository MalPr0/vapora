package upnp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ErrNoGateway means nothing answered the multicast search. It is the normal
// outcome on a network whose router has UPnP switched off, which is most of
// them, and it is not on its own a reason to stop.
var ErrNoGateway = errors.New("upnp: no internet gateway device found on the local network")

const (
	ssdpMulticastAddr = "239.255.255.250:1900"
	ssdpMaxWait       = 2
)

var searchTargets = []string{
	"urn:schemas-upnp-org:service:WANIPConnection:1",
	"urn:schemas-upnp-org:service:WANPPPConnection:1",
	"urn:schemas-upnp-org:device:InternetGatewayDevice:1",
}

// Discover performs an SSDP M-SEARCH on every multicast-capable interface and
// returns the first gateway exposing a usable WAN connection service.
func Discover(ctx context.Context, timeout time.Duration) (*Gateway, error) {
	locations, err := searchLocations(ctx, timeout)
	if err != nil {
		return nil, err
	}
	if len(locations) == 0 {
		return nil, ErrNoGateway
	}

	var lastErr error
	for _, location := range locations {
		gateway, err := gatewayFromLocation(ctx, location)
		if err != nil {
			lastErr = err
			continue
		}
		return gateway, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("upnp: no usable gateway among %d responses: %w", len(locations), lastErr)
	}
	return nil, ErrNoGateway
}

// DiscoverAt queries a single host with a unicast M-SEARCH. It is the way to
// reach an upstream router in a double NAT setup, where the second gateway is
// routable but sits outside the multicast domain of this host.
func DiscoverAt(ctx context.Context, host string, timeout time.Duration) (*Gateway, error) {
	locations, err := searchUnicast(ctx, host, timeout)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, location := range locations {
		gateway, err := gatewayFromLocation(ctx, location)
		if err != nil {
			lastErr = err
			continue
		}
		return gateway, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("upnp: %s answered but exposes no usable gateway: %w", host, lastErr)
	}
	return nil, fmt.Errorf("%w: %s did not answer a unicast M-SEARCH", ErrNoGateway, host)
}

func searchUnicast(ctx context.Context, host string, timeout time.Duration) ([]string, error) {
	target, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, "1900"))
	if err != nil {
		return nil, fmt.Errorf("upnp: invalid gateway host %q: %w", host, err)
	}

	conn, err := net.DialUDP("udp4", nil, target)
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot reach %s: %w", host, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	for _, searchTarget := range searchTargets {
		if _, err := conn.Write(searchRequest(searchTarget)); err != nil {
			return nil, fmt.Errorf("upnp: cannot send M-SEARCH to %s: %w", host, err)
		}
	}

	var locations []string
	seen := map[string]bool{}
	buffer := make([]byte, 8192)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}
		location, err := parseLocation(buffer[:n])
		if err != nil || seen[location] {
			continue
		}
		seen[location] = true
		locations = append(locations, location)
	}
	return locations, nil
}

func searchLocations(ctx context.Context, timeout time.Duration) ([]string, error) {
	interfaces, err := multicastInterfaces()
	if err != nil {
		return nil, err
	}
	if len(interfaces) == 0 {
		return nil, errors.New("upnp: no multicast-capable network interface available")
	}

	found := make(chan string)
	var wg sync.WaitGroup
	for _, addr := range interfaces {
		wg.Add(1)
		go func(local net.IP) {
			defer wg.Done()
			searchOnInterface(ctx, local, timeout, found)
		}(addr)
	}
	go func() {
		wg.Wait()
		close(found)
	}()

	var locations []string
	seen := map[string]bool{}
	for location := range found {
		if seen[location] {
			continue
		}
		seen[location] = true
		locations = append(locations, location)
	}
	return locations, nil
}

func searchOnInterface(ctx context.Context, local net.IP, timeout time.Duration, found chan<- string) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: local})
	if err != nil {
		return
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)

	target, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		return
	}
	for _, searchTarget := range searchTargets {
		if _, err := conn.WriteToUDP(searchRequest(searchTarget), target); err != nil {
			return
		}
	}

	buffer := make([]byte, 8192)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		location, err := parseLocation(buffer[:n])
		if err != nil {
			continue
		}
		select {
		case found <- location:
		case <-ctx.Done():
			return
		}
	}
}

func searchRequest(searchTarget string) []byte {
	return []byte(fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"MX: %d\r\n"+
			"ST: %s\r\n"+
			"\r\n",
		ssdpMulticastAddr, ssdpMaxWait, searchTarget))
}

func parseLocation(response []byte) (string, error) {
	reader := bufio.NewReader(bytes.NewReader(response))
	head, err := http.ReadResponse(reader, nil)
	if err != nil {
		return "", fmt.Errorf("upnp: malformed ssdp response: %w", err)
	}
	defer head.Body.Close()

	location := head.Header.Get("LOCATION")
	if location == "" {
		return "", errors.New("upnp: ssdp response without LOCATION header")
	}
	if _, err := url.Parse(location); err != nil {
		return "", fmt.Errorf("upnp: invalid LOCATION %q: %w", location, err)
	}
	return location, nil
}

func multicastInterfaces() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot list network interfaces: %w", err)
	}

	var addresses []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ipv4 := ipNet.IP.To4()
			if ipv4 == nil || ipv4.IsLinkLocalUnicast() {
				continue
			}
			addresses = append(addresses, ipv4)
		}
	}
	return addresses, nil
}
