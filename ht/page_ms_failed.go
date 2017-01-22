package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/vdobler/ht/ht"
)

func init() {
	RegisterTest(page02(), page02())
}

var page02Counter int

func page02() (t *ht.Test) {
	page02Counter++
	t = &ht.Test{
		Name:        fmt.Sprintf("Request to micro service failed, iteration %d", page02Counter),
		Description: `Tries to load from a nonexisitent micro service and displays a custom error message`,
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page_ms_failed.html",
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
				Contains: "MS9999 not available",
				Count:    1,
			},
			&ht.Body{
				Contains: `class="page02ErrMsg18MS"`,
				Count:    1,
			},
		},
	}
	return
}
