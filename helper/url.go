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

package helper

import (
	"net"
	"net/url"

	"github.com/corestoreio/errors"
)

// defaultPoolConnectionParameters this var also exists in the test file
var defaultPoolConnectionParameters = [...]string{
	"db", "0",
	"max_active", "10",
	"max_idle", "400",
	"idle_timeout", "240s",
	"cancellable", "0",
	"lazy", "0", // if 1 disables the ping to redis during caddy startup
}

// ParseNoSQLURL parses a given URL using a custom URI scheme.
// For example:
// 		redis://localhost:6379/?db=3
// 		memcache://localhost:1313/?lazy=1
// 		redis://:6380/?db=0 => connects to localhost:6380
// 		redis:// => connects to localhost:6379 with DB 0
// 		memcache:// => connects to localhost:11211
//		memcache://?server=localhost:11212&server=localhost:11213
//			=> connects to: localhost:11211, localhost:11212, localhost:11213
// 		redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/?db=0
// Available parameters: db, max_active (int, Connections), max_idle (int,
// Connections), idle_timeout (time.Duration, Connection), cancellable (0,1
// request towards redis), lazy (0, 1 disables ping during connection setup). On
// successful parse the key "scheme" is also set in the params return value.
func ParseNoSQLURL(raw string) (address, password string, params url.Values, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", nil, errors.Errorf("[backend] url.Parse: %s", err)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if sErr, ok := err.(*net.AddrError); ok && sErr != nil && sErr.Err == "too many colons in address" {
		return "", "", nil, errors.Errorf("[backend] SplitHostPort: %s", err)
	}
	if err != nil {
		// assume port is missing
		host = u.Host
		if port == "" {
			switch u.Scheme {
			case "redis":
				port = "6379"
			case "memcache":
				port = "11211"
			default:
				// might leak password because raw URL gets output ...
				return "", "", nil, errors.NewNotSupportedf("[backend] ParseNoSQLURL unsupported scheme %q because Port is empty. URL: %q", u.Scheme, raw)
			}
		}
	}
	if host == "" {
		host = "localhost"
	}
	address = net.JoinHostPort(host, port)

	if u.User != nil {
		password, _ = u.User.Password()
	}

	params, err = url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", "", nil, errors.NewNotValidf("[backend] ParseNoSQLURL: Failed to parse %q for parameters in URL %q with error %s", u.RawQuery, raw, err)
	}

	for i := 0; i < len(defaultPoolConnectionParameters); i = i + 2 {
		if params.Get(defaultPoolConnectionParameters[i]) == "" {
			params.Set(defaultPoolConnectionParameters[i], defaultPoolConnectionParameters[i+1])
		}
	}
	params.Set("scheme", u.Scheme)

	return
}
