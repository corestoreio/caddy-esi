// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
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
	RegisterConcurrentTest(40, pageRedisNoServer())
	RegisterConcurrentTest(41, pageRedisNoServer())
}

var tc04 int // tc = test counter

func pageRedisNoServer() (t *ht.Test) {
	tc04++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page Redis unavailable %d", tc04),
		Description: `Request to redis server fails because the server URI was misspelled and lazy loaded`,
		Request:     makeRequestGET("page_redis_unavailable.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Contains: "<td>Redis on google cloud platform one not found</td>",
				Count:    1,
			},
			&ht.Body{
				Contains: `<td>Resource not available</td>`,
				Count:    1,
			},
			&ht.Body{
				Contains: `<td><b>
    We're sorry but the requested service cannot be reach€d!
</b>
</td>`,
				Count: 1,
			},
			&ht.Body{
				Contains: ` class="redisfailure01"`,
				Count:    3,
			},
		),
	}
	return
}
