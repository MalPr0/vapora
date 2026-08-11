package main

import "encoding/binary"

// The wire format, which is the whole of what this game says to the other side.
//
// Two tags, in this program's own numbering, carried inside the payload the
// transport moves. The transport's frame kinds are invisible from here and
// cannot collide with these.
const (
	tagPaddle byte = 1 // joiner → host: where my paddle is
	tagState  byte = 2 // host → joiner: everything else
)

// Field is the play area in game units. Positions travel as fractions of it, so
// the two terminals can be different sizes and still agree about the game.
const (
	fieldWidth  = 1000
	fieldHeight = 1000
)

// State is the whole game, and it is sent complete on every tick.
//
// This is the opposite of how the chat uses the same channel. A conversation
// sends events, and a lost line is lost. A game sends state, and a lost packet
// is corrected by the next one thirty milliseconds later — so nothing here
// needs acknowledgements, retransmission or ordering, and none of it is missed.
type State struct {
	BallX, BallY  uint16
	LeftY, RightY uint16
	LeftScore     uint8
	RightScore    uint8
	Serving       bool
}

const stateBytes = 11

func encodeState(state State) []byte {
	payload := make([]byte, 1+stateBytes)
	payload[0] = tagState
	binary.BigEndian.PutUint16(payload[1:], state.BallX)
	binary.BigEndian.PutUint16(payload[3:], state.BallY)
	binary.BigEndian.PutUint16(payload[5:], state.LeftY)
	binary.BigEndian.PutUint16(payload[7:], state.RightY)
	payload[9] = state.LeftScore
	payload[10] = state.RightScore
	if state.Serving {
		payload[11] = 1
	}
	return payload
}

// decodeState reads a state, and refuses anything that is not one.
//
// Everything here arrived from the network. A length that does not match is not
// this program on the other end, and positions are clamped rather than trusted:
// a peer claiming the ball is at 60000 would otherwise index off the screen.
func decodeState(payload []byte) (State, bool) {
	if len(payload) != 1+stateBytes || payload[0] != tagState {
		return State{}, false
	}

	state := State{
		BallX:      clamp16(binary.BigEndian.Uint16(payload[1:]), fieldWidth),
		BallY:      clamp16(binary.BigEndian.Uint16(payload[3:]), fieldHeight),
		LeftY:      clamp16(binary.BigEndian.Uint16(payload[5:]), fieldHeight),
		RightY:     clamp16(binary.BigEndian.Uint16(payload[7:]), fieldHeight),
		LeftScore:  payload[9],
		RightScore: payload[10],
		Serving:    payload[11] == 1,
	}
	return state, true
}

func encodePaddle(y uint16) []byte {
	payload := make([]byte, 3)
	payload[0] = tagPaddle
	binary.BigEndian.PutUint16(payload[1:], y)
	return payload
}

func decodePaddle(payload []byte) (uint16, bool) {
	if len(payload) != 3 || payload[0] != tagPaddle {
		return 0, false
	}
	return clamp16(binary.BigEndian.Uint16(payload[1:]), fieldHeight), true
}

func clamp16(value, most uint16) uint16 {
	if value > most {
		return most
	}
	return value
}
