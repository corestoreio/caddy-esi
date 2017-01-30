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

package caddyesi

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esicache"
	"github.com/SchumacherFM/caddyesi/esikv"
	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/corestoreio/log/logw"
	"github.com/dustin/go-humanize"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

func init() {
	caddy.RegisterPlugin("esi", caddy.Plugin{
		ServerType: "http",
		Action:     PluginSetup,
	})
}

// PluginSetup used internally by Caddy to set up this middleware
func PluginSetup(c *caddy.Controller) error {
	pcs, err := configEsiParse(c)
	if err != nil {
		return errors.Wrap(err, "[caddyesi] Failed to parse configuration")
	}

	cfg := httpserver.GetConfig(c)

	mw := &Middleware{
		Root:        cfg.Root,
		FileSys:     http.Dir(cfg.Root),
		PathConfigs: pcs,
	}

	cfg.AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		mw.Next = next
		return mw
	})

	c.OnShutdown(func() error {
		return errors.Wrap(backend.CloseAllResourceHandler(), "[caddyesi] OnShutdown")
	})
	c.OnRestart(func() error {
		// really necessary? investigate later
		for _, pc := range pcs {
			pc.purgeESICache()
		}
		return errors.Wrap(backend.CloseAllResourceHandler(), "[caddyesi] OnRestart")
	})

	return nil
}

func configEsiParse(c *caddy.Controller) (PathConfigs, error) {
	pcs := make(PathConfigs, 0, 2)

	for c.Next() {
		pc := NewPathConfig()

		// Get the path scope
		args := c.RemainingArgs()
		switch len(args) {
		case 0:
			pc.Scope = "/"
		case 1:
			pc.Scope = args[0]
		default:
			return nil, c.ArgErr()
		}

		// Load any other configuration parameters
		for c.NextBlock() {
			if err := configLoadParams(c, pc); err != nil {
				return nil, errors.Wrap(err, "[caddyesi] Failed to load params")
			}
		}
		if err := setupLogger(pc); err != nil {
			return nil, errors.Wrap(err, "[caddyesi] Failed to setup Logger")
		}

		if pc.MaxBodySize == 0 {
			pc.MaxBodySize = DefaultMaxBodySize
		}
		if pc.Timeout == 0 {
			pc.Timeout = DefaultTimeOut
		}
		if len(pc.OnError) == 0 {
			pc.OnError = []byte(DefaultOnError)
		}

		pcs = append(pcs, pc)
	}
	return pcs, nil
}

// mocked out for testing
var osStdErr io.Writer = os.Stderr
var osStdOut io.Writer = os.Stdout

func setupLogger(pc *PathConfig) error {
	pc.Log = log.BlackHole{}
	lvl := 0
	switch pc.LogLevel {
	case "debug":
		lvl = logw.LevelDebug
	case "info":
		lvl = logw.LevelInfo
	case "fatal":
		lvl = logw.LevelFatal
	}
	if lvl == 0 {
		// logging disabled
		return nil
	}

	var w io.Writer
	switch pc.LogFile {
	case "stderr":
		w = osStdErr
	case "stdout":
		w = osStdOut
	case "":
		// logging disabled
		return nil
	default:
		var err error
		w, err = os.OpenFile(pc.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		// maybe handle file close on server restart or shutdown
		if err != nil {
			return errors.NewFatalf("[caddyesi] Failed to open file %q with error: %s", pc.LogFile, err)
		}
	}

	// TODO(CyS) the output format of the logger isn't very machine parsing friendly
	pc.Log = logw.NewLog(logw.WithWriter(w), logw.WithLevel(lvl))
	return nil
}

func configLoadParams(c *caddy.Controller, pc *PathConfig) error {
	switch key := c.Val(); key {

	case "timeout":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] timeout: %s", c.ArgErr())
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return errors.NewNotValidf("[caddyesi] Invalid duration in timeout configuration: %q Error: %s", c.Val(), err)
		}
		pc.Timeout = d

	case "ttl":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] ttl: %s", c.ArgErr())
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return errors.NewNotValidf("[caddyesi] Invalid duration in ttl configuration: %q Error: %s", c.Val(), err)
		}
		pc.TTL = d

	case "max_body_size":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] max_body_size: %s", c.ArgErr())
		}
		d, err := humanize.ParseBytes(c.Val())
		if err != nil {
			return errors.NewNotValidf("[caddyesi] Invalid max body size value configuration: %q Error: %s", c.Val(), err)
		}
		pc.MaxBodySize = d

	case "cache":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] cache: %s", c.ArgErr())
		}

		if err := esicache.MainRegistry.Register(pc.Scope, c.Val()); err != nil {
			return errors.Wrapf(err, "[caddyesi] esicache.MainRegistry.Register Key %q with URL: %q", key, c.Val())
		}

	case "page_id_source":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] page_id_source: %s", c.ArgErr())
		}
		pc.PageIDSource = helpers.CommaListToSlice(c.Val())

	case "allowed_methods":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] allowed_methods: %s", c.ArgErr())
		}
		pc.AllowedMethods = helpers.CommaListToSlice(strings.ToUpper(c.Val()))
	case "cmd_header_name":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] cmd_header_name: %s", c.ArgErr())
		}
		pc.CmdHeaderName = http.CanonicalHeaderKey(c.Val())
	case "on_error":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] allowed_methods: %s", c.ArgErr())
		}
		if err := pc.parseOnError(c.Val()); err != nil {
			return errors.Wrap(err, "[caddyesi] PathConfig.parseOnError")
		}
	case "log_file":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] log_file: %s", c.ArgErr())
		}
		pc.LogFile = c.Val()
	case "log_level":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] log_level: %s", c.ArgErr())
		}
		pc.LogLevel = strings.ToLower(c.Val())
	case "resources":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] resources: %s", c.ArgErr())
		}
		// c.Val() contains the file name or raw-content ;-)
		items, err := esikv.ConfigUnmarshal(c.Val())
		if err != nil {
			return errors.Wrapf(err, "[caddyesi] Failed to unmarshal resource config %q", c.Val())
		}
		for _, item := range items {
			f, err := esikv.NewResourceHandler(item)
			if err != nil {
				// may disclose passwords which are stored in the URL
				return errors.Wrapf(err, "[caddyesi] esikv Service init failed for URL %q in file %q", item.URL, c.Val())
			}
			backend.RegisterResourceHandler(item.Alias, f)
		}
	default:
		c.NextArg()
		return errors.NewNotSupportedf("[caddyesi] Key %q with value %q not supported", key, c.Val())
	}

	return nil
}
