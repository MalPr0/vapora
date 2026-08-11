package punch

import (
	"testing"
	"time"
)

// recordingObserver captures what a session delivers, which is bytes: this
// package has no idea whether they are a line, a file, or a move in a game.
type recordingObserver struct {
	payloads chan []byte
}

func newRecorder(size int) *recordingObserver {
	return &recordingObserver{payloads: make(chan []byte, size)}
}

func (o *recordingObserver) Data(payload []byte) {
	o.payloads <- append([]byte(nil), payload...)
}

// waitFor returns the next payload as a string, which is all these tests need
// since they send text through on purpose.
func (o *recordingObserver) waitFor(t *testing.T, within time.Duration) (string, bool) {
	t.Helper()
	select {
	case payload := <-o.payloads:
		return string(payload), true
	case <-time.After(within):
		return "", false
	}
}
