// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package caddyesi

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strconv"
)

type responseBufferWriter interface {
	http.ResponseWriter
	TriggerRealWrite(addContentLength int)
}

// responseWrapBuffer wraps an http.ResponseWriter, returning a proxy which only writes
// into the provided io.Writer.
func responseWrapBuffer(buf io.Writer, w http.ResponseWriter) responseBufferWriter {
	_, cn := w.(http.CloseNotifier)
	_, fl := w.(http.Flusher)
	_, hj := w.(http.Hijacker)
	_, rf := w.(io.ReaderFrom)

	bw := bufferedWriter{
		rw:     w,
		buf:    buf,
		header: make(http.Header),
	}
	if cn && fl && hj && rf {
		return &bufferedFancyWriter{bw}
	}
	if fl {
		return &bufferedFlushWriter{bw}
	}
	return &bw
}

// bufferedWriter wraps a http.ResponseWriter that implements the minimal
// http.ResponseWriter interface.
type bufferedWriter struct {
	rw     http.ResponseWriter
	buf    io.Writer
	header http.Header
	// addContentLength rewrites the Content-Length header to the correct
	// returned length. Value can also be negative when the error message in an
	// Tag tag is shorter than the length of the Tag tag.
	addContentLength int
	code             int
	wroteHeader      bool
	// writeReal does not write to the buffer and writes directly to the original
	// rw.
	writeReal bool
}

func (b *bufferedWriter) TriggerRealWrite(addContentLength int) {
	b.writeReal = true
	b.addContentLength = addContentLength
}

func (b *bufferedWriter) Header() http.Header {
	return b.header
}

func (b *bufferedWriter) WriteHeader(code int) {
	// WriteHeader gets called before TriggerRealWrite
	if b.code == 0 {
		b.code = code
	}
}

// Write does not write to the client instead it writes in the underlying
// buffer.
func (b *bufferedWriter) Write(p []byte) (int, error) {
	if !b.writeReal {
		return b.buf.Write(p)
	}

	const clName = "Content-Length"
	if !b.wroteHeader {
		b.wroteHeader = true
		if b.addContentLength != 0 {
			clRaw := b.header.Get(clName)
			cl, _ := strconv.Atoi(clRaw) // ignoring that err ... for now
			b.header.Set(clName, strconv.Itoa(cl+b.addContentLength))
		}

		for k, v := range b.header {
			b.rw.Header()[k] = v
		}
		b.rw.WriteHeader(b.code)
	}
	return b.rw.Write(p)
}

// bufferedFancyWriter is a writer that additionally satisfies
// http.CloseNotifier, http.Flusher, http.Hijacker, and io.ReaderFrom. It exists
// for the common case of wrapping the http.ResponseWriter that package http
// gives you, in order to make the proxied object support the full method set of
// the proxied object.
type bufferedFancyWriter struct {
	bufferedWriter
}

func (f *bufferedFancyWriter) CloseNotify() <-chan bool {
	cn := f.bufferedWriter.rw.(http.CloseNotifier)
	return cn.CloseNotify()
}
func (f *bufferedFancyWriter) Flush() {
	fl := f.bufferedWriter.rw.(http.Flusher)
	fl.Flush()
}
func (f *bufferedFancyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.bufferedWriter.rw.(http.Hijacker)
	return hj.Hijack()
}
func (f *bufferedFancyWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := f.bufferedWriter.rw.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return nil
}

// ReadFrom writes r into the underlying buffer
func (f *bufferedFancyWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(&f.bufferedWriter, r)
}

// bufferedFlushWriter implements only http.Flusher mostly used
type bufferedFlushWriter struct {
	bufferedWriter
}

func (f *bufferedFlushWriter) Flush() {
	fl := f.bufferedWriter.rw.(http.Flusher)
	fl.Flush()
}
