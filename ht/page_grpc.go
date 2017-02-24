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

import "github.com/vdobler/ht/ht"

func init() {
	// Must run at the end because cannot run concurrent.
	RegisterAfterTest(pageGRPC())
	RegisterAfterTest(pageGRPC2())
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

func pageGRPC() (t *ht.Test) {
	t = &ht.Test{
		Name:        "Page GRPC Latency Iteration",
		Description: `Request loads page_grpc.html from a GRPC micro service. One of the three requests has a coalesce attribute set true.`,
		Request:     makeRequestGET("page_grpc.html"),
		Checks: makeChecklist200(
			&ht.Latency{
				N:                  2240,
				Concurrent:         20,
				Limits:             "0.9995 ≤ 0.4s",
				IndividualSessions: false,
			},
		),
	}
	t.Checks = append(t.Checks, grpcCommonChecks...)
	return
}

func pageGRPC2() (t *ht.Test) {
	t = &ht.Test{
		Name:        "Page GRPC Check Latency Iteration",
		Description: `Request loads page_grpc.html and checks if coalesce requests are much lower than non-coalesce requests`,
		Request:     makeRequestGET("page_grpc.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Regexp: "coalesce_enabled=[0-9]{2,3}",
				Count:  1,
			},
			&ht.Body{
				Regexp: "coalesce_disabled=[0-9]{4}",
				Count:  1,
			},
		),
	}
	t.Checks = append(t.Checks, grpcCommonChecks...)
	return
}
