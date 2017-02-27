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
	"os"

	"github.com/vdobler/ht/ht"
)

func init() {
	// All tests must run at the end because cannot run concurrent.
	// https://docs.travis-ci.com/user/environment-variables/#Default-Environment-Variables
	// Disable latency tests as they fail on travis OSX because of some weird
	// bugs ... maybe in ht library. I cannot reproduce the bug on my machine.
	osName := os.Getenv("TRAVIS_OS_NAME")
	forceOSX := os.Getenv("ESI_OSX_FORCE")
	if osName != "osx" || forceOSX == "1" {
		RegisterAfterTest(pageGRPC())
		RegisterAfterTest(pageGRPC2())
	}
	RegisterAfterTest(pageGRPC3())
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
				N:          2240,
				Concurrent: 20,
				// This test also runs on Travis and compiled with -race so must
				// set the limit to a higher seconds value. Usually it can be
				// run with "0.9995 ≤ 0.2s", now 0.8s should pass the tests.
				Limits:             "0.9995 ≤ 0.8s",
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

func pageGRPC3() (t *ht.Test) {
	t = &ht.Test{
		Name:        "Page GRPC PrintDebug",
		Description: `Request loads page_grpc.html and checks for the printdebug output`,
		Request:     makeRequestGET("page_grpc_printdebug.html"),
		Checks: makeChecklist200(
			&ht.Body{
				Contains: `<!-- Duration:`,
				Count:    2,
			},
			&ht.Body{
				Contains: `Error:none`,
				Count:    2,
			},
			&ht.Body{
				Contains: `Tag:include src="grpc_integration_01"`,
				Count:    2,
			},
			&ht.Body{
				Contains: `Hoppla - Something went wrong`,
				Count:    2,
			},
			&ht.Body{
				Contains: "Arg Key=printdebug_1",
				Count:    1,
			},
			&ht.Body{
				Contains: "Arg Key=printdebug_2",
				Count:    1,
			},
			&ht.Body{
				Contains: ` class="gRPCSuccess"`,
				Count:    2,
			},
		),
	}
	return
}
