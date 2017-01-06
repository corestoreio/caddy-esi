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
	"gopkg.in/redis.v5"
)

type esiRedis struct {
	cl *redis.Client
}

// NewRedis provides, for now, a basic implementation for simple key fetching.
func NewRedis(rawURL string) (backend.RequestFunc, error) {
	addr, pw, db, err := ParseRedisURL(rawURL)
	if err != nil {
		return nil, errors.Errorf("[esikv] Redis error parsing URL %q => %s", rawURL, err)
	}
	r := &esiRedis{
		cl: redis.NewClient(&redis.Options{
			// The network type, either tcp or unix.
			// Default is tcp.
			Network: "tcp",
			// host:port address.
			Addr: addr,

			// Optional password. Must match the password specified in the
			// requirepass server configuration option.
			Password: pw,
			// Database to be selected after connecting to the server.
			DB: db,

			//// Maximum number of retries before giving up.
			//// Default is to not retry failed commands.
			//MaxRetries int
			//
			//// Dial timeout for establishing new connections.
			//// Default is 5 seconds.
			//DialTimeout time.Duration
			//// Timeout for socket reads. If reached, commands will fail
			//// with a timeout instead of blocking.
			//// Default is 3 seconds.
			//ReadTimeout time.Duration
			//// Timeout for socket writes. If reached, commands will fail
			//// with a timeout instead of blocking.
			//// Default is 3 seconds.
			//WriteTimeout time.Duration
			//
			//// Maximum number of socket connections.
			//// Default is 10 connections.
			//PoolSize int
			//// Amount of time client waits for connection if all connections
			//// are busy before returning an error.
			//// Default is ReadTimeout + 1 second.
			//PoolTimeout time.Duration
			//// Amount of time after which client closes idle connections.
			//// Should be less than server's timeout.
			//// Default is to not close idle connections.
			//IdleTimeout time.Duration
			//// Frequency of idle checks.
			//// Default is 1 minute.
			//// When minus value is set, then idle check is disabled.
			//IdleCheckFrequency time.Duration
			//
			//// TLS Config to use. When set TLS will be negotiated.
			//TLSConfig *tls.Config
		}),
	}

	pong, err := r.cl.Ping().Result()
	if err != nil {
		return nil, errors.NewFatalf("[esikv] Redis Ping failed: %s", err)
	}
	if pong != "PONG" {
		return nil, errors.NewFatalf("[esikv] Redis Ping not Pong: %q", pong)
	}

	return r.Get, nil
}

// Get returns a value from the field Key in the args argument. Header is not
// supported.
func (r *esiRedis) Get(args *backend.RequestFuncArgs) (_ http.Header, content []byte, err error) {
	// TODO context cancellation and deadline

	key := args.Key
	if key == "" {
		return nil, nil, errors.NewEmptyf("[esikv] Redis.Get Key is empty for resource %q", args.URL)
	}

	nKey, err := args.TemplateToURL(args.KeyTemplate)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esikv] Redis.Get.TemplateToURL %q => %q", args.URL, r.cl.String())
	}
	if nKey != "" {
		key = nKey
	}

	v, err := r.cl.Get(key).Bytes()
	if err == redis.Nil {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esikv] Redis.Get %q => %q", args.URL, r.cl.String())
	}

	if mbs := int(args.MaxBodySize); len(v) > mbs && mbs > 0 {
		v = v[:mbs] // not the nicest solution but works for now
	}

	return nil, v, nil
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
//
// TODO: More sophisticated to add also configuration values for the connection pool.
// 		redis://localhost:6379/DatabaseID/ReadTimeout=2s/
func ParseRedisURL(raw string) (address, password string, db int, err error) {
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
			db64, err := strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				return "", "", 0, errors.Errorf("[esikv] Invalid database: %q in %q", u.Path[1:], match[1])
			}
			db = int(db64)
		}
	} else if u.Path != "" {
		return "", "", 0, errors.Errorf("[esikv] Invalid database: %q", u.Path[1:])
	}
	return
}
