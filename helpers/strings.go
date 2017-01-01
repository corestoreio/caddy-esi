package helpers

import (
	"strconv"
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

// StringsToInts converts stringified integers into real ints. If an error
// occurs during converting the entries gets skipped.
func StringsToInts(ssl []string) []int {
	ret := make([]int, 0, len(ssl))
	for _, s := range ssl {
		if v, err := strconv.Atoi(s); err == nil {
			ret = append(ret, v)
		}

	}
	return ret
}
