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
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/vdobler/ht/ht"
)

var (
	testCollection = map[int]*ht.Test{}
	afterTests     []*ht.Test
)

// RegisterConcurrentTest adds a set of tests to the collection
func RegisterConcurrentTest(position int, test *ht.Test) {
	if _, ok := testCollection[position]; ok {
		panic(fmt.Sprintf("Position %d already exists with test: %#v", position, test))
	}
	testCollection[position] = test
}

// RegisterAfterTest register a test to run after the main concurrent test loop.
func RegisterAfterTest(tests ...*ht.Test) {
	afterTests = append(afterTests, tests...)
}

func handleTestResult(test *ht.Test) (exitStatus int) {
	if err := test.PrintReport(os.Stdout); err != nil {
		panic(err)
	}
	if test.Status > ht.Pass {
		exitStatus = 1
		color.Red("Failed %s", test.Name)
		if test.Response.BodyErr != nil {
			color.Yellow(fmt.Sprintf("Response Body Error: %s\n", test.Response.BodyErr))
		}
		color.Yellow("Response Body: %q\n", test.Response.BodyStr)
	}
	return
}

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
				Min: 8, Max: 18}},
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
