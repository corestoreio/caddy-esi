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
	"net/http"
	"time"

	"github.com/vdobler/ht/ht"
)

func init() {
	RegisterTest(pageRedisNoServer(), pageRedisNoServer())
}

var pageRedisNoServerCounter int

func pageRedisNoServer() (t *ht.Test) {
	pageRedisNoServerCounter++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page Redis no server %d", pageRedisNoServerCounter),
		Description: `Request to redis server fails because the server URI was misspelled and lazy loaded`,
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page_redis_no_server.html",
			Header: http.Header{
				"Accept":          []string{"text/html"},
				"Accept-Encoding": []string{"gzip, deflate, br"},
			},
			Timeout: 1 * time.Second,
		},
		Checks: ht.CheckList{
			ht.StatusCode{Expect: 200},
			&ht.Header{
				Header: "Etag",
				Condition: ht.Condition{
					Min: 14, Max: 18}},
			&ht.Header{
				Header: "Accept-Ranges",
				Condition: ht.Condition{
					Equals: `bytes`}},
			&ht.Header{
				Header: "Last-Modified",
				Condition: ht.Condition{
					Min: 29, Max: 29}},
			&ht.None{
				Of: ht.CheckList{
					&ht.HTMLContains{
						Selector: `html`,
						Text:     []string{"<esi:"},
					}}},
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
		},
	}
	return
}
