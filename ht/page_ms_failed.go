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
	RegisterConcurrentTest(20, page02())
	RegisterConcurrentTest(21, page02())
}

var tc02 int // tc = test counter

func page02() (t *ht.Test) {
	tc02++
	t = &ht.Test{
		Name:        fmt.Sprintf("Request to micro service failed, iteration %d", tc02),
		Description: `Tries to load from a nonexisitent micro service and displays a custom error message`,
		Request:     makeRequestGET("page_ms_failed.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Contains: "MS9999 not available",
				Count:    1,
			},
			&ht.Body{
				Contains: `class="page02ErrMsg18MS"`,
				Count:    1,
			},
		),
	}
	return
}
