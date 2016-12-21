package esitag

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"text/template"
	"time"

	"strings"

	"github.com/pkg/errors"
)

const DefaultTimeOut = 30 * time.Second

// MaxBackOffs allow up to (1<<12)/60 minutes (68min) of back off time
const MaxBackOffs = 12

// DefaultClientTransport our own transport for all ESI tag resources instead of
// relying on net/http.DefaultTransport.
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
	// Key defines a key in a KeyValue server to fetch the value from.
	Key string
	// KeyTemplate gets created when the Key field contains the template
	// identifier. Then the Key field would be empty.
	KeyTemplate *template.Template
	// backOff exponentially calculated
	backOff    uint
	backedOff  int // number of calls to continue
	lastFailed time.Time
}

func NewResource() *Resource {
	return &Resource{}
}

func (r *Resource) applyKey(val string) error {
	r.Key = val
	if strings.Contains(val, TemplateIdentifier) {
		var err error
		r.KeyTemplate, err = template.New("resource_tpl").Parse(val)
		if err != nil {
			return errors.Errorf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nResource: %#v", val, err, r)
		}
		r.Key = "" // unset Key because we have a template
	}
	return nil
}

// Resources contains multiple unique Resource entries, aka backend systems
// likes redis instances. Resources occur within one single ESI tag. The
// resource attribute (src="") can occurr multiple times.
type Resources struct {
	Logf   func(format string, v ...interface{})
	Client *http.Client
	Items  []*Resource
}

func (r Resources) initClient() {
	if r.Client != nil {
		return
	}
	r.Client = &http.Client{
		Transport: DefaultClientTransport,
		Timeout:   DefaultTimeOut,
	}
}

// DoRequest iterates over the resources and executes http requests. If one
// resource fails it will be marked as timed out and the next resource gets
// tried. The exponential back-off stops when MaxBackOffs have been reached and
// then tries again.
func (rs Resources) DoRequest(timeout time.Duration, externalReq *http.Request) ([]byte, error) {
	rs.initClient()

	if timeout < 1 {
		timeout = DefaultTimeOut
	}
	rs.Client.Timeout = timeout

	for i, r := range rs.Items {

		url := r.URL
		if r.URLTemplate != nil {
			var buf bytes.Buffer
			if err := r.URLTemplate.Execute(&buf, externalReq); err != nil {
				return nil, errors.Wrapf(err, "[esitag] Index %d Resource %#v Template error", i, r)
			}
			url = buf.String()
		}

		if r.lastFailed.After(time.Now()) {
			rs.Logf("[esitag] Index (%d/%d) DoRequest for URL %q Skipping due to previous errors", i, len(rs.Items), url)
			if r.backedOff >= MaxBackOffs {
				rs.Logf("[esitag] Index (%d/%d) DoRequest restarted for URL %q", i, len(rs.Items), url)
				r.lastFailed = time.Time{} // restart
			}
			continue
		}

		resp, err := rs.Client.Get(url)
		if err != nil {
			rs.Logf("[esitag] Index (%d/%d) DoRequest for URL %q failed. Continuing", i, len(rs.Items), url)
			r.backOff++
			var dur time.Duration = 1 << r.backOff // exponentially calculated
			r.lastFailed = time.Now().Add(dur * time.Second)
			continue
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "[esitag] Index %d DoRequest.ioutil.ReadAll for URL %q failed", i, url)
		}

		return data, nil
	}

	return nil, errors.Errorf("[esitag] Should maybe not happen? TODO investigate")
}
