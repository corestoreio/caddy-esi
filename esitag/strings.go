package esitag

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

func trimStrings(sl []string) {
	for i := range sl {
		sl[i] = strings.Map(dropSpaces, sl[i])
	}
}
