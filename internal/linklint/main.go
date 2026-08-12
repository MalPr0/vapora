// Command linklint checks the markdown that faces the community.
//
// Two properties, both of which rot silently: every relative link resolves, and
// a translated page never drops its reader back into another language. The
// second one matters because sections get copied across from the English page
// and bring their links with them.
//
// Translations live under docs/<lang>/ so the repository root stays readable.
//
//	go run ./internal/linklint
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var link = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// languages in the order the selector lists them. English is the original and
// lives where GitHub expects to find it.
var languages = []string{"en", "es", "zh", "ja", "pt", "ar", "fr", "it", "de", "ru"}

// available is which languages each page exists in. The README is the page that
// brings people in, so it is translated everywhere; the two technical documents
// are for somebody already reading the code, which is in English.
//
// A selector must offer exactly what exists — offering a language that is not
// there is a broken promise, and leaving one out hides a translation somebody
// wrote.
var available = map[string][]string{
	"readme": {"en", "es", "zh", "ja", "pt", "ar", "fr", "it", "de", "ru"},
	"arch":   {"en", "es"},
	"pong":   {"en", "es"},
}

// pages maps a page to its path per language.
func pages(page, lang string) string {
	if lang == "en" {
		switch page {
		case "readme":
			return "README.md"
		case "arch":
			return "ARCHITECTURE.md"
		default:
			return "examples/pong/README.md"
		}
	}
	switch page {
	case "readme":
		return filepath.Join("docs", lang, "README.md")
	case "arch":
		return filepath.Join("docs", lang, "ARCHITECTURE.md")
	default:
		return filepath.Join("docs", lang, "pong.md")
	}
}

func main() {
	var problems []string
	var checked int

	for _, page := range []string{"readme", "arch", "pong"} {
		for _, lang := range available[page] {
			path := pages(page, lang)
			body, err := os.ReadFile(path)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: missing", path))
				continue
			}
			checked++
			problems = append(problems, inspect(path, lang, page, string(body))...)
		}
	}

	if len(problems) > 0 {
		fmt.Println(strings.Join(problems, "\n"))
		os.Exit(1)
	}
	fmt.Printf("%d pages: every link resolves, every selector matches what exists\n", checked)
}

func inspect(path, lang, page, body string) []string {
	var problems []string
	base := filepath.Dir(path)

	// The selector is the one place a page links to other languages.
	selector := ""
	if index := strings.Index(body, "\n\n"); index > 0 {
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "**English**") || strings.Contains(line, "[English](") {
				selector = line
				break
			}
		}
	}
	if selector == "" {
		problems = append(problems, fmt.Sprintf("%s: no language selector", path))
	}
	for _, other := range languages {
		offered := strings.Contains(selector, name(other))
		exists := slices.Contains(available[page], other)
		switch {
		case exists && !offered:
			problems = append(problems, fmt.Sprintf("%s: selector is missing %s", path, name(other)))
		case !exists && offered:
			problems = append(problems, fmt.Sprintf("%s: selector offers %s, which does not exist", path, name(other)))
		}
	}

	for _, found := range link.FindAllStringSubmatch(body, -1) {
		text, target := found[1], found[2]
		if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
			continue
		}

		resolved := filepath.Join(base, strings.SplitN(target, "#", 2)[0])
		if _, err := os.Stat(resolved); err != nil {
			problems = append(problems, fmt.Sprintf("%s: [%s](%s) leads nowhere", path, text, target))
			continue
		}

		// Inside a translation, a link must not leave its language — except
		// the selector, and except links into code, which has no translation.
		if lang == "en" || isSelectorLink(text) || !strings.HasSuffix(resolved, ".md") {
			continue
		}
		// Leaving the language is only a fault when a translation exists and
		// was not used. Where there is none, English is the only option and
		// the page says so in words.
		if !strings.HasPrefix(resolved, filepath.Join("docs", lang)) &&
			translatedInto(resolved, lang) {
			problems = append(problems, fmt.Sprintf(
				"%s: [%s](%s) drops the reader out of %s, and a %s version exists",
				path, text, target, lang, lang))
		}
	}
	return problems
}

// isSelectorLink reports whether the text is a language name, which is the one
// kind of link that crosses on purpose.
func isSelectorLink(text string) bool {
	for _, lang := range languages {
		if text == name(lang) {
			return true
		}
	}
	return false
}

// translatedInto reports whether the document at this path has a version in the
// given language. Linking English when none exists is the only option, and not
// a fault; linking English when a translation is right there is a leak.
func translatedInto(path, lang string) bool {
	var page string
	switch {
	case path == "README.md":
		page = "readme"
	case path == "ARCHITECTURE.md":
		page = "arch"
	case strings.HasPrefix(path, "examples/pong"):
		page = "pong"
	default:
		return false
	}
	return slices.Contains(available[page], lang)
}

func name(lang string) string {
	return map[string]string{
		"en": "English", "es": "Español", "zh": "中文", "ja": "日本語",
		"pt": "Português", "ar": "العربية", "fr": "Français",
		"it": "Italiano", "de": "Deutsch", "ru": "Русский",
	}[lang]
}
