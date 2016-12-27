package esitag

import (
	"net"
	"net/http"
	"text/template"
	"time"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
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
	Log         log.Logger
	// Items define multiple URLs to different resources but all share the same
	// protocol. For example you have 3 shopping cart micro services which would
	// be coded as three src="" attributes in the ESI tag. Those three resources
	// will be used as a fall back whenever the previous queried resource
	// failed.
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

		var lFields log.Fields
		now := time.Now()
		if rs.Log.IsDebug() {
			lFields = log.Fields{log.Int("bacled_off", r.backedOff),
				log.Int("index", i), log.Int("items_length", len(rs.Items)), log.String("url", url), log.Time("last_failed", r.lastFailed), log.Time("now", now),
			}
		}

		if r.lastFailed.After(now) {
			if rs.Log.IsDebug() {
				rs.Log.Debug("esitag.Resources.DoRequest.lastFailed.After", lFields...)
			}
			if r.backedOff >= MaxBackOffs {
				if rs.Log.IsDebug() {
					rs.Log.Debug("esitag.Resources.DoRequest.backedOff", lFields...)
				}
				r.lastFailed = time.Time{} // restart
			}
			continue
		}

		data, err := rs.ResourceRequestFunc(url, timeout, maxBodySize)
		if err != nil {
			if rs.Log.IsDebug() {
				rs.Log.Debug("esitag.Resources.DoRequest.ResourceRequestFunc", log.Err(err), lFields.Add())
			}
			r.backOff++
			var dur time.Duration = 1 << r.backOff // exponentially calculated
			r.lastFailed = time.Now().Add(dur * time.Second)
			continue
		}

		return data, nil
	}

	return nil, errors.Errorf("[esitag] Should maybe not happen? TODO investigate")
}
