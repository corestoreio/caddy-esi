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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SchumacherFM/caddyesi"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// BenchmarkMiddleware_ServeHTTP-4   	   20000	     60994 ns/op	   44397 B/op	      52 allocs/op
func BenchmarkMiddleware_ServeHTTP(b *testing.B) {

	defer esitag.RegisterResourceHandler("bmServe01", esitesting.MockRequestContent("Hello 2017!")).DeferredDeregister()

	const serveFile = `testdata/page03.html`
	const caddyFile = `esi {
			timeout 19s
			on_error "my important global error message"
			allowed_methods GET
			# log_file ./benchmark_serve.log
			# log_level debug
		}`

	ctc := caddy.NewTestController("http", caddyFile)

	if err := caddyesi.PluginSetup(ctc); err != nil {
		b.Fatal(err)
	}

	httpserver.GetConfig(ctc).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		return httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {

			f, err := os.Open(serveFile)
			if err != nil {
				b.Fatal(err)
			}
			defer f.Close()

			if _, err := io.Copy(w, f); err != nil {
				b.Fatal(err)
			}

			return http.StatusOK, nil
		})
	})

	mids := httpserver.GetConfig(ctc).Middleware()
	finalHandler := httpserver.HandlerFunc(func(w http.ResponseWriter, r *http.Request) (int, error) {
		return http.StatusNotImplemented, errors.New("Should not be called! Or File not found")
	})

	var stack httpserver.Handler = finalHandler
	for i := len(mids) - 1; i >= 0; i-- {
		stack = mids[i](stack)
	}

	req := httptest.NewRequest("GET", "https://my.blog/any/path", nil)
	req.Header = http.Header{
		"Host":                      []string{"www.example.com"},
		"Connection":                []string{"keep-alive"},
		"Pragma":                    []string{"no-cache"},
		"Cache-Control":             []string{"no-cache"},
		"Upgrade-Insecure-Requests": []string{"1"},
		"User-Agent":                []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10)"},
		"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
		"DNT":                       []string{"1"},
		"Referer":                   []string{"https://www.example.com/"},
		"Accept-Encoding":           []string{"gzip, deflate, sdch, br"},
		"Avail-Dictionary":          []string{"lhdx6rYE"},
		"Accept-Language":           []string{"en-US,en;q=0.8"},
		"Cookie":                    []string{"x-wl-uid=1vnTVF5WyZIe5Fymf2a4H+pFPyJa4wxNmzCKdImj1UqQPV5ecUs2sm46vDbGJUI+sE=", "session-token=AIo5Vf+c/GhoTRWq4V; JSESSIONID=58B7C7A24731R869B75D142E970CEAD4; csm-hit=D5P2DBNF895ZDJTCTEQ7+s-D5P2DBNF895ZDJTCTEQ7|1483297885458; session-id-time=2082754801l"},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {

		rec := httptest.NewRecorder() // 3 allocs
		code, err := stack.ServeHTTP(rec, req)
		if err != nil {
			b.Fatalf("%+v", err)
		}
		if code != http.StatusOK {
			b.Fatalf("Code must be StatusOK but got %d", code)
		}
	}
}
