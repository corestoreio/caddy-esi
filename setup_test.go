package caddyesi

import (
	"testing"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

var _ Backender = (*backendMock)(nil)
var _ ResourceFetcher = (*resourceMock)(nil)

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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:     "/",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]ResourceFetcher{},
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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:   "/",
						Timeout: time.Millisecond * 5,
						TTL:     time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]ResourceFetcher{},
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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:     "/",
						Backends:  nil,
						Resources: map[string]ResourceFetcher{},
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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:   "/",
						Timeout: time.Millisecond * 5,
						TTL:     time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]ResourceFetcher{},
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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:   "/",
						Timeout: time.Millisecond * 5,
						TTL:     time.Millisecond * 10,
						Backends: Backends{
							backendMock{},
							backendMock{},
						},
						Resources: map[string]ResourceFetcher{},
					},
				},
			},
			"",
		},
		{
			`esi /blog
			esi /guestbook`,
			&RootConfig{
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:     "/blog",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]ResourceFetcher{},
					},
					&PathConfig{
						Scope:     "/guestbook",
						Timeout:   0,
						TTL:       0,
						Backends:  nil,
						Resources: map[string]ResourceFetcher{},
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
				PathConfigs: PathConfigs{
					&PathConfig{
						Scope:   "/catalog/product",
						Timeout: time.Millisecond * 122,
						TTL:     time.Millisecond * 123,
						Backends: Backends{
							backendMock{},
							backendMock{},
						},
						Resources: map[string]ResourceFetcher{
							"redisAWS1":   resourceMock{},
							"redisLocal1": resourceMock{},
							"redisLocal2": resourceMock{},
						},
					},
					&PathConfig{
						Scope:   "/checkout/cart",
						Timeout: time.Millisecond * 131,
						TTL:     time.Millisecond * 132,
						Backends: Backends{
							backendMock{},
						},
						Resources: map[string]ResourceFetcher{
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
		assert.Exactly(t, len(test.wantRC.PathConfigs), len(myHandler.rc.PathConfigs), "Index %d", i)
		for j, wantC := range test.wantRC.PathConfigs {

			if wantC.Backends != nil {
				assert.NotNil(t, myHandler.rc.PathConfigs[j].Backends, "Index %d => %d", i, j)
				assert.Exactly(t, len(wantC.Backends), len(myHandler.rc.PathConfigs[j].Backends), "Index %d => %d", i, j)
				// set to nil or assert.Extactly at the end will fail because different pointers.
				wantC.Backends = nil
				myHandler.rc.PathConfigs[j].Backends = nil
			}

			if len(wantC.Resources) > 0 {
				assert.NotNil(t, myHandler.rc.PathConfigs[j].Resources, "Index %d => %d", i, j)
				assert.Exactly(t, len(wantC.Resources), len(myHandler.rc.PathConfigs[j].Resources), "Index %d => %d", i, j)
				// set to nil or assert.Extactly at the end will fail because different pointers.
				wantC.Resources = nil
				myHandler.rc.PathConfigs[j].Resources = nil
			}
			assert.Exactly(t, wantC, myHandler.rc.PathConfigs[j], "Index %d => %d", i, j)
		}
	}
}
