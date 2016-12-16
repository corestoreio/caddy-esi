package caddyesi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
)

func init() {
	caddy.RegisterPlugin("esi", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

// setup used internally by Caddy to set up this middleware
func setup(c *caddy.Controller) error {
	rc, err := configEsiParse(c)
	if err != nil {
		return err
	}

	cfg := httpserver.GetConfig(c)

	e := ESI{
		Root:    cfg.Root,
		FileSys: http.Dir(cfg.Root),
		rc:      rc,
	}

	cfg.AddMiddleware(func(next httpserver.Handler) httpserver.Handler {
		e.Next = next
		return e
	})

	c.OnShutdown(func() error {
		// todo close all open connections to the backends
		return nil
	})
	c.OnRestart(func() error {
		// todo clear all internal caches
		e.rc.mu.Lock()
		defer e.rc.mu.Unlock()
		e.rc.cache = make(map[uint64]esitag.Entities)
		return nil
	})

	return nil
}

func configEsiParse(c *caddy.Controller) (rc *RootConfig, _ error) {

	// todo: parse it that way that only one pointer gets created for multiple equal
	// resource/backend connections.

	for c.Next() {
		esi := &PathConfig{
			Resources: make(map[string]ResourceFetcher),
		}

		// Get the path scope
		args := c.RemainingArgs()
		switch len(args) {
		case 0:
			esi.Scope = "/"
		case 1:
			esi.Scope = args[0]
		default:
			return nil, c.ArgErr()
		}

		// Load any other configuration parameters
		for c.NextBlock() {
			if err := configLoadParams(c, esi); err != nil {
				return nil, err
			}
		}
		if rc == nil {
			// lazy init
			rc = NewRootConfig()
		}
		rc.PathConfigs = append(rc.PathConfigs, esi)
	}
	return rc, nil
}

func configLoadParams(c *caddy.Controller, esic *PathConfig) error {

	switch key := c.Val(); key {
	case "timeout":
		if !c.NextArg() {
			return c.ArgErr()
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return fmt.Errorf("[caddyesi] Invalid duration in timeout configuration: %q", c.Val())
		}
		esic.Timeout = d
		return nil
	case "ttl":
		if !c.NextArg() {
			return c.ArgErr()
		}
		d, err := time.ParseDuration(c.Val())
		if err != nil {
			return fmt.Errorf("[caddyesi] Invalid duration in ttl configuration: %q", c.Val())
		}
		esic.TTL = d
		return nil
	case "backend":
		if !c.NextArg() {
			return c.ArgErr()
		}
		be, err := parseBackendUrl(c.Val())
		if err != nil {
			return err
		}
		esic.Backends = append(esic.Backends, be)
		return nil

	default:
		//catch all
		if !c.NextArg() {
			return c.ArgErr()
		}
		if key == "" || c.Val() == "" {
			return nil // continue
		}
		// todo generic resource loading and parsing
		rc, err := NewRedis(c.Val())
		if err != nil {
			return fmt.Errorf("[caddyesi] Cannot parse URL %q with key %q. Error: %s", c.Val(), key, err)
		}
		esic.Resources[key] = rc
		return nil
	}
}
