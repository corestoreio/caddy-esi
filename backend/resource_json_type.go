// +build ignore

package backend

//go:generate easyjson -snake_case -omit_empty resource_json_type.go

// uncomment here, generate and then edit the easyjson file and adjust the types

import (
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Request used to hack easyjson generation. Type has removed interfaces and functions.
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
	MultipartForm    *multipart.Form
	Trailer          http.Header
	RemoteAddr       string
	RequestURI       string
}

// RequestFuncArgs2 only for easyjson
//easyjson:json
type RequestFuncArgs2 struct {
	ExternalReq       *Request
	URL               string
	Timeout           time.Duration
	MaxBodySize       uint64
	Log               log.Logger `json:"-"`
	Key               string
	KeyTemplate       TemplateExecer `json:"-"`
	TTL               time.Duration
	ForwardHeaders    []string
	ForwardHeadersAll bool
	ReturnHeaders     []string
	ReturnHeadersAll  bool
}
