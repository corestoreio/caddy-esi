// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package esigrpc_test

import (
	"net/http"
	"testing"

	"github.com/corestoreio/caddy-esi/esitag/backend/esigrpc"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

func TestStringSliceToHeader(t *testing.T) {
	t.Run("imbalanaced", func(t *testing.T) {
		x := []string{"X-Content-Value"}
		h, err := esigrpc.StringSliceToHeader(x...)
		assert.Nil(t, h)
		assert.True(t, errors.NotValid.Match(err), "%+v", err)
	})
	t.Run("balanaced", func(t *testing.T) {
		x := []string{"X-Content-Value", "a", "X-Content-Value", "b"}
		h, err := esigrpc.StringSliceToHeader(x...)
		assert.NoError(t, err, "%+v", err)
		assert.Exactly(t, http.Header{"X-Content-Value": []string{"a", "b"}}, h)
	})
}
