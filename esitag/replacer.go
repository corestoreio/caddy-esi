// Copied from github.com/mholt/caddy/caddyhttp/httpserver/replacer.go and stripped
// a lot of things off.
// Copyright Caddy Contributors and Matt Holt.  Apache License Version 2.0, January 2004

package esitag

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/corestoreio/caddy-esi/helper"
)

// now mocked out for testing
var now = time.Now

// Replacer is a type which can replace placeholder substrings in a string with
// actual values from a http.Request.
type Replacer interface {
	Replace(string) string
}

// replacer implements Replacer.
type replacer struct {
	request *http.Request
	// cookies lazily initialized
	cookies    []*http.Cookie
	emptyValue string
}

// MakeReplacer makes a new replacer based on r which are used for request
// placeholders. Request placeholders are created immediately. emptyValue should
// be the string that is used in place of empty string (can still be empty
// string).
func MakeReplacer(r *http.Request, emptyValue string) Replacer {
	// TODO(CyS) maybe add user agent parsing https://github.com/mssola/user_agent
	// TODO(CyS) add geo location detection based on the IP address.
	return &replacer{
		emptyValue: emptyValue,
		request:    r,
	}
}

// Replace performs a replacement of values on s and returns the string with the
// replaced values. Placeholders must be all lower case.
func (r *replacer) Replace(s string) string {
	// Do not attempt replacements if no placeholder is found.
	if !strings.ContainsAny(s, "{}") {
		return s
	}

	result := ""
	for {
		idxStart := strings.Index(s, "{")
		if idxStart == -1 {
			// no placeholder anymore
			break
		}
		idxEnd := strings.Index(s[idxStart:], "}")
		if idxEnd == -1 {
			// unpaired placeholder
			break
		}
		idxEnd += idxStart

		// get a replacement
		placeholder := s[idxStart : idxEnd+1]
		replacement := r.getSubstitution(placeholder)

		// append prefix + replacement
		result += s[:idxStart] + replacement

		// strip out scanned parts
		s = s[idxEnd+1:]
	}

	// append unscanned parts
	return result + s
}

// getSubstitution retrieves value from corresponding key
func (r *replacer) getSubstitution(key string) string {

	switch key[1] {
	case 'H': // search request headers {HX-Header-Y} then
		want := key[2 : len(key)-1]
		for key, values := range r.request.Header {
			// Header placeholders (case-insensitive)
			if strings.EqualFold(key, want) {
				return strings.Join(values, ",")
			}
		}
	case 'C': // search request cookies {CMy-Cookie-Key} then
		if r.cookies == nil {
			r.cookies = r.request.Cookies() // pre parse cookies
		}
		want := key[2 : len(key)-1]
		for _, c := range r.cookies {
			// cookie placeholders (case-sensitive)
			if c.Name == want {
				return c.Value
			}
		}
	case 'F': // search request form {FMy_form_or_get_field_name} then
		want := key[2 : len(key)-1]
		v := r.request.FormValue(want)
		if v != "" {
			return v
		}
		return r.emptyValue
	}

	// search default replacements in the end
	switch key {
	case "{method}":
		return r.request.Method
	case "{scheme}":
		if r.request.TLS != nil {
			return "https"
		}
		return "http"
	case "{hostname}":
		name, err := os.Hostname()
		if err != nil {
			return r.emptyValue
		}
		return name
	case "{host}":
		return r.request.Host
	case "{hostonly}":
		host, _, err := net.SplitHostPort(r.request.Host)
		if err != nil {
			return r.request.Host
		}
		return host
	case "{path}":
		// if a rewrite has happened, the original URI should be used as the path
		// rather than the rewritten URI
		p := r.request.Header.Get("Caddy-Rewrite-Original-URI")
		if p == "" {
			p = r.request.URL.Path
		}
		return p
	case "{path_escaped}":
		p := r.request.Header.Get("Caddy-Rewrite-Original-URI")
		if p == "" {
			p = r.request.URL.Path
		}
		return url.QueryEscape(p)
	case "{rewrite_path}":
		return r.request.URL.Path
	case "{rewrite_path_escaped}":
		return url.QueryEscape(r.request.URL.Path)
	case "{query}":
		return r.request.URL.RawQuery
	case "{query_escaped}":
		return url.QueryEscape(r.request.URL.RawQuery)
	case "{fragment}":
		return r.request.URL.Fragment
	case "{proto}":
		return r.request.Proto
	case "{remote}":
		host, _, err := net.SplitHostPort(r.request.RemoteAddr)
		if err != nil {
			return r.request.RemoteAddr
		}
		return host
	case "{port}":
		_, port, err := net.SplitHostPort(r.request.RemoteAddr)
		if err != nil {
			return r.emptyValue
		}
		return port
	case "{real_remote}":
		host, _, err := net.SplitHostPort(helper.RealIP(r.request))
		if err != nil {
			return r.request.RemoteAddr
		}
		return host
	case "{uri}":
		return r.request.URL.RequestURI()
	case "{uri_escaped}":
		return url.QueryEscape(r.request.URL.RequestURI())
	case "{when}":
		return now().Format(timeFormat)
	case "{when_iso}":
		return now().UTC().Format(timeFormatISOUTC)
		// more date functions can be implemented ...
	case "{file}":
		_, file := path.Split(r.request.URL.Path)
		return file
	case "{dir}":
		dir, _ := path.Split(r.request.URL.Path)
		return dir
	}
	return r.emptyValue
}

const (
	timeFormat       = "02/Jan/2006:15:04:05 -0700"
	timeFormatISOUTC = "2006-01-02T15:04:05Z" // ISO 8601 with timezone to be assumed as UTC
)
