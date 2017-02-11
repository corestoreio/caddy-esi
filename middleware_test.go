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

package caddyesi_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitag/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/header"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy/caddyhttp/templates"
	"github.com/stretchr/testify/assert"
)

var mwTestHeaders = http.Header{"X-Esi-Test": []string{"GopherX"}}

func mwTestHandler(t *testing.T, caddyFile string) httpserver.Handler {
	ctc := caddy.NewTestController("http", caddyFile)

	httpserver.GetConfig(ctc).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return header.Headers{
			Next: next,
			Rules: []header.Rule{
				{
					Path:    "/",
					Headers: mwTestHeaders,
				},
			},
		}
	})

	if err := caddyesi.PluginSetup(ctc); err != nil {
		t.Fatal(err)
	}

	httpserver.GetConfig(ctc).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return templates.Templates{
			Next: next,
			Rules: []templates.Rule{
				{
					Path:       "/",
					Extensions: []string{".html"},
					IndexFiles: []string{"index.html"},
				},
			},
			Root:    "testdata/",
			FileSys: http.Dir("testdata/"),
		}
	})

	mids := httpserver.GetConfig(ctc).Middleware()

	finalHandler := httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		return http.StatusNotImplemented, errors.New("Should not be called! Or File not found")
	})

	var stack httpserver.Handler = finalHandler

	for i := len(mids) - 1; i >= 0; i-- {
		stack = mids[i](stack)
	}
	return stack
}

func mwTestRunner(caddyFile string, r *http.Request, bodyContains string, wantErrBhf errors.BehaviourFunc) func(*testing.T) {

	// Add here the middlewares Header and Template just to make sure that
	// caddyesi middleware processes the other middlewares correctly.

	return func(t *testing.T) {

		stack := mwTestHandler(t, caddyFile)
		// first iteration loads the WrapBuffer ResponseWriter.
		// second iteration loads the WrapPiped ResponseWriter to get the
		// already parsed ESI tags from the internal map.
		for ii := 1; ii <= 2; ii++ {
			rec := httptest.NewRecorder()
			code, err := stack.ServeHTTP(rec, r)
			if wantErrBhf != nil {
				assert.True(t, wantErrBhf(err), "Code %d Error: %s", code, err)
				return
			} else if err != nil {
				t.Fatalf("Iteration %d Code %d\n%+v", ii, code, err)
			}

			for key := range mwTestHeaders {
				val := mwTestHeaders.Get(key)
				assert.Exactly(t, val, rec.Header().Get(key), "Iteration %d Header Key %q", ii, key)
			}

			if rec.Body.Len() == 0 {
				t.Errorf("Unexpected empty Body !Iteration %d ", ii)
			}

			if bodyContains != "" {
				assert.Contains(t, rec.Body.String(), bodyContains, "Iteration %d Body should contain in Test: %s", ii, t.Name())
			} else {
				t.Logf("Iteration %d Code: %d", ii, code)
				t.Logf("Header: %#v", rec.Header())
				t.Logf("Body: %q", rec.Body.String())
			}
		}
	}
}

func TestMiddleware_ServeHTTP_Once(t *testing.T) {
	// t.Parallel() not possible due to the global map in backend

	const errMsg = `mwTest01: A random micro service error`
	defer esitag.RegisterResourceHandler("mwtest01", backend.MockRequestError(errors.NewWriteFailedf(errMsg))).DeferredDeregister()

	t.Run("Protocol scheme in ESI tag not supported triggers error", mwTestRunner(
		`esi {
			allowed_methods GET
		}`,
		httptest.NewRequest("GET", "/page06.html", nil),
		"XXX<esi:include   src=\"unsupported://micro.service/esi/foo\"",
		errors.IsNotSupported,
	))

	t.Run("Middleware inactive due to GET allowed but POST request supplied", mwTestRunner(
		`esi {
			allowed_methods GET
		}`,
		httptest.NewRequest("POST", "/page01.html", nil),
		"<esi:include   src=\"mwTest01://micro.service/esi/foo\"",
		nil,
	))

	{
		tmpLogFile, clean := esitesting.Tempfile(t)
		defer clean()
		t.Log("tmpLogFile", tmpLogFile)
		t.Run("Replace a single ESI Tag in page01.html but error in backend request", mwTestRunner(
			`esi {
			on_error "my important global error message"
			allowed_methods GET
			log_file `+tmpLogFile+`
			log_level debug
		}`,
			httptest.NewRequest("GET", "/page01.html", nil),
			`my important global error message`,
			nil,
		))
		logContent, err := ioutil.ReadFile(tmpLogFile)
		if err != nil {
			t.Fatal(err)
		}

		assert.Exactly(t, 2,
			strings.Count(string(logContent), `"error":"[esibackend] Resource.Handler.DoRequest: `+errMsg+`"`),
			"Should contain 2 occurrences")
		assert.Exactly(t, 2, strings.Count(string(logContent), `"resource_url":"mwTest01://micro.service/esi/foo"`), "Should contain 2 occurrences")
	}

	t.Run("Replace a single ESI Tag in page01.html but error in backend triggers default on_error message", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page01.html", nil),
		caddyesi.DefaultOnError,
		nil,
	))

	defer esitag.RegisterResourceHandler("mwtest02a", backend.MockRequestContent("Micro1Service1")).DeferredDeregister()
	defer esitag.RegisterResourceHandler("mwtest02b", backend.MockRequestContent("Micro2Service2")).DeferredDeregister()
	defer esitag.RegisterResourceHandler("mwtest02c", backend.MockRequestContent("Micro3Service3")).DeferredDeregister()
	t.Run("Load from three resources in page02.html successfully", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page02.html", nil),
		`<p>Micro1Service1 "mwTest02A://microService1" Timeout 5ms MaxBody 10 kB</p>
<p>Micro2Service2 "mwTest02B://microService2" Timeout 6ms MaxBody 20 kB</p>
<p>Micro3Service3 "mwTest02C://microService3" Timeout 7ms MaxBody 30 kB</p>`,
		nil,
	))

	t.Run("ESI tags not present in page07.html", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page07.html", nil),
		`<esi_include   src="whuuusaa://micro.service/esi/foo" />`,
		nil,
	))

}

