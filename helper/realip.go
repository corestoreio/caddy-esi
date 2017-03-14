// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package helper

import (
	"net"
	"net/http"
	"strings"
	"unicode"

	"github.com/corestoreio/caddy-esi/bufpool"
)

// Available HTTP header keys for reading the real IP address.
const (
	ClientIP         = "Client-Ip"
	Forwarded        = "Forwarded"
	ForwardedFor     = "Forwarded-For"
	XClusterClientIP = "X-Cluster-Client-Ip"
	XForwarded       = "X-Forwarded"
	XForwardedFor    = "X-Forwarded-For"
	XRealIP          = "X-Real-Ip"
)

// ForwardedIPHeaders contains a list of available headers which
// might contain the client IP address.
var ForwardedIPHeaders = headers{XForwarded, XForwardedFor, Forwarded, ForwardedFor, XRealIP, ClientIP, XClusterClientIP}

type headers [7]string

func (hs headers) findIP(r *http.Request) string {
	for _, h := range hs {
		addresses := strings.Split(r.Header.Get(h), ",")
		// march from right to left until we get a public address
		// that will be the address right before our proxy.
		for i := len(addresses) - 1; i >= 0; i-- {
			// header can contain spaces too, strip those out.
			addr := filterIP(addresses[i])
			if addr == "" {
				continue
			}
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			realIP := net.ParseIP(host)
			if !realIP.IsGlobalUnicast() {
				// bad address, go to next
				continue
			}

			if realIP != nil {
				return host
			}
		}
	}
	return ""
}

// RealIP extracts the remote address from a request and takes care of different
// headers in which an IP address can be stored. Checks if the IP in one of the
// header fields lies in net.PrivateIPRanges. Return value can be empty. A check
// for the RealIP costs 8 allocs, for now. This implementation trusts the values
// found in the forward headers.
func RealIP(r *http.Request) string {
	// Courtesy https://husobee.github.io/golang/ip-address/2015/12/17/remote-ip-go.html

	if ip := ForwardedIPHeaders.findIP(r); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(filterIP(host))
	if ip == nil {
		return "" // otherwise output would be "<nil"
	}
	return ip.String() // validate correctly the IP address
}

func filterIP(ip string) string {
	buf := bufpool.Get()
	defer bufpool.Put(buf)
	for _, r := range ip {
		switch {
		case unicode.IsDigit(r), unicode.IsLetter(r), unicode.IsPunct(r):
			_, _ = buf.WriteRune(r)
		}
	}
	return buf.String()
}
