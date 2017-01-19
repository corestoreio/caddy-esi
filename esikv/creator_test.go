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

package esikv

import (
	"testing"

	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewResourceHandler_Mock(t *testing.T) {
	rh, err := NewResourceHandler("mockTimeout://4s")
	assert.NoError(t, err)
	_, ok := rh.(resourceMock)
	assert.True(t, ok, "It should be type resourceMock")

	n1, n2, err := rh.DoRequest(nil)
	assert.Nil(t, n1)
	assert.Nil(t, n2)
	assert.True(t, errors.IsTimeout(err), "Error should have behaviour timeout: %+v", err)
}
