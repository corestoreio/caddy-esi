package backend

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/corestoreio/errors"
)

// ResourceRequestRegister functions handling the different protocols to resources.
// Also useful for testing.
var ResourceRequestRegister = map[string]ResourceRequestFunc{
	"http":  FetchHTTP,
	"https": FetchHTTP,
}

const (
	// DefaultMaxBodySize the body size of a reuqest which can be received from a
	// micro service.
	DefaultMaxBodySize int64 = 5 << 20 // 5 MB is a lot of text.
	// DefaultTimeOut time to wait until a request to a micro service gets marked as
	// failed.
	DefaultTimeOut = 30 * time.Second
)

// ResourceRequestFunc performs a request to a backend service via a specific
// protocol.
type ResourceRequestFunc func(url string, timeout time.Duration, maxBodySize int64) ([]byte, error)

// Resource specifies the location to a 3rd party remote system within an ESI
// tag. A resource attribute (src="") can occur n-times.
type Resource struct {
	// Index specifies the number of occurrence within the include tag to
	// allowing sorting and hence having a priority list.
	Index int
	// URL to a remote 3rd party service which gets used by http.Client OR the
	// URL contains an alias to a connection to a KeyValue server (defined in
	// the Caddyfile) to fetch a value via the field "Key" or "KeyTemplate".
	URL string
	// URLTemplate gets created when the URL contains the template identifier. Then
	// the URL field would be empty.
	URLTemplate *template.Template
	// IsURL set to true if the URL contains "://" and hence we must trigger
	// http.Client. If false we know that the URL field relates to a configured
	// resource in the Caddyfile, for example an alias to a Redis instance.
	IsURL bool
	// circuit breaker http://martinfowler.com/bliki/CircuitBreaker.html
	cbFailures        *uint64
	cbLastFailureTime *uint64 //  in UnixNano
}

// NewResource creates a new resource to one backend.
func NewResource(idx int, url string) *Resource {
	return &Resource{
		Index:             idx,
		IsURL:             strings.Contains(url, "://"),
		URL:               url,
		cbFailures:        new(uint64),
		cbLastFailureTime: new(uint64),
	}
}

// CBState declares the different states for the circuit breaker (CB)
const (
	CBStateOpen = iota + 1
	CBStateHalfOpen
	CBStateClosed
)

// MaxFailures maximum amount of failures before the circuit breaker is half
// open to try the next request.
var CBMaxFailures uint64 = 12

// CBThresholdCalc calculates the threshold how long the CB should wait until to set the HalfOpen state.
// Default implementation returns an exponentially calculated duration
var CBThresholdCalc = func(failures uint64) time.Duration {
	return time.Duration((1 << failures) * time.Second)
}

func (r *Resource) CBFailures() uint64 {
	return atomic.LoadUint64(r.cbFailures)
}

func (r *Resource) CBState() (state int, lastFailure time.Time) {
	var thresholdPassed bool

	failures := atomic.LoadUint64(r.cbFailures)
	lastFailed := int64(atomic.LoadUint64(r.cbLastFailureTime))
	// increment the lastFailed with an exponential time out
	lastFailed += CBThresholdCalc(failures).Nanoseconds()

	secs := lastFailed / int64(time.Second)
	tn := time.Now()
	if secs > 0 {
		lastFailure = time.Unix(secs, lastFailed%secs)
		// only when the current time is in the future of the lastFailure then the
		// circuit breaker is half open.
		thresholdPassed = tn.After(lastFailure)
	}

	switch {
	case failures >= CBMaxFailures && thresholdPassed:
		state = CBStateHalfOpen
	case failures >= CBMaxFailures:
		state = CBStateOpen
	default:
		state = CBStateClosed
	}
	return state, lastFailure
}

func (r *Resource) CBReset() {
	atomic.StoreUint64(r.cbLastFailureTime, 0)
	atomic.StoreUint64(r.cbFailures, 0)
}

func (r *Resource) CBRecordFailure() (failedUnixNano int64) {
	atomic.AddUint64(r.cbFailures, 1)
	failedUnixNano = time.Now().UnixNano()
	atomic.StoreUint64(r.cbLastFailureTime, uint64(failedUnixNano))
	return failedUnixNano
}

// DefaultClientTransport our own transport for all ESI tag resources instead of
// relying on net/http.DefaultTransport. This transport gets also mocked for
// tests.
var DefaultClientTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// TestClient mocked out for testing
var TestClient *http.Client

var httpClientPool = &sync.Pool{
	New: func() interface{} {
		return &http.Client{
			Transport: DefaultClientTransport,
			Timeout:   DefaultTimeOut,
		}
	},
}

func newHttpClient() *http.Client {
	return httpClientPool.Get().(*http.Client)
}

func putHttpClient(c *http.Client) {
	c.Timeout = DefaultTimeOut
	httpClientPool.Put(c)
}

// FetchHTTP implements ResourceRequestFunc and is registered in variable
// ResourceRequestRegister.
func FetchHTTP(url string, timeout time.Duration, maxBodySize int64) ([]byte, error) {
	var c = TestClient
	if c == nil {
		c = newHttpClient()
		defer putHttpClient(c)
	}
	if timeout < 1 {
		timeout = DefaultTimeOut
	}
	c.Timeout = timeout

	resp, err := c.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "[esitag] FetchHTTP error for URL %q", url)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, maxBodySize))
	// todo log or report when we reach EOF to let the admin know that the content is too large.
	_ = resp.Body.Close() // for now ignore it ...
	if err != nil && err != io.EOF {
		return nil, errors.Wrapf(err, "[esitag] FetchHTTP.ReadFrom Body for URL %q failed", url)
	}
	return buf.Bytes(), nil
}
