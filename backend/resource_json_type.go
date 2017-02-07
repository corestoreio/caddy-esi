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

// +build ignore

package backend

//go:generate easyjson -snake_case -omit_empty resource_json_type.go

// uncomment here, generate and then edit the easyjson file and adjust the types

import (
	"net/http"
	"net/url"
)

// Request used to hack easyjson generation. Type has removed interfaces and
// functions.
type Request struct {
	Method           string
	URL              *url.URL
	Proto            string // "HTTP/1.0"
	ProtoMajor       int    // 1
	ProtoMinor       int    // 0
	Header           http.Header
	ContentLength    int64
	TransferEncoding []string
	Close            bool
	Host             string
	Form             url.Values
	PostForm         url.Values
	// MultipartForm    *multipart.Form
	Trailer    http.Header
	RemoteAddr string
	RequestURI string
}

// ResourceArgs only for easyjson. Same as backend.ResourceArgs but stripped of
// some fields for security reasons.
//easyjson:json
type ResourceArgs struct {
	ExternalReq      *Request
	URL              string
	MaxBodySize      uint64
	Key              string
	ReturnHeaders    []string
	ReturnHeadersAll bool
}
