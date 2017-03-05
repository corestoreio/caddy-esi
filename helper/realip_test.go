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

package helper_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi/helper"
	"github.com/stretchr/testify/assert"
)

func TestGetRealIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		r      *http.Request
		wantIP string
	}{
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Real-IP", "123.123.123.123")
			return r
		}(), ("123.123.123.123")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("Forwarded-For", "200.100.50.3")
			return r
		}(), ("200.100.50.3")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded", "2002:0db8:85a3:0000:0000:8a2e:0370:7335")
			return r
		}(), ("2002:0db8:85a3:0000:0000:8a2e:0370:7335")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded-For", "200.100.54.4, 192.168.0.100:8080")
			return r
		}(), ("192.168.0.100")}, // maybe a bug because it returns the internal proxy IP address
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Cluster-Client-Ip", "127.0.0.1:8080")
			r.RemoteAddr = "200.100.54.6:8181"
			return r
		}(), ("200.100.54.6")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.RemoteAddr = "100.200.50.3"
			return r
		}(), ("100.200.50.3")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded-For", "127.0.0.1:8080")
			r.RemoteAddr = "2002:0db8:85a3:0000:0000:8a2e:0370:7334"
			return r
		}(), ("2002:db8:85a3::8a2e:370:7334")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.RemoteAddr = "100.200.a.3"
			return r
		}(), ""},
	}

	for i, test := range tests {
		haveIP := helper.RealIP(test.r)
		assert.Exactly(t, test.wantIP, haveIP, "Index: %d Want %s Have %s", i, test.wantIP, haveIP)
	}
}
