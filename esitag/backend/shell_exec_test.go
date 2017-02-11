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

package backend_test

import (
	"io/ioutil"
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
		assert.Len(t, string(data), 1047)
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
