package names

// Adjectives are the last resort, used only when a colour and an animal still
// name two people in the same room.
var adjectives = []string{
	"SWIFT", "QUIET", "BOLD", "SLY", "CALM", "KEEN", "WILD", "LUCKY",
	"BRAVE", "CLEVER", "GENTLE", "PROUD", "SLEEPY", "EAGER", "STEADY", "NIMBLE",
	"FIERCE", "HUMBLE", "MERRY", "SOLEMN", "RESTLESS", "PATIENT", "CURIOUS", "STUBBORN",
	"GRACEFUL", "RUGGED", "TIDY", "WEARY", "SUNNY", "STORMY", "FROSTY", "DUSTY",
}

// Colours pair with the animals to name a participant. Sixty four of each is
// four thousand names, which keeps two members of a room of eight sharing one
// under a percent, and every word is common enough to say out loud.
// Every word here has to be legible as ink on a dark console. A colour that
// names the background is a name nobody can read, so the dark end of the
// spectrum is deliberately absent: there is no NAVY and no COAL.
var colours = []string{
	"CRIMSON", "SCARLET", "CHERRY", "RUBY", "FLAME", "EMBER", "RUST", "COPPER",
	"BRONZE", "AMBER", "GOLD", "SAFFRON", "MARIGOLD", "HONEY", "BUTTER", "LEMON",
	"STRAW", "CREAM", "IVORY", "LINEN", "PEARL", "SNOW", "ASH", "SILVER",
	"STEEL", "SLATE", "KHAKI", "SAND", "TAN", "CLAY", "OCHRE", "SIENNA",
	"CORAL", "SALMON", "PEACH", "APRICOT", "ROSE", "BLUSH", "MAGENTA", "ORCHID",
	"LILAC", "LAVENDER", "PERIWINKLE", "VIOLET", "PLUM", "INDIGO", "COBALT", "AZURE",
	"SKY", "FROST", "CYAN", "AQUA", "TURQUOISE", "SEAFOAM", "TEAL", "MINT",
	"JADE", "EMERALD", "SAGE", "MOSS", "FERN", "OLIVE", "LIME", "CHARTREUSE",
}

// animals is the pool both sides pick from. Keeping it a power of two makes the
// pick uniform without the modulo bias a ragged list would introduce.
var animals = []string{
	"OTTER", "BADGER", "FALCON", "MARTEN", "HERON", "LYNX", "RAVEN", "BISON",
	"WOMBAT", "PANGOLIN", "CARACAL", "OKAPI", "TAPIR", "IBEX", "MARMOT", "OSPREY",
	"NARWHAL", "AXOLOTL", "QUOKKA", "SERVAL", "KESTREL", "MANATEE", "JERBOA", "FOSSA",
	"COYOTE", "PUFFIN", "GECKO", "MAGPIE", "WALRUS", "SHRIKE", "CIVET", "DINGO",
	"ORYX", "SALAMANDER", "TOUCAN", "URCHIN", "VIPER", "WEASEL", "YAK", "ZEBU",
	"ALBATROSS", "BEAVER", "CHAMOIS", "DUGONG", "EIDER", "FERRET", "GIBBON", "HOOPOE",
	"IGUANA", "JACKAL", "KRILL", "LEMUR", "MOOSE", "NUTHATCH", "OCELOT", "PLOVER",
	"QUAIL", "RACCOON", "STOAT", "TERN", "UAKARI", "VICUNA", "WAPITI", "XERUS",
}

// Nicknames are the two names a session shows. Both peers derive the same pair
// from the shared secret, so nothing has to be negotiated and neither side can
// choose how it is labelled on the other's screen.
type Nicknames struct {
	Inviter string
	Joiner  string
}
