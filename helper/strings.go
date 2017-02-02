// Copyright 2016-2017, Cyrill @ Schumacher.fm and the CaddyESI Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package helper

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
