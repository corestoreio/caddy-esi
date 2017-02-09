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

package backend_test

import (
	"net/http"
	"net/http/httptest"
)

func getExternalReqWithExtendedHeaders() *http.Request {
	req := httptest.NewRequest("GET", "https://caddyserver.com/any/path", nil)
	req.Header = http.Header{
		"Host":                      []string{"www.example.com"},
		"Connection":                []string{"keep-alive"},
		"Pragma":                    []string{"no-cache"},
		"Cache-Control":             []string{"no-cache"},
		"Upgrade-Insecure-Requests": []string{"1"},
		"User-Agent":                []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10)"},
		"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
		"DNT":                       []string{"1"},
		"Referer":                   []string{"https://www.example.com/"},
		"Accept-Encoding":           []string{"gzip, deflate, sdch, br"},
		"Avail-Dictionary":          []string{"lhdx6rYE"},
		"Accept-Language":           []string{"en-US,en;q=0.8"},
		"Cookie":                    []string{"x-wl-uid=1vnTVF5WyZIe5Fymf2a4H+pFPyJa4wxNmzCKdImj1UqQPV5ecUs2sm46vDbGJUI+sE=", "session-token=AIo5Vf+c/GhoTRWq4V; JSESSIONID=58B7C7A24731R869B75D142E970CEAD4; csm-hit=D5P2DBNF895ZDJTCTEQ7+s-D5P2DBNF895ZDJTCTEQ7|1483297885458; session-id-time=2082754801l"},
	}
	return req
}

var resourceRespWithExtendedHeaders = http.Header{
	"Server":                    []string{"Server"},
	"Date":                      []string{"Mon, 02 Jan 2017 08:58:08 GMT"},
	"Content-Type":              []string{"text/html;charset=UTF-8"},
	"Transfer-Encoding":         []string{"chunked"},
	"Connection":                []string{"keep-alive"},
	"Strict-Transport-Security": []string{"max-age=47474747; includeSubDomains; preload"},
	"x-dmz-id-1":                []string{"XBXAV6DKR823M418TZ8Y"},
	"X-Frame-Options":           []string{"SAMEORIGIN"},
	"Cache-Control":             []string{"no-transform"},
	"Content-Encoding":          []string{"gzip"},
	"Vary":                      []string{"Accept-Encoding,Avail-Dictionary,User-Agent"},
	"Set-Cookie":                []string{"ubid-acbde=253-9771841-6878311; Domain=.example.com; Expires=Sun, 28-Dec-2036 08:58:08 GMT; Path=/"},
	"x-sdch-encode":             []string{"0"},
}
