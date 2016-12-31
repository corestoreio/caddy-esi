package caddyesi

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

func TestPluginSetup(t *testing.T) {
	t.Parallel()

	runner := func(config string, wantPC PathConfigs, wantErrBhf errors.BehaviourFunc) func(*testing.T) {
		return func(t *testing.T) {
			c := caddy.NewTestController("http", config)

			if err := PluginSetup(c); wantErrBhf != nil {
				assert.True(t, wantErrBhf(err), "%+v", err)
				return
			} else if err != nil {
				t.Fatalf("Expected no errors, got: %+v", err)
			}

			mids := httpserver.GetConfig(c).Middleware()
			if len(mids) != 1 {
				t.Fatalf("Expected one middleware, got %d instead", len(mids))
			}
			handler := mids[0](httpserver.EmptyNext)
			myHandler, ok := handler.(*Middleware)
			if !ok {
				t.Fatalf("Expected handler to be type ESI, got: %#v", handler)
			}
			assert.Exactly(t, len(wantPC), len(myHandler.PathConfigs))
			for j, wantC := range wantPC {
				haveC := myHandler.PathConfigs[j]

				assert.Exactly(t, wantC.Scope, haveC.Scope, "Scope (Path) %s", t.Name())
				assert.Exactly(t, wantC.Timeout, haveC.Timeout, "Timeout %s", t.Name())
				assert.Exactly(t, wantC.TTL, haveC.TTL, "TTL %s", t.Name())
				assert.Exactly(t, wantC.PageIDSource, haveC.PageIDSource, "PageIDSource %s", t.Name())
				assert.Exactly(t, wantC.AllowedMethods, haveC.AllowedMethods, "AllowedMethods %s", t.Name())
				assert.Exactly(t, wantC.LogFile, haveC.LogFile, "LogFile %s", t.Name())
				assert.Exactly(t, wantC.LogLevel, haveC.LogLevel, "LogLevel %s", t.Name())
				if len(wantC.OnError) > 0 {
					assert.Exactly(t, string(wantC.OnError), string(haveC.OnError), "OnError %s", t.Name())
				}

				assert.Len(t, haveC.KVServices, len(wantC.KVServices), "Index %d %s", j, t.Name())
				for key := range wantC.KVServices {
					_, ok := haveC.KVServices[key]
					assert.True(t, ok, "Index %d %s", j, t.Name())
				}

				assert.Len(t, haveC.Caches, len(wantC.Caches), "Index  %d", j)
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
		nil,
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
		nil,
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
		nil,
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
		nil,
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
		nil,
	))

	t.Run("config with page_id_source", runner(
		`esi {
			page_id_source "pAth,host , IP, header-X-GitHub-Request-Id, header-Server, cookie-__Host-user_session_same_site"
		}`,
		PathConfigs{
			&PathConfig{
				Scope:        "/",
				Timeout:      DefaultTimeOut,
				PageIDSource: []string{"pAth", "host", "IP", "header-X-GitHub-Request-Id", "header-Server", "cookie-__Host-user_session_same_site"},
			},
		},
		nil,
	))

	t.Run("config with page_id_source but errors", runner(
		`esi {
			page_id_source "path,host , ip
		}`,
		nil,
		errors.IsNotValid,
	))

	t.Run("Parse timeout fails", runner(
		`esi {
			timeout Dms
		}`,
		nil,
		errors.IsNotValid,
	))

	t.Run("Invalid KVService URL", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			backend redis//localhost:6379/0
		}`,
		nil,
		errors.IsNotValid,
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
		nil,
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
		nil,
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
		nil,
	))

	t.Run("2x esi directives with different KVServices", runner(
		`esi /catalog/product {
		   timeout 122ms
		   ttl 123ms
		   log_file stderr
		   log_level debug

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
				LogFile:  "stderr",
				LogLevel: "debug",
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
		nil,
	))

	t.Run("Log level info and file stderr", runner(
		`esi /catalog/product {
		   log_file stderr
		   log_level INFO
		}`,
		PathConfigs{
			&PathConfig{
				Scope:    "/catalog/product",
				Timeout:  20 * time.Second,
				LogFile:  "stderr",
				LogLevel: "info",
			},
		},
		nil,
	))

	t.Run("OnError String", runner(
		`esi /catalog/product {
		   on_error "Resource content unavailable"
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/catalog/product",
				Timeout: 20 * time.Second,
				OnError: []byte("Resource content unavailable"),
			},
		},
		nil,
	))

	t.Run("OnError File", runner(
		`esi /catalog/product {
		   on_error "testdata/on_error.txt"
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/catalog/product",
				Timeout: 20 * time.Second,
				OnError: []byte("Output on a backend connection error\n"),
			},
		},
		nil,
	))

	t.Run("OnError File not found", runner(
		`esi / {
		   on_error "testdataXX/on_error.txt"
		}`,
		nil,
		errors.IsFatal,
	))
}

