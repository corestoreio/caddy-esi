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
