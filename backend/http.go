package backend

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/corestoreio/errors"
)

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
		}
	},
}

func newHttpClient() *http.Client {
	return httpClientPool.Get().(*http.Client)
}

func putHttpClient(c *http.Client) {
	httpClientPool.Put(c)
}

// FetchHTTP implements RequestFunc and is registered in
// RegisterRequestFunc for http and https scheme.
func FetchHTTP(args RequestFuncArgs) (http.Header, []byte, error) {
	var c = TestClient
	if c == nil {
		c = newHttpClient()
		defer putHttpClient(c)
	}
	if args.Timeout < 1 {
		return nil, nil, errors.NewEmptyf("[esibackend] For FetchHTTP %q the timeout value is empty", args.URL)
	}
	if args.MaxBodySize == 0 {
		return nil, nil, errors.NewEmptyf("[esibackend] For FetchHTTP %q the maxBodySize value is empty", args.URL)
	}
	c.Timeout = args.Timeout

	req, err := http.NewRequest("GET", args.URL, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "[esibackend] Failed NewRequest for %q", args.URL)
	}

	for hdr, i := args.PrepareForwardHeaders(), 0; i < len(hdr); i = i + 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchHTTP error for URL %q", args.URL)
	}

	// TODO(CyS) return headers

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, int64(args.MaxBodySize))) // overflow of uint into int ?
	// todo log or report when we reach EOF to let the admin know that the content is too large has been cut off.
	_ = resp.Body.Close() // for now ignore it ...
	if err != nil && err != io.EOF {
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchHTTP.ReadFrom Body for URL %q failed", args.URL)
	}
	return nil, buf.Bytes(), nil
}