func TestMiddleware_ServeHTTP_Parallel(t *testing.T) {
	// t.Parallel() not possible due to the global map in backend

	// This test delivers food for the race detector.
	// This tests creates 10 requests for each of the 20 users. All 200 requests
	// occur in 900ms. We have three backend micro services in the HTML page.
	// Each micro service receives 200 requests. In total this produces 600
	// requests to backend services.
	// Despite we have 200 incoming requests, the HTML page gets only parsed
	// once.

	var reqCount2a = new(uint64)
	var reqCount2b = new(uint64)
	var reqCount2c = new(uint64)

	defer esitag.RegisterResourceHandler("mwtest02a", backend.MockRequestContentCB("Micro1Service11", func() error {
		atomic.AddUint64(reqCount2a, 1)
		return nil
	})).DeferredDeregister()
	defer esitag.RegisterResourceHandler("mwtest02b", backend.MockRequestContentCB("Micro2Service22", func() error {
		atomic.AddUint64(reqCount2b, 1)
		return nil
	})).DeferredDeregister()
	defer esitag.RegisterResourceHandler("mwtest02c", backend.MockRequestContentCB("Micro3Service33", func() error {
		atomic.AddUint64(reqCount2c, 1)
		return nil
	})).DeferredDeregister()

	hpu := esitesting.NewHTTPParallelUsers(20, 10, 900, time.Millisecond)
	hpu.AssertResponse = func(rec *httptest.ResponseRecorder, code int, err error) {
		assert.Contains(t, rec.Body.String(), `<p>Micro1Service11 "mwTest02A://microService1" Timeout 5ms MaxBody 10 kB</p>`)
		assert.Contains(t, rec.Body.String(), `<p>Micro2Service22 "mwTest02B://microService2" Timeout 6ms MaxBody 20 kB</p>`)
		assert.Contains(t, rec.Body.String(), `<p>Micro3Service33 "mwTest02C://microService3" Timeout 7ms MaxBody 30 kB</p>`)
	}

	tmpLogFile, clean := esitesting.Tempfile(t)
	defer clean()
	t.Log(tmpLogFile)

	hpu.ServeHTTPNewRequest(func() *http.Request {
		return httptest.NewRequest("GET", "/page02.html", nil)
	}, mwTestHandler(t, `esi {
			on_error "my important global error message"
			allowed_methods GET
			log_file `+tmpLogFile+`
			log_level debug
	}`))

	// 200 == 20 * 10 @see NewHTTPParallelUsers
	assert.Exactly(t, 200, int(*reqCount2a), "Calls to Micro Service 1")
	assert.Exactly(t, 200, int(*reqCount2b), "Calls to Micro Service 2")
	assert.Exactly(t, 200, int(*reqCount2c), "Calls to Micro Service 3")

	logContent, err := ioutil.ReadFile(tmpLogFile)
	if err != nil {
		t.Fatal(err)
	}
	assert.Exactly(t, 1, strings.Count(string(logContent), `caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse","error":"<nil>"`), `caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse error: "<nil>" MUST only occur once!!!`)
	assert.Exactly(t, 600, strings.Count(string(logContent), `esitag.Entity.QueryResources.ResourceHandler.CBStateClosed`), `esitag.Entity.QueryResources.ResourceHandler.CBStateClosed`)
}

func TestMiddleware_HandleHeaderCommands(t *testing.T) {
	t.Parallel()

	const myMsg = `mwTest01: Another random micro service message`
	defer esitag.RegisterResourceHandler("mwtest01", backend.MockRequestContent(myMsg)).DeferredDeregister()

	tmpLogFile, clean := esitesting.Tempfile(t)
	defer clean()
	//t.Log(tmpLogFile)

	stack := mwTestHandler(t, `esi /page01 {
			on_error "my unimportant global error message"
			cmd_header_name X-Esi-CMD
			log_file `+tmpLogFile+`
			log_level debug
		}`)

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest("GET", "/page01.html", nil)
		if i == 3 {
			req.Header.Set("X-Esi-Cmd", "purge")
		}
		rec := httptest.NewRecorder()
		code, err := stack.ServeHTTP(rec, req)
		if code != http.StatusOK {
			t.Errorf("HTTP Code Have %d, Want %d", code, http.StatusOK)
		}
		if err != nil {
			t.Fatalf("%+v", err)
		}
	}

	logContent, err := ioutil.ReadFile(tmpLogFile)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, string(logContent), `caddyesi.PathConfig.purgeESICache","path_scope":"/page01"`)
	assert.Exactly(t, 2, strings.Count(string(logContent), `caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse","error":"<nil>"`))
}
