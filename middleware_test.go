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
	"github.com/SchumacherFM/caddyesi/backend"
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

func mwTestRunner(caddyFile string, r *http.Request, bodyContains string) func(*testing.T) {

	// Add here the middlewares Header and Template just to make sure that
	// caddyesi middleware processes the other middlewares correctly.

	return func(t *testing.T) {

		stack := mwTestHandler(t, caddyFile)

		rec := httptest.NewRecorder()
		code, err := stack.ServeHTTP(rec, r)
		if err != nil {
			t.Fatalf("Code %d\n%+v", code, err)
		}

		for key := range mwTestHeaders {
			val := mwTestHeaders.Get(key)
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

func TestMiddleware_ServeHTTP_StatusCodes(t *testing.T) {
	defer backend.RegisterRequestFunc("mwtest01", backend.MockRequestContent("Hello 2017!")).DeferredDeregister()

	t.Run("404 Code not allowed", func(t *testing.T) {

		hndl := mwTestHandler(t, `esi {		}`)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/page0999.html", nil)
		code, err := hndl.ServeHTTP(rec, req)
		assert.Exactly(t, http.StatusNotFound, code)
		assert.NoError(t, err)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("404 Code allowed", func(t *testing.T) {
		// stupid test ... must be refactored
		hndl := mwTestHandler(t, `esi {
				allowed_status_codes 404
			}`)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/page0998.html", nil)
		code, err := hndl.ServeHTTP(rec, req)
		assert.Exactly(t, http.StatusNotFound, code)
		assert.NoError(t, err)
		assert.Empty(t, rec.Body.String())
	})
}

func TestMiddleware_ServeHTTP_Once(t *testing.T) {

	defer backend.RegisterRequestFunc("mwtest01", backend.MockRequestError(errors.NewWriteFailedf("write failed"))).DeferredDeregister()

	t.Run("Middleware inactive due to GET allowed but POST request supplied", mwTestRunner(
		`esi {
			allowed_methods GET
		}`,
		httptest.NewRequest("POST", "/page01.html", nil),
		"<esi:include   src=\"mwTest01://micro.service/esi/foo\"",
	))

	{
		tmpLogFile, clean := esitesting.Tempfile(t)
		defer clean()
		t.Run("Replace a single ESI Tag in page01.html but error in backend request", mwTestRunner(
			`esi {
			on_error "my important global error message"
			allowed_methods GET
			log_file `+tmpLogFile+`
			log_level debug
		}`,
			httptest.NewRequest("GET", "/page01.html", nil),
			`my important global error message`,
		))
		logContent, err := ioutil.ReadFile(tmpLogFile)
		if err != nil {
			t.Fatal(err)
		}
		assert.Contains(t, string(logContent), `error: "write failed"`)
		assert.Contains(t, string(logContent), `url: "mwTest01://micro.service/esi/foo"`)
	}

	t.Run("Replace a single ESI Tag in page01.html but error in backend triggers default on_error message", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page01.html", nil),
		caddyesi.DefaultOnError,
	))

	defer backend.RegisterRequestFunc("mwtest02a", backend.MockRequestContent("Micro1Service1")).DeferredDeregister()
	defer backend.RegisterRequestFunc("mwtest02b", backend.MockRequestContent("Micro2Service2")).DeferredDeregister()
	defer backend.RegisterRequestFunc("mwtest02c", backend.MockRequestContent("Micro3Service3")).DeferredDeregister()
	t.Run("Load from three resources in page02.html successfully", mwTestRunner(
		`esi`,
		httptest.NewRequest("GET", "/page02.html", nil),
		`<p>Micro1Service1 "mwTest02A://microService1" Timeout 5ms MaxBody 10 kB</p>
<p>Micro2Service2 "mwTest02B://microService2" Timeout 6ms MaxBody 20 kB</p>
<p>Micro3Service3 "mwTest02C://microService3" Timeout 7ms MaxBody 30 kB</p>`,
	))
}

func TestMiddleware_ServeHTTP_Parallel(t *testing.T) {

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

	defer backend.RegisterRequestFunc("mwtest02a", backend.MockRequestContentCB("Micro1Service11", func() error {
		atomic.AddUint64(reqCount2a, 1)
		return nil
	})).DeferredDeregister()
	defer backend.RegisterRequestFunc("mwtest02b", backend.MockRequestContentCB("Micro2Service22", func() error {
		atomic.AddUint64(reqCount2b, 1)
		return nil
	})).DeferredDeregister()
	defer backend.RegisterRequestFunc("mwtest02c", backend.MockRequestContentCB("Micro3Service33", func() error {
		atomic.AddUint64(reqCount2c, 1)
		return nil
	})).DeferredDeregister()

	hpu := esitesting.NewHTTPParallelUsers(20, 10, 900, time.Millisecond)
	hpu.AssertResponse = func(rec *httptest.ResponseRecorder, code int, err error) {
		assert.Contains(t, rec.Body.String(), `<p>Micro1Service11 "mwTest02A://microService1" Timeout 5ms MaxBody 10 kB</p>`)
		assert.Contains(t, rec.Body.String(), `<p>Micro2Service22 "mwTest02B://microService2" Timeout 6ms MaxBody 20 kB</p>`)
		assert.Contains(t, rec.Body.String(), `<p>Micro3Service33 "mwTest02C://microService3" Timeout 7ms MaxBody 30 kB</p>`)
	}

	tmpLogFile, _ := esitesting.Tempfile(t)
	//defer clean()
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
	assert.Exactly(t, 1, strings.Count(string(logContent), `caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse error: "<nil>"`), `caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse error: "<nil>" MUST only occur once!!!`)
	assert.Exactly(t, 600, strings.Count(string(logContent), `esitag.Entity.QueryResources.RequestFunc.CBStateClosed`), `esitag.Entity.QueryResources.RequestFunc.CBStateClosed`)
}
