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

package esitesting

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
)

// HTTPTrip used for mocking the Transport field in http.Client.
type HTTPTrip struct {
	GenerateResponse func(*http.Request) *http.Response
	Err              error
	sync.Mutex
	RequestCache map[*http.Request]struct{}
}

// NewHTTPTrip creates a new http.RoundTripper
func NewHTTPTrip(code int, body string, err error) *HTTPTrip {
	return &HTTPTrip{
		// use buffer pool
		GenerateResponse: func(_ *http.Request) *http.Response {
			return &http.Response{
				StatusCode: code,
				Body:       ioutil.NopCloser(bytes.NewBufferString(body)),
			}
		},
		Err:          err,
		RequestCache: make(map[*http.Request]struct{}),
	}
}

// NewHTTPTripBytes creates a new http.RoundTripper but body is a byte slice. In
// benchmarks this performs better with large bodies.
func NewHTTPTripBytes(code int, body []byte, err error) *HTTPTrip {
	return &HTTPTrip{
		// use buffer pool
		GenerateResponse: func(_ *http.Request) *http.Response {
			return &http.Response{
				StatusCode: code,
				Body:       ioutil.NopCloser(bytes.NewReader(body)),
			}
		},
		Err:          err,
		RequestCache: make(map[*http.Request]struct{}),
	}
}

// RoundTrip implements http.RoundTripper and adds the Request to the
// field Req for later inspection.
func (tp *HTTPTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	tp.Mutex.Lock()
	defer tp.Mutex.Unlock()
	tp.RequestCache[r] = struct{}{}

	if tp.Err != nil {
		return nil, tp.Err
	}
	return tp.GenerateResponse(r), nil
}

// RequestsMatchAll checks if all requests in the cache matches the predicate
// function f.
func (tp *HTTPTrip) RequestsMatchAll(t interface {
	Errorf(format string, args ...interface{})
}, f func(*http.Request) bool) {
	tp.Mutex.Lock()
	defer tp.Mutex.Unlock()

	for req := range tp.RequestCache {
		if !f(req) {
			t.Errorf("[cstesting] Request does not match predicate f: %#v", req)
		}
	}
}

// RequestsCount counts the requests in the cache and compares it with your
// expected value.
func (tp *HTTPTrip) RequestsCount(t interface {
	Errorf(format string, args ...interface{})
}, expected int) {
	tp.Mutex.Lock()
	defer tp.Mutex.Unlock()

	if have, want := len(tp.RequestCache), expected; have != want {
		t.Errorf("RequestsCount: Have %d vs. Want %d", have, want)
	}
}
