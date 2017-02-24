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

// Package main adds http testing via integration tests using package
// github.com/vdobler/ht.
package main

import (
	"math/rand"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

const caddyAddress = `http://127.0.0.1:2017/`

func main() {
	// FYI: file noise.go adds lots of background requests

	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}

	var testKeys = make([]int, 0, len(testCollection))
	for k := range testCollection {
		testKeys = append(testKeys, k)
	}
	sort.Ints(testKeys)

	c := ht.Collection{
		Tests: make([]*ht.Test, 0, len(testCollection)),
	}
	//var buf bytes.Buffer
	//lg := log.New(&buf, "", log.LstdFlags)
	for _, k := range testKeys {
		//testCollection[k].Log = lg
		//testCollection[k].Execution.Verbosity = 5 // 5 = max verbosity
		testCollection[k].Execution.PreSleep = time.Duration(rand.Intn(50)) * time.Millisecond
		c.Tests = append(c.Tests, testCollection[k])
	}

	var exitStatus int
	_ = c.ExecuteConcurrent(runtime.NumCPU(), jar)

	for _, test := range c.Tests {
		if es := handleTestResult(test); es > 0 {
			exitStatus = 57
		}
	}

	for _, test := range afterTests {
		_ = test.Run()
		if es := handleTestResult(test); es > 0 {
			exitStatus = 64
		}
	}

	//println("\n", buf.String(), "\n")

	// Travis CI requires an exit code for the build to fail. Anything not 0
	// will fail the build.
	os.Exit(exitStatus)
}
