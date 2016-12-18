package caddyesi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/header"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy/caddyhttp/templates"
	"github.com/stretchr/testify/assert"
)

func mwTestRunner(caddyFile string, r *http.Request) func(*testing.T) {

	// just to see that headers gets passed through our middleware
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
				Root:    "esitag/testdata/",
				FileSys: http.Dir("esitag/testdata/"),
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
			t.Fatal("Code", code, "Error:", err.Error())
		}

		for key := range wantHeaders {
			val := wantHeaders.Get(key)
			assert.Exactly(t, val, rec.Header().Get(key), "Header Key %q", key)
		}

		if rec.Body.Len() == 0 {
			t.Error("Unexpected empty Body!")
		}
		// assert.NotContains(t, rec.Body.String(), `<esi:`, "Body should not contain any kind of ESI tag")

		t.Logf("Code: %d", code)
		t.Logf("Header: %#v", rec.Header())
		t.Logf("Body: %q", rec.Body.String())
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {

	t.Run("Replace a single ESI Tag in page0.html", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page0.html", nil),
	))

}
