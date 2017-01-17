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
	"context"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
	"github.com/garyburd/redigo/redis"
)

// Maybe replace redis client with redigo even better if there is a redis client with build in context
// https://github.com/garyburd/redigo/issues/207 <-- context to be added to the package: declined.

type esiRedis struct {
	url  string
	pool *redis.Pool
}

// NewRedis provides, for now, a basic implementation for simple key fetching.
func NewRedis(rawURL string) (backend.ResourceHandler, error) {
	addr, pw, db, err := ParseRedisURL(rawURL)
	if err != nil {
		return nil, errors.Errorf("[esikv] Redis error parsing URL %q => %s", rawURL, err)
	}
	r := &esiRedis{
		url: rawURL,
		pool: &redis.Pool{
			MaxActive:   5,                 // just guessed that number, make it configurable in the connection URI
			MaxIdle:     40,                // just guessed that number, make it configurable in the connection URI
			IdleTimeout: 240 * time.Second, // make it configurable in the connection URI
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", addr)
				if err != nil {
					return nil, errors.Wrap(err, "[esikv] Redis Dial failed")
				}
				if pw != "" {
					if _, err := c.Do("AUTH", pw); err != nil {
						c.Close()
						return nil, errors.Wrap(err, "[esikv] Redis AUTH failed")
					}
				}
				if _, err := c.Do("SELECT", db); err != nil {
					c.Close()
					return nil, errors.Wrap(err, "[esikv] Redis DB select failed")
				}
				return c, nil
			},
		},
	}

	conn := r.pool.Get()
	defer conn.Close()

	pong, err := redis.String(conn.Do("PING"))
	if err != nil && err != redis.ErrNil {
		return nil, errors.NewFatalf("[esikv] Redis Ping failed: %s", err)
	}
	if pong != "PONG" {
		return nil, errors.NewFatalf("[esikv] Redis Ping not Pong: %#v", pong)
	}

	return r, nil
}

// Closes closes the resource when Caddy restarts or reloads. If supported
// by the resource.
func (er *esiRedis) Close() error {
	return errors.Wrapf(er.pool.Close(), "[esikv] Redis Close. URL %q", er.url)
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (er *esiRedis) DoRequest(args *backend.ResourceArgs) (_ http.Header, content []byte, err error) {
	switch {
	case args.Key == "":
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %#v the URL value is empty", args)
	case args.ExternalReq == nil:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the ExternalReq value is nil", args.URL)
	case args.Timeout < 1:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the timeout value is empty", args.URL)
	case args.MaxBodySize == 0:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the maxBodySize value is empty", args.URL)
	}
	if err != nil {
		return nil, nil, err
	}

	key := args.Key

	// See git history for a version without context.WithTimeout. A bit faster and less allocs.
	ctx, cancel := context.WithTimeout(args.ExternalReq.Context(), args.Timeout)
	defer cancel()

	var retErr error
	done := make(chan struct{})
	go func() {
		defer func() {
			done <- struct{}{}
		}()

		conn := er.pool.Get()
		defer conn.Close()

		nKey, err := args.TemplateToURL(args.KeyTemplate)
		if err != nil {
			retErr = errors.Wrapf(err, "[esikv] Redis.Get.TemplateToURL %q => %q", args.URL, er.url)
			return
		}
		if nKey != "" {
			key = nKey
		}

		content, err = redis.Bytes(conn.Do("GET", key))
		if err == redis.ErrNil {
			return
		}
		if err != nil {
			retErr = errors.Wrapf(err, "[esikv] Redis.Get %q => %q", args.URL, er.url)
			return
		}

		if mbs := int(args.MaxBodySize); len(content) > mbs && mbs > 0 {
			content = content[:mbs] // not the nicest solution but works for now
		}
	}()

	select {
	case <-ctx.Done():
		retErr = errors.Wrapf(ctx.Err(), "[esikv] Redits Get Context cancelled. Previous possible error: %+v", retErr)
	case <-done:
	}
	return nil, content, retErr
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
