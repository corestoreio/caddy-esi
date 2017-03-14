// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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

	"github.com/corestoreio/caddy-esi/esicache"
	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/caddy-esi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/stretchr/testify/assert"
)

func testPluginSetup(config string, wantPC PathConfigs, cacheCount int, requestFuncs []string, wantErrBhf errors.BehaviourFunc) func(*testing.T) {
	return func(t *testing.T) {
		defer esicache.MainRegistry.Clear()

		c := caddy.NewTestController("http", config)

		if err := PluginSetup(c); wantErrBhf != nil {
			assert.True(t, wantErrBhf(err), "(%s):\n%+v", t.Name(), err)
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
			t.Fatalf("Expected handler to be type Tag, got: %#v", handler)
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
			rf, ok := esitag.LookupResourceHandler(kvName)
			assert.True(t, ok, "Should have been registered %q", kvName)
			assert.NotNil(t, rf, "Should have a non-nil func %q", kvName)
		}

	}
}

func TestPluginSetup(t *testing.T) {
	t.Parallel()

	t.Run("Default Config", testPluginSetup(
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

	t.Run("config with allowed_methods", testPluginSetup(
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

	t.Run("config with cmd_header_name", testPluginSetup(
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
	t.Run("config with cmd_header_name but value not provided", testPluginSetup(
		`esi {
			cmd_header_name
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	t.Run("config with page_id_source", testPluginSetup(
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

	t.Run("config with page_id_source but errors", testPluginSetup(
		`esi {
			page_id_source
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	t.Run("Parse timeout fails", testPluginSetup(
		`esi {
			timeout Dms
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	t.Run("esi with path to /blog and /guestbook", testPluginSetup(
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

	t.Run("Log level info and file stderr", testPluginSetup(
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

	t.Run("OnError String", testPluginSetup(
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

	t.Run("OnError File", testPluginSetup(
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

	t.Run("OnError File not found", testPluginSetup(
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
