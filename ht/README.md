# ht = http testing

Contains integration test run with https://github.com/vdobler/ht

Those tests can be either written in hjson (human json) or directly in Go. The
later provides a better documentation.

```
$ go run *.go
```

Output might look like:

```
$ go run ht/*.go
PASS: Load page01 with a GET request
  Started: 2017-01-19 21:17:24.380901176 +0100 CET   Duration: 3.221206ms   Request: 3.05208ms
  GET http://127.0.0.1:2017/page01.html
  HTTP/1.1 200 OK
  Checks:
     0. Pass    StatusCode      {"Expect":200}
     1. Pass    Header          {"Header":"Etag","Min":10}
     2. Pass    None            {"Of":[{"Check":"HTMLContains","Selector":"html","Text":["\u003cesi:"]}]}
     3. Pass    Body            {"Contains":"demo-store.shop/autumn-pullie.html","Count":2}
PASS: Load page01 with a GET request
  Started: 2017-01-19 21:17:24.380901176 +0100 CET   Duration: 3.221206ms   Request: 3.05208ms
  GET http://127.0.0.1:2017/page01.html
  HTTP/1.1 200 OK
  Checks:
     0. Pass    StatusCode      {"Expect":200}
     1. Pass    Header          {"Header":"Etag","Min":10}
     2. Pass    None            {"Of":[{"Check":"HTMLContains","Selector":"html","Text":["\u003cesi:"]}]}
     3. Pass    Body            {"Contains":"demo-store.shop/autumn-pullie.html","Count":2}
PASS: Load page01 with a GET request
  Started: 2017-01-19 21:17:24.380901176 +0100 CET   Duration: 3.221206ms   Request: 3.05208ms
  GET http://127.0.0.1:2017/page01.html
  HTTP/1.1 200 OK
  Checks:
     0. Pass    StatusCode      {"Expect":200}
     1. Pass    Header          {"Header":"Etag","Min":10}
     2. Pass    None            {"Of":[{"Check":"HTMLContains","Selector":"html","Text":["\u003cesi:"]}]}
     3. Pass    Body            {"Contains":"demo-store.shop/autumn-pullie.html","Count":2}
```
