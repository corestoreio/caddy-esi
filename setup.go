package caddyesi

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

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
		// todo close all open connections to the backends
		return nil
	})
	c.OnRestart(func() error {
		// todo clear all internal caches
		//e.rc.mu.Lock()
		//defer e.rc.mu.Unlock()
		//e.rc.cache = make(map[uint64]esitag.Entities)
		return nil
	})

	return nil
}

func configEsiParse(c *caddy.Controller) (PathConfigs, error) {
	pcs := make(PathConfigs, 0, 2)

	// todo: parse it that way that only one pointer gets created for multiple equal resource/backend connections.

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
		// todo handle file close on server restart or shutdown
		if err != nil {
			return errors.NewFatalf("[caddyesi] Failed to open file %q with error: %s", pc.LogFile, err)
		}
	}

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

		cchr, err := newCacher(c.Val())
		if err != nil {
			return errors.Wrapf(err, "[caddyesi] Failed to instantiate a new cache object for %q with URL: %q", key, c.Val())
		}
		pc.Caches = append(pc.Caches, cchr)

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
	case "log_file":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] log_file: %s", c.ArgErr())
		}
		pc.LogFile = c.Val()
	case "log_level":
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] log_level: %s", c.ArgErr())
		}
		pc.LogFile = strings.ToLower(c.Val())

	default:
		//catch all
		if !c.NextArg() {
			return errors.NewNotValidf("[caddyesi] any key: %s", c.ArgErr())
		}
		if key == "" || c.Val() == "" {
			return nil // continue
		}
		if pc.KVServices == nil {
			pc.KVServices = make(map[string]KVFetcher, 10)
		}
		kvf, err := newKVFetcher(c.Val())
		if err != nil {
			return errors.Wrapf(err, "[caddyesi] newKVFetcher failed for Key %q and Value %q", key, c.Val())
		}
		pc.KVServices[key] = kvf
	}

	return nil
}
