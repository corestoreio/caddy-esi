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
	// For now we must create new pointers each time we want to run a test. A
	// single test cannot be shared between goroutines. This is a limitation
	// which can maybe fixed by a special handling of the Request and Jar field
	// in ht. This change might complicate things ...
	RegisterTest(1000, pageGRPC())
	RegisterTest(1010, pageGRPC2())
}

var grpcCommonChecks = ht.CheckList{
	&ht.Body{
		Contains: `Hoppla - Something went wrong`,
		Count:    -1, // Inverts the check so: NotContains
	},
	&ht.Body{
		Contains: "Arg Key=coalesce_enabled",
		Count:    1,
	},
	&ht.Body{
		Contains: "Arg Key=coalesce_disabled",
		Count:    1,
	},
	&ht.Body{
		Contains: ` class="gRPCSuccess"`,
		Count:    2,
	},
}

var tcGRPC int // tc = test counter

func pageGRPC() (t *ht.Test) {
	tcGRPC++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page GRPC Latency Iteration %d", tcGRPC),
		Description: `Request loads page_grpc.html from a GRPC micro service. One of the three requests has a coalesce attribute set true.`,
		Request:     makeRequestGET("page_grpc.html"),
		Checks: makeChecklist200(
			&ht.Latency{
				N:                  5000,
				Concurrent:         24,
				Limits:             "0.9995 ≤ 0.9s",
				IndividualSessions: false,
			},
		),
	}
	t.Checks = append(t.Checks, grpcCommonChecks...)
	return
}

func pageGRPC2() (t *ht.Test) {
	tcGRPC++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page GRPC Check Latency Iteration %d", tcGRPC),
		Description: `Request loads page_grpc.html and checks if coalesce requests are much lower than non-coalesce requests`,
		Request:     makeRequestGET("page_grpc.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Regexp: "coalesce_enabled=[45][0-9]{2}", // 4xx
				Count:  1,
			},
			&ht.Body{
				Regexp: "coalesce_disabled=5[0-9]{3}", // 5xxx
				Count:  1,
			},
		),
	}
	t.Checks = append(t.Checks, grpcCommonChecks...)
	return
}
