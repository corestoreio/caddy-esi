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
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
)

type Redis struct {
	// todo
}

func NewRedis(rawURL string) (backend.RequestFunc, error) {
	_, _, _, err := ParseRedisURL(rawURL)
	if err != nil {
		return nil, errors.Errorf("[esikv] Error parsing URL %q => %s", rawURL, err)
	}
	r := &Redis{}
	return r.Get, nil
}

func (r *Redis) Get(args *backend.RequestFuncArgs) (_ http.Header, content []byte, err error) {
	// todo
	return nil, nil, nil
}

func (r *Redis) Close() error {
	// todo
	return nil
}

var pathDBRegexp = regexp.MustCompile(`/(\d*)\z`)

// ParseRedisURL parses a given URL using the Redis
// URI scheme. URLs should follow the draft IANA specification for the
// scheme (https://www.iana.org/assignments/uri-schemes/prov/redis).
//
//
// For example:
// 		redis://localhost:6379/3
// 		redis://:6380/0 => connects to localhost:6380
// 		redis:// => connects to localhost:6379 with DB 0
// 		redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
func ParseRedisURL(raw string) (address, password string, db int64, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, errors.Errorf("[esikv] url.Parse: %s", err)
	}

	if u.Scheme != "redis" {
		return "", "", 0, errors.Errorf("[esikv] Invalid Redis URL scheme: %q", u.Scheme)
	}

	// As per the IANA draft spec, the host defaults to localhost and
	// the port defaults to 6379.
	host, port, err := net.SplitHostPort(u.Host)
	if sErr, ok := err.(*net.AddrError); ok && sErr != nil && sErr.Err == "too many colons in address" {
		return "", "", 0, errors.Errorf("[esikv] SplitHostPort: %s", err)
	}
	if err != nil {
		// assume port is missing
		host = u.Host
		port = "6379"
		err = nil
	}
	if host == "" {
		host = "localhost"
	}
	address = net.JoinHostPort(host, port)

	if u.User != nil {
		password, _ = u.User.Password()
	}

	match := pathDBRegexp.FindStringSubmatch(u.Path)
	if len(match) == 2 {
		if len(match[1]) > 0 {
			db, err = strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				return "", "", 0, errors.Errorf("[esikv] Invalid database: %q in %q", u.Path[1:], match[1])
			}
		}
	} else if u.Path != "" {
		return "", "", 0, errors.Errorf("[esikv] Invalid database: %q", u.Path[1:])
	}
	return
}
