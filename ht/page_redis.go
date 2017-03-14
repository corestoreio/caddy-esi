// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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
	RegisterConcurrentTest(30, pageRedis())
	RegisterConcurrentTest(31, pageRedis())
}

var tc03 int // tc = test counter

func pageRedis() (t *ht.Test) {
	tc03++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page Redis success %d", tc03),
		Description: `Request loads two keys from a redis server`,
		Request:     makeRequestGET("page_redis.html"),
		Checks: makeChecklist200(
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
		),
	}
	return
}
