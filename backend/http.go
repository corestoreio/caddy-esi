package backend

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
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

func newHTTPClient() *http.Client {
	return httpClientPool.Get().(*http.Client)
}

func putHTTPClient(c *http.Client) {
	httpClientPool.Put(c)
}

// FetchHTTP implements RequestFunc and is registered in RegisterRequestFunc for
// http and https scheme. The only allowed response code from the queried server
// is http.StatusOK. All other response codes trigger a NotSupported error
// behaviour.
func FetchHTTP(args *RequestFuncArgs) (http.Header, []byte, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchHTTP.args.Validate")
	}

	var c = TestClient
	if c == nil {
		c = newHTTPClient()
		defer putHTTPClient(c)
	}

	c.Timeout = args.Timeout

	req, err := http.NewRequest("GET", args.URL, nil)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] Failed NewRequest for %q", args.URL)
	}

	for hdr, i := args.PrepareForwardHeaders(), 0; i < len(hdr); i = i + 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}

	ctx := args.ExternalReq.Context()
	resp, err := c.Do(req.WithContext(ctx))
	// If we got an error, and the context has been canceled,
	// the context's error is probably more useful.
	if err != nil {
		select {
		case <-ctx.Done():
			err = errors.Wrap(ctx.Err(), "[esibackend] Context Done")
		default:
		}
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchHTTP error for URL %q", args.URL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK { // this can be made configurable in an ESI tag
		return nil, nil, errors.NewNotSupportedf("[backend] FetchHTTP: Response Code %q not supported for URL %q", resp.StatusCode, args.URL)
	}

	buf := new(bytes.Buffer)
	mbs := int64(args.MaxBodySize) // overflow of uint into int ?
	n, err := buf.ReadFrom(io.LimitReader(resp.Body, mbs))
	if err != nil && err != io.EOF {
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchHTTP.ReadFrom Body for URL %q failed", args.URL)
	}

	if n >= mbs && args.Log != nil && args.Log.IsInfo() { // body has been cut off
		args.Log.Info("esibackend.FetchHTTP.LimitReader",
			log.String("url", args.URL), log.Int64("bytes_read", n), log.Int64("bytes_max_read", mbs),
		)
	}

	return args.PrepareReturnHeaders(resp.Header), buf.Bytes(), nil
}
