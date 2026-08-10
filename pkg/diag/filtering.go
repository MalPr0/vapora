package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/MalPr0/vapora/pkg/stun"
	"github.com/MalPr0/vapora/pkg/upnp"
)

// Verdict is which router the filtering was attributed to.
type Verdict int

const (
	VerdictUnknown Verdict = iota
	// VerdictAlreadyOpen means the chain never filtered, so a one way invite
	// already works.
	VerdictAlreadyOpen
	// VerdictInnerRouterFilters means the mapping opened the chain: the
	// router that speaks UPnP was the one dropping unannounced packets.
	VerdictInnerRouterFilters
	// VerdictUpstreamFilters means the mapping installed and changed nothing,
	// so the restriction lives further out, beyond our reach.
	VerdictUpstreamFilters
	// VerdictInconclusive means a confounder invalidated the comparison.
	VerdictInconclusive
)

func (v Verdict) String() string {
	switch v {
	case VerdictAlreadyOpen:
		return "the chain was already open"
	case VerdictInnerRouterFilters:
		return "the inner router was the one filtering"
	case VerdictUpstreamFilters:
		return "the upstream router is the one filtering"
	case VerdictInconclusive:
		return "inconclusive"
	default:
		return "unknown"
	}
}

type SocketFiltering struct {
	Filtering stun.Filtering
	Server    string
	Err       error
}

func (s SocketFiltering) String() string {
	if s.Err != nil {
		return fmt.Sprintf("unmeasured (%v)", s.Err)
	}
	return s.Filtering.String()
}

type FilterResult struct {
	SubjectPort, ControlPort         int
	ExternalPortAsked                int
	BaselineSubject, BaselineControl SocketFiltering
	MappedSubject, MappedControl     SocketFiltering
	Installed                        *upnp.PortMapping
	EndpointBefore, EndpointAfter    *net.UDPAddr
	Verdict                          Verdict
	Confounders                      []string
}

// FilterExperiment installs a port mapping for one socket and measures whether
// that changed what the chain lets in, with a second socket carried through the
// same windows as the control.
type FilterExperiment struct {
	Mapper PortMapper
	Probe  FilterProbe
	Lease  time.Duration
	Label  string
}

// Run measures subject and control before and after the mapping. The control is
// what tells a mapping that opened the filter apart from a network that changed
// under our feet.
func (e FilterExperiment) Run(ctx context.Context, subject, control *net.UDPConn, externalPort int) (*FilterResult, error) {
	subjectPort, err := localPort(subject)
	if err != nil {
		return nil, err
	}
	controlPort, err := localPort(control)
	if err != nil {
		return nil, err
	}
	if externalPort == 0 {
		externalPort = subjectPort
	}

	result := &FilterResult{
		SubjectPort:       subjectPort,
		ControlPort:       controlPort,
		ExternalPortAsked: externalPort,
	}

	result.BaselineSubject = e.measure(ctx, subject)
	result.BaselineControl = e.measure(ctx, control)
	result.EndpointBefore, _ = e.Probe.Endpoint(ctx, subject)

	if result.BaselineSubject.Err == nil && result.BaselineSubject.Filtering.AcceptsUnknownPeers() {
		result.Verdict = VerdictAlreadyOpen
		return result, nil
	}

	label := e.Label
	if label == "" {
		label = "vapora diag"
	}
	if err := e.Mapper.AddPortMapping(ctx, "UDP", externalPort, subjectPort, label, e.lease()); err != nil {
		return result, fmt.Errorf("diag: cannot install the mapping: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = e.Mapper.DeletePortMapping(cleanupCtx, "UDP", externalPort)
	}()

	result.Installed, err = e.Mapper.GetPortMapping(ctx, "UDP", externalPort)
	if err != nil {
		// A gateway that answers the add and installs nothing is common
		// enough that the verdict cannot rest on the add alone.
		reason := "the gateway did not confirm the mapping"
		if errors.Is(err, upnp.ErrMappingNotFound) {
			reason = fmt.Sprintf("no mapping on external port %d, the gateway may have assigned another one", externalPort)
		}
		result.Confounders = append(result.Confounders, reason)
	}

	result.MappedSubject = e.measure(ctx, subject)
	result.MappedControl = e.measure(ctx, control)
	result.EndpointAfter, _ = e.Probe.Endpoint(ctx, subject)

	result.Verdict = e.conclude(result, subjectPort)
	return result, nil
}

func (e FilterExperiment) conclude(result *FilterResult, subjectPort int) Verdict {
	result.Confounders = append(result.Confounders, checkInstalled(result.Installed, subjectPort)...)
	result.Confounders = append(result.Confounders, checkEndpoint(result.EndpointBefore, result.EndpointAfter)...)

	if result.MappedSubject.Err != nil || result.MappedControl.Err != nil {
		result.Confounders = append(result.Confounders, "a probe failed after the mapping, the windows are not comparable")
		return VerdictInconclusive
	}

	subjectOpened := result.MappedSubject.Filtering.AcceptsUnknownPeers()
	controlOpened := result.MappedControl.Filtering.AcceptsUnknownPeers()

	switch {
	case subjectOpened && controlOpened:
		result.Confounders = append(result.Confounders, "the unmapped control opened too, so something changed globally")
		return VerdictInconclusive
	case len(result.Confounders) > 0:
		return VerdictInconclusive
	case subjectOpened:
		return VerdictInnerRouterFilters
	default:
		return VerdictUpstreamFilters
	}
}

func checkInstalled(installed *upnp.PortMapping, subjectPort int) []string {
	if installed == nil {
		return nil
	}

	var confounders []string
	if installed.InternalPort != subjectPort {
		confounders = append(confounders,
			fmt.Sprintf("the mapping points at internal port %d, not the subject port %d", installed.InternalPort, subjectPort))
	}
	if !installed.Enabled {
		confounders = append(confounders, "the gateway installed the mapping disabled")
	}
	return confounders
}

func checkEndpoint(before, after *net.UDPAddr) []string {
	if before == nil || after == nil {
		return []string{"the observed endpoint could not be read on both sides of the mapping"}
	}
	if before.String() != after.String() {
		return []string{fmt.Sprintf("the observed endpoint moved from %s to %s during the experiment", before, after)}
	}
	return nil
}

func (e FilterExperiment) measure(ctx context.Context, conn *net.UDPConn) SocketFiltering {
	filtering, server, err := e.Probe.Filtering(ctx, conn)
	return SocketFiltering{Filtering: filtering, Server: server, Err: err}
}

func (e FilterExperiment) lease() time.Duration {
	if e.Lease <= 0 {
		return time.Hour
	}
	return e.Lease
}

func localPort(conn *net.UDPConn) (int, error) {
	address, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("diag: unexpected local address type %T", conn.LocalAddr())
	}
	return address.Port, nil
}
