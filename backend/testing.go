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
	"fmt"
	"net/http"

	"github.com/corestoreio/errors"
)

func init() {
	RegisterResourceHandlerFactory("mockTimeout", func(cfg *ConfigItem) (ResourceHandler, error) {
		return ResourceMock{
			DoRequestFn: func(*ResourceArgs) (_ http.Header, content []byte, err error) {
				// mockTimeout://duration
				return nil, nil, errors.NewTimeoutf("[backend] Timeout after %q", cfg.URL)
			},
		}, nil
	})
}

const mockRequestMsg = "%s %q Timeout %s MaxBody %s"

// ResourceMock exported for testing
type ResourceMock struct {
	DoRequestFn func(args *ResourceArgs) (http.Header, []byte, error)
	CloseFn     func() error
}

// DoRequest calls DoRequestFn
func (rm ResourceMock) DoRequest(a *ResourceArgs) (http.Header, []byte, error) {
	return rm.DoRequestFn(a)
}

// Close returns nil if CloseFn is nil otherwise calls CloseFn
func (rm ResourceMock) Close() error {
	if rm.CloseFn == nil {
		return nil
	}
	return rm.CloseFn()
}

// MockRequestContent for testing purposes only.
func MockRequestContent(content string) ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(args *ResourceArgs) (http.Header, []byte, error) {
			if args.URL == "" && args.Key == "" {
				panic(fmt.Sprintf("[esibackend] URL and Key cannot be empty: %#v\n", args))
			}
			return nil, []byte(fmt.Sprintf(mockRequestMsg, content, args.URL, args.Timeout, args.MaxBodySizeHumanized())), nil
		},
	}
}

// MockRequestContentCB for testing purposes only. Call back gets executed
// before the function returns.
func MockRequestContentCB(content string, callback func() error) ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(args *ResourceArgs) (http.Header, []byte, error) {
			if err := callback(); err != nil {
				return nil, nil, errors.Wrapf(err, "MockRequestContentCB with URL %q", args.URL)
			}
			return nil, []byte(fmt.Sprintf(mockRequestMsg, content, args.URL, args.Timeout, args.MaxBodySizeHumanized())), nil
		},
	}
}

// MockRequestError for testing purposes only.
func MockRequestError(err error) ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(_ *ResourceArgs) (http.Header, []byte, error) {
			return nil, nil, err
		},
	}
}

// MockRequestPanic just panics
func MockRequestPanic(msg interface{}) ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(_ *ResourceArgs) (http.Header, []byte, error) {
			panic(msg)
		},
	}
}
