package caddyesi

import (
	"net/http"
	"strings"
	"time"

	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pkg/errors"
)

func init() {
	caddy.RegisterPlugin("esi", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

// setup used internally by Caddy to set up this middleware
func setup(c *caddy.Controller) error {
	pcs, err := configEsiParse(c)
	if err != nil {
		return err
	}

	cfg := httpserver.GetConfig(c)

	mw := Middleware{
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
				return nil, err
			}
		}
		pcs = append(pcs, pc)
	}
	return pcs, nil
}

func configLoadParams(c *caddy.Controller, pc *PathConfig) error {
	switch key := c.Val(); key {

	case "timeout":
		if !c.NextArg() {
			return c.ArgErr()
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return errors.Errorf("[caddyesi] Invalid duration in timeout configuration: %q", c.Val())
		}
		pc.Timeout = d

	case "ttl":
		if !c.NextArg() {
			return c.ArgErr()
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return errors.Errorf("[caddyesi] Invalid duration in ttl configuration: %q", c.Val())
		}
		pc.TTL = d

	case "cache":
		if !c.NextArg() {
			return c.ArgErr()
		}

		cchr, err := newCacher(c.Val())
		if err != nil {
			return errors.Errorf("[caddyesi] Failed to instantiate a new cache object for %q with URL: %q", key, c.Val())
		}
		pc.Caches = append(pc.Caches, cchr)

	case "request_id_hash":
		if !c.NextArg() {
			return c.ArgErr()
		}
		pc.RequestIDSource = helpers.CommaListToSlice(c.Val())

	case "allowed_methods":
		if !c.NextArg() {
			return c.ArgErr()
		}
		pc.AllowedMethods = helpers.CommaListToSlice(strings.ToUpper(c.Val()))

	default:
		//catch all
		if !c.NextArg() {
			return c.ArgErr()
		}
		if key == "" || c.Val() == "" {
			return nil // continue
		}
		if pc.KVServices == nil {
			pc.KVServices = make(map[string]KVFetcher, 10)
		}
		kvf, err := newKVFetcher(c.Val())
		if err != nil {
			return errors.Wrapf(err, "[caddyesi] configLoadParams for Key %q and Value %q", key, c.Val())
		}
		pc.KVServices[key] = kvf
	}

	return nil
}
