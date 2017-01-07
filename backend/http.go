// Copyright 2016-2017, Cyrill @ Schumacher.fm and the CaddyESI Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

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
	f := NewFetchHTTP(DefaultClientTransport)
	RegisterResourceHandler("http", f)
	RegisterResourceHandler("https", f)
}

// DefaultClientTransport our own transport for all ESI tag resources instead of
// relying on net/http.DefaultTransport. This transport gets also mocked for
// tests. Only used in init(), see above.
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

// NewFetchHTTP creates a new HTTP/S backend fetcher which lives the whole
// application running time. Thread safe.
func NewFetchHTTP(tr http.RoundTripper) *fetchHTTP {
	f := &fetchHTTP{
		clientPool: sync.Pool{
			New: func() interface{} {
				return &http.Client{
					Transport: tr,
				}
			},
		},
	}
	return f
}

type fetchHTTP struct {
	clientPool sync.Pool
}

func (fh *fetchHTTP) newHTTPClient() *http.Client {
	return fh.clientPool.Get().(*http.Client)
}

func (fh *fetchHTTP) putHTTPClient(c *http.Client) {
	fh.clientPool.Put(c)
}

// DoRequest implements ResourceHandler and is registered in RegisterResourceHandler for
// http and https scheme. The only allowed response code from the queried server
// is http.StatusOK. All other response codes trigger a NotSupported error
// behaviour.
func (fh *fetchHTTP) DoRequest(args *ResourceArgs) (http.Header, []byte, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchHTTP.args.Validate")
	}

	c := fh.newHTTPClient()
	defer fh.putHTTPClient(c)

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

// Close noop function
func (fh *fetchHTTP) Close() error {
	return nil
}
