package upnp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrMappingNotFound is returned when the gateway holds no mapping for the query.
var ErrMappingNotFound = errors.New("upnp: port mapping not found")

// PortMapping describes a mapping as the gateway reports it back.
type PortMapping struct {
	Protocol      string
	ExternalPort  int
	InternalPort  int
	InternalHost  string
	Description   string
	LeaseDuration time.Duration
	Enabled       bool
}

func (g *Gateway) ExternalIP(ctx context.Context) (string, error) {
	values, err := g.call(ctx, "GetExternalIPAddress", nil)
	if err != nil {
		return "", err
	}
	address := values["NewExternalIPAddress"]
	if address == "" {
		return "", errors.New("upnp: gateway returned an empty external address")
	}
	return address, nil
}

// AddPortMapping forwards externalPort on the gateway to internalPort on this
// host. Gateways that reject timed leases are retried with a permanent one.
func (g *Gateway) AddPortMapping(ctx context.Context, protocol string, externalPort, internalPort int, description string, lease time.Duration) error {
	return g.AddPortMappingFor(ctx, protocol, externalPort, internalPort, g.LocalAddress, description, lease)
}

// AddPortMappingFor targets an arbitrary internal client, which is what an
// upstream gateway needs when the mapping points at another router instead of
// at this host.
func (g *Gateway) AddPortMappingFor(ctx context.Context, protocol string, externalPort, internalPort int, client, description string, lease time.Duration) error {
	err := g.addPortMapping(ctx, protocol, externalPort, internalPort, client, description, lease)
	if errorCode(err) == errorCodeOnlyPermanentLeases && lease > 0 {
		return g.addPortMapping(ctx, protocol, externalPort, internalPort, client, description, 0)
	}
	return err
}

func (g *Gateway) addPortMapping(ctx context.Context, protocol string, externalPort, internalPort int, client, description string, lease time.Duration) error {
	_, err := g.call(ctx, "AddPortMapping", []soapArgument{
		{Name: "NewRemoteHost", Value: ""},
		{Name: "NewExternalPort", Value: strconv.Itoa(externalPort)},
		{Name: "NewProtocol", Value: protocol},
		{Name: "NewInternalPort", Value: strconv.Itoa(internalPort)},
		{Name: "NewInternalClient", Value: client},
		{Name: "NewEnabled", Value: "1"},
		{Name: "NewPortMappingDescription", Value: description},
		{Name: "NewLeaseDuration", Value: strconv.Itoa(int(lease.Seconds()))},
	})
	return err
}

func (g *Gateway) DeletePortMapping(ctx context.Context, protocol string, externalPort int) error {
	_, err := g.call(ctx, "DeletePortMapping", []soapArgument{
		{Name: "NewRemoteHost", Value: ""},
		{Name: "NewExternalPort", Value: strconv.Itoa(externalPort)},
		{Name: "NewProtocol", Value: protocol},
	})
	if code := errorCode(err); code == errorCodeNoSuchEntryInArray || code == errorCodeSpecifiedArrayIndexValid {
		return fmt.Errorf("%w: %s/%d", ErrMappingNotFound, protocol, externalPort)
	}
	return err
}

func (g *Gateway) GetPortMapping(ctx context.Context, protocol string, externalPort int) (*PortMapping, error) {
	values, err := g.call(ctx, "GetSpecificPortMappingEntry", []soapArgument{
		{Name: "NewRemoteHost", Value: ""},
		{Name: "NewExternalPort", Value: strconv.Itoa(externalPort)},
		{Name: "NewProtocol", Value: protocol},
	})
	if code := errorCode(err); code == errorCodeNoSuchEntryInArray || code == errorCodeSpecifiedArrayIndexValid {
		return nil, fmt.Errorf("%w: %s/%d", ErrMappingNotFound, protocol, externalPort)
	}
	if err != nil {
		return nil, err
	}

	internalPort, _ := strconv.Atoi(values["NewInternalPort"])
	leaseSeconds, _ := strconv.Atoi(values["NewLeaseDuration"])
	return &PortMapping{
		Protocol:      protocol,
		ExternalPort:  externalPort,
		InternalPort:  internalPort,
		InternalHost:  values["NewInternalClient"],
		Description:   values["NewPortMappingDescription"],
		LeaseDuration: time.Duration(leaseSeconds) * time.Second,
		Enabled:       values["NewEnabled"] == "1",
	}, nil
}

// IsConflict reports whether the gateway refused the mapping because another
// device already owns that external port.
func IsConflict(err error) bool {
	return errorCode(err) == errorCodeConflictInMappingEntry
}
