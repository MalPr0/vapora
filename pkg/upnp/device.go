package upnp

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const descriptionTimeout = 5 * time.Second

var wanServiceTypes = []string{
	"urn:schemas-upnp-org:service:WANIPConnection:2",
	"urn:schemas-upnp-org:service:WANIPConnection:1",
	"urn:schemas-upnp-org:service:WANPPPConnection:1",
}

type deviceDescription struct {
	XMLName xml.Name `xml:"root"`
	Device  device   `xml:"device"`
}

type device struct {
	FriendlyName string    `xml:"friendlyName"`
	DeviceType   string    `xml:"deviceType"`
	Services     []service `xml:"serviceList>service"`
	Devices      []device  `xml:"deviceList>device"`
}

type service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

// Gateway is a discovered IGD ready to accept port mapping commands.
type Gateway struct {
	FriendlyName string
	ServiceType  string
	ControlURL   string
	LocalAddress string

	httpClient *http.Client
}

func gatewayFromLocation(ctx context.Context, location string) (*Gateway, error) {
	baseURL, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("upnp: invalid description url %q: %w", location, err)
	}

	description, err := fetchDescription(ctx, location)
	if err != nil {
		return nil, err
	}

	found, owner := findWANService(description.Device)
	if found == nil {
		return nil, fmt.Errorf("upnp: %s exposes no WAN connection service", location)
	}

	controlURL, err := resolveURL(baseURL, found.ControlURL)
	if err != nil {
		return nil, err
	}

	localAddress, err := localAddressFor(baseURL.Host)
	if err != nil {
		return nil, err
	}

	return &Gateway{
		FriendlyName: owner,
		ServiceType:  found.ServiceType,
		ControlURL:   controlURL,
		LocalAddress: localAddress,
		httpClient:   &http.Client{Timeout: soapTimeout},
	}, nil
}

// Address is the IP of the gateway itself, taken from where its control URL
// lives, which is the only address the device reliably tells us about.
func (g *Gateway) Address() (string, error) {
	parsed, err := url.Parse(g.ControlURL)
	if err != nil {
		return "", fmt.Errorf("upnp: cannot read the gateway address from %q: %w", g.ControlURL, err)
	}
	return parsed.Hostname(), nil
}

func fetchDescription(ctx context.Context, location string) (*deviceDescription, error) {
	requestCtx, cancel := context.WithTimeout(ctx, descriptionTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, location, nil)
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot build description request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot fetch description from %s: %w", location, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upnp: description request to %s returned %s", location, response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("upnp: cannot read description from %s: %w", location, err)
	}

	description := &deviceDescription{}
	if err := xml.Unmarshal(body, description); err != nil {
		return nil, fmt.Errorf("upnp: cannot parse description from %s: %w", location, err)
	}
	return description, nil
}

func findWANService(root device) (*service, string) {
	for _, wanted := range wanServiceTypes {
		if found, owner := findServiceByType(root, wanted); found != nil {
			return found, owner
		}
	}
	return nil, ""
}

func findServiceByType(current device, wanted string) (*service, string) {
	for i := range current.Services {
		if current.Services[i].ServiceType == wanted {
			return &current.Services[i], current.FriendlyName
		}
	}
	for _, child := range current.Devices {
		if found, owner := findServiceByType(child, wanted); found != nil {
			if owner == "" {
				owner = current.FriendlyName
			}
			return found, owner
		}
	}
	return nil, ""
}

func resolveURL(base *url.URL, reference string) (string, error) {
	parsed, err := url.Parse(reference)
	if err != nil {
		return "", fmt.Errorf("upnp: invalid control url %q: %w", reference, err)
	}
	return base.ResolveReference(parsed).String(), nil
}

// localAddressFor returns the local IPv4 the OS would use to reach the gateway,
// which is the address that must be registered as the port mapping target.
func localAddressFor(gatewayHost string) (string, error) {
	conn, err := net.DialTimeout("udp4", gatewayHost, descriptionTimeout)
	if err != nil {
		return "", fmt.Errorf("upnp: cannot determine local address towards %s: %w", gatewayHost, err)
	}
	defer conn.Close()

	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("upnp: unexpected local address type %T", conn.LocalAddr())
	}
	return address.IP.String(), nil
}
