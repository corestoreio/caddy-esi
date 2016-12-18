package caddyesi

import (
	"testing"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	t.Parallel()

	runner := func(config string, wantPC PathConfigs, wantErr string) func(*testing.T) {
		return func(ts *testing.T) {
			c := caddy.NewTestController("http", config)
			err := PluginSetup(c)
			if wantErr != "" {
				assert.Contains(ts, err.Error(), wantErr)
				return
			}
			if err != nil {
				ts.Errorf("Expected no errors, got: %v", err)
			}
			mids := httpserver.GetConfig(c).Middleware()
			if len(mids) != 1 {
				ts.Fatalf("Expected one middleware, got %d instead", len(mids))
			}
			handler := mids[0](httpserver.EmptyNext)
			myHandler, ok := handler.(Middleware)
			if !ok {
				ts.Fatalf("Expected handler to be type ESI, got: %#v", handler)
			}
			assert.Exactly(ts, len(wantPC), len(myHandler.PathConfigs))
			for j, wantC := range wantPC {
				haveC := myHandler.PathConfigs[j]

				assert.Exactly(ts, wantC.Scope, haveC.Scope, "Scope (Path)")
				assert.Exactly(ts, wantC.Timeout, haveC.Timeout, "Timeout")
				assert.Exactly(ts, wantC.TTL, haveC.TTL, "TTL")
				assert.Exactly(ts, wantC.RequestIDSource, haveC.RequestIDSource, "RequestIDSource")
				assert.Exactly(ts, wantC.AllowedMethods, haveC.AllowedMethods, "AllowedMethods")

				assert.Len(ts, haveC.KVServices, len(wantC.KVServices), "Index %d", j)
				for key := range wantC.KVServices {
					_, ok := haveC.KVServices[key]
					assert.True(ts, ok, "Index %d", j)
				}

				assert.Len(ts, haveC.Resources, len(wantC.Resources), "Index  %d", j)
				for key := range wantC.Resources {
					_, ok := haveC.Resources[key]
					assert.True(ts, ok, "Index %d", j)
				}

				assert.Len(ts, haveC.Caches, len(wantC.Caches), "Index  %d", j)
			}
		}
	}

	t.Run("Default Config", runner(
		`esi`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: DefaultTimeOut,
				TTL:     0,
			},
		},
		"",
	))

	t.Run("With timeout, ttl and 1x Cacher", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://localhost:6379/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
				Caches: []Cacher{
					cacherMock{},
				},
			},
		},
		"",
	))

	t.Run("With timeout, ttl and 2x Cacher", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://localhost:6379/0
			cache redis://localhost:6380/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
				Caches: []Cacher{
					cacherMock{},
					cacherMock{},
				},
			},
		},
		"",
	))

	t.Run("With timeout, ttl and KVService", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			backend redis://localhost:6379/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
				KVServices: map[string]KVFetcher{
					"backend": kvFetchMock{},
				},
			},
		},
		"",
	))

	t.Run("config with allowed_methods", runner(
		`esi {
			allowed_methods "GET,pUT , POsT"
		}`,
		PathConfigs{
			&PathConfig{
				Scope:          "/",
				Timeout:        DefaultTimeOut,
				AllowedMethods: []string{"GET", "PUT", "POST"},
			},
		},
		"",
	))

	t.Run("config with request_id_source", runner(
		`esi {
			request_id_source "pAth,host , IP, header-X-GitHub-Request-Id, header-Server, cookie-__Host-user_session_same_site"
		}`,
		PathConfigs{
			&PathConfig{
				Scope:           "/",
				Timeout:         DefaultTimeOut,
				RequestIDSource: []string{"pAth", "host", "IP", "header-X-GitHub-Request-Id", "header-Server", "cookie-__Host-user_session_same_site"},
			},
		},
		"",
	))

	t.Run("config with request_id_source but errors", runner(
		`esi {
			request_id_source "path,host , ip
		}`,
		nil,
		`Wrong argument count or unexpected line ending after 'path,host , ip`,
	))

	t.Run("Parse timeout fails", runner(
		`esi {
			timeout Dms
		}`,
		nil,
		`[caddyesi] Invalid duration in timeout configuration: "Dms"`,
	))

	t.Run("Invalid KVService URL", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			backend redis//localhost:6379/0
		}`,
		nil,
		`Unknown URL: "redis//localhost:6379/0". Does not contain ://`,
	))

	t.Run("Overwrite duplicate key in KVServices", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			backend redis://localhost:6379/0
			backend redis://localhost:6380/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
				KVServices: map[string]KVFetcher{
					"backend": kvFetchMock{},
				},
			},
		},
		"",
	))

	t.Run("Create two KVServices", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			redis1 redis://localhost:6379/0
			redis2 redis://localhost:6380/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
				KVServices: map[string]KVFetcher{
					"redis1": kvFetchMock{},
					"redis2": kvFetchMock{},
				},
			},
		},
		"",
	))

	t.Run("esi with path to /blog and /guestbook", runner(
		`esi /blog
		esi /guestbook`,
		PathConfigs{
			&PathConfig{
				Scope:   "/blog",
				Timeout: DefaultTimeOut,
				TTL:     0,
			},
			&PathConfig{
				Scope:   "/guestbook",
				Timeout: DefaultTimeOut,
				TTL:     0,
			},
		},
		"",
	))

	t.Run("2x esi directives with different KVServices", runner(
		`esi /catalog/product {
		   timeout 122ms
		   ttl 123ms

		   redisAWS1 redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
		   redisLocal1 redis://localhost:6379/3
		   redisLocal2 redis://localhost:6380/1

		}
		esi /checkout/cart {
		   timeout 131ms
		   ttl 132ms

		   redisLocal3 redis://localhost:6379/3
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/catalog/product",
				Timeout: time.Millisecond * 122,
				TTL:     time.Millisecond * 123,
				KVServices: map[string]KVFetcher{
					"redisAWS1":   kvFetchMock{},
					"redisLocal1": kvFetchMock{},
					"redisLocal2": kvFetchMock{},
				},
			},
			&PathConfig{
				Scope:   "/checkout/cart",
				Timeout: time.Millisecond * 131,
				TTL:     time.Millisecond * 132,
				KVServices: map[string]KVFetcher{
					"redisLocal3": kvFetchMock{},
				},
			},
		},
		"",
	))
}
