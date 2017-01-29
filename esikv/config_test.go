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

package esikv_test

import (
	"testing"

	"github.com/SchumacherFM/caddyesi/esikv"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

func TestConfigUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("File extension not supported", func(t *testing.T) {
		items, err := esikv.ConfigUnmarshal("./testdata/config_01.txt")
		assert.True(t, errors.IsNotSupported(err))
		assert.Nil(t, items, "Items should be nil")
	})

	t.Run("XML unmarshalling failed", func(t *testing.T) {
		data, err := esikv.ConfigUnmarshal("./testdata/config_00.xml")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})
	t.Run("JSON unmarshalling failed", func(t *testing.T) {
		data, err := esikv.ConfigUnmarshal("./testdata/config_00.json")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})

	t.Run("Load XML and JSON which must be equal", func(t *testing.T) {

		var want = esikv.ConfigItems{
			&esikv.ConfigItem{
				Alias: "redis01",
				URL:   "redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4",
				Query: "",
			},
			&esikv.ConfigItem{
				Alias: "grpc01",
				URL:   "grpc://127.0.0.1:53044/?pem=../path/to/root.pem",
				Query: "",
			},
			&esikv.ConfigItem{
				Alias: "mysql01",
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT `value` FROM tableX WHERE key='?'",
			},
			&esikv.ConfigItem{
				Alias: "mysql02",
				// the alias mysql-1 got resolved to the correct URL data
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT `value` FROM tableY WHERE another_key=?",
			},
		}

		xmlI, err := esikv.ConfigUnmarshal("./testdata/config_01.xml")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		jsonI, err := esikv.ConfigUnmarshal("./testdata/config_01.json")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, xmlI, jsonI, "Both unmarshalled slices are not equal")
		assert.Exactly(t, want, jsonI, "JSON unmarshalling failed")
		assert.Exactly(t, want, xmlI)

	})

}
