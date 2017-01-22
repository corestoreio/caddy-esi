package main

import (
	"fmt"
	"net/http"
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

var page01Counter int

func page01() (t *ht.Test) {
	page01Counter++
	t = &ht.Test{
		Name:        fmt.Sprintf("Page MS Cart Tiny Iteration %d", page01Counter),
		Description: `Request loads ms_cart_tiny.html from a micro service and embeds the checkout cart into its HTML`,
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page_cart_tiny.html",
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
					Min: 14, Max: 18}},
			&ht.Header{
				Header: "Accept-Ranges",
				Condition: ht.Condition{
					Equals: `bytes`}},
			&ht.Header{
				Header: "Last-Modified",
				Condition: ht.Condition{
					Min: 29, Max: 29}},
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
			&ht.Body{
				Contains: ` class="page01CartLoaded"`,
				Count:    1,
			},
		},
	}
	return
}
