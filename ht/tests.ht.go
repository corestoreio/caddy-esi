package main

import (
	"net/http"
	"time"

	"github.com/vdobler/ht/ht"
)

var page01 = &ht.Test{
	Name: "Load page01 with a GET request",
	Request: ht.Request{
		Method: "GET",
		URL:    "{{PROTOCOL}}://{{HOST}}/page01.html",
		Header: http.Header{
			"Accept":          []string{"text/html"},
			"Accept-Encoding": []string{"gzip, deflate, br"},
		},
		Timeout: 1 * time.Second,
	},
	Checks: ht.CheckList{
		ht.StatusCode{Expect: 200},
		&ht.Header{
			Header: "Etag",
			Condition: ht.Condition{
				Min: 10}},
		&ht.None{
			Of: ht.CheckList{
				&ht.HTMLContains{
					Text: []string{"<esi:"},
				}}},
		&ht.Body{
			Contains: "demo-store.shop/autumn-pullie.html",
			Count:    1,
		},
	},
}
