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
	"net/http/httputil"
	"os"
	"runtime"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

const caddyAddress = `http://127.0.0.1:2017/`

func main() {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	c := ht.Collection{
		Tests: testCollection,
	}

	var exitStatus int
	if err := c.ExecuteConcurrent(runtime.NumCPU(), jar); err != nil {
		exitStatus = 26 // line number ;-)
		println("ExecuteConcurrent:", err.Error())
	}

	for _, test := range c.Tests {
		if err := test.PrintReport(os.Stdout); err != nil {
			panic(err)
		}
		if test.Status > ht.Pass {
			exitStatus = 35 // line number ;-)

			reqData, err := httputil.DumpRequest(test.Request.Request, true)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stdout, "Request:\n%s\n", reqData)
			resData, err := httputil.DumpResponse(test.Response.Response, false)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stdout, "Response:\n%s\nError: %s\n\n", resData, test.Response.BodyErr)
		}
	}

	// Travis CI requires an exit code for the build to fail. Anything not 0
	// will fail the build.
	os.Exit(exitStatus)
}

// RegisterTest adds a set of tests to the collection
func RegisterTest(tests ...*ht.Test) {
	testCollection = append(testCollection, tests...)
}

var testCollection []*ht.Test
