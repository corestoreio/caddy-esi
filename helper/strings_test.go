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

package helper_test

import (
	"testing"

	"github.com/corestoreio/caddy-esi/helper"
	"github.com/stretchr/testify/assert"
)

func TestCommaListToSlice(t *testing.T) {
	t.Parallel()

	assert.Exactly(t,
		[]string{"GET", "POST", "PATCH"},
		helper.CommaListToSlice(`GET , POST, PATCH  `),
	)
	assert.Exactly(t,
		[]string{},
		helper.CommaListToSlice(`   `),
	)
}

func TestStringsToInts(t *testing.T) {
	assert.Exactly(t, []int{300, 400}, helper.StringsToInts([]string{"300", "400"}))
	assert.Exactly(t, []int{300}, helper.StringsToInts([]string{"300", "#"}))
	assert.Exactly(t, []int{}, helper.StringsToInts([]string{"x", "y"}))
	assert.Exactly(t, []int{}, helper.StringsToInts([]string{}))
}
