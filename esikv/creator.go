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

package esikv

import (
	"net/http"
	"strings"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
)

type resourceMock struct {
	DoRequestFn func(args *backend.ResourceArgs) (http.Header, []byte, error)
	CloseFn     func() error
}

func (rm resourceMock) DoRequest(a *backend.ResourceArgs) (http.Header, []byte, error) {
	return rm.DoRequestFn(a)
}

func (rm resourceMock) Close() error {
	if rm.CloseFn == nil {
		return nil
	}
	return rm.CloseFn()
}

// NewResourceHandler a given URL gets checked which service it should instantiate
// and connect to. Supported schemes: redis:// for now.
func NewResourceHandler(cfg *ConfigItem) (backend.ResourceHandler, error) {
	idx := strings.Index(cfg.URL, "://")
	if idx < 0 {
		return nil, errors.NewNotValidf("[esikv] Unknown URL: %q. Does not contain ://", cfg.URL)
	}
	scheme := cfg.URL[:idx]

	switch scheme {
	case "redis":
		r, err := NewRedis(cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "[esikv] Failed to create new Redis object: %q", cfg.URL)
		}
		return r, nil
		//case "memcache":
		//case "mysql":
		//case "pgsql":
		//case "grpc":
	case "mockTimeout":
		return resourceMock{
			DoRequestFn: func(*backend.ResourceArgs) (_ http.Header, content []byte, err error) {
				// mockTimeout://duration
				return nil, nil, errors.NewTimeoutf("[esikv] Timeout after %q", cfg.URL[idx+3:])
			},
		}, nil
	}
	return nil, errors.NewNotSupportedf("[esikv] Unknown URL: %q. No driver defined for scheme: %q", cfg.URL, scheme)
}
