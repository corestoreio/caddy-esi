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
	RegisterTest(pageRedis(), pageRedis())
}

var pageRedisCounter int

func pageRedis() (t *ht.Test) {
	pageRedisCounter++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page Redis success %d", pageRedisCounter),
		Description: `Request loads two keys from a redis server`,
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page_redis.html",
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
				Contains: "Catalog Product 001", // see integration.sh
				Count:    1,
			},
			&ht.Body{
				Contains: "Catalog Category Tree", // see integration.sh
				Count:    1,
			},
			&ht.Body{
				Contains: "You have 10 items in your cart", // see integration.sh
				Count:    1,
			},
			&ht.Body{
				Contains: ` class="redisSuccess"`,
				Count:    3,
			},
		},
	}
	return
}
