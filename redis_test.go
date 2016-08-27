package esi

import "testing"

var _ Backender = (*Redis)(nil)

func TestParseRedis(t *testing.T) {
	tests := []struct {
		raw          string
		wantAddress  string
		wantPassword string
		wantDB       int64
		wantErr      bool
	}{
		{
			"localhost",
			"",
			"",
			0,
			true, // "invalid redis URL scheme",
		},
		// The error message for invalid hosts is diffferent in different
		// versions of Go, so just check that there is an error message.
		{
			"redis://weird url",
			"",
			"",
			0,
			true,
		},
		{
			"redis://foo:bar:baz",
			"",
			"",
			0,
			true,
		},
		{
			"http://www.google.com",
			"",
			"",
			0,
			true, // "invalid redis URL scheme: http",
		},
		{
			"redis://localhost:6379/abc123",
			"",
			"",
			0,
			true, // "invalid database: abc123",
		},
		{
			"redis://localhost:6379/123",
			"localhost:6379",
			"",
			123,
			false,
		},
		{
			"redis://:6379/123",
			"localhost:6379",
			"",
			123,
			false,
		},
		{
			"redis://",
			"localhost:6379",
			"",
			0,
			false,
		},
		{
			"redis://192.168.0.234/123",
			"192.168.0.234:6379",
			"",
			123,
			false,
		},
		{
			"redis://192.168.0.234/",
			"",
			"",
			0,
			true,
		},
		{
			"redis://empty:SuperSecurePa55w0rd@192.168.0.234/3",
			"192.168.0.234:6379",
			"SuperSecurePa55w0rd",
			3,
			false,
		},
	}
	for i, test := range tests {

		haveAddress, havePW, haveDB, haveErr := parseRedisURL(test.raw)

		if have, want := haveAddress, test.wantAddress; have != want {
			t.Errorf("(%d) Address: Have: %v Want: %v", i, have, want)
		}
		if have, want := havePW, test.wantPassword; have != want {
			t.Errorf("(%d) Password: Have: %v Want: %v", i, have, want)
		}
		if have, want := haveDB, test.wantDB; have != want {
			t.Errorf("(%d) DB: Have: %v Want: %v", i, have, want)
		}
		if test.wantErr {
			if have, want := test.wantErr, haveErr != nil; have != want {
				t.Errorf("(%d) Error: Have: %v Want: %v", i, have, want)
			}
		} else {
			if haveErr != nil {
				t.Errorf("(%d) Did not expect an Error: %+v", i, haveErr)
			}
		}
	}
}
