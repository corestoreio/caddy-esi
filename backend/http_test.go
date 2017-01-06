package backend_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

var _ backend.RequestFunc = backend.FetchHTTP

func TestFetchHTTP(t *testing.T) {
	// All tests modifying TestClient cannot be run in parallel.

	rfa := &backend.RequestFuncArgs{
		URL: "http://whatever.anydomain/page.html",
		ExternalReq: func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Last-Viewed_Products", "1,2,3")
			req.Header.Set("X-Cart-ID", "1234567890")
			req.Header.Set("Cookie", "x-wl-uid=1vnTVF5WyZIe5Fymf2a4H+pFPyJa4wxNmzCKdImj1UqQPV5ecUs1sn46vDbGJUI+sE=")
			return req
		}(),
		Timeout:        time.Second,
		MaxBodySize:    15,
		ForwardHeaders: []string{"X-Cart-Id", "Cookie"},
		ReturnHeaders:  []string{"Set-Cookie"},
	}

	t.Run("Forward and Return Headers", func(t *testing.T) {
		backend.TestClient = &http.Client{
			Transport: &esitesting.HTTPTrip{
				// use buffer pool
				GenerateResponse: func(req *http.Request) *http.Response {

					assert.Exactly(t, `1234567890`, req.Header.Get("X-Cart-Id"))
					assert.Exactly(t, `x-wl-uid=1vnTVF5WyZIe5Fymf2a4H+pFPyJa4wxNmzCKdImj1UqQPV5ecUs1sn46vDbGJUI+sE=`, req.Header.Get("Cookie"))

					return &http.Response{
						StatusCode: http.StatusOK,
						Header: http.Header{
							"Set-Cookie": []string{"ubid-acbde=253-9771841-6878311; Domain=.example.com; Expires=Sun, 28-Dec-2036 08:58:08 GMT; Path=/"},
						},
						Body: ioutil.NopCloser(bytes.NewBufferString(`Just a simple response`)),
					}
				},
				RequestCache: make(map[*http.Request]struct{}),
			},
		}

		rfa2 := new(backend.RequestFuncArgs)
		*rfa2 = *rfa
		rfa2.MaxBodySize = 3000

		hdr, content, err := backend.FetchHTTP(rfa2)
		assert.Exactly(t, `ubid-acbde=253-9771841-6878311; Domain=.example.com; Expires=Sun, 28-Dec-2036 08:58:08 GMT; Path=/`, hdr.Get("Set-cookie"), "set cookie")
		assert.Exactly(t, `Just a simple response`, string(content))
		assert.NoError(t, err)
	})

	t.Run("LimitedReader", func(t *testing.T) {
		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", nil),
		}

		rfa2 := new(backend.RequestFuncArgs)
		*rfa2 = *rfa
		rfa2.ReturnHeaders = nil

		hdr, content, err := backend.FetchHTTP(rfa2)
		assert.Nil(t, hdr, "Header")
		assert.Exactly(t, `A response long`, string(content))
		assert.NoError(t, err)
	})

	t.Run("Error Reading body", func(t *testing.T) {
		haveErr := errors.NewAlreadyClosedf("Brain already closed")
		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", haveErr),
		}

		hdr, content, err := backend.FetchHTTP(rfa)
		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content)
		assert.Contains(t, err.Error(), `Brain already closed`)
	})

	t.Run("Status Code not 200", func(t *testing.T) {

		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(204, "A response longer than 15 bytes", nil),
		}

		hdr, content, err := backend.FetchHTTP(rfa)
		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content)
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
	})

	t.Run("Request context deadline", func(t *testing.T) {
		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", errors.New("any weird error")),
		}

		rfa2 := new(backend.RequestFuncArgs)
		*rfa2 = *rfa

		rfa2.ReturnHeaders = nil
		rfa2.MaxBodySize = 300
		ctx, cancel := context.WithCancel(rfa2.ExternalReq.Context())
		rfa2.ExternalReq = rfa2.ExternalReq.WithContext(ctx)
		cancel()

		hdr, content, err := backend.FetchHTTP(rfa2)

		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content, "Content must be empty")
		assert.EqualError(t, errors.Cause(err), context.Canceled.Error())
	})

	t.Run("HTTP Client Timeout", func(t *testing.T) {
		t.Skip("Currently unsure how to test that. So TODO")
	})
}
