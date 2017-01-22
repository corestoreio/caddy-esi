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
	"strconv"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
	"github.com/garyburd/redigo/redis"
)

// https://github.com/garyburd/redigo/issues/207 <-- context to be added to the package: declined.

type esiRedis struct {
	isCancellable bool
	url           string
	pool          *redis.Pool
}

// NewRedis provides, for now, a basic implementation for simple key fetching.
func NewRedis(rawURL string) (backend.ResourceHandler, error) {
	addr, pw, params, err := ParseRedisURL(rawURL)
	if err != nil {
		return nil, errors.Errorf("[esikv] Redis error parsing URL %q => %s", rawURL, err)
	}

	maxActive, err := strconv.Atoi(params.Get("max_active"))
	if err != nil {
		return nil, errors.NewNotValidf("[esikv] NewRedis.ParseRedisURL. Parameter max_active not valid in  %q", rawURL)
	}
	maxIdle, err := strconv.Atoi(params.Get("max_idle"))
	if err != nil {
		return nil, errors.NewNotValidf("[esikv] NewRedis.ParseRedisURL. Parameter max_idle not valid in  %q", rawURL)
	}
	idleTimeout, err := time.ParseDuration(params.Get("idle_timeout"))
	if err != nil {
		return nil, errors.NewNotValidf("[esikv] NewRedis.ParseRedisURL. Parameter idle_timeout not valid in  %q", rawURL)
	}

	r := &esiRedis{
		isCancellable: params.Get("cancellable") == "1",
		url:           rawURL,
		pool: &redis.Pool{
			MaxActive:   maxActive,
			MaxIdle:     maxIdle,
			IdleTimeout: idleTimeout,
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
				if _, err := c.Do("SELECT", params.Get("db")); err != nil {
					c.Close()
					return nil, errors.Wrap(err, "[esikv] Redis DB select failed")
				}
				return c, nil
			},
		},
	}

	if params.Get("lazy") == "1" {
		return r, nil
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
	return errors.Wrapf(er.pool.Close(), "[esikv] Redis Close. URI %q", er.url)
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (er *esiRedis) DoRequest(args *backend.ResourceArgs) (_ http.Header, _ []byte, err error) {
	if er.isCancellable {
		// 50000	     28794 ns/op	    1026 B/op	      33 allocs/op
		return er.doRequestCancel(args)
	}
	// 50000	     25071 ns/op	     529 B/op	      25 allocs/op
	return er.doRequest(args)
}

func (er *esiRedis) validateArgs(args *backend.ResourceArgs) (err error) {
	switch {
	case args.Key == "":
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %#v the URL value is empty", args)
	case args.ExternalReq == nil:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the ExternalReq value is nil", er.url, args.Key)
	case args.Timeout < 1:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the timeout value is empty", er.url, args.Key)
	case args.MaxBodySize == 0:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the maxBodySize value is empty", er.url, args.Key)
	}
	return
}

func (er *esiRedis) doRequest(args *backend.ResourceArgs) (_ http.Header, _ []byte, err error) {
	if err := er.validateArgs(args); err != nil {
		return nil, nil, errors.Wrap(err, "[esikv] doRequest.validateArgs")
	}

	key := args.Key

	conn := er.pool.Get()
	defer conn.Close()

	nKey, err := args.TemplateToURL(args.KeyTemplate)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esikv] Redis.Get.TemplateToURL %q => %q", er.url, args.Key)
	}
	if nKey != "" {
		key = nKey
	}

	value, err := redis.Bytes(conn.Do("GET", key))
	if err == redis.ErrNil {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esikv] Redis.Get %q => %q", er.url, args.Key)
	}

	if mbs := int(args.MaxBodySize); len(value) > mbs && mbs > 0 {
		value = value[:mbs]
	}

	return nil, value, err
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (er *esiRedis) doRequestCancel(args *backend.ResourceArgs) (_ http.Header, _ []byte, err error) {
	if err := er.validateArgs(args); err != nil {
		return nil, nil, errors.Wrap(err, "[esikv] doRequestCancel.validateArgs")
	}

	key := args.Key

	// See git history for a version without context.WithTimeout. A bit faster and less allocs.
	ctx, cancel := context.WithTimeout(args.ExternalReq.Context(), args.Timeout)
	defer cancel()

	content := make(chan []byte)
	retErr := make(chan error)
	go func() {

		conn := er.pool.Get()
		defer conn.Close()

		nKey, err := args.TemplateToURL(args.KeyTemplate)
		if err != nil {
			retErr <- errors.Wrapf(err, "[esikv] Redis.Get.TemplateToURL %q => %q", er.url, args.Key)
			return
		}
		if nKey != "" {
			key = nKey
		}

		value, err := redis.Bytes(conn.Do("GET", key))
		if err == redis.ErrNil {
			content <- []byte{}
			return
		}
		if err != nil {
			retErr <- errors.Wrapf(err, "[esikv] Redis.Get %q => %q", er.url, args.Key)
			return
		}

		if mbs := int(args.MaxBodySize); len(value) > mbs && mbs > 0 {
			value = value[:mbs]
		}
		content <- value
	}()

	var value []byte
	select {
	case <-ctx.Done():
		err = errors.Wrapf(ctx.Err(), "[esikv] Redits Get Context cancelled. Previous possible error: %+v", retErr)
	case value = <-content:
	case err = <-retErr:
	}
	return nil, value, err
}

// defaultPoolConnectionParameters this var also exists in the test file
var defaultPoolConnectionParameters = [...]string{
	"db", "0",
	"max_active", "10",
	"max_idle", "400",
	"idle_timeout", "240s",
	"cancellable", "0",
	"lazy", "0", // if 1 disables the ping to redis during caddy startup
}

// ParseRedisURL parses a given URL using the Redis
// URI scheme. URLs should follow the draft IANA specification for the
// scheme (https://www.iana.org/assignments/uri-schemes/prov/redis).
// For example:
// 		redis://localhost:6379/?db=3
// 		redis://:6380/?db=0 => connects to localhost:6380
// 		redis:// => connects to localhost:6379 with DB 0
// 		redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/?db=0
// Available parameters: db, max_active (int, Connections), max_idle (int,
// Connections), idle_timeout (time.Duration, Connection), cancellable (0,1
// request towards redis), lazy (0, 1 disables ping during connection setup).
func ParseRedisURL(raw string) (address, password string, params url.Values, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", nil, errors.Errorf("[esikv] url.Parse: %s", err)
	}

	if u.Scheme != "redis" {
		return "", "", nil, errors.Errorf("[esikv] Invalid Redis URL scheme: %q", u.Scheme)
	}

	// As per the IANA draft spec, the host defaults to localhost and
	// the port defaults to 6379.
	host, port, err := net.SplitHostPort(u.Host)
	if sErr, ok := err.(*net.AddrError); ok && sErr != nil && sErr.Err == "too many colons in address" {
		return "", "", nil, errors.Errorf("[esikv] SplitHostPort: %s", err)
	}
	if err != nil {
		// assume port is missing
		host = u.Host
		port = "6379"
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
		return "", "", nil, errors.NewNotValidf("[esikv] ParseRedisURL: Failed to parse %q for parameters in URL %q with error %s", u.RawQuery, raw, err)
	}

	for i := 0; i < len(defaultPoolConnectionParameters); i = i + 2 {
		if params.Get(defaultPoolConnectionParameters[i]) == "" {
			params.Set(defaultPoolConnectionParameters[i], defaultPoolConnectionParameters[i+1])
		}
	}

	return
}
