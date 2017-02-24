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

// +build esiall esishell

package backend_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitag/backend"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchShellExec(t *testing.T) {
	t.Parallel()

	const stdOutFileName = "testdata/fromGo.txt"

	t.Run("Bash script writes arg1 to a file", func(t *testing.T) {
		defer os.Remove(stdOutFileName)

		rfa := esitag.NewResourceArgs(
			getExternalReqWithExtendedHeaders(),
			"sh://testdata/stdOutToFile.sh",
			esitag.Config{
				Timeout:     5 * time.Second,
				MaxBodySize: 333,
			},
		)
		header, content, err := backend.NewFetchShellExec().DoRequest(rfa)
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, header)
		assert.Exactly(t, []byte{}, content)

		data, err := ioutil.ReadFile(stdOutFileName)
		if err != nil {
			t.Fatal(err)
		}
		assert.Len(t, string(data), 1076)
	})

	t.Run("Bash script writes to stdErr and triggers a fatal error", func(t *testing.T) {

		rfa := esitag.NewResourceArgs(
			getExternalReqWithExtendedHeaders(),
			"sh://testdata/stdErr.sh",
			esitag.Config{
				Timeout:     5 * time.Second,
				MaxBodySize: 333,
			},
		)
		header, content, err := backend.NewFetchShellExec().DoRequest(rfa)
		require.Error(t, err, "%+v", err)
		assert.True(t, errors.IsFatal(err))
		assert.Contains(t, err.Error(), `I'm an evil error`)
		assert.Nil(t, header)
		assert.Nil(t, content)

	})

	t.Run("Bash script writes to stdOut = happy path", func(t *testing.T) {

		rfa := esitag.NewResourceArgs(
			getExternalReqWithExtendedHeaders(),
			"sh://testdata/stdOut.sh",
			esitag.Config{
				Timeout:     5 * time.Second,
				MaxBodySize: 333,
			},
		)
		header, content, err := backend.NewFetchShellExec().DoRequest(rfa)
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, header)
		assert.Contains(t, string(content), `datetime="2017-01-04T20:01:40Z"`)

	})
}

// Benchmark reads from the HDD.
// BenchmarkNewFetchShellExec_Parallel-4   	    1000	   2418130 ns/op	   32713 B/op	     130 allocs/op <- no goroutine
// BenchmarkNewFetchShellExec_Parallel-4   	    1000	   2409384 ns/op	   33137 B/op	     138 allocs/op <- with goroutine
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2591573 ns/op	   34581 B/op	     140 allocs/op
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2563336 ns/op	   33895 B/op	     137 allocs/op
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2702138 ns/op	   76056 B/op	     147 allocs/op no pool
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2567728 ns/op	   33883 B/op	     137 allocs/op pool
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2473684 ns/op	   31218 B/op	     105 allocs/op full path to binary
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2561815 ns/op	   32589 B/op	     108 allocs/op full path + goroutine

func BenchmarkNewFetchShellExec_Parallel(b *testing.B) {
	wantContent, err := ioutil.ReadFile("testdata/cart_example.html")
	if err != nil {
		b.Fatal(err)
	}

	fh := backend.NewFetchShellExec()

	// ProTip: providing the full path to the script/binary reduces lookup time
	// and searching in the env PATH variable. So we use /bin/cat
	rfa := &esitag.ResourceArgs{
		ExternalReq: getExternalReqWithExtendedHeaders(),
		URL:         "sh:///bin/cat testdata/cart_example.html",
		Tag: esitag.Config{
			Timeout:     time.Second,
			MaxBodySize: 22001,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var content []byte
		var hdr http.Header
		var err error

		for pb.Next() {
			hdr, content, err = fh.DoRequest(rfa)
			if err != nil {
				b.Fatalf("%+v", err)
			}
			if hdr != nil {
				b.Fatal("Header should be nil")
			}
			if len(content) != len(wantContent) {
				b.Fatalf("Want %q\nHave %q", wantContent, content)
			}
		}
	})
}
