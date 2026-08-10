package diag

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/MalPr0/vapora/pkg/stun"
	"github.com/MalPr0/vapora/pkg/upnp"
)

type fakeMapper struct {
	installed   *upnp.PortMapping
	addErr      error
	getErr      error
	installPort int
	disabled    bool
	deleted     bool
}

func (m *fakeMapper) AddPortMapping(_ context.Context, protocol string, externalPort, internalPort int, _ string, lease time.Duration) error {
	if m.addErr != nil {
		return m.addErr
	}
	if m.installPort != 0 {
		internalPort = m.installPort
	}
	m.installed = &upnp.PortMapping{
		Protocol:      protocol,
		ExternalPort:  externalPort,
		InternalPort:  internalPort,
		InternalHost:  "192.168.1.20",
		LeaseDuration: lease,
		Enabled:       !m.disabled,
	}
	return nil
}

func (m *fakeMapper) GetPortMapping(context.Context, string, int) (*upnp.PortMapping, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.installed, nil
}

func (m *fakeMapper) DeletePortMapping(context.Context, string, int) error {
	m.deleted = true
	return nil
}

// fakeProbe answers from a script keyed by local port, so subject and control
// can be given different behaviour across the two windows.
type fakeProbe struct {
	script map[int][]stun.Filtering
	calls  map[int]int
	fail   map[int]bool
}

func newFakeProbe(subject, control []stun.Filtering, subjectPort, controlPort int) *fakeProbe {
	return &fakeProbe{
		script: map[int][]stun.Filtering{subjectPort: subject, controlPort: control},
		calls:  map[int]int{},
		fail:   map[int]bool{},
	}
}

func (p *fakeProbe) Filtering(_ context.Context, conn *net.UDPConn) (stun.Filtering, string, error) {
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if p.fail[port] {
		return stun.FilteringUnknown, "", errors.New("probe failed")
	}

	steps := p.script[port]
	index := p.calls[port]
	p.calls[port]++
	if index >= len(steps) {
		index = len(steps) - 1
	}
	return steps[index], "fake", nil
}

func (p *fakeProbe) Endpoint(_ context.Context, conn *net.UDPConn) (*net.UDPAddr, error) {
	return &net.UDPAddr{IP: net.IPv4(203, 0, 113, 7), Port: 41001}, nil
}

func sockets(t *testing.T) (*net.UDPConn, *net.UDPConn) {
	t.Helper()

	subject, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	control, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	t.Cleanup(func() { subject.Close(); control.Close() })
	return subject, control
}

func ports(t *testing.T, subject, control *net.UDPConn) (int, int) {
	t.Helper()
	return subject.LocalAddr().(*net.UDPAddr).Port, control.LocalAddr().(*net.UDPAddr).Port
}

const (
	restricted = stun.FilteringAddressAndPortDependent
	open       = stun.FilteringEndpointIndependent
)

func TestVerdicts(t *testing.T) {
	cases := []struct {
		name    string
		subject []stun.Filtering
		control []stun.Filtering
		want    Verdict
	}{
		{"already open", []stun.Filtering{open}, []stun.Filtering{open}, VerdictAlreadyOpen},
		{"inner router filters", []stun.Filtering{restricted, open}, []stun.Filtering{restricted, restricted}, VerdictInnerRouterFilters},
		{"upstream filters", []stun.Filtering{restricted, restricted}, []stun.Filtering{restricted, restricted}, VerdictUpstreamFilters},
		{"control opened too", []stun.Filtering{restricted, open}, []stun.Filtering{restricted, open}, VerdictInconclusive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			subject, control := sockets(t)
			subjectPort, controlPort := ports(t, subject, control)

			mapper := &fakeMapper{}
			experiment := FilterExperiment{
				Mapper: mapper,
				Probe:  newFakeProbe(testCase.subject, testCase.control, subjectPort, controlPort),
			}

			result, err := experiment.Run(context.Background(), subject, control, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Verdict != testCase.want {
				t.Fatalf("got %q, want %q (confounders %v)", result.Verdict, testCase.want, result.Confounders)
			}
		})
	}
}

// A verdict that would otherwise be conclusive must not survive a confounder.
func TestConfoundersForceInconclusive(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fakeMapper, *fakeProbe, int)
	}{
		{"mapping not confirmed", func(m *fakeMapper, _ *fakeProbe, _ int) {
			m.getErr = upnp.ErrMappingNotFound
		}},
		{"mapping points elsewhere", func(m *fakeMapper, _ *fakeProbe, _ int) {
			m.installPort = 9999
		}},
		{"mapping installed disabled", func(m *fakeMapper, _ *fakeProbe, _ int) {
			m.disabled = true
		}},
		{"probe failed after mapping", func(_ *fakeMapper, p *fakeProbe, subjectPort int) {
			p.fail[subjectPort] = true
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			subject, control := sockets(t)
			subjectPort, controlPort := ports(t, subject, control)

			mapper := &fakeMapper{}
			probe := newFakeProbe(
				[]stun.Filtering{restricted, open},
				[]stun.Filtering{restricted, restricted},
				subjectPort, controlPort)
			testCase.setup(mapper, probe, subjectPort)

			result, err := FilterExperiment{Mapper: mapper, Probe: probe}.Run(context.Background(), subject, control, 0)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Verdict != VerdictInconclusive {
				t.Fatalf("got %q, expected inconclusive", result.Verdict)
			}
			if len(result.Confounders) == 0 {
				t.Fatal("an inconclusive verdict must name its confounder")
			}
		})
	}
}

func TestFilterExperimentAlwaysRemovesTheMapping(t *testing.T) {
	subject, control := sockets(t)
	subjectPort, controlPort := ports(t, subject, control)

	mapper := &fakeMapper{}
	probe := newFakeProbe([]stun.Filtering{restricted, restricted}, []stun.Filtering{restricted, restricted}, subjectPort, controlPort)

	if _, err := (FilterExperiment{Mapper: mapper, Probe: probe}).Run(context.Background(), subject, control, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mapper.deleted {
		t.Fatal("the experiment left its mapping behind")
	}
}

// An already open chain needs no mapping at all.
func TestFilterExperimentSkipsTheMappingWhenAlreadyOpen(t *testing.T) {
	subject, control := sockets(t)
	subjectPort, controlPort := ports(t, subject, control)

	mapper := &fakeMapper{}
	probe := newFakeProbe([]stun.Filtering{open}, []stun.Filtering{open}, subjectPort, controlPort)

	result, err := (FilterExperiment{Mapper: mapper, Probe: probe}).Run(context.Background(), subject, control, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != VerdictAlreadyOpen || mapper.installed != nil {
		t.Fatalf("got %q with installed=%v", result.Verdict, mapper.installed)
	}
}

func TestFilterExperimentReportsAFailedAdd(t *testing.T) {
	subject, control := sockets(t)
	subjectPort, controlPort := ports(t, subject, control)

	mapper := &fakeMapper{addErr: errors.New("conflict")}
	probe := newFakeProbe([]stun.Filtering{restricted}, []stun.Filtering{restricted}, subjectPort, controlPort)

	if _, err := (FilterExperiment{Mapper: mapper, Probe: probe}).Run(context.Background(), subject, control, 0); err == nil {
		t.Fatal("a mapping that cannot be installed must surface as an error")
	}
}
