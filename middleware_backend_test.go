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

// +build esiall

// above build tag triggers inclusion of all backend resource connectors

package caddyesi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/alicebob/miniredis"
)

func TestMiddleware_ServeHTTP_Redis(t *testing.T) {
	t.Parallel()

	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	if err := mr.Set("myKey_9876", "Gopher01"); err != nil {
		t.Fatal(err)
	}
	if err := mr.Set("myKey02", "Rustafarian02"); err != nil {
		t.Fatal(err)
	}

	esiCfg, clean := esitesting.WriteXMLTempFile(t, backend.ConfigItems{
		backend.NewConfigItem(`redis://`+mr.Addr()+`/0`, "miniRedis"),
	})
	defer clean()
	t.Run("Query in page04.html successfully", mwTestRunner(
		`esi {
			resources `+esiCfg+`
			# miniRedis redis://`+mr.Addr()+`/0
			# log_file +tmpLogFile+
			# log_level debug
		}`,
		func() *http.Request {
			req := httptest.NewRequest("GET", "/page04.html", nil)
			req.Header.Set("X-Gopher-ID", "9876")
			return req
		}(),
		`<p>Gopher01</p>
<p>Rustafarian02</p>`,
		nil,
	))

	//tmpLogFile, _ := esitesting.Tempfile(t)
	//t.Log(tmpLogFile)
	// defer clean()
	esiCfg, clean = esitesting.WriteXMLTempFile(t, backend.ConfigItems{
		backend.NewConfigItem(`redis://`+mr.Addr()+`/0`, "miniRedis"),
		backend.NewConfigItem(`mockTimeout://50s`, "miniRedisTimeout"),
	})
	defer clean()
	t.Run("Query in page05.html but timeout in server 2 and fall back to server 1", mwTestRunner(
		`esi {
			   resources `+esiCfg+`
			  #log_file  +tmpLogFile+
			  #log_level debug
		}`,
		func() *http.Request {
			req := httptest.NewRequest("GET", "/page05.html", nil)
			req.Header.Set("X-Gopher-ID", "9876")
			return req
		}(),
		`<p>Gopher01</p>
<p>Rustafarian02</p>`,
		nil,
	))
}
