package esi

import (
	"fmt"

	"github.com/mholt/caddy"
)

func init() {
	caddy.RegisterPlugin("esi", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

// setup used internally by Caddy to set up this middleware
func setup(c *caddy.Controller) error {
	_, err := esiParse(c)
	if err != nil {
		return err
	}

	c.OnShutdown(func() error {
		// close all open connections to the backends
		return nil
	})

	// @see markdown middleware

	fmt.Printf("%#v\n\n", c)

	return nil
}

func esiParse(c *caddy.Controller) (*Config, error) {

	return nil,nil
}
