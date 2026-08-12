package punch

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// reset lets each test start from a process that has never said it. The real
// one is a package-level Once on purpose: a room opens a path per pair, and the
// line is meant to be said once, not once per peer.
func resetFirstContact(t *testing.T) {
	t.Helper()
	firstContact = sync.Once{}
	t.Cleanup(func() { firstContact = sync.Once{} })
}

// A caller that draws its own screen passes no writer, and must never have a
// line appear in the middle of what it painted.
func TestNothingIsSaidWithoutAWriter(t *testing.T) {
	resetFirstContact(t)

	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	// nil output: this is what the full screen front end passes.
	leftSession := wired(t, left, plainCodec{}, nil)
	rightSession := wired(t, right, plainCodec{}, nil)
	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Opening a path must not panic on the absent writer, and must stay silent.
	openBoth(t, ctx, leftSession, rightSession)
}

// With a writer — which is what plain mode hands over — it is said once the
// path is open, and not before.
func TestItIsSaidWhenThePathOpens(t *testing.T) {
	resetFirstContact(t)

	left, right := listen(t), listen(t)
	defer left.Close()
	defer right.Close()

	// Only one side gets a writer. Both open the same path, and the line is
	// said once per process — so giving the other side a writer too would be a
	// race for the Once, and the line would land in the wrong buffer half the
	// time. That is a fault in the test, not in the code, and it took two runs
	// disagreeing to see it.
	output := &syncBuffer{}
	leftSession := wired(t, left, plainCodec{}, output)
	rightSession := wired(t, right, plainCodec{}, nil)

	if strings.Contains(output.String(), "sus propias decisiones") {
		t.Fatal("it was said before there was a path")
	}

	leftSession.SetPeer(localAddr(t, right))
	rightSession.SetPeer(localAddr(t, left))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	openBoth(t, ctx, leftSession, rightSession)

	waitFor(t, output, "sus propias decisiones")
}

// A room opens a path per pair. Seven of these in a row is not an easter egg,
// it is a bug with a sense of humour.
func TestItIsSaidOnlyOnce(t *testing.T) {
	resetFirstContact(t)

	output := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < 3; i++ {
		left, right := listen(t), listen(t)
		defer left.Close()
		defer right.Close()

		leftSession := wired(t, left, plainCodec{}, output)
		rightSession := wired(t, right, plainCodec{}, nil)
		leftSession.SetPeer(localAddr(t, right))
		rightSession.SetPeer(localAddr(t, left))

		openBoth(t, ctx, leftSession, rightSession)
	}

	waitFor(t, output, "sus propias decisiones")
	time.Sleep(200 * time.Millisecond)

	if said := strings.Count(output.String(), "sus propias decisiones"); said != 1 {
		t.Fatalf("said %d times across three paths, want once", said)
	}
}
