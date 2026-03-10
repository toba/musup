package state

import (
	"regexp"
	"strings"
)

var (
	reParenBracket = regexp.MustCompile(`[\(\[][^)\]]*[\)\]]`)
	rePunctuation  = regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	reWhitespace   = regexp.MustCompile(`\s+`)
)

// Normalize returns a normalized version of s for fuzzy matching.
// It lowercases, strips parenthetical/bracketed content, removes punctuation,
// and collapses whitespace.
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = reParenBracket.ReplaceAllString(s, "")
	s = rePunctuation.ReplaceAllString(s, "")
	s = reWhitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
