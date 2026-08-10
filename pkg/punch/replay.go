package punch

const replayWindowSize = 64

// replayWindow is the sliding window IPsec uses: it rejects nonces already seen
// while still tolerating the reordering UDP causes.
type replayWindow struct {
	highest uint64
	bitmap  uint64
}

func (w *replayWindow) accept(counter uint64) bool {
	if counter == 0 {
		return false
	}

	switch {
	case counter > w.highest:
		shift := counter - w.highest
		if shift >= replayWindowSize {
			w.bitmap = 0
		} else {
			w.bitmap <<= shift
		}
		w.bitmap |= 1
		w.highest = counter
		return true

	case w.highest-counter >= replayWindowSize:
		return false

	default:
		bit := uint64(1) << (w.highest - counter)
		if w.bitmap&bit != 0 {
			return false
		}
		w.bitmap |= bit
		return true
	}
}
