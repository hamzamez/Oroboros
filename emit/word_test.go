package emit

import (
	"regexp"
	"strings"
)

// withWord gives a test fixture the word the Go, JVM and windows target files
// declare, when the fixture says none. ADR 0026 makes the word required of
// every target — the compiler holds no default window — and a fixture about
// something else should not have to repeat it. A fixture that declares its own
// word, or tests its absence, is left alone.
func withWord(src string) string {
	if strings.Contains(src, " word)") || strings.Contains(src, "no-word") {
		return src
	}
	loc := targetHead.FindStringIndex(src)
	if loc == nil {
		return src
	}
	return src[:loc[1]] + " (repr (int -9223372036854775808 9223372036854775807) word)" + src[loc[1]:]
}

var targetHead = regexp.MustCompile(`\(target\s+[^\s()]+`)
