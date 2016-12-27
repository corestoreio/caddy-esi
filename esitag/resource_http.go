package esitag

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/corestoreio/errors"
)

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

// FetchHTTP implements ResourceRequestFunc
func FetchHTTP(url string, timeout time.Duration, maxBodySize int64) ([]byte, error) {
	c := newHttpClient()
	defer putHttpClient(c)

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
