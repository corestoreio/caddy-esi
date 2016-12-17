package helpers

import (
	"strings"
	"unicode"
)

func dropSpaces(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}

// CommaListToSlice transforms a comma separated string into a slice with
// trimmed spaces.
func CommaListToSlice(str string) []string {
	sl := strings.Split(str, ",")
	for i := range sl {
		sl[i] = strings.Map(dropSpaces, sl[i])
	}
	if len(sl) == 1 && sl[0] == "" {
		return []string{}
	}
	return sl
}
