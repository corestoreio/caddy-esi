package backend

import (
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"
)

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

// LookupRequestFunc if ok sets to true the rf cannot be nil.
func LookupRequestFunc(scheme string) (rf RequestFunc, ok bool) {
	rrfRegister.RLock()
	defer rrfRegister.RUnlock()
	rf, ok = rrfRegister.fetchers[scheme]
	return
}

// RequestFunc performs a request to a backend service via a specific
// protocol.
type RequestFunc func(url string, timeout time.Duration, maxBodySize int64) ([]byte, error)

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
