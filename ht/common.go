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
	"net/http"
	"time"

	"github.com/vdobler/ht/ht"
)

func makeRequestGET(path string) (r ht.Request) {
	r = ht.Request{
		Method: "GET",
		URL:    caddyAddress + path,
		Header: http.Header{
			"Accept":          []string{"text/html"},
			"Accept-Encoding": []string{"gzip, deflate, br"},
		},
		Timeout: 1 * time.Second,
	}
	return
}

func makeChecklist200(checks ...ht.Check) (cl ht.CheckList) {
	cl = ht.CheckList{
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
	}
	cl = append(cl, checks...)
	return
}
