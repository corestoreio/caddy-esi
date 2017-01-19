package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/vdobler/ht/ht"
)

const caddyAddress = `http://127.0.0.1:2017/`

var page01Counter = new(int32)

func page01() *ht.Test {
	return &ht.Test{
		Name: fmt.Sprintf("Load page01 with a GET request. Iteration %d", atomic.AddInt32(page01Counter, 1)),
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page01.html",
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
						Selector: `html`,
						Text:     []string{"<esi:"},
					}}},
			&ht.Body{
				Contains: "demo-store.shop/autumn-pullie.html",
				Count:    2,
			},
		},
	}
}
