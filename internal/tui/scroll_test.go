package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func chatWith(count int, scroll int) State {
	state := State{Phase: PhaseChat, Me: "OTTER", Peer: "BADGER", Scroll: scroll}
	for i := 0; i < count; i++ {
		state.Messages = append(state.Messages, Message{Speaker: "BADGER", Body: fmt.Sprintf("line-%02d", i)})
	}
	return state
}

// A chat reads upward from the input, so the newest line sits against the
// footer and an empty conversation leaves the gap above it.
func TestMessagesAreAnchoredToTheBottom(t *testing.T) {
	screen := drawState(State{
		Phase:    PhaseChat,
		Peer:     "BADGER",
		Messages: []Message{{Speaker: "BADGER", Body: "solo"}},
	})

	lines := strings.Split(plain(screen), "\n")
	_, height := screen.Size()

	lastMessageRow := height - footerHeight - 1
	if !strings.Contains(lines[lastMessageRow], "solo") {
		t.Fatalf("the only message is not against the footer:\n%s", plain(screen))
	}
}

func TestScrollRevealsOlderLines(t *testing.T) {
	bottom := plain(drawState(chatWith(60, 0)))
	if !strings.Contains(bottom, "line-59") || strings.Contains(bottom, "line-00") {
		t.Fatalf("the unscrolled view is not showing the newest lines:\n%s", bottom)
	}

	top := MaxScroll(chatWith(60, 0), 80, 24)
	scrolled := plain(drawState(chatWith(60, top)))
	if !strings.Contains(scrolled, "line-00") {
		t.Fatalf("scrolling up did not reach the oldest line:\n%s", scrolled)
	}
	if strings.Contains(scrolled, "line-59") {
		t.Fatal("the newest line is still visible after scrolling to the top")
	}
}

// An offset larger than the history, which a resize can produce, must clamp
// rather than slice out of range.
func TestScrollClampsToTheHistory(t *testing.T) {
	frame := plain(drawState(chatWith(5, 9999)))
	if !strings.Contains(frame, "line-00") {
		t.Fatalf("an over-scrolled view lost its content:\n%s", frame)
	}

	if got := MaxScroll(chatWith(3, 0), 80, 24); got != 0 {
		t.Fatalf("a history that fits reported %d lines of scroll", got)
	}
	if got := MaxScroll(chatWith(100, 0), 80, 24); got <= 0 {
		t.Fatal("a long history reported no scroll room")
	}
}

func TestScrollMarkerCountsWhatIsHidden(t *testing.T) {
	if strings.Contains(plain(drawState(chatWith(3, 0))), "above") {
		t.Fatal("a short conversation must not claim hidden lines")
	}

	scrolled := plain(drawState(chatWith(60, 20)))
	if !strings.Contains(scrolled, "below") || !strings.Contains(scrolled, "above") {
		t.Fatalf("a scrolled view does not say what is hidden:\n%s", scrolled)
	}
}

// The wordmark is the first thing anyone sees, and a font built from whole
// blocks came out unreadable. Half block pixels are what keeps it square.
func TestBannerIsDrawnAsHalfBlockPixels(t *testing.T) {
	screen := NewScreen(80, 24)
	Draw(screen, State{Phase: PhaseChat})

	halves, wholes := 0, 0
	for y := 0; y < headerHeight; y++ {
		for x := 0; x < 80; x++ {
			switch screen.At(x, y).Rune {
			case upperHalf:
				halves++
			case '█':
				wholes++
			}
		}
	}

	if wholes > 0 {
		t.Fatalf("the banner used %d whole blocks, which render twice as tall as they are wide", wholes)
	}
	if halves < 40 {
		t.Fatalf("the banner only painted %d pixel cells", halves)
	}
}

func TestBannerFitsTheMinimumWidth(t *testing.T) {
	if BannerWidth()+4 > minWidth {
		t.Fatalf("the wordmark needs %d columns but the minimum width is %d", BannerWidth()+4, minWidth)
	}
}

func TestPageKeysDecode(t *testing.T) {
	key, consumed := DecodeKey([]byte("\x1b[5~"))
	if key.Kind != KeyPageUp || consumed != 4 {
		t.Fatalf("page up decoded as kind %d consuming %d", key.Kind, consumed)
	}

	key, consumed = DecodeKey([]byte("\x1b[6~"))
	if key.Kind != KeyPageDown || consumed != 4 {
		t.Fatalf("page down decoded as kind %d consuming %d", key.Kind, consumed)
	}

	if _, consumed := DecodeKey([]byte("\x1b[5")); consumed != 0 {
		t.Fatalf("a partial page key consumed %d", consumed)
	}
}

// A healthy path says almost nothing; the badge exists for when it stops.
func TestLinkBadgeOnlySpeaksWhenSomethingIsWrong(t *testing.T) {
	alive := plain(drawState(State{Phase: PhaseChat, Peer: "BADGER", Link: LinkAlive}))
	if strings.Contains(alive, "LINK LOST") || strings.Contains(alive, "no reply") {
		t.Fatalf("a healthy link raised an alarm:\n%s", alive)
	}

	stale := plain(drawState(State{
		Phase: PhaseChat, Peer: "BADGER", Link: LinkStale, Silence: 14 * time.Second,
	}))
	if !strings.Contains(stale, "no reply 14s") {
		t.Fatalf("a stale link did not report its silence:\n%s", stale)
	}
	if !strings.Contains(stale, "◐") {
		t.Fatal("a stale link kept the healthy marker")
	}

	lost := plain(drawState(State{Phase: PhaseChat, Peer: "BADGER", Link: LinkLost, Frame: 0}))
	if !strings.Contains(lost, "LINK LOST") || !strings.Contains(lost, "○") {
		t.Fatalf("a lost link did not say so:\n%s", lost)
	}
}

// The lost badge blinks, which is the one signal a console can give that the
// eye catches without moving.
func TestLostLinkBlinks(t *testing.T) {
	on := plain(drawState(State{Phase: PhaseChat, Peer: "BADGER", Link: LinkLost, Frame: 0}))
	off := plain(drawState(State{Phase: PhaseChat, Peer: "BADGER", Link: LinkLost, Frame: 5}))
	if on == off {
		t.Fatal("the lost badge did not blink")
	}
}

func TestRoundTripIsShownWhenKnown(t *testing.T) {
	frame := plain(drawState(State{
		Phase: PhaseChat, Peer: "BADGER", Link: LinkAlive, RTT: 42 * time.Millisecond,
	}))
	if !strings.Contains(frame, "42ms") {
		t.Fatalf("the round trip is not shown:\n%s", frame)
	}
}
