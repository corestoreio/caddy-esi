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

package esigrpc

import (
	"net/http"

	"github.com/corestoreio/errors"
)

// StringSliceToHeader converts the balanced slice "parts" into an http.Header.
// It returns an error if the parts slice isn't balanced.
func StringSliceToHeader(parts ...string) (http.Header, error) {
	if len(parts)%2 == 1 {
		return nil, errors.NotValid.Newf("[esigrpc] Slice %v not balanced", parts)
	}
	h := http.Header{}
	for i := 0; i < len(parts); i = i + 2 {
		h.Add(parts[i], parts[i+1])
	}
	return h, nil
}
