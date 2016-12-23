package esitag

import (
	"net"
	"net/http"
	"text/template"
	"time"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/corestoreio/errors"
)

const (
	DefaultMaxBodySize int64 = 5 << 20 // 5 MB is a lot of text.
	DefaultTimeOut           = 30 * time.Second
)

// MaxBackOffs allow up to (1<<12)/60 minutes (68min) of back off time
const MaxBackOffs = 12

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
	// backOff exponentially calculated
	backOff    uint
	backedOff  int // number of calls to continue
	lastFailed time.Time
}

// Resources contains multiple unique Resource entries, aka backend systems
// likes redis instances. Resources occur within one single ESI tag. The
// resource attribute (src="") can occur multiple times. The first item which
// successfully returns data gets its content used in the response. If one item
// fails and we have multiple resources, the next resource gets tried.
type Resources struct {
	ResourceRequestFunc
	MaxBodySize int64 // DefaultMaxBodySize; TODO(CyS) implement in ESI tag
	Logf        func(format string, v ...interface{})
	// Items multiple URLs to different resources but all share the same protocol.
	Items []*Resource
}

// DoRequest iterates over the resources and executes http requests. If one
// resource fails it will be marked as timed out and the next resource gets
// tried. The exponential back-off stops when MaxBackOffs have been reached and
// then tries again.
func (rs *Resources) DoRequest(timeout time.Duration, externalReq *http.Request) ([]byte, error) {

	maxBodySize := DefaultMaxBodySize
	if rs.MaxBodySize > 0 {
		maxBodySize = rs.MaxBodySize
	}

	for i, r := range rs.Items {

		url := r.URL
		if r.URLTemplate != nil {
			buf := bufpool.Get()
			if err := r.URLTemplate.Execute(buf, externalReq); err != nil {
				bufpool.Put(buf)
				return nil, errors.Wrapf(err, "[esitag] Index %d Resource %#v Template error", i, r)
			}
			url = buf.String()
			bufpool.Put(buf)
		}

		if r.lastFailed.After(time.Now()) {
			rs.Logf("[esitag] Index (%d/%d) DoRequest for URL %q Skipping due to previous errors", i, len(rs.Items), url)
			if r.backedOff >= MaxBackOffs {
				rs.Logf("[esitag] Index (%d/%d) DoRequest restarted for URL %q", i, len(rs.Items), url)
				r.lastFailed = time.Time{} // restart
			}
			continue
		}

		data, err := rs.ResourceRequestFunc(url, timeout, maxBodySize)
		if err != nil {
			rs.Logf("[esitag] Index (%d/%d) DoRequest for URL %q failed. Continuing", i, len(rs.Items), url)
			r.backOff++
			var dur time.Duration = 1 << r.backOff // exponentially calculated
			r.lastFailed = time.Now().Add(dur * time.Second)
			continue
		}

		return data, nil
	}

	return nil, errors.Errorf("[esitag] Should maybe not happen? TODO investigate")
}
