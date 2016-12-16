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

func commaListToSlice(str string) []string {
	sl := strings.Split(str, ",")
	for i := range sl {
		sl[i] = strings.Map(dropSpaces, sl[i])
	}
	return sl
}
