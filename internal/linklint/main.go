// Command linklint checks the markdown that faces the community: that every
// relative link resolves, and that a translated page does not send its reader
// back into another language.
//
// Both are the kind of thing that rots silently — a file gets renamed, or a new
// section is copied from the English page and its links come along.
//
//	go run ./internal/linklint
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var link = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// pages are the documents meant to be read by people rather than by the
// compiler. AGENTS.md is deliberately not translated: it is the working
// reference for the code, and the code is in English.
var pages = []string{
	"README.md", "README.es.md",
	"ARCHITECTURE.md", "ARCHITECTURE.es.md",
	"examples/pong/README.md", "examples/pong/README.es.md",
}

func main() {
	var problems []string

	for _, page := range pages {
		body, err := os.ReadFile(page)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", page, err))
			continue
		}

		base := filepath.Dir(page)
		translated := strings.HasSuffix(page, ".es.md")

		for _, found := range link.FindAllStringSubmatch(string(body), -1) {
			text, target := found[1], found[2]
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
				continue
			}

			resolved := filepath.Join(base, strings.SplitN(target, "#", 2)[0])
			if _, err := os.Stat(resolved); err != nil {
				problems = append(problems, fmt.Sprintf("%s: [%s](%s) leads nowhere", page, text, target))
				continue
			}

			// The language selector is the one link that crosses on purpose.
			if !translated || text == "English" {
				continue
			}
			if spanish := strings.TrimSuffix(resolved, ".md") + ".es.md"; strings.HasSuffix(resolved, ".md") &&
				!strings.HasSuffix(resolved, ".es.md") && exists(spanish) {
				problems = append(problems, fmt.Sprintf(
					"%s: [%s](%s) drops the reader into English, and %s exists", page, text, target, spanish))
			}
		}
	}

	if len(problems) > 0 {
		fmt.Println(strings.Join(problems, "\n"))
		os.Exit(1)
	}
	fmt.Printf("%d pages: every link resolves and stays in its language\n", len(pages))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
