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

package caddyesi

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/stretchr/testify/assert"
)

// Check if types have the interfaces implemented.
var _ http.CloseNotifier = (*injectingFancyWriter)(nil)
var _ http.Flusher = (*injectingFancyWriter)(nil)
var _ http.Hijacker = (*injectingFancyWriter)(nil)
var _ http.Pusher = (*injectingFancyWriter)(nil)
var _ io.ReaderFrom = (*injectingFancyWriter)(nil)
var _ http.Flusher = (*injectingFlushWriter)(nil)

// Check if types have the interfaces implemented.
var _ http.CloseNotifier = (*responseMock)(nil)
var _ http.Flusher = (*responseMock)(nil)
var _ http.Hijacker = (*responseMock)(nil)
var _ http.Pusher = (*responseMock)(nil)
var _ io.ReaderFrom = (*responseMock)(nil)
var _ http.Flusher = (*responseMock)(nil)

var _ io.Reader = (*simpleReader)(nil)

type responseMock struct {
	http.ResponseWriter
}

func newResponseMock() http.ResponseWriter {
	return &responseMock{
		ResponseWriter: httptest.NewRecorder(),
	}
}

func (f *responseMock) CloseNotify() <-chan bool {
	return nil
}
func (f *responseMock) Flush() {}
func (f *responseMock) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}
func (f *responseMock) Push(target string, opts *http.PushOptions) error {
	return nil
}

// ReadFrom writes r into the underlying buffer
func (f *responseMock) ReadFrom(r io.Reader) (int64, error) {
	return 0, nil
}

func TestResponseWrapInjector(t *testing.T) {
	t.Parallel()

	t.Run("WriteHeader with additional Content-Length (Idempotence)", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{End: 5, Start: 1} // Final calculation 0-5-1 = -4
		close(dtChan)

		rec := httptest.NewRecorder()
		rwi := responseWrapInjector(dtChan, rec)
		rwi.Header().Set("Content-LENGTH", "300")

		for i := 0; i < 3; i++ {
			// Test for being idempotent
			rwi.WriteHeader(http.StatusMultipleChoices)
			assert.Exactly(t, http.StatusMultipleChoices, rec.Code, "Expecting http.StatusMultipleChoices")
			assert.Exactly(t, "296", rec.Header().Get("Content-Length"), "Expecting Content-Length value")
		}
	})

	t.Run("Get injecting Flush Writer", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{}
		close(dtChan)

		rwi := responseWrapInjector(dtChan, httptest.NewRecorder())
		_, ok := rwi.(*injectingFlushWriter)
		assert.True(t, ok, "Expecting a injectingFlushWriter type")
	})

	t.Run("Get injecting Fancy Writer", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{}
		close(dtChan)

		rwi := responseWrapInjector(dtChan, newResponseMock())
		_, ok := rwi.(*injectingFancyWriter)
		assert.True(t, ok, "Expecting a injectingFancyWriter type")
	})

	t.Run("Dot not run injector on binary data", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{End: 5, Start: 1} // Final calculation 0-5-1 = -4
		close(dtChan)

		rec := httptest.NewRecorder()
		rwi := responseWrapInjector(dtChan, rec)
		png := []byte("\x89\x50\x4E\x47\x0D\x0A\x1A\x0A")
		if _, err := rwi.Write(png); err != nil {
			t.Fatal(err)
		}
		if _, err := rwi.Write(png); err != nil {
			t.Fatal(err)
		}
		assert.Exactly(t, append(png, png...), rec.Body.Bytes())
	})

	t.Run("Run injector once on text data", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{Data: []byte(`Hello XML`), Start: 12, End: 16}
		close(dtChan)

		rec := httptest.NewRecorder()
		rwi := responseWrapInjector(dtChan, rec)
		html := []byte(`<HtMl><bOdY>blah blah blah</body></html>`)
		if _, err := rwi.Write(html); err != nil {
			t.Fatal(err)
		}
		assert.Exactly(t, `<HtMl><bOdY>Hello XML blah blah</body></html>`, rec.Body.String())
	})

	t.Run("Run injector twice on text data", func(t *testing.T) {
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- esitag.DataTag{Data: []byte(`<Hello><world status="sinking"></world></Hello>`), Start: 13, End: 34}
		close(dtChan)

		rec := httptest.NewRecorder()
		rwi := responseWrapInjector(dtChan, rec)
		html1 := []byte(`<HtMl><bOdY> <esi:include src=""/>|`)
		html2 := []byte(`<data>Text and much more content.</data></body></html>`)
		if _, err := rwi.Write(html1); err != nil {
			t.Fatal(err)
		}
		if _, err := rwi.Write(html2); err != nil {
			t.Fatal(err)
		}

		assert.Exactly(t,
			"<HtMl><bOdY> <Hello><world status=\"sinking\"></world></Hello>|<data>Text and much more content.</data></body></html>",
			rec.Body.String())
	})

	t.Run("Run injector on large file with multiple different sized writes", func(t *testing.T) {
		// Changing the page09.html content, you have also to adjust the DataTag ...

		backendData := []byte("<table border='1' cellpadding='3' cellspacing='2'><tr><th>Key</th><th>Value</th></tr>\n<tr><td>Session</td><td>session_</td></tr>\n<tr><td>Next Session Integer</td><td>5</td></tr>\n<tr><td>RequestURI</td><td>/page_blog_post.html</td></tr>\n<tr><td>Headers</td><td>User-Agent: curl/7.47.1<br>\n</td></tr>\n<tr><td>Time</td><td>Sun, 05 Mar 2017 20:15:14 +0100</td></tr>\n</table>\n\n<!-- Duration:565.039µs Error:none Tag:include src=\"grpcServerDemo\" printdebug=\"1\" key=\"session_{Fsession}\" forwardheaders=\"all\" timeout=\"4ms\" onerror=\"Demo gRPC server unavailable :-(\" -->\n")

		tg := esitag.DataTag{Start: 25297, End: 25450, Data: backendData}
		dtChan := make(chan esitag.DataTag, 1)
		dtChan <- tg
		close(dtChan)

		html, err := ioutil.ReadFile("testdata/page09.html")
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		rwi := responseWrapInjector(dtChan, rec)

		written := 0
		for _, parts := range bytes.SplitAfter(html, []byte(`</div>`)) {
			n, err := rwi.Write(parts)
			if err != nil {
				t.Fatal(err)
			}
			written += n
		}

		//if err := ioutil.WriteFile("testdata/page09_out.html", rec.Body.Bytes(), 0644); err != nil {
		//	t.Fatal(err)
		//}
		assert.Exactly(t, 42701, rec.Body.Len())
		assert.Exactly(t, written+len(backendData)-(tg.End-tg.Start), rec.Body.Len())
	})
}
