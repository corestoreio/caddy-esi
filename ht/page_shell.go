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

package main

import (
	"fmt"

	"github.com/vdobler/ht/ht"
)

func init() {
	RegisterTest(50, pageShell())
	RegisterTest(51, pageShell())
}

var tc05 int // tc = test counter

func pageShell() (t *ht.Test) {
	tc05++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page Shell success %d", tc05),
		Description: `Request loads test from a shell script`,
		Request:     makeRequestGET("page_shell.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Contains: `<relative-time datetime="2017-01-23T20:07:40Z">Jan 23, 2017</relative-time>`,
				Count:    1,
			},
			&ht.Body{
				// As we're writing the resource args we check for the JSON encoded args
				Contains: `"request_uri":"/page_shell.html"`,
				Count:    1,
			},
			&ht.Body{
				Contains: `"url":"sh://./testdata/www_stdOut.sh"`,
				Count:    1,
			},
			&ht.Body{
				Contains: `<td>Resource not available</td>`,
				Count:    1,
			},
			&ht.Body{
				Contains: ` class="shellSuccess"`,
				Count:    2,
			},
		),
	}
	return
}
