package caddyesi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi"
	"github.com/corestoreio/errors"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/header"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/mholt/caddy/caddyhttp/templates"
	"github.com/stretchr/testify/assert"
)

func mwTestRunner(caddyFile string, r *http.Request) func(*testing.T) {

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
		// assert.NotContains(t, rec.Body.String(), `<esi:`, "Body should not contain any kind of ESI tag")

		t.Logf("Code: %d", code)
		t.Logf("Header: %#v", rec.Header())
		t.Logf("Body: %q", rec.Body.String())
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {

	//defer backend.RegisterRequestFunc("mwTest01", func(url string, timeout time.Duration, maxBodySize int64) ([]byte, error) {
	//	return nil, errors.NewAlreadyExistsf("[whops] todo")
	//})

	t.Run("Replace a single ESI Tag in page0.html", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page0.html", nil),
	))

}
