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

package caddyesi_test

import (
	"bytes"
	"testing"

	"github.com/corestoreio/caddy-esi"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

func TestConfigUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("File extension not supported", func(t *testing.T) {
		items, err := caddyesi.UnmarshalResourceItems("./testdata/config_01.txt")
		assert.True(t, errors.IsNotSupported(err))
		assert.Nil(t, items, "Items should be nil")
	})

	t.Run("File not found", func(t *testing.T) {
		data, err := caddyesi.UnmarshalResourceItems("./testdata/config_99.xml")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})
	t.Run("XML unmarshalling failed", func(t *testing.T) {
		data, err := caddyesi.UnmarshalResourceItems("./testdata/config_00.xml")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})
	t.Run("JSON unmarshalling failed", func(t *testing.T) {
		data, err := caddyesi.UnmarshalResourceItems("./testdata/config_00.json")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})
	t.Run("Unknown content type", func(t *testing.T) {
		data, err := caddyesi.UnmarshalResourceItems("this is a text file")
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
		assert.Nil(t, data, "Data should be nil")
	})

	t.Run("Load XML and JSON which must be equal", func(t *testing.T) {

		var want = caddyesi.ResourceItems{
			caddyesi.NewResourceItem(
				"redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4",
				"redis01",
			),
			caddyesi.NewResourceItem(
				"grpc://127.0.0.1:53044/?pem=../path/to/root.pem",
				"grpc01",
			),
			&caddyesi.ResourceItem{
				Alias: "mysql01",
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT `value` FROM tableX WHERE key='?'",
			},
			caddyesi.NewResourceItem(
				// the alias mysql01 got resolved to the correct URL data
				"user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				"mysql02",
				"SELECT `value` FROM tableY WHERE another_key=?",
			),
		}

		xmlI, err := caddyesi.UnmarshalResourceItems("./testdata/config_01.xml")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		jsonI, err := caddyesi.UnmarshalResourceItems("./testdata/config_01.json")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, xmlI, jsonI, "Both unmarshalled slices are not equal")
		assert.Exactly(t, want, jsonI, "JSON unmarshalling failed")
		assert.Exactly(t, want, xmlI)
	})

	t.Run("Load XML and JSON from string", func(t *testing.T) {

		var want = caddyesi.ResourceItems{
			&caddyesi.ResourceItem{
				Alias: "redis01",
				URL:   "redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4",
				Query: "",
			},
			&caddyesi.ResourceItem{
				Alias: "mysql01",
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT value FROM tableX WHERE key='?'",
			},
		}

		xmlI, err := caddyesi.UnmarshalResourceItems(`<?xml version="1.0"?>
<items>
    <item>
        <alias>redis01</alias>
        <url><![CDATA[redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4]]></url>
        <!--<query>Unused and hence optional</query>-->
    </item>
    <item>
        <alias>mysql01</alias>
        <url><![CDATA[user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify]]></url>
        <query><![CDATA[SELECT value FROM tableX WHERE key='?']]></query>
    </item>
</items>
`)
		if err != nil {
			t.Fatalf("%+v", err)
		}
		jsonI, err := caddyesi.UnmarshalResourceItems(`[
  {
    "alias": "redis01",
    "url": "redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4"
  },
  {
    "alias": "mysql01",
    "url": "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
    "query": "SELECT value FROM tableX WHERE key='?'"
  }
]
`)
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, xmlI, jsonI, "Both unmarshalled slices are not equal")
		assert.Exactly(t, want, jsonI, "JSON unmarshalling failed")
		assert.Exactly(t, want, xmlI)
	})

	t.Run("ResourceItems to XML String", func(t *testing.T) {
		items := caddyesi.ResourceItems{
			&caddyesi.ResourceItem{
				Alias: "redis01",
				URL:   "redis://127.0.0.1:6379/?db=0&max_active=10&max_idle=4",
			},
			&caddyesi.ResourceItem{
				Alias: "grpc01",
				URL:   "grpc://127.0.0.1:53044/?pem=../path/to/root.pem",
			},
			&caddyesi.ResourceItem{
				Alias: "mysql01",
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT `value` FROM tableX WHERE key='?'",
			},
			&caddyesi.ResourceItem{
				Alias: "mysql02",
				// the alias mysql-1 got resolved to the correct URL data
				URL:   "user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&tls=skip-verify",
				Query: "SELECT `value` FROM tableY WHERE another_key<>?",
			},
		}

		const wantXML = "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?><items><item><alias>redis01</alias><url>redis://127.0.0.1:6379/?db=0&amp;max_active=10&amp;max_idle=4</url></item><item><alias>grpc01</alias><url>grpc://127.0.0.1:53044/?pem=../path/to/root.pem</url></item><item><alias>mysql01</alias><url>user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&amp;tls=skip-verify</url><query>SELECT `value` FROM tableX WHERE key=&#39;?&#39;</query></item><item><alias>mysql02</alias><url>user:password@tcp(localhost:5555)/dbname?charset=utf8mb4,utf8&amp;tls=skip-verify</url><query>SELECT `value` FROM tableY WHERE another_key&lt;&gt;?</query></item></items>"
		haveXML := items.MustToXML()
		assert.Exactly(t, wantXML, haveXML)

		var buf bytes.Buffer
		written, err := items.WriteTo(&buf)
		assert.Exactly(t, int64(0), written)
		assert.NoError(t, err, "%+v", err)

		items2, err := caddyesi.UnmarshalResourceItems(haveXML)
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, items, items2, "Encoded items should be equal to decoded items2")
	})
}
