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

package esitag_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/log"
	"github.com/stretchr/testify/assert"
)

func TestResourceArgs_JSON(t *testing.T) {
	t.Parallel()

	rfa := esitag.ResourceArgs{
		URL:         "https://micro.service/cart?id=3",
		ExternalReq: httptest.NewRequest("PATCH", "/", strings.NewReader("URL encoded body")),
		Tag: esitag.Config{
			Log:               log.BlackHole{},
			ForwardHeaders:    []string{"fwd-header1", "fwd-header2"},
			ReturnHeaders:     []string{"ret-header1", "ret-header2"},
			ForwardPostData:   true,
			ForwardHeadersAll: true,
			ReturnHeadersAll:  true,
			Timeout:           2 * time.Second,
			TTL:               4 * time.Second,
			MaxBodySize:       1e7,
			Key:               "a key stored in redis",
			Coalesce:          true,
			PrintDebug:        true,
		},
	}
	rfa.ExternalReq.Header = make(http.Header)
	rfa.ExternalReq.Header.Set("X-Content-ESI", "My-Tag") // only value because a map does not guarantee the order

	rfa.ExternalReq.Trailer = make(http.Header)
	rfa.ExternalReq.Trailer.Set("X-Trailer-ESI", "My-Nacht") // jokes are getting worse :-(

	rfa.ExternalReq.TransferEncoding = []string{"UTF-9", "UTF-15"}
	rfa.ExternalReq.Close = true

	rfa.ExternalReq.Form = make(url.Values)
	rfa.ExternalReq.Form.Set("input1", "val1") // only value because a map does not guarantee the order

	rfa.ExternalReq.PostForm = make(url.Values)
	rfa.ExternalReq.PostForm.Set("input3", "val3") // only value because a map does not guarantee the order

	jData, err := rfa.MarshalJSON()
	if err != nil {
		t.Fatalf("%+v", err)
	}

	const wantJSON = "{\"external_req\":{\"method\":\"PATCH\",\"url\":{\"path\":\"/\"},\"proto\":\"HTTP/1.1\",\"proto_major\":1,\"proto_minor\":1,\"header\":{\"X-Content-Esi\":[\"My-Tag\"]},\"content_length\":16,\"transfer_encoding\":[\"UTF-9\",\"UTF-15\"],\"close\":true,\"host\":\"example.com\",\"form\":{\"input1\":[\"val1\"]},\"post_form\":{\"input3\":[\"val3\"]},\"trailer\":{\"X-Trailer-Esi\":[\"My-Nacht\"]},\"remote_addr\":\"192.0.2.1:1234\",\"request_uri\":\"/\",\"body\":\"VE9ETyByZWFkIGJvZHk=\"},\"url\":\"https://micro.service/cart?id=3\",\"tag\":{\"forward_headers\":[\"fwd-header1\",\"fwd-header2\"],\"return_headers\":[\"ret-header1\",\"ret-header2\"],\"forward_post_data\":true,\"forward_headers_all\":true,\"return_headers_all\":true,\"timeout\":2000000000,\"ttl\":4000000000,\"max_body_size\":10000000,\"key\":\"a key stored in redis\",\"coalesce\":true,\"print_debug\":true}}"
	assert.Exactly(t, wantJSON, string(jData))

	rfa2 := new(esitag.ResourceArgs)
	if err := rfa2.UnmarshalJSON(jData); err != nil {
		t.Fatalf("%+v", err)
	}

	//wantRFA2 := &esitag.ResourceArgs{
	//	URL:         "https://micro.service/cart?id=3",
	//	ExternalReq: httptest.NewRequest("PATCH", "/", nil),
	//	Tag: esitag.Config{
	//		ForwardHeaders:    []string{"fwd-header1", "fwd-header2"},
	//		ReturnHeaders:     []string{"ret-header1", "ret-header2"},
	//		ForwardPostData:   true,
	//		ForwardHeadersAll: true,
	//		ReturnHeadersAll:  true,
	//		Timeout:           2 * time.Second,
	//		TTL:               4 * time.Second,
	//		MaxBodySize:       1e7,
	//		Key:               "a key stored in redis",
	//		Coalesce:          true,
	//		PrintDebug:        true,
	//	},
	//}

	have := fmt.Sprintf("%+v", rfa2)

	shouldContain := [...]string{
		`URL:https://micro.service/cart?id=3`,
		`ForwardHeaders:[fwd-header1 fwd-header2]`,
		`ReturnHeaders:[ret-header1 ret-header2]`,
		`ForwardPostData:true`,
		`ForwardHeadersAll:true `,
		`Log:<nil>`,
		`ReturnHeadersAll:true`,
		`Timeout:2s`,
		`TTL:4s`,
		`MaxBodySize:10000000`,
		`Key:a key stored in redis`,
		`Coalesce:true`,
		`PrintDebug:true`,
	}
	for _, contains := range shouldContain {
		assert.Contains(t, have, contains)
	}
}
