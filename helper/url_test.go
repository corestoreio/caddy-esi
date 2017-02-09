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

package helper_test

import (
	"net/url"
	"testing"

	"github.com/SchumacherFM/caddyesi/helper"
	"github.com/stretchr/testify/assert"
)

func TestParseNoSQLURL(t *testing.T) {
	t.Parallel()

	var defaultPoolConnectionParameters = map[string][]string{
		"db":           {"0"},
		"max_active":   {"10"},
		"max_idle":     {"400"},
		"idle_timeout": {"240s"},
		"cancellable":  {"0"},
	}

	runner := func(raw string, wantAddress string, wantPassword string, wantParams url.Values, wantErr bool) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			haveAddress, havePW, params, haveErr := helper.ParseNoSQLURL(raw)
			if wantErr {
				if have, want := wantErr, haveErr != nil; have != want {
					t.Errorf("(%q)\nError: Have: %v Want: %v\n%+v", t.Name(), have, want, haveErr)
				}
				return
			}

			if haveErr != nil {
				t.Errorf("(%q) Did not expect an Error: %+v", t.Name(), haveErr)
			}

			if have, want := haveAddress, wantAddress; have != want {
				t.Errorf("(%q) Address: Have: %v Want: %v", t.Name(), have, want)
			}
			if have, want := havePW, wantPassword; have != want {
				t.Errorf("(%q) Password: Have: %v Want: %v", t.Name(), have, want)
			}
			if wantParams == nil {
				wantParams = defaultPoolConnectionParameters
			}

			for k := range wantParams {
				assert.Exactly(t, wantParams[k], params[k], "Test %q Parameter %q", t.Name(), k)
			}
		}
	}
	t.Run("invalid redis URL scheme none", runner("localhost", "", "", nil, true))
	t.Run("invalid redis URL scheme http", runner("http://www.google.com", "", "", nil, true))
	t.Run("invalid redis URL string", runner("redis://weird url", "", "", nil, true))
	t.Run("too many colons in URL", runner("redis://foo:bar:baz", "", "", nil, true))
	t.Run("ignore path in URL", runner("redis://localhost:6379/abc123", "localhost:6379", "", nil, false))
	t.Run("URL contains only scheme", runner("redis://", "localhost:6379", "", nil, false))

	t.Run("set DB with hostname", runner(
		"redis://localh0Rst:6379/?db=123",
		"localh0Rst:6379",
		"",
		map[string][]string{
			"db":           {"123"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
			"scheme":       {"redis"},
		},
		false))
	t.Run("set DB without hostname", runner(
		"redis://:6379/?db=345",
		"localhost:6379",
		"",
		map[string][]string{
			"db":           {"345"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
			"scheme":       {"redis"},
		},
		false))
	t.Run("URL contains IP address", runner(
		"redis://192.168.0.234/?db=123",
		"192.168.0.234:6379",
		"",
		map[string][]string{
			"db":           {"123"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
			"scheme":       {"redis"},
		},
		false))
	t.Run("URL contains password", runner(
		"redis://empty:SuperSecurePa55w0rd@192.168.0.234/?db=3",
		"192.168.0.234:6379",
		"SuperSecurePa55w0rd",
		map[string][]string{
			"db":           {"3"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
			"scheme":       {"redis"},
		},
		false))
	t.Run("Apply all params", runner(
		"redis://empty:SuperSecurePa55w0rd@192.168.0.234/?db=4&max_active=2718&max_idle=3141&idle_timeout=5h3s&cancellable=1",
		"192.168.0.234:6379",
		"SuperSecurePa55w0rd",
		map[string][]string{
			"db":           {"4"},
			"max_active":   {"2718"},
			"max_idle":     {"3141"},
			"idle_timeout": {"5h3s"},
			"cancellable":  {"1"},
			"scheme":       {"redis"},
		},
		false))
	t.Run("Memcache default", runner(
		"memcache://",
		"localhost:11211",
		"",
		map[string][]string{
			"scheme": {"memcache"},
		},
		false))
	t.Run("Memcache default with additional servers", runner(
		"memcache://?server=localhost:11212&server=localhost:11213",
		"localhost:11211",
		"",
		map[string][]string{
			"scheme": {"memcache"},
			"server": {"localhost:11212", "localhost:11213"},
		},
		false))
	t.Run("Memcache custom port", runner(
		"memcache://192.123.432.232:334455",
		"192.123.432.232:334455",
		"",
		map[string][]string{
			"scheme": {"memcache"},
		},
		false))
	t.Run("GRPC no port", runner(
		"grpc://192.123.432.232",
		"",
		"",
		nil,
		true))
	t.Run("GRPC port", runner(
		"grpc://192.123.432.232:33",
		"192.123.432.232:33",
		"",
		map[string][]string{
			"scheme": {"grpc"},
		},
		false))
}
