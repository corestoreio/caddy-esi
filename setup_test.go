package esi

import (
	"testing"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

var _ Backender = (*backendMock)(nil)
var _ Resourcer = (*resourceMock)(nil)

type backendMock struct{}

func (backendMock) Set(key, val []byte) error {
	return nil
}
func (backendMock) Get(key []byte) ([]byte, error) {
	return nil, nil
}

type resourceMock struct{}

func (resourceMock) Get(key []byte) ([]byte, error) {
	return nil, nil
}

func TestSetup(t *testing.T) {
	tests := []struct {
		config  string
		wantRC  *RootConfig
		wantErr string
	}{
		{
			`esi`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]Resourcer{},
					},
				},
			},
			"",
		},
		{
			`esi {
				timeout 5ms
				ttl 10ms
				backend redis://localhost:6379/0
			}`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/",
						Timeout:   time.Millisecond * 5,
						TTL:       time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]Resourcer{},
					},
				},
			},
			"",
		},
		{
			`esi {
				timeout Dms
			}`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/",
						Backends:  nil,
						Resources: map[string]Resourcer{},
					},
				},
			},
			`[caddyesi] Invalid duration in timeout configuration: "Dms"`,
		},
		{
			`esi {
				timeout 5ms
				ttl 10ms
				backend redis//localhost:6379/0
			}`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/",
						Timeout:   time.Millisecond * 5,
						TTL:       time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]Resourcer{},
					},
				},
			},
			`Unknown URL: "redis//localhost:6379/0". Does not contain ://`,
		},
		{
			`esi {
				timeout 5ms
				ttl 10ms
				backend redis://localhost:6379/0
				backend redis://localhost:6380/0
			}`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/",
						Timeout:   time.Millisecond * 5,
						TTL:       time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
							backendMock{},
						},
						Resources: map[string]Resourcer{},
					},
				},
			},
			"",
		},
		{
			`esi /blog
			esi /guestbook`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/blog",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]Resourcer{},
					},
					&Config{
						PathScope: "/guestbook",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]Resourcer{},
					},
				},
			},
			"",
		},
		{
			`esi /catalog/product {
				timeout 122ms
				ttl 123ms
				backend redis://localhost:6379/0
				backend redis://localhost:6380/0

				redisAWS1 redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
        		redisLocal1 redis://localhost:6379/3
		        redisLocal2 redis://localhost:6380/1

			}
			esi /checkout/cart {
				timeout 131ms
				ttl 132ms
				backend redis://localhost:6379/0

				redisLocal1 redis://localhost:6379/3
			}`,
			&RootConfig{
				Configs: Configs{
					&Config{
						PathScope: "/catalog/product",
						Timeout:   time.Millisecond * 122,
						TTL:       time.Millisecond * 123,
						Backends: Backends{
							backendMock{},
							backendMock{},
						},
						Resources: map[string]Resourcer{
							"redisAWS1":   resourceMock{},
							"redisLocal1": resourceMock{},
							"redisLocal2": resourceMock{},
						},
					},
					&Config{
						PathScope: "/checkout/cart",
						Timeout:   time.Millisecond * 131,
						TTL:       time.Millisecond * 132,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]Resourcer{
							"redisLocal1": resourceMock{},
						},
					},
				},
			},
			"",
		},
	}
	for i, test := range tests {
		c := caddy.NewTestController("http", test.config)
		err := setup(c)
		if test.wantErr != "" {
			assert.Contains(t, err.Error(), test.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%d: Expected no errors, got: %v", i, err)
		}
		mids := httpserver.GetConfig(c).Middleware()
		if len(mids) == 0 {
			t.Fatalf("%d: Expected middleware, got 0 instead", i)
		}
		handler := mids[0](httpserver.EmptyNext)
		myHandler, ok := handler.(ESI)
		if !ok {
			t.Fatalf("%d: Expected handler to be type ESI, got: %#v", i, handler)
		}
		assert.Exactly(t, len(test.wantRC.Configs), len(myHandler.rc.Configs), "Index %d", i)
		for j, wantC := range test.wantRC.Configs {

			if wantC.Backends != nil {
				assert.NotNil(t, myHandler.rc.Configs[j].Backends, "Index %d => %d", i, j)
				assert.Exactly(t, len(wantC.Backends), len(myHandler.rc.Configs[j].Backends), "Index %d => %d", i, j)
				// set to nil or assert.Extactly at the end will fail because different pointers.
				wantC.Backends = nil
				myHandler.rc.Configs[j].Backends = nil
			}

			if len(wantC.Resources) > 0 {
				assert.NotNil(t, myHandler.rc.Configs[j].Resources, "Index %d => %d", i, j)
				assert.Exactly(t, len(wantC.Resources), len(myHandler.rc.Configs[j].Resources), "Index %d => %d", i, j)
				// set to nil or assert.Extactly at the end will fail because different pointers.
				wantC.Resources = nil
				myHandler.rc.Configs[j].Resources = nil
			}
			assert.Exactly(t, wantC, myHandler.rc.Configs[j], "Index %d => %d", i, j)
		}
	}
}
