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
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esicache"
	"github.com/SchumacherFM/caddyesi/esikv"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/alicebob/miniredis"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

func TestPluginSetup(t *testing.T) {
	t.Parallel()

	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	runner := func(config string, wantPC PathConfigs, cacheCount int, requestFuncs []string, wantErrBhf errors.BehaviourFunc) func(*testing.T) {
		return func(t *testing.T) {
			defer esicache.MainRegistry.Clear()

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

				assert.Exactly(t, cacheCount, esicache.MainRegistry.Len(haveC.Scope), "Mismatch esicache.MainRegistry.Len")
			}

			for _, kvName := range requestFuncs {
				rf, ok := backend.LookupResourceHandler(kvName)
				assert.True(t, ok, "Should have been registered %q", kvName)
				assert.NotNil(t, rf, "Should have a non-nil func %q", kvName)
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
		0,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("With timeout, ttl and 1x Cacher", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://`+mr.Addr()+`/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		1,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("With timeout, ttl and 2x Cacher", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://`+mr.Addr()+`/0
			cache redis://`+mr.Addr()+`/1
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		2,   // cache length
		nil, // kv services []string
		nil,
	))

	esiCfg, clean := esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/0`, "backend"),
	})
	defer clean()
	t.Run("With timeout, ttl and resources", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0,                   // cache length
		[]string{"backend"}, // kv services []string
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
		0,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("config with cmd_header_name", runner(
		`esi {
			cmd_header_name X-Esi-CMD
		}`,
		PathConfigs{
			&PathConfig{
				Scope:         "/",
				Timeout:       DefaultTimeOut,
				CmdHeaderName: `X-Esi-Cmd`,
			},
		},
		0,   // cache length
		nil, // kv services []string
		nil,
	))
	t.Run("config with cmd_header_name but value not provided", runner(
		`esi {
			cmd_header_name
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
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
		0,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("config with page_id_source but errors", runner(
		`esi {
			page_id_source
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	t.Run("Parse timeout fails", runner(
		`esi {
			timeout Dms
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis//`+mr.Addr()+`/0`, "backend"), // missing colon
	})
	defer clean()
	t.Run("Invalid KVService URL", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/0`, "backend"),
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/1`, "backend"),
	})
	defer clean()
	t.Run("Overwrite duplicate key in KVServices", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+` 
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0,                   // cache length
		[]string{"backend"}, // kv services []string
		nil,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/0`, "redis1"),
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/1`, "redis2"),
	})
	defer clean()
	t.Run("Create two KVServices", runner(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0, // cache length
		[]string{"redis1", "redis2"}, // kv services []string
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
		0,   // cache length
		nil, // kv services []string
		nil,
	))

	esiCfg1, clean := esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/2`, "redisAWS1"),
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/3`, "redisLocal1"),
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/1`, "redisLocal2"),
	})
	defer clean()
	esiCfg2, clean := esitesting.WriteXMLTempFile(t, esikv.ConfigItems{
		esikv.NewConfigItem(`redis://`+mr.Addr()+`/3`, "redisLocal3"),
	})
	defer clean()
	t.Run("2x esi directives with different KVServices", runner(
		`esi /catalog/product {
		   timeout 122ms
		   ttl 123ms
		   log_file stderr
		   log_level debug
		   resources `+esiCfg1+`
		}
		esi /checkout/cart {
		   timeout 131ms
		   ttl 132ms
			resources `+esiCfg2+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:    "/catalog/product",
				Timeout:  time.Millisecond * 122,
				TTL:      time.Millisecond * 123,
				LogFile:  "stderr",
				LogLevel: "debug",
			},
			&PathConfig{
				Scope:   "/checkout/cart",
				Timeout: time.Millisecond * 131,
				TTL:     time.Millisecond * 132,
			},
		},
		0, // cache length
		[]string{"redisAWS1", "redisLocal1", "redisLocal2", "redisLocal3"}, // kv services []string
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
		0,   // cache length
		nil, // kv services []string
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
		0,   // cache length
		nil, // kv services []string
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
		0,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("OnError File not found", runner(
		`esi / {
		   on_error "testdataXX/on_error.txt"
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
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
	assert.Contains(t, buf.String(), `"msg":"DebugStdErr","debug01":"stderr01"`)

	pc = &PathConfig{
		LogLevel: "info",
		LogFile:  "stdout",
	}
	t.Run("Info Stdout", runner(pc, nil))
	assert.True(t, pc.Log.IsInfo(), "Loglevel should be info")
	pc.Log.Info("InfoStdOut", log.String("info01", "stdout01"))
	assert.Contains(t, buf.String(), `InfoStdOut","info01":"stdout01"`)

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
	assert.Contains(t, string(tmpFileContent), `"msg":"InfoWriteToTempFile","info03":2412`)
	assert.Contains(t, string(tmpFileContent), `"msg":"DebugWriteToTempFile","debugo04":2512`)
}
