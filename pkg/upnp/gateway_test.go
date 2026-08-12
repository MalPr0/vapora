package upnp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A router is the one thing these tests cannot have, so they stand one up over
// HTTP instead. That is not a compromise: everything that has ever gone wrong
// here is in what comes back — descriptions nested three deep, services in an
// order nobody promised, and refusals that only mean something if you read the
// numeric code out of the fault body.

const description = `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <friendlyName>Test Router</friendlyName>
    <deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType>
    <deviceList>
      <device>
        <deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
        <deviceList>
          <device>
            <deviceType>urn:schemas-upnp-org:device:WANConnectionDevice:1</deviceType>
            <serviceList>
              <service>
                <serviceType>urn:schemas-upnp-org:service:WANPPPConnection:1</serviceType>
                <controlURL>/ppp</controlURL>
              </service>
              <service>
                <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
                <controlURL>/ipc</controlURL>
              </service>
            </serviceList>
          </device>
        </deviceList>
      </device>
    </deviceList>
  </device>
</root>`

// router serves a description and answers SOAP with whatever the test decides.
func router(t *testing.T, answer func(action string) (string, int)) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/desc.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, description)
	})

	handle := func(w http.ResponseWriter, r *http.Request) {
		body, status := answer(r.Header.Get("SOAPAction"))
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/ipc", handle)
	mux.HandleFunc("/ppp", handle)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func envelope(body string) string {
	return `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<s:Body>` + body + `</s:Body></s:Envelope>`
}

func dial(t *testing.T, server *httptest.Server) *Gateway {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gateway, err := gatewayFromLocation(ctx, server.URL+"/desc.xml")
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}

// The connection service can be three devices deep, and which of several is
// chosen is not a matter of order in the list.
func TestTheServiceIsFoundHoweverDeeplyNested(t *testing.T) {
	gateway := dial(t, router(t, func(string) (string, int) { return "", 200 }))

	if !strings.Contains(gateway.ServiceType, "WANIPConnection") {
		t.Fatalf("chose %q, want the IP connection service", gateway.ServiceType)
	}
	if gateway.FriendlyName != "Test Router" {
		t.Fatalf("friendly name is %q", gateway.FriendlyName)
	}

	// A control URL in a description is relative. Until it is resolved against
	// where the description came from, it is not something anything can call.
	if !strings.HasPrefix(gateway.ControlURL, "http://") {
		t.Fatalf("the control url was never resolved: %q", gateway.ControlURL)
	}
}

// The local address comes from the route to the router rather than the
// interface list. A machine with a VPN and a container bridge has several, and
// a mapping installed for the wrong one silently does nothing.
func TestTheLocalAddressIsTheOneThatReachesTheRouter(t *testing.T) {
	gateway := dial(t, router(t, func(string) (string, int) { return "", 200 }))

	if gateway.LocalAddress == "" {
		t.Fatal("no local address was worked out")
	}
	if strings.Contains(gateway.LocalAddress, ":") {
		t.Fatalf("the local address carries a port: %q", gateway.LocalAddress)
	}
}

func TestExternalIPIsRead(t *testing.T) {
	gateway := dial(t, router(t, func(action string) (string, int) {
		if !strings.Contains(action, "GetExternalIPAddress") {
			return "", 500
		}
		return envelope(`<u:GetExternalIPAddressResponse><NewExternalIPAddress>203.0.113.7` +
			`</NewExternalIPAddress></u:GetExternalIPAddressResponse>`), 200
	}))

	address, err := gateway.ExternalIP(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if address != "203.0.113.7" {
		t.Fatalf("read %q", address)
	}
}

// A router that refuses says why in a numbered fault, and the number is the
// message: 718 means somebody else holds that port, 725 means it only does
// permanent leases. Those call for opposite responses, so the code has to
// survive being wrapped.
func TestARefusalKeepsItsCode(t *testing.T) {
	gateway := dial(t, router(t, func(string) (string, int) {
		return envelope(`<s:Fault><detail><UPnPError ` +
			`xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>718</errorCode>` +
			`<errorDescription>ConflictInMappingEntry</errorDescription>` +
			`</UPnPError></detail></s:Fault>`), 500
	}))

	err := gateway.AddPortMapping(context.Background(), "UDP", 41000, 41000, "test", time.Hour)
	if err == nil {
		t.Fatal("a refusal was read as success")
	}

	var refusal *UPnPError
	if !errors.As(err, &refusal) {
		t.Fatalf("the code was lost on the way out: %v", err)
	}
	if refusal.Code != 718 {
		t.Fatalf("code %d", refusal.Code)
	}
	if !IsConflict(err) {
		t.Fatal("718 was not recognised as a conflict")
	}
	if !strings.Contains(err.Error(), "718") {
		t.Fatalf("the message hides the code: %q", err.Error())
	}
}

// Routers accept mappings and then make a different one, so reading it back is
// the only way to know what exists.
func TestAMappingIsReadBack(t *testing.T) {
	gateway := dial(t, router(t, func(action string) (string, int) {
		if strings.Contains(action, "GetSpecificPortMappingEntry") {
			return envelope(`<u:GetSpecificPortMappingEntryResponse>` +
				`<NewInternalPort>41000</NewInternalPort>` +
				`<NewInternalClient>192.168.1.9</NewInternalClient>` +
				`<NewEnabled>1</NewEnabled>` +
				`<NewPortMappingDescription>test</NewPortMappingDescription>` +
				`<NewLeaseDuration>3600</NewLeaseDuration>` +
				`</u:GetSpecificPortMappingEntryResponse>`), 200
		}
		return envelope(`<u:AddPortMappingResponse/>`), 200
	}))

	ctx := context.Background()
	if err := gateway.AddPortMapping(ctx, "UDP", 41000, 41000, "test", time.Hour); err != nil {
		t.Fatal(err)
	}

	mapping, err := gateway.GetPortMapping(ctx, "UDP", 41000)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.InternalPort != 41000 || mapping.InternalHost != "192.168.1.9" {
		t.Fatalf("read back %+v", mapping)
	}
}

// A mapping left behind outlives the process, and on a router that ignores
// lease times it outlives the reboot too.
func TestAMappingCanBeRemoved(t *testing.T) {
	var asked bool
	gateway := dial(t, router(t, func(action string) (string, int) {
		if strings.Contains(action, "DeletePortMapping") {
			asked = true
		}
		return envelope(`<u:DeletePortMappingResponse/>`), 200
	}))

	if err := gateway.DeletePortMapping(context.Background(), "UDP", 41000); err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("nothing was asked to be deleted")
	}
}

// A description naming no connection service is a device that cannot map
// anything. Saying so beats handing back a gateway that fails on every call.
func TestADeviceWithNoServiceIsRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><root><device>`+
			`<friendlyName>Printer</friendlyName></device></root>`)
	}))
	defer server.Close()

	if _, err := gatewayFromLocation(context.Background(), server.URL+"/desc.xml"); err == nil {
		t.Fatal("a device with no connection service was accepted")
	}
}

// All of this comes off the network from a device nobody here wrote.
func TestMalformedDescriptionsAreRefused(t *testing.T) {
	cases := map[string]string{
		"not xml at all":                "<<<>>>",
		"an empty body":                 "",
		"xml that is not a description": `<?xml version="1.0"?><hello/>`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer server.Close()

			if _, err := gatewayFromLocation(context.Background(), server.URL+"/desc.xml"); err == nil {
				t.Fatalf("%q was accepted as a description", body)
			}
		})
	}
}

// An upstream router only exists behind NAT. Guessing one from a public address
// points a search at a stranger's machine on the internet, which is what
// happened when this was called from outside the one place that checked first.
func TestUpstreamIsOnlyGuessedBehindNAT(t *testing.T) {
	behind := map[string]string{
		"192.168.1.9":  "192.168.1.1",
		"10.0.0.44":    "10.0.0.1",
		"172.16.5.200": "172.16.5.1",
	}
	for external, want := range behind {
		if got := GuessUpstream(external); got != want {
			t.Fatalf("%s → %q, want %q", external, got, want)
		}
	}

	for _, public := range []string{"203.0.113.7", "8.8.8.8", "198.51.100.1", "nonsense", ""} {
		if got := GuessUpstream(public); got != "" {
			t.Fatalf("%q is not behind NAT but guessed %q", public, got)
		}
	}

	// A gateway already sitting on the .1 address has nothing above it.
	if got := GuessUpstream("192.168.1.1"); got != "" {
		t.Fatalf("the subnet router guessed itself: %q", got)
	}
}
