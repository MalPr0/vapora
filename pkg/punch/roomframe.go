package punch

// Room frames. Everything here travels over a pair channel except hello and
// full, which are the only two that can: before a pair channel exists there is
// nothing but the room secret, and after it exists there is no reason to fall
// back to something everybody holds.
const (
	// kindHello introduces a newcomer. It is sealed with the room key, which
	// is the one thing an arriving member shares with a member already there.
	kindHello byte = 0x10
	// kindWelcome answers it over the pair channel and carries the roster.
	// Sealing it per pair is what stops a stranger with the room secret from
	// impersonating the greeter: it can open a hello, but not answer one.
	kindWelcome byte = 0x11
	// kindRoster is the gossip that makes the room converge when anyone can
	// invite.
	kindRoster byte = 0x12
	// kindIntro names one newcomer to one member, which is all the introducing
	// member does: it never relays anything.
	kindIntro byte = 0x13
	// kindFull turns a newcomer away.
	kindFull byte = 0x14
)

// layer is which key a frame is allowed to arrive under.
type layer int

const (
	layerRoom layer = iota
	layerPair
)

// allowedUnder is the whole table, in one place, because the rule it encodes is
// the room's security: a frame that carries anything worth having must not be
// openable by everyone who was ever handed the invite.
func allowedUnder(kind byte, under layer) bool {
	switch kind {
	case kindHello, kindFull:
		return under == layerRoom
	default:
		return under == layerPair
	}
}
