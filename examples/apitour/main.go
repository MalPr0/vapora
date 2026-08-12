// Command apitour is every code snippet in ARCHITECTURE.md, in one file that
// the compiler checks.
//
// It is not meant to be run — it dials nothing useful and ignores every error
// on purpose. It exists so that documentation cannot drift: if a signature
// changes and the document is not updated, `go build ./...` says so. Two of the
// snippets here were wrong when first written, and this is how that was caught.
package main

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/MalPr0/vapora/pkg/diag"
	"github.com/MalPr0/vapora/pkg/pcp"
	"github.com/MalPr0/vapora/pkg/punch"
	"github.com/MalPr0/vapora/pkg/stun"
	"github.com/MalPr0/vapora/pkg/upnp"
)

func main() {
	ctx := context.Background()

	// Step 1
	conn, _ := net.ListenUDP("udp4", &net.UDPAddr{})
	watcher := stun.NewWatcher(stun.DefaultServers, stun.DefaultKeepalive)
	_, _ = watcher.Wait(ctx, 10*time.Second)
	watcher.OnChange(func(was, now *net.UDPAddr) {})

	report, _ := stun.Probe(ctx, stun.DefaultServers, 4*time.Second)
	_ = report.Mapping
	_ = report.Filtering

	// Step 2
	mine := diag.Profile{Mapping: report.Mapping, Filtering: report.Filtering}
	_ = mine.Code()
	theirs, _ := diag.ParseProfile("CONE-OPEN-64")
	advice := diag.Pair(mine, theirs)
	_, _, _ = advice.Works, advice.Invites, advice.Publisher

	mesh := diag.MeshOf([]diag.Member{{Name: "ana", Profile: mine}})
	_, _, _, _ = mesh.Closes, mesh.Broken, mesh.Isolated, mesh.Hosts

	// Step 4
	mux := punch.NewMux(conn)
	go mux.Run(ctx)
	mux.Fallback(punch.SinkFunc(watcher.Handle))

	secret, _ := punch.NewSecret()
	codec, _ := punch.NewSecretCodec(secret, punch.RoleInviter)
	session := punch.NewSession(mux, codec, nil)
	mux.Fallback(session)
	session.Observe(punch.ObserverFunc(func(payload []byte) {}))
	go session.Run(ctx)
	_ = session.Open(ctx, 3*time.Minute)
	session.Send([]byte("anything at all"))

	const tagPart byte = 2
	session.Send(append([]byte{tagPart}, []byte("chunk")...))

	// Step 5
	identity, _ := punch.NewIdentity()
	room, _ := punch.NewRoom(punch.RoomOptions{
		Identity: identity,
		Secret:   secret,
		Mux:      mux,
		Local:    punch.LocalAddr(41000),
	})
	room.Observe(punch.RoomObserverFunc(func(from punch.Member, payload []byte) { _ = from.Key }))
	invite := room.Invite(&net.UDPAddr{})
	_ = room.Join(ctx, invite, 3*time.Minute)
	room.Broadcast([]byte("to everyone"))
	_ = room.SendTo(identity.Public(), []byte("to one person"))

	// Step 6
	_, _ = punch.RendezvousKey(secret)
	meeting, _ := punch.NewRendezvous(mux, secret, 41000)
	mux.Fallback(punch.SinkFunc(meeting.Deliver))
	go meeting.Publish(ctx, func(peers []*net.UDPAddr) {
		for _, peer := range peers {
			room.Reach(ctx, peer)
		}
	})

	go watcher.Run(ctx, conn)

	// Step 3
	gateway, _ := upnp.Discover(ctx, 3*time.Second)
	_, _ = gateway.ExternalIP(ctx)
	_ = gateway.AddPortMapping(ctx, "UDP", 41000, 41000, "vapora", time.Hour)

	client, _ := pcp.Dial(netip.MustParseAddr("192.0.2.1"))
	_, _ = client.Detect(ctx)
	_, _ = client.Map(ctx, pcp.MapRequest{})
}
