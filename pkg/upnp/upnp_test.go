package upnp

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestBuildEnvelopeEscapesArguments(t *testing.T) {
	body := buildEnvelope("urn:service:1", "AddPortMapping", []soapArgument{
		{Name: "NewPortMappingDescription", Value: `a & b <c>`},
	})

	if !strings.Contains(string(body), `<u:AddPortMapping xmlns:u="urn:service:1">`) {
		t.Fatalf("missing action element: %s", body)
	}
	if strings.Contains(string(body), "a & b <c>") {
		t.Fatalf("argument was not escaped: %s", body)
	}
	if err := xml.Unmarshal(body, new(struct{ XMLName xml.Name })); err != nil {
		t.Fatalf("envelope is not valid xml: %v", err)
	}
}

func TestParseResponseReadsValues(t *testing.T) {
	payload := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
<u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1">
<NewExternalIPAddress>203.0.113.7</NewExternalIPAddress>
</u:GetExternalIPAddressResponse></s:Body></s:Envelope>`)

	values, fault := parseResponse(payload)
	if fault != nil {
		t.Fatalf("unexpected fault: %v", fault)
	}
	if values["NewExternalIPAddress"] != "203.0.113.7" {
		t.Fatalf("got %q", values["NewExternalIPAddress"])
	}
}

func TestParseResponseDetectsFault(t *testing.T) {
	payload := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault>
<faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring>
<detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
<errorCode>718</errorCode><errorDescription>ConflictInMappingEntry</errorDescription>
</UPnPError></detail></s:Fault></s:Body></s:Envelope>`)

	_, fault := parseResponse(payload)
	if fault == nil {
		t.Fatal("expected a fault")
	}
	if fault.Code != errorCodeConflictInMappingEntry {
		t.Fatalf("got code %d", fault.Code)
	}
	fault.Action = "AddPortMapping"
	if !IsConflict(fault) {
		t.Fatal("conflict not recognised")
	}
}

func TestParseLocationRequiresHeader(t *testing.T) {
	withHeader := []byte("HTTP/1.1 200 OK\r\nLOCATION: http://192.0.2.1:5000/desc.xml\r\nST: upnp:rootdevice\r\n\r\n")
	location, err := parseLocation(withHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if location != "http://192.0.2.1:5000/desc.xml" {
		t.Fatalf("got %q", location)
	}

	if _, err := parseLocation([]byte("HTTP/1.1 200 OK\r\nST: upnp:rootdevice\r\n\r\n")); err == nil {
		t.Fatal("expected an error when LOCATION is missing")
	}
}

func TestFindWANServicePrefersNewestAndWalksChildren(t *testing.T) {
	root := device{
		FriendlyName: "Root",
		Devices: []device{{
			FriendlyName: "WANConnectionDevice",
			Services: []service{
				{ServiceType: "urn:schemas-upnp-org:service:WANPPPConnection:1", ControlURL: "/ppp"},
				{ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1", ControlURL: "/ip"},
			},
		}},
	}

	found, owner := findWANService(root)
	if found == nil || found.ControlURL != "/ip" {
		t.Fatalf("got %+v", found)
	}
	if owner != "WANConnectionDevice" {
		t.Fatalf("got owner %q", owner)
	}
}

func TestChainAddressClassification(t *testing.T) {
	cases := []struct {
		address string
		private bool
		cgnat   bool
	}{
		{"10.0.0.84", true, false},
		{"10.0.0.1", true, false},
		{"100.70.1.1", false, true},
		{"203.0.113.7", false, false},
	}

	for _, testCase := range cases {
		if got := isPrivate(testCase.address); got != testCase.private {
			t.Errorf("isPrivate(%s) = %v", testCase.address, got)
		}
		if got := isCarrierGrade(testCase.address); got != testCase.cgnat {
			t.Errorf("isCarrierGrade(%s) = %v", testCase.address, got)
		}
	}
}

func TestGuessUpstream(t *testing.T) {
	if got := guessUpstream("10.0.0.84"); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := guessUpstream("10.0.0.1"); got != "" {
		t.Fatalf("expected no guess for a gateway that is already .1, got %q", got)
	}
}

func TestPublicAddressReportsReachability(t *testing.T) {
	chain := &Chain{Protocol: "TCP", Port: 40404, Hops: []Hop{{ExternalIP: "10.0.0.84"}}}
	if address, public := chain.PublicAddress(); public || address != "10.0.0.84:40404" {
		t.Fatalf("got %q public=%v", address, public)
	}

	chain.Hops = append(chain.Hops, Hop{ExternalIP: "203.0.113.7"})
	if address, public := chain.PublicAddress(); !public || address != "203.0.113.7:40404" {
		t.Fatalf("got %q public=%v", address, public)
	}
}
