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

// +build esiall esiredis

package backend

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/corestoreio/errors"
	"github.com/garyburd/redigo/redis"
)

// https://github.com/garyburd/redigo/issues/207 <-- context to be added to the package: declined.

func init() {
	RegisterResourceHandlerFactory("redis", NewRedis)
}

type esiRedis struct {
	isCancellable bool
	url           string
	pool          *redis.Pool
}

// NewRedis provides, for now, a basic implementation for simple key fetching.
func NewRedis(cfg *ConfigItem) (ResourceHandler, error) {
	addr, pw, params, err := ParseNoSQLURL(cfg.URL)
	if err != nil {
		return nil, errors.Errorf("[backend] Redis error parsing URL %q => %s", cfg.URL, err)
	}

	maxActive, err := strconv.Atoi(params.Get("max_active"))
	if err != nil {
		return nil, errors.NewNotValidf("[backend] NewRedis.ParseNoSQLURL. Parameter max_active not valid in  %q", cfg.URL)
	}
	maxIdle, err := strconv.Atoi(params.Get("max_idle"))
	if err != nil {
		return nil, errors.NewNotValidf("[backend] NewRedis.ParseNoSQLURL. Parameter max_idle not valid in  %q", cfg.URL)
	}
	idleTimeout, err := time.ParseDuration(params.Get("idle_timeout"))
	if err != nil {
		return nil, errors.NewNotValidf("[backend] NewRedis.ParseNoSQLURL. Parameter idle_timeout not valid in  %q", cfg.URL)
	}

	r := &esiRedis{
		isCancellable: params.Get("cancellable") == "1",
		url:           cfg.URL,
		pool: &redis.Pool{
			MaxActive:   maxActive,
			MaxIdle:     maxIdle,
			IdleTimeout: idleTimeout,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", addr)
				if err != nil {
					return nil, errors.Wrap(err, "[backend] Redis Dial failed")
				}
				if pw != "" {
					if _, err := c.Do("AUTH", pw); err != nil {
						c.Close()
						return nil, errors.Wrap(err, "[backend] Redis AUTH failed")
					}
				}
				if _, err := c.Do("SELECT", params.Get("db")); err != nil {
					c.Close()
					return nil, errors.Wrap(err, "[backend] Redis DB select failed")
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
		return nil, errors.NewFatalf("[backend] Redis Ping failed: %s", err)
	}
	if pong != "PONG" {
		return nil, errors.NewFatalf("[backend] Redis Ping not Pong: %#v", pong)
	}

	return r, nil
}

// Closes closes the resource when Caddy restarts or reloads. If supported
// by the resource.
func (er *esiRedis) Close() error {
	return errors.Wrapf(er.pool.Close(), "[backend] Redis Close. URI %q", er.url)
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (er *esiRedis) DoRequest(args *ResourceArgs) (_ http.Header, _ []byte, err error) {
	if er.isCancellable {
		// 50000	     28794 ns/op	    1026 B/op	      33 allocs/op
		return er.doRequestCancel(args)
	}
	// 50000	     25071 ns/op	     529 B/op	      25 allocs/op
	return er.doRequest(args)
}

func (er *esiRedis) validateArgs(args *ResourceArgs) (err error) {
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

func (er *esiRedis) doRequest(args *ResourceArgs) (_ http.Header, _ []byte, err error) {
	if err := er.validateArgs(args); err != nil {
		return nil, nil, errors.Wrap(err, "[backend] doRequest.validateArgs")
	}

	key := args.Key

	conn := er.pool.Get()
	defer conn.Close()

	nKey, err := args.TemplateToURL(args.KeyTemplate)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[backend] Redis.Get.TemplateToURL %q => %q", er.url, args.Key)
	}
	if nKey != "" {
		key = nKey
	}

	value, err := redis.Bytes(conn.Do("GET", key))
	if err == redis.ErrNil {
		return nil, nil, errors.NewNotFoundf("[backend] URL %q: Key %q not found", er.url, args.Key)
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[backend] Redis.Get %q => %q", er.url, args.Key)
	}

	if mbs := int(args.MaxBodySize); len(value) > mbs && mbs > 0 {
		value = value[:mbs]
	}

	return nil, value, err
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (er *esiRedis) doRequestCancel(args *ResourceArgs) (_ http.Header, _ []byte, err error) {
	if err := er.validateArgs(args); err != nil {
		return nil, nil, errors.Wrap(err, "[backend] doRequestCancel.validateArgs")
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
			retErr <- errors.Wrapf(err, "[backend] Redis.Get.TemplateToURL %q => %q", er.url, args.Key)
			return
		}
		if nKey != "" {
			key = nKey
		}

		value, err := redis.Bytes(conn.Do("GET", key))
		if err == redis.ErrNil {
			retErr <- errors.NewNotFoundf("[backend] URL %q: Key %q not found", er.url, args.Key)
			return
		}
		if err != nil {
			retErr <- errors.Wrapf(err, "[backend] Redis.Get %q => %q", er.url, args.Key)
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
		err = errors.Wrapf(ctx.Err(), "[backend] Redits Get Context cancelled. Previous possible error: %+v", retErr)
	case value = <-content:
	case err = <-retErr:
	}
	return nil, value, err
}
