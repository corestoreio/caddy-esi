package backend

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/SchumacherFM/mailout/bufpool"
	"github.com/corestoreio/errors"
	"github.com/dustin/go-humanize"
)

// TemplateIdentifier if some strings contain these characters then a
// template.Template will be created. For now a resource key or an URL is
// supported.
const TemplateIdentifier = "{{"

var rrfRegister = &struct {
	sync.RWMutex
	fetchers map[string]RequestFunc
}{
	fetchers: make(map[string]RequestFunc),
}

// RegisterRequestFunc scheme is a protocol before the ://. This function
// returns a closure which lets you deregister the scheme once a test has
// finished. Use the defer word.
func RegisterRequestFunc(scheme string, f RequestFunc) struct{ DeferredDeregister func() } {
	rrfRegister.Lock()
	defer rrfRegister.Unlock()
	rrfRegister.fetchers[scheme] = f
	return struct {
		DeferredDeregister func()
	}{
		DeferredDeregister: func() { DeregisterRequestFunc(scheme) },
	}
}

// DeregisterRequestFunc removes a previously registered scheme
func DeregisterRequestFunc(scheme string) {
	rrfRegister.Lock()
	defer rrfRegister.Unlock()
	delete(rrfRegister.fetchers, scheme)
}

// lookupRequestFunc if ok sets to true the rf cannot be nil.
func lookupRequestFunc(scheme string) (rf RequestFunc, ok bool) {
	rrfRegister.RLock()
	defer rrfRegister.RUnlock()
	rf, ok = rrfRegister.fetchers[scheme]
	return
}

// RequestFunc performs a request to a backend service via a specific
// protocol.
type RequestFunc func(url string, timeout time.Duration, maxBodySize uint64) ([]byte, error)

// MockRequestContent for testing purposes only.
func MockRequestContent(content string) RequestFunc {
	return func(url string, timeout time.Duration, maxBodySize uint64) ([]byte, error) {
		return []byte(fmt.Sprintf("%s %q Timeout %s MaxBody %s", content, url, timeout, humanize.Bytes(maxBodySize))), nil
	}
}

// MockRequestError for testing purposes only.
func MockRequestError(err error) RequestFunc {
	return func(url string, timeout time.Duration, maxBodySize uint64) ([]byte, error) {
		return nil, err
	}
}

// Resource specifies the location to a 3rd party remote system within an ESI
// tag. A resource attribute (src="") can occur n-times.
type Resource struct {
	// Index specifies the number of occurrence within the include tag to
	// allowing sorting and hence having a priority list.
	Index int
	// URL to a remote 3rd party service which gets used by http.Client OR the
	// URL contains an alias to a connection to a KeyValue server (defined in
	// the Caddyfile) to fetch a value via the field "Key" or "KeyTemplate".
	url string
	// URLTemplate gets created when the URL contains the template identifier. Then
	// the URL field would be empty.
	urlTemplate *template.Template
	// IsURL set to true if the URL contains "://" and hence we must trigger
	// http.Client. If false we know that the URL field relates to a configured
	// resource in the Caddyfile, for example an alias to a Redis instance.
	isURL bool

	// RequestFunc performs a request to a backend service via a specific
	// protocol.
	rf RequestFunc
	// circuit breaker http://martinfowler.com/bliki/CircuitBreaker.html
	cbFailures        *uint64
	cbLastFailureTime *uint64 //  in UnixNano
}

// MustNewResource same as NewResource but panics on error.
func MustNewResource(idx int, url string) *Resource {
	r, err := NewResource(idx, url)
	if err != nil {
		panic(err)
	}
	return r
}

// NewResource creates a new resource to one backend. Inspects the URL if it
// contains a template and parses that template. Looks also up the HTTP Fetcher
// function depending on the scheme.
func NewResource(idx int, url string) (*Resource, error) {
	r := &Resource{
		Index:             idx,
		isURL:             strings.Contains(url, "://"),
		url:               url,
		cbFailures:        new(uint64),
		cbLastFailureTime: new(uint64),
	}

	if r.isURL && strings.Contains(r.url, TemplateIdentifier) {
		var err error
		r.urlTemplate, err = template.New("resource_tpl").Parse(r.url)
		if err != nil {
			return nil, errors.NewFatalf("[esibackend] NewResource. Failed to parse (Index %d) %q as template with error: %s", idx, r.url, err)
		}
	}

	if pos := strings.Index(r.url, "://"); pos > 1 {
		scheme := strings.ToLower(r.url[:pos])
		var ok bool
		r.rf, ok = lookupRequestFunc(scheme)
		if !ok {
			return nil, errors.NewNotSupportedf("[esibackend] NewResource protocal %q not yet supported in URL %q", scheme, r.url)
		}
	}

	return r, nil
}

// String returns a resource identifier, most likely the underlying URL and the
// template name, if defined.
func (r *Resource) String() string {
	tplName := ""
	if r.urlTemplate != nil {
		tplName = " Template: " + r.urlTemplate.ParseName
	}
	return fmt.Sprintf("%s%s", r.url, tplName)
}

// DoRequest performs the request to the backend. Uses externalReq as data when
// rendering the URL template.
func (r *Resource) DoRequest(externalReq *http.Request, timeout time.Duration, maxBodySize uint64) ([]byte, error) {
	url := r.url
	if r.urlTemplate != nil {
		buf := bufpool.Get()
		defer bufpool.Put(buf)

		if err := r.urlTemplate.Execute(buf, struct {
			// These are the currently available template variables. which is only "r" for
			// the request object.
			r *http.Request
		}{
			r: externalReq,
		}); err != nil {
			return nil, errors.NewTemporaryf("[esitag] Resource %q Template error: %s\nContent: %s", r.String(), err, buf)
		}
		url = buf.String()
	}
	return r.rf(url, timeout, maxBodySize)
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
