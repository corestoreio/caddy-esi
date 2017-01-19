package main

import (
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/suite"
)

func suite01() *suite.Suite {
	return &suite.Suite{
		Name:        "Initial GET tests",
		Description: "query the frontend server and see the response from a 3rd party micro service",
		KeepCookies: false,

		Tests: []*ht.Test{
			page01,
		},
	}
}

//{
//    Name: "Initial ht tests",
//    Description: "query the frontend server and see the response from a 3rd party micro service",
//
//    KeepCookies: false
//
//    Setup: [
//        // for now no setup
//    ]
//
//    Main: [
//        {File: "basic.ht"}
//    ]
//
//    Teardown: [
//        // nothing
//    ]
//
//    Variables: {
//         PROTOCOL:       "http",
//         HOST:     "127.0.0.1:2017",
//    }
//}
