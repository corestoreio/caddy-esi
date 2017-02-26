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

package esitag_test

import (
	"fmt"
	"testing"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/stretchr/testify/assert"
)

var _ fmt.Stringer = (*esitag.DataTag)(nil)
var _ fmt.Stringer = (*esitag.DataTags)(nil)

func TestDataTags_String(t *testing.T) {
	t.Parallel()

	tags := esitag.DataTags{
		esitag.DataTag{
			Data:  []byte(`Content "testE2b://micro2.service2" Timeout 2s MaxBody 3.0 kB`),
			Start: 100,
			End:   200,
		},
		esitag.DataTag{
			Data:  []byte(`Content "testE1b://micro2.service2" Timeout 3s MaxBody 5.0 kB`),
			Start: 1000,
			End:   2000,
		},
	}
	assert.Exactly(t,
		"IDX(1/2): Start:000100 End:000200 Tag:\"Content \\\"testE2b://micro2.service2\\\" Timeout 2s MaxBody 3.0 kB\"\nIDX(2/2): Start:001000 End:002000 Tag:\"Content \\\"testE1b://micro2.service2\\\" Timeout 3s MaxBody 5.0 kB\"\n",
		tags.String())
}
