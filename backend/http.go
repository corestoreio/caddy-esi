package backend

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// DefaultHTTPTimeOut time to wait until a request to a micro service gets marked as
// failed.
const DefaultHTTPTimeOut = 20 * time.Second

func init() {
	RegisterRequestFunc("http", FetchHTTP)
	RegisterRequestFunc("https", FetchHTTP)
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
			Timeout:   DefaultHTTPTimeOut,
		}
	},
}

func newHttpClient() *http.Client {
	return httpClientPool.Get().(*http.Client)
}

func putHttpClient(c *http.Client) {
	c.Timeout = DefaultHTTPTimeOut
	httpClientPool.Put(c)
}

// FetchHTTP implements RequestFunc and is registered in
// RegisterRequestFunc for http and https scheme.
func FetchHTTP(url string, timeout time.Duration, maxBodySize int64) ([]byte, error) {
	var c = TestClient
	if c == nil {
		c = newHttpClient()
		defer putHttpClient(c)
	}
	if timeout < 1 {
		timeout = DefaultHTTPTimeOut
	}
	c.Timeout = timeout

	resp, err := c.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "[esibackend] FetchHTTP error for URL %q", url)
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, maxBodySize))
	// todo log or report when we reach EOF to let the admin know that the content is too large.
	_ = resp.Body.Close() // for now ignore it ...
	if err != nil && err != io.EOF {
		return nil, errors.Wrapf(err, "[esibackend] FetchHTTP.ReadFrom Body for URL %q failed", url)
	}
	return buf.Bytes(), nil
}
