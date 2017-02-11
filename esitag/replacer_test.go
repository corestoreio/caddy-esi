// Copied from github.com/mholt/caddy/caddyhttp/httpserver/replacer_test.go and
// stripped a lot of things off.
// Copyright Caddy Contributors and Matt Holt.  Apache License Version 2.0, January 2004

package esitag

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func BenchmarkReplacer_Replace(b *testing.B) {
	const key = `product_{HX-Product-ID}`
	const wantKey = `product_GopherPlushXXL`

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Product-ID", "GopherPlushXXL")
	rpl := MakeReplacer(req, "")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		have := rpl.Replace(key)
		if have != wantKey {
			b.Errorf("Have: %v Want: %v", have, wantKey)
		}
	}
}
func TestNewReplacer(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest("POST", "http://localhost", strings.NewReader(`{"username": "caddyESI"}`))

	rep := MakeReplacer(request, "")

	switch v := rep.(type) {
	case replacer:
		if v.getSubstitution("{host}") != "localhost" {
			t.Error("Expected host to be localhost")
		}
		if v.getSubstitution("{method}") != "POST" {
			t.Error("Expected request method  to be POST")
		}
	default:
		t.Fatalf("Expected *replacer underlying Replacer type, got: %#v", rep)
	}
}

func TestReplace(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "http://localhost?password=12345678", strings.NewReader(`username=caddyESI`))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	req.AddCookie(&http.Cookie{Name: "xtestKeks", Value: "xVal"})
	req.Header.Set("Custom", "foobarbaz")
	req.Header.Set("ShorterVal", "1")
	repl := MakeReplacer(req, "-")
	// add some headers after creating replacer
	req.Header.Set("CustomAdd", "caddy")

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal("Failed to determine hostname\n")
	}

	old := now
	now = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, 02, time.FixedZone("hardcoded", -7))
	}
	defer func() {
		now = old
	}()
	testCases := []struct {
		template string
		expect   string
	}{
		{"This hostname is {hostname}", "This hostname is " + hostname},
		{"This host is {host}.", "This host is localhost."},
		{"This req method is {method}.", "This req method is POST."},
		{"{when}", "02/Jan/2006:15:04:05 +0000"},
		{"{when_iso}", "2006-01-02T15:04:12Z"},

		{"The Custom header is {HCustom}.", "The Custom header is foobarbaz."},
		{"The CustomAdd header is {HCustomAdd}.", "The CustomAdd header is caddy."},
		{"The cUsToM header is {HcUsToM}...", "The cUsToM header is foobarbaz..."},

		{"The Custom cookie is {CxtestKeks}.", "The Custom cookie is xVal."},
		{"The Custom cookie is {CxTestKeks}.", "The Custom cookie is -."},

		{"The Custom POST form is {Fusername}.", "The Custom POST form is caddyESI."},
		{"The Custom POST form is {FUsername}.", "The Custom POST form is -."},

		{"The Custom GET form is {Fpassword}.", "The Custom GET form is 12345678."},
		{"The Custom GET form is {FPassword}.", "The Custom GET form is -."},

		{"The Non-Existent header is {HNon-Existent}.", "The Non-Existent header is -."},
		{"Bad {host placeholder...", "Bad {host placeholder..."},
		{"Bad {HCustom placeholder", "Bad {HCustom placeholder"},
		{"Bad {HCustom placeholder {HShorterVal}", "Bad -"},
	}

	for _, c := range testCases {
		if expected, actual := c.expect, repl.Replace(c.template); expected != actual {
			t.Errorf("for template '%s', expected '%s', got '%s'", c.template, expected, actual)
		}
	}

}
