package helpers

import (
	"net"
	"net/http"
	"strings"
	"unicode"

	"github.com/SchumacherFM/caddyesi/bufpool"
)

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

func (hs headers) findIP(r *http.Request) net.IP {
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
				return realIP
			}
		}
	}
	return nil
}

// RealIP extracts the remote address from a request and takes care of different
// headers in which an IP address can be stored. Checks if the IP in one of the
// header fields lies in net.PrivateIPRanges. Return value can be nil. A check for
// the RealIP costs 8 allocs, for now.
func RealIP(r *http.Request) net.IP {
	// Courtesy https://husobee.github.io/golang/ip-address/2015/12/17/remote-ip-go.html

	if ip := ForwardedIPHeaders.findIP(r); ip != nil {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(filterIP(host))
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
