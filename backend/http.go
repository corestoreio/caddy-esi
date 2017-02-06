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
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	loghttp "github.com/corestoreio/log/http"
)

func init() {
	f := NewFetchHTTP(DefaultHTTPTransport)
	RegisterResourceHandler("http", f)
	RegisterResourceHandler("https", f)
}

// DefaultHTTPTransport our own transport for all ESI tag resources instead of
// relying on net/http.DefaultTransport. This transport gets also mocked for
// tests. Only used in init(), see above.
var DefaultHTTPTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   DefaultTimeOut,
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
func NewFetchHTTP(tr http.RoundTripper) ResourceHandler {
	f := &fetchHTTP{
		client: &http.Client{
			Transport: tr,
			Timeout:   DefaultTimeOut,
		},
	}
	return f
}

// requestCanceller implemented in http.Transport
type requestCanceller interface {
	CancelRequest(req *http.Request)
}

type fetchHTTP struct {
	client *http.Client
}

// DoRequest implements ResourceHandler and is registered in RegisterResourceHandler for
// http and https scheme. The only allowed response code from the queried server
// is http.StatusOK. All other response codes trigger a NotSupported error
// behaviour.
func (fh *fetchHTTP) DoRequest(args *ResourceArgs) (http.Header, []byte, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchHTTP.args.Validate")
	}

	// TODO(CyS) external POST requests or GET with query string should forward
	// this data. So the http.NewRequest should then change to POST if the
	// configuration for this specific ESI tag allows it.

	req, err := http.NewRequest("GET", args.URL, nil)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] Failed NewRequest for %q", args.URL)
	}

	for hdr, i := args.PrepareForwardHeaders(), 0; i < len(hdr); i = i + 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}

	// do we overwrite here the Timeout from args.ExternalReq ? or just adding our
	// own timeout?
	ctx, cancel := context.WithTimeout(args.ExternalReq.Context(), args.Timeout)
	defer cancel()

	resp, err := fh.client.Do(req.WithContext(ctx))
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	// If we got an error, and the context has been canceled,
	// the context's error is probably more useful.
	if err != nil {
		select {
		case <-ctx.Done():
			if cncl, ok := fh.client.Transport.(requestCanceller); ok {
				if args.Log.IsInfo() {
					args.Log.Info("esibackend.FetchHTTP.DoRequest.client.Transport.requestCanceller",
						log.String("url", args.URL), loghttp.Request("backend_request", req),
					)
				}
				cncl.CancelRequest(req)
			}
			err = errors.Wrap(ctx.Err(), "[esibackend] Context Done")
		default:
		}
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchHTTP error for URL %q", args.URL)
	}

	if resp.StatusCode != http.StatusOK { // this can be made configurable in an ESI tag
		return nil, nil, errors.NewNotSupportedf("[backend] FetchHTTP: Response Code %q not supported for URL %q", resp.StatusCode, args.URL)
	}

	// not yet worth to put the resp.Body reader into its own goroutine

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

	//buf := new(bytes.Buffer)       // no pool possible
	//mbs := int64(args.MaxBodySize) // overflow of uint into int ?
	//
	//done := make(chan struct{})
	//go func() {
	//	var n int64
	//	n, err = buf.ReadFrom(io.LimitReader(resp.Body, mbs))
	//	if err != nil && err != io.EOF {
	//		err = errors.Wrapf(err, "[esibackend] FetchHTTP.ReadFrom Body for URL %q failed", args.URL)
	//	}
	//	if n >= mbs && args.Log != nil && args.Log.IsInfo() { // body has been cut off
	//		args.Log.Info("esibackend.FetchHTTP.LimitReader",
	//			log.String("url", args.URL), log.Int64("bytes_read", n), log.Int64("bytes_max_read", mbs),
	//		)
	//	}
	//
	//	done <- struct{}{}
	//}()
	//<-done

	return args.PrepareReturnHeaders(resp.Header), buf.Bytes(), nil
}

// Close noop function
func (fh *fetchHTTP) Close() error {
	return nil
}
