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

package backend_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sync"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewFetchHTTP_Serial(t *testing.T) {
	t.Parallel()

	rfa := &backend.ResourceArgs{
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

		rfa2 := new(backend.ResourceArgs)
		*rfa2 = *rfa
		rfa2.MaxBodySize = 3000

		hdr, content, err := backend.NewFetchHTTP(&esitesting.HTTPTrip{
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
		}).DoRequest(rfa2)
		assert.Exactly(t, `ubid-acbde=253-9771841-6878311; Domain=.example.com; Expires=Sun, 28-Dec-2036 08:58:08 GMT; Path=/`, hdr.Get("Set-cookie"), "set cookie")
		assert.Exactly(t, `Just a simple response`, string(content))
		assert.NoError(t, err)
	})

	t.Run("LimitedReader", func(t *testing.T) {

		rfa2 := new(backend.ResourceArgs)
		*rfa2 = *rfa
		rfa2.ReturnHeaders = nil

		hdr, content, err := backend.NewFetchHTTP(esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", nil)).DoRequest(rfa2)
		assert.Nil(t, hdr, "Header")
		assert.Exactly(t, `A response long`, string(content))
		assert.NoError(t, err)
	})

	t.Run("Error Reading body", func(t *testing.T) {
		haveErr := errors.NewAlreadyClosedf("Brain already closed")

		hdr, content, err := backend.NewFetchHTTP(esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", haveErr)).DoRequest(rfa)
		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content)
		assert.Contains(t, err.Error(), `Brain already closed`)
	})

	t.Run("Status Code not 200", func(t *testing.T) {

		hdr, content, err := backend.NewFetchHTTP(esitesting.NewHTTPTrip(204, "A response longer than 15 bytes", nil)).DoRequest(rfa)
		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content)
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
	})

	t.Run("Request context cancel", func(t *testing.T) {

		rfa2 := new(backend.ResourceArgs)
		*rfa2 = *rfa

		rfa2.ReturnHeaders = nil
		rfa2.MaxBodySize = 300
		ctx, cancel := context.WithCancel(rfa2.ExternalReq.Context())
		rfa2.ExternalReq = rfa2.ExternalReq.WithContext(ctx)
		cancel()

		hdr, content, err := backend.NewFetchHTTP(esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", errors.New("any weird error"))).DoRequest(rfa2)

		assert.Nil(t, hdr, "Header")
		assert.Empty(t, content, "Content must be empty")
		assert.EqualError(t, errors.Cause(err), context.Canceled.Error())
	})

	t.Run("HTTP Client Context Timeout", func(t *testing.T) {

		t.Skip("Seems somehow not possible to test :-(((")

		//rfa2 := new(backend.ResourceArgs)
		//*rfa2 = *rfa
		//
		//rfa2.ReturnHeaders = nil
		//rfa2.MaxBodySize = 300
		//rfa2.Timeout = 10 * time.Millisecond
		//
		//hdr, content, err := backend.NewFetchHTTP(&esitesting.HTTPTrip{
		//	GenerateResponse: func(req *http.Request) *http.Response {
		//
		//		time.Sleep(400 * time.Millisecond)
		//
		//		return &http.Response{
		//			StatusCode: http.StatusOK,
		//			Header:     http.Header{},
		//			Body:       ioutil.NopCloser(bytes.NewBufferString(`Just a simple response`)),
		//		}
		//	},
		//	RequestCache: make(map[*http.Request]struct{}),
		//}).DoRequest(rfa2)
		//
		//assert.Nil(tb, hdr, "Header")
		//assert.Empty(tb, string(content), "Content must be empty")
		//assert.EqualError(tb, errors.Cause(err), context.DeadlineExceeded.Error())

	})
}

func TestNewFetchHTTP_Parallel(t *testing.T) {
	t.Parallel()

	wantContent := []byte(`Moral of the story: even insane-looking problems are sometimes r€al.`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(wantContent)
	}))
	defer srv.Close()

	fh := backend.NewFetchHTTP(backend.DefaultHTTPTransport)

	const iterations = 10
	var wg sync.WaitGroup
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			hdr, content, err := fh.DoRequest(&backend.ResourceArgs{
				ExternalReq: getExternalReqWithExtendedHeaders(),
				URL:         srv.URL,
				Timeout:     time.Second,
				MaxBodySize: 300,
			})
			if err != nil {
				t.Fatalf("%+v", err)
			}
			if !bytes.Equal(content, wantContent) {
				t.Fatalf("Want %q\nHave %q", wantContent, content)
			}
			if hdr != nil {
				t.Fatal("Header should be nil")
			}

		}(&wg)
	}
	wg.Wait()
}