func TestSetupLogger(t *testing.T) {

	buf := new(bytes.Buffer)
	osStdOut = buf
	osStdErr = buf
	defer func() {
		osStdOut = os.Stdout
		osStdErr = os.Stderr
	}()

	runner := func(pc *PathConfig, wantErrBhf errors.BehaviourFunc) func(*testing.T) {
		return func(t *testing.T) {
			haveErr := setupLogger(pc)
			if wantErrBhf != nil {
				assert.True(t, wantErrBhf(haveErr), "%+v", haveErr)
			} else {
				assert.NoError(t, haveErr, "%+v", haveErr)
			}
		}
	}
	pc := &PathConfig{
		LogLevel: "debug",
		LogFile:  "stderr",
	}
	t.Run("Debug Stderr", runner(pc, nil))
	assert.True(t, pc.Log.IsDebug())
	pc.Log.Debug("DebugStdErr", log.String("debug01", "stderr01"))
	assert.Contains(t, buf.String(), `DebugStdErr debug01: "stderr01"`)

	pc = &PathConfig{
		LogLevel: "info",
		LogFile:  "stdout",
	}
	t.Run("Info Stdout", runner(pc, nil))
	assert.True(t, pc.Log.IsInfo())
	pc.Log.Info("InfoStdOut", log.String("info01", "stdout01"))
	assert.Contains(t, buf.String(), `InfoStdOut info01: "stdout01"`)

	pc = &PathConfig{
		LogLevel: "",
		LogFile:  "stdout",
	}
	t.Run("No Log Level, Blackhole", runner(pc, nil))
	assert.False(t, pc.Log.IsInfo())
	assert.False(t, pc.Log.IsDebug())
	pc.Log.Info("NoLogLevelStdOut", log.String("info01", "noLogLevelstdout01"))
	assert.NotContains(t, buf.String(), `NoLogLevelStdOut`)

	pc = &PathConfig{
		LogLevel: "debug",
		LogFile:  "",
	}
	t.Run("No Log File, Blackhole", runner(pc, nil))
	assert.False(t, pc.Log.IsInfo())
	assert.False(t, pc.Log.IsDebug())
	pc.Log.Info("NoLogFileStdOut", log.String("info01", "noLogFilestdout01"))
	assert.NotContains(t, buf.String(), `NoLogFileStdOut`)

	pc = &PathConfig{
		LogLevel: "debug",
		LogFile:  "/root",
	}
	t.Run("Log File open fails", runner(pc, errors.IsFatal))
	assert.Exactly(t, pc.Log.(log.BlackHole), log.BlackHole{})

	tmpFile, clean := esitesting.Tempfile(t)
	defer clean()

	pc = &PathConfig{
		LogLevel: "debug",
		LogFile:  tmpFile,
	}
	t.Run("Log File open success write", runner(pc, nil))
	assert.True(t, pc.Log.IsInfo())
	assert.True(t, pc.Log.IsDebug())
	pc.Log.Info("InfoWriteToTempFile", log.Int("info03", 2412))
	pc.Log.Debug("DebugWriteToTempFile", log.Int("debugo04", 2512))

	tmpFileContent, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, string(tmpFileContent), `InfoWriteToTempFile info03: 2412`)
	assert.Contains(t, string(tmpFileContent), `DebugWriteToTempFile debugo04: 2512`)
}
