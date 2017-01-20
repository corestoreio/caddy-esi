package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/vdobler/ht/ht"
)

func init() {
	RegisterTest(page02(), page02())
}

var page02Counter = new(int32)

func page02() *ht.Test {
	return &ht.Test{
		Name:        fmt.Sprintf("Page02 Iteration %d", atomic.AddInt32(page02Counter, 1)),
		Description: `Page02 tries to load from a nonexisitent micro service and displays a custom error message`,
		Request: ht.Request{
			Method: "GET",
			URL:    caddyAddress + "page02.html",
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
				Contains: "MS9999 not available",
				Count:    1,
			},
		},
	}
}
