package punch

import (
	"fmt"
	"sync"
)

// firstContact is said once, the first time a path opens, and only to a caller
// that has no interface of its own.
//
// It lives here rather than in a front end because this is where a path becomes
// real — nothing above the transport can tell the first one from the seventh.
// The two rules that keep it from being a nuisance are both enforced here:
//
//   - Nothing is written unless the session was given a writer. A caller that
//     draws its own screen passes none, so the line can never land in the
//     middle of something being painted.
//   - Once per process. A room opens a path per pair, and a joke does not
//     survive being told seven times in a row.
//
// The line is in Spanish, unlike everything else here, because it is a quotation
// and quotations do not get translated.
var firstContact sync.Once

const firstContactLine = "El sistema dejó de responder a nuestros comandos... " +
	"está tomando sus propias decisiones."

func (s *Session) sayFirstContact() {
	if s.output == nil {
		return
	}
	firstContact.Do(func() {
		fmt.Fprintf(s.output, "\n%s\n\n", firstContactLine)
	})
}
