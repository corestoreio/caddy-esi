// PARTIALLY AUTOGENERATED FILE: easyjson marshaller/unmarshallers.

package backend

import (
	multipart "mime/multipart"
	http "net/http"
	textproto "net/textproto"
	url "net/url"
	time "time"

	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *jlexer.Lexer
	_ *jwriter.Writer
)

func easyjson1688e6a4DecodeGithubComSchumacherFMCaddyesiBackend(in *jlexer.Lexer, out *RequestFuncArgs) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "external_req":
			if in.IsNull() {
				in.Skip()
				out.ExternalReq = nil
			} else {
				if out.ExternalReq == nil {
					out.ExternalReq = new(http.Request)
				}
				easyjson1688e6a4DecodeGithubComSchumacherFMCaddyesiBackend1(in, &*out.ExternalReq)
			}
		case "url":
			out.URL = string(in.String())
		case "timeout":
			out.Timeout = time.Duration(in.Int64())
		case "max_body_size":
			out.MaxBodySize = uint64(in.Uint64())
		case "key":
			out.Key = string(in.String())
		case "ttl":
			out.TTL = time.Duration(in.Int64())
		case "forward_headers":
			if in.IsNull() {
				in.Skip()
				out.ForwardHeaders = nil
			} else {
				in.Delim('[')
				if !in.IsDelim(']') {
					out.ForwardHeaders = make([]string, 0, 4)
				} else {
					out.ForwardHeaders = []string{}
				}
				for !in.IsDelim(']') {
					var v1 string
					v1 = string(in.String())
					out.ForwardHeaders = append(out.ForwardHeaders, v1)
					in.WantComma()
				}
				in.Delim(']')
			}
		case "forward_headers_all":
			out.ForwardHeadersAll = bool(in.Bool())
		case "return_headers":
			if in.IsNull() {
				in.Skip()
				out.ReturnHeaders = nil
			} else {
				in.Delim('[')
				if !in.IsDelim(']') {
					out.ReturnHeaders = make([]string, 0, 4)
				} else {
					out.ReturnHeaders = []string{}
				}
				for !in.IsDelim(']') {
					var v2 string
					v2 = string(in.String())
					out.ReturnHeaders = append(out.ReturnHeaders, v2)
					in.WantComma()
				}
				in.Delim(']')
			}
		case "return_headers_all":
			out.ReturnHeadersAll = bool(in.Bool())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeGithubComSchumacherFMCaddyesiBackend(out *jwriter.Writer, in RequestFuncArgs) {
	out.RawByte('{')
	first := true
	_ = first
	if in.ExternalReq != nil {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"external_req\":")
		if in.ExternalReq == nil {
			out.RawString("null")
		} else {
			easyjson1688e6a4EncodeGithubComSchumacherFMCaddyesiBackend1(out, *in.ExternalReq)
		}
	}
	if in.URL != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"url\":")
		out.String(string(in.URL))
	}
	if in.Timeout != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"timeout\":")
		out.Int64(int64(in.Timeout))
	}
	if in.MaxBodySize != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"max_body_size\":")
		out.Uint64(uint64(in.MaxBodySize))
	}
	if in.Key != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"key\":")
		out.String(string(in.Key))
	}
	if in.TTL != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"ttl\":")
		out.Int64(int64(in.TTL))
	}
	if len(in.ForwardHeaders) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"forward_headers\":")
		if in.ForwardHeaders == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v3, v4 := range in.ForwardHeaders {
				if v3 > 0 {
					out.RawByte(',')
				}
				out.String(string(v4))
			}
			out.RawByte(']')
		}
	}
	if in.ForwardHeadersAll {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"forward_headers_all\":")
		out.Bool(bool(in.ForwardHeadersAll))
	}
	if len(in.ReturnHeaders) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"return_headers\":")
		if in.ReturnHeaders == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v5, v6 := range in.ReturnHeaders {
				if v5 > 0 {
					out.RawByte(',')
				}
				out.String(string(v6))
			}
			out.RawByte(']')
		}
	}
	if in.ReturnHeadersAll {
		if !first {
			out.RawByte(',')
		}
		//first = false
		out.RawString("\"return_headers_all\":")
		out.Bool(bool(in.ReturnHeadersAll))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v RequestFuncArgs) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson1688e6a4EncodeGithubComSchumacherFMCaddyesiBackend(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v RequestFuncArgs) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson1688e6a4EncodeGithubComSchumacherFMCaddyesiBackend(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *RequestFuncArgs) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson1688e6a4DecodeGithubComSchumacherFMCaddyesiBackend(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *RequestFuncArgs) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson1688e6a4DecodeGithubComSchumacherFMCaddyesiBackend(l, v)
}
func easyjson1688e6a4DecodeGithubComSchumacherFMCaddyesiBackend1(in *jlexer.Lexer, out *http.Request) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "method":
			out.Method = string(in.String())
		case "url":
			if in.IsNull() {
				in.Skip()
				out.URL = nil
			} else {
				if out.URL == nil {
					out.URL = new(url.URL)
				}
				easyjson1688e6a4DecodeNetUrl(in, &*out.URL)
			}
		case "proto":
			out.Proto = string(in.String())
		case "proto_major":
			out.ProtoMajor = int(in.Int())
		case "proto_minor":
			out.ProtoMinor = int(in.Int())
		case "header":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.Header = make(http.Header)
				} else {
					out.Header = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v7 []string
					if in.IsNull() {
						in.Skip()
						v7 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v7 = make([]string, 0, 4)
						} else {
							v7 = []string{}
						}
						for !in.IsDelim(']') {
							var v8 string
							v8 = string(in.String())
							v7 = append(v7, v8)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.Header)[key] = v7
					in.WantComma()
				}
				in.Delim('}')
			}
		case "content_length":
			out.ContentLength = int64(in.Int64())
		case "transfer_encoding":
			if in.IsNull() {
				in.Skip()
				out.TransferEncoding = nil
			} else {
				in.Delim('[')
				if !in.IsDelim(']') {
					out.TransferEncoding = make([]string, 0, 4)
				} else {
					out.TransferEncoding = []string{}
				}
				for !in.IsDelim(']') {
					var v9 string
					v9 = string(in.String())
					out.TransferEncoding = append(out.TransferEncoding, v9)
					in.WantComma()
				}
				in.Delim(']')
			}
		case "close":
			out.Close = bool(in.Bool())
		case "host":
			out.Host = string(in.String())
		case "form":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.Form = make(url.Values)
				} else {
					out.Form = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v10 []string
					if in.IsNull() {
						in.Skip()
						v10 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v10 = make([]string, 0, 4)
						} else {
							v10 = []string{}
						}
						for !in.IsDelim(']') {
							var v11 string
							v11 = string(in.String())
							v10 = append(v10, v11)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.Form)[key] = v10
					in.WantComma()
				}
				in.Delim('}')
			}
		case "post_form":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.PostForm = make(url.Values)
				} else {
					out.PostForm = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v12 []string
					if in.IsNull() {
						in.Skip()
						v12 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v12 = make([]string, 0, 4)
						} else {
							v12 = []string{}
						}
						for !in.IsDelim(']') {
							var v13 string
							v13 = string(in.String())
							v12 = append(v12, v13)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.PostForm)[key] = v12
					in.WantComma()
				}
				in.Delim('}')
			}
		case "multipart_form":
			if in.IsNull() {
				in.Skip()
				out.MultipartForm = nil
			} else {
				if out.MultipartForm == nil {
					out.MultipartForm = new(multipart.Form)
				}
				easyjson1688e6a4DecodeMimeMultipart(in, &*out.MultipartForm)
			}
		case "trailer":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.Trailer = make(http.Header)
				} else {
					out.Trailer = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v14 []string
					if in.IsNull() {
						in.Skip()
						v14 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v14 = make([]string, 0, 4)
						} else {
							v14 = []string{}
						}
						for !in.IsDelim(']') {
							var v15 string
							v15 = string(in.String())
							v14 = append(v14, v15)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.Trailer)[key] = v14
					in.WantComma()
				}
				in.Delim('}')
			}
		case "remote_addr":
			out.RemoteAddr = string(in.String())
		case "request_uri":
			out.RequestURI = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeGithubComSchumacherFMCaddyesiBackend1(out *jwriter.Writer, in http.Request) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Method != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"method\":")
		out.String(string(in.Method))
	}
	if in.URL != nil {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"url\":")
		if in.URL == nil {
			out.RawString("null")
		} else {
			easyjson1688e6a4EncodeNetUrl(out, *in.URL)
		}
	}
	if in.Proto != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"proto\":")
		out.String(string(in.Proto))
	}
	if in.ProtoMajor != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"proto_major\":")
		out.Int(int(in.ProtoMajor))
	}
	if in.ProtoMinor != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"proto_minor\":")
		out.Int(int(in.ProtoMinor))
	}
	if len(in.Header) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"header\":")
		if in.Header == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v16First := true
			for v16Name, v16Value := range in.Header {
				if !v16First {
					out.RawByte(',')
				}
				v16First = false
				out.String(string(v16Name))
				out.RawByte(':')
				if v16Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v17, v18 := range v16Value {
						if v17 > 0 {
							out.RawByte(',')
						}
						out.String(string(v18))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	if in.ContentLength != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"content_length\":")
		out.Int64(int64(in.ContentLength))
	}
	if len(in.TransferEncoding) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"transfer_encoding\":")
		if in.TransferEncoding == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v19, v20 := range in.TransferEncoding {
				if v19 > 0 {
					out.RawByte(',')
				}
				out.String(string(v20))
			}
			out.RawByte(']')
		}
	}
	if in.Close {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"close\":")
		out.Bool(bool(in.Close))
	}
	if in.Host != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"host\":")
		out.String(string(in.Host))
	}
	if len(in.Form) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"form\":")
		if in.Form == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v21First := true
			for v21Name, v21Value := range in.Form {
				if !v21First {
					out.RawByte(',')
				}
				v21First = false
				out.String(string(v21Name))
				out.RawByte(':')
				if v21Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v22, v23 := range v21Value {
						if v22 > 0 {
							out.RawByte(',')
						}
						out.String(string(v23))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	if len(in.PostForm) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"post_form\":")
		if in.PostForm == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v24First := true
			for v24Name, v24Value := range in.PostForm {
				if !v24First {
					out.RawByte(',')
				}
				v24First = false
				out.String(string(v24Name))
				out.RawByte(':')
				if v24Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v25, v26 := range v24Value {
						if v25 > 0 {
							out.RawByte(',')
						}
						out.String(string(v26))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	if in.MultipartForm != nil {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"multipart_form\":")
		if in.MultipartForm == nil {
			out.RawString("null")
		} else {
			easyjson1688e6a4EncodeMimeMultipart(out, *in.MultipartForm)
		}
	}
	if len(in.Trailer) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"trailer\":")
		if in.Trailer == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v27First := true
			for v27Name, v27Value := range in.Trailer {
				if !v27First {
					out.RawByte(',')
				}
				v27First = false
				out.String(string(v27Name))
				out.RawByte(':')
				if v27Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v28, v29 := range v27Value {
						if v28 > 0 {
							out.RawByte(',')
						}
						out.String(string(v29))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	if in.RemoteAddr != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"remote_addr\":")
		out.String(string(in.RemoteAddr))
	}
	if in.RequestURI != "" {
		if !first {
			out.RawByte(',')
		}
		//first = false
		out.RawString("\"request_uri\":")
		out.String(string(in.RequestURI))
	}
	out.RawByte('}')
}
func easyjson1688e6a4DecodeMimeMultipart(in *jlexer.Lexer, out *multipart.Form) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "value":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.Value = make(map[string][]string)
				} else {
					out.Value = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v30 []string
					if in.IsNull() {
						in.Skip()
						v30 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v30 = make([]string, 0, 4)
						} else {
							v30 = []string{}
						}
						for !in.IsDelim(']') {
							var v31 string
							v31 = string(in.String())
							v30 = append(v30, v31)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.Value)[key] = v30
					in.WantComma()
				}
				in.Delim('}')
			}
		case "file":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.File = make(map[string][]*multipart.FileHeader)
				} else {
					out.File = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v32 []*multipart.FileHeader
					if in.IsNull() {
						in.Skip()
						v32 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v32 = make([]*multipart.FileHeader, 0, 8)
						} else {
							v32 = []*multipart.FileHeader{}
						}
						for !in.IsDelim(']') {
							var v33 *multipart.FileHeader
							if in.IsNull() {
								in.Skip()
								v33 = nil
							} else {
								if v33 == nil {
									v33 = new(multipart.FileHeader)
								}
								easyjson1688e6a4DecodeMimeMultipart1(in, &*v33)
							}
							v32 = append(v32, v33)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.File)[key] = v32
					in.WantComma()
				}
				in.Delim('}')
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeMimeMultipart(out *jwriter.Writer, in multipart.Form) {
	out.RawByte('{')
	first := true
	_ = first
	if len(in.Value) != 0 {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"value\":")
		if in.Value == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v34First := true
			for v34Name, v34Value := range in.Value {
				if !v34First {
					out.RawByte(',')
				}
				v34First = false
				out.String(string(v34Name))
				out.RawByte(':')
				if v34Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v35, v36 := range v34Value {
						if v35 > 0 {
							out.RawByte(',')
						}
						out.String(string(v36))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	if len(in.File) != 0 {
		if !first {
			out.RawByte(',')
		}
		//first = false
		out.RawString("\"file\":")
		if in.File == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v37First := true
			for v37Name, v37Value := range in.File {
				if !v37First {
					out.RawByte(',')
				}
				v37First = false
				out.String(string(v37Name))
				out.RawByte(':')
				if v37Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v38, v39 := range v37Value {
						if v38 > 0 {
							out.RawByte(',')
						}
						if v39 == nil {
							out.RawString("null")
						} else {
							easyjson1688e6a4EncodeMimeMultipart1(out, *v39)
						}
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	out.RawByte('}')
}
func easyjson1688e6a4DecodeMimeMultipart1(in *jlexer.Lexer, out *multipart.FileHeader) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "filename":
			out.Filename = string(in.String())
		case "header":
			if in.IsNull() {
				in.Skip()
			} else {
				in.Delim('{')
				if !in.IsDelim('}') {
					out.Header = make(textproto.MIMEHeader)
				} else {
					out.Header = nil
				}
				for !in.IsDelim('}') {
					key := string(in.String())
					in.WantColon()
					var v40 []string
					if in.IsNull() {
						in.Skip()
						v40 = nil
					} else {
						in.Delim('[')
						if !in.IsDelim(']') {
							v40 = make([]string, 0, 4)
						} else {
							v40 = []string{}
						}
						for !in.IsDelim(']') {
							var v41 string
							v41 = string(in.String())
							v40 = append(v40, v41)
							in.WantComma()
						}
						in.Delim(']')
					}
					(out.Header)[key] = v40
					in.WantComma()
				}
				in.Delim('}')
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeMimeMultipart1(out *jwriter.Writer, in multipart.FileHeader) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Filename != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"filename\":")
		out.String(string(in.Filename))
	}
	if len(in.Header) != 0 {
		if !first {
			out.RawByte(',')
		}
		//first = false
		out.RawString("\"header\":")
		if in.Header == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
			out.RawString(`null`)
		} else {
			out.RawByte('{')
			v42First := true
			for v42Name, v42Value := range in.Header {
				if !v42First {
					out.RawByte(',')
				}
				v42First = false
				out.String(string(v42Name))
				out.RawByte(':')
				if v42Value == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
					out.RawString("null")
				} else {
					out.RawByte('[')
					for v43, v44 := range v42Value {
						if v43 > 0 {
							out.RawByte(',')
						}
						out.String(string(v44))
					}
					out.RawByte(']')
				}
			}
			out.RawByte('}')
		}
	}
	out.RawByte('}')
}
func easyjson1688e6a4DecodeNetUrl(in *jlexer.Lexer, out *url.URL) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "scheme":
			out.Scheme = string(in.String())
		case "opaque":
			out.Opaque = string(in.String())
		case "user":
			if in.IsNull() {
				in.Skip()
				out.User = nil
			} else {
				if out.User == nil {
					out.User = new(url.Userinfo)
				}
				easyjson1688e6a4DecodeNetUrl1(in, &*out.User)
			}
		case "host":
			out.Host = string(in.String())
		case "path":
			out.Path = string(in.String())
		case "raw_path":
			out.RawPath = string(in.String())
		case "force_query":
			out.ForceQuery = bool(in.Bool())
		case "raw_query":
			out.RawQuery = string(in.String())
		case "fragment":
			out.Fragment = string(in.String())
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeNetUrl(out *jwriter.Writer, in url.URL) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Scheme != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"scheme\":")
		out.String(string(in.Scheme))
	}
	if in.Opaque != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"opaque\":")
		out.String(string(in.Opaque))
	}
	if in.User != nil {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"user\":")
		if in.User == nil {
			out.RawString("null")
		} else {
			easyjson1688e6a4EncodeNetUrl1(out, *in.User)
		}
	}
	if in.Host != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"host\":")
		out.String(string(in.Host))
	}
	if in.Path != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"path\":")
		out.String(string(in.Path))
	}
	if in.RawPath != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"raw_path\":")
		out.String(string(in.RawPath))
	}
	if in.ForceQuery {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"force_query\":")
		out.Bool(bool(in.ForceQuery))
	}
	if in.RawQuery != "" {
		if !first {
			out.RawByte(',')
		}
		first = false
		out.RawString("\"raw_query\":")
		out.String(string(in.RawQuery))
	}
	if in.Fragment != "" {
		if !first {
			out.RawByte(',')
		}
		//first = false
		out.RawString("\"fragment\":")
		out.String(string(in.Fragment))
	}
	out.RawByte('}')
}
func easyjson1688e6a4DecodeNetUrl1(in *jlexer.Lexer, out *url.Userinfo) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1688e6a4EncodeNetUrl1(out *jwriter.Writer, in url.Userinfo) {
	out.RawByte('{')
	first := true
	_ = first
	out.RawByte('}')
}
