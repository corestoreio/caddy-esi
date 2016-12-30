package caddyesi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi"
	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/header"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy/caddyhttp/templates"
	"github.com/stretchr/testify/assert"
)

func mwTestRunner(caddyFile string, r *http.Request, bodyContains string) func(*testing.T) {

	// Add here the middlewares Header and Template just to make sure that
	// caddyesi middleware processes the other middlewares correctly.

	wantHeaders := http.Header{"X-Esi-Test": []string{"GopherX"}}

	return func(t *testing.T) {
		ctc := caddy.NewTestController("http", caddyFile)

		httpserver.GetConfig(ctc).AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
			return header.Headers{
				Next: next,
				Rules: []header.Rule{
					{
						Path:    "/",
						Headers: wantHeaders,
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

		rec := httptest.NewRecorder()
		code, err := stack.ServeHTTP(rec, r)
		if err != nil {
			t.Fatalf("Code %d\n%+v", code, err)
		}

		for key := range wantHeaders {
			val := wantHeaders.Get(key)
			assert.Exactly(t, val, rec.Header().Get(key), "Header Key %q", key)
		}

		if rec.Body.Len() == 0 {
			t.Error("Unexpected empty Body!")
		}

		if bodyContains != "" {
			assert.Contains(t, rec.Body.String(), bodyContains, "Body should contain in Test: %s", t.Name())
		} else {

			t.Logf("Code: %d", code)
			t.Logf("Header: %#v", rec.Header())
			t.Logf("Body: %q", rec.Body.String())
		}
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {

	defer backend.RegisterRequestFunc("mwtest01", backend.MockRequestError(errors.NewWriteFailedf("write failed"))).DeferredDeregister()

	t.Run("Middleware inactive due to GET allowed but POST request supplied", mwTestRunner(
		`esi {
			allowed_methods GET
		}`,
		httptest.NewRequest("POST", "/page01.html", nil),
		"<esi:include   src=\"mwTest01://micro.service/esi/foo\"",
	))

	t.Run("Replace a single ESI Tag in page01.html but error in backend request", mwTestRunner(
		`esi {
			on_error "my important global error message"
			allowed_methods GET
			log_file stdout
			log_level debug
		}`,
		httptest.NewRequest("GET", "/page01.html", nil),
		`my important global error message`,
	))

	t.Run("Replace a single ESI Tag in page01.html but error in backend triggers default on_error message", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page01.html", nil),
		caddyesi.DefaultOnError,
	))

}
