package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/vdobler/ht/ht"
)

func init() {
	// For now we must create new pointers each time we want to run a test. A
	// single test cannot be shared between goroutines. This is a limitation
	// which can maybe fixed by a special handling of the Request and Jar field
	// in ht. This change might complicate things ...
	RegisterTest(page01(), page01(), page01())
}

var page01Counter = new(int32)

func page01() *ht.Test {
	return &ht.Test{
		Name:        fmt.Sprintf("Page01 Iteration %d", atomic.AddInt32(page01Counter, 1)),
		Description: `Page01 loads ms_cart_tiny from a micro service and embeds the checkout cart into its page01 HTML`,
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
