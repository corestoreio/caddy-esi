package caddyesi

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
)

type Redis struct {
	// todo
}

func NewRedis(rawURL string) (*Redis, error) {
	_, _, _, err := parseRedisURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("[caddyesi] Error parsing URL %q => %s", rawURL, err)
	}
	return &Redis{}, nil
}

func mustNewRedisConfig(rawURL string) *Redis {
	r, err := NewRedis(rawURL)
	if err != nil {
		panic(err)
	}
	return r
}

func (r *Redis) Set(key, val []byte) error {
	// todo
	return nil
}
func (r *Redis) Get(key []byte) ([]byte, error) {
	// todo
	return nil, nil
}

var pathDBRegexp = regexp.MustCompile(`/(\d*)\z`)

// ParseRedis parses a given URL using the Redis
// URI scheme. URLs should follow the draft IANA specification for the
// scheme (https://www.iana.org/assignments/uri-schemes/prov/redis).
//
//
// For example:
// 		redis://localhost:6379/3
// 		redis://:6380/0 => connects to localhost:6380
// 		redis:// => connects to localhost:6379 with DB 0
// 		redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
func parseRedisURL(raw string) (address, password string, db int64, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, fmt.Errorf("[url.ParseRedis] url.Parse: %s", err)
	}

	if u.Scheme != "redis" {
		return "", "", 0, fmt.Errorf("[url.ParseRedis] Invalid Redis URL scheme: %q", u.Scheme)
	}

	// As per the IANA draft spec, the host defaults to localhost and
	// the port defaults to 6379.
	host, port, err := net.SplitHostPort(u.Host)
	if sErr, ok := err.(*net.AddrError); ok && sErr != nil && sErr.Err == "too many colons in address" {
		return "", "", 0, fmt.Errorf("[url.ParseRedis] SplitHostPort: %s", err)
	}
	if err != nil {
		// assume port is missing
		host = u.Host
		port = "6379"
		err = nil
	}
	if host == "" {
		host = "localhost"
	}
	address = net.JoinHostPort(host, port)

	if u.User != nil {
		password, _ = u.User.Password()
	}

	match := pathDBRegexp.FindStringSubmatch(u.Path)
	if len(match) == 2 {
		if len(match[1]) > 0 {
			db, err = strconv.ParseInt(match[1], 10, 64)
			if err != nil {
				return "", "", 0, fmt.Errorf("[url.ParseRedis] Invalid database: %q in %q", u.Path[1:], match[1])
			}
		}
	} else if u.Path != "" {
		return "", "", 0, fmt.Errorf("[url.ParseRedis] Invalid database: %q", u.Path[1:])
	}
	return
}
