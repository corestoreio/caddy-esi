# ESI Tags for Caddy Server (Beta)

This plugin implements partial ESI [Edge Side Includes](https://en.wikipedia.org/wiki/Edge_Side_Includes) support
for the [Caddy Webserver](https://caddyserver.com).

[![Build Status](https://travis-ci.org/SchumacherFM/caddyesi.svg?branch=master)](https://travis-ci.org/SchumacherFM/caddyesi)

#### Some features:

- No full ESI support
- Multiple incoming requests trigger only one single parsing of the ESI tags per
page
- Querying multiple backend server parallel and concurrent.
- Coalesce multiple incoming requests into one single request to a backend
server
- Forwarding and returning of HTTP headers from backend servers
- Query multiple backend servers sequentially as a fall back mechanism
- Query multiple backend servers parallel and use the first returned result and
discard other responses
- Support for NoSQL Server to query a key and simply display its value
- Variables support based on Cookie or Request parameters
- Error handling and fail over. Either display a text from a string or a static
file content when a backend server is unavailable.
- Supports Go >= 1.8

## High level overview

![Architectural overview](caddy-esi-archi.png)

## Plugin configuration (optional)

```
https://cyrillschumacher.local:2718 {
    ...
    other caddy directives
    ...
    esi [/path_optional] {
        [timeout 5ms|100us|1m|...]
        [ttl 5ms|100us|1m|...]
        [max_body_size 500kib|5MB|10GB|2EB|etc]
        [page_id_source [host,path,ip, etc]]
        [allowed_methods [GET,POST,etc]]
        [cmd_header_name [X-What-Ever]]
        [cache redis://localhost:6379/0]
        [cache redis://localhost:6380/0]
        [cache memcache://localhost:11211/2]
        [cache inmemory]
        [on_error (filename|"any text")]
        [log_file (filename|stdout|stderr)]
        [log_level (fatal|info|debug)]

        # optional: next 3 are used for ESI includes
        redisAWS1 redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
        redisLocal1 redis://localhost:6381/3
        memcacheLocal2 memcache://localhost:11211/1
    }
}
```

Most of the **global optional configuration values** can be overwritten with a different value in a single ESI tag.

`esi` defines the namespace in the Caddy configuration file. All keys in the
`esi` namespace which are not reserved key words will be treated as a `src` aka.
resource to load data from. The value of the not reserved keywords must be a
valid URI. Reserved keys are:

| Config Name |  Default | Support in ESI tag | Description |
| ----------- |  ------- | ----------- |  ----------- |
| `timeout`   | 20s    | Yes | Time when a request to a resource should be canceled. [time.Duration](https://golang.org/pkg/time/#Duration) |
| `ttl`      | disabled  | Yes | Time-to-live value in the NoSQL cache for data returned from the backend resources. |
| `max_body_size` | 5MB       | Yes |  Limits the size of the returned body from a backend resource. |
| `cache` | disabled | No | Defines a cache service which stores the retrieved data from a backend resource but only when the ttl (within an ESI tag) has been set. Can only occur multiple times! |
| `page_id_source` | `host`, `path` | No | Special directive on how to identify a request to the same page. The following settings can be used to calculate the hash value. Available settings: `remoteaddr`, `realip`, `scheme`, `host`, `path`, `rawpath`, `rawquery` and `url`. Special feature to access cookies and headers: Prefix with `cookie-` or `header-` to access the appropriate value. Attention: The more granular you define the higher possibility occurs that your RAM will be filled up (will be fixed ...). |
| `allowed_methods` | `GET` | No | Any method listed here triggers the ESI middleware |
| `cmd_header_name` | Disabled | No | Specify here a header name to enable certain commands for e.g. purging the internal ESI tag cache |
| `log_file` | disabled | No | Put in here either a file name or the wordings `stderr` or `stdout` to write to those file descriptors. |
| `log_level` | disabled | No | Available key words `debug` the most verbose and `info`, less verbose. |
| `log_format` | n/a | No | Not yet supported. Ideas? |

`cmd_header_name` current supported values are:

- `purge` use the value `purge` with your defined `cmd_header_name` to purge the
ESI tag cache. `X-Esi-Cmd: purge`
- tbd

## Supported ESI Tags and their attributes

Implemented:

- [x] Caddy configuration parsing
- [x] ESI tag parsing
- [x] ESI tag replacement
- [x] Background Fetcher workers
- [x] Basic ESI Tag
- [x] With timeout
- [ ] With ttl
- [x] Load local file after timeout
- [ ] Flip src to AJAX call after timeout
- [x] Forward all headers
- [x] Forward some headers
- [ ] Return all headers
- [ ] Return some headers
- [x] Multiple sources
- [ ] Multiple sources with `race="true"`
- [x] Dynamic sources
- [ ] Conditional tag loading
- [x] Redis access
- [x] Handle compressed content from backends (Go http.Client)
- [ ] Coalesce multiple requests into one backend request
- [x] Query HTTP/S backend servers
- [x] Query and execute shell scripts/programs (stderr|out|in) communication

Implementation ideas:

ESI Tags within a pages must get replaced with a stream replacement algorithm.
Take a stream of bytes and look for the bytes “<esi:include...>” and when they
are found, replace them with the fetched content from the backend. The code
cannot assume that there are any line feeds or other delimiters in the stream
and the code must assume that the stream is of any arbitrary length. The
solution cannot meaningfully buffer to the end of the stream and then process
the replacement.

### TL;DR ESI tag options

Quick overview which options are available for two different kinds of ESI tags.

1. Querying an HTTP backend

```
<esi:include src="https://micro1.service/esi/foo" src="https://microN.service/esi/foo" 
    timeout="time.Duration" ttl="time.Duration" 
    onerror="text or path to file" maxbodysize="bytes"
    forwardheaders="all or specific comma separated list of header names"
    returnheaders="all or specific comma separated list of header names"
    coalesce="true|false"
/>
```

2. Querying a NOSQL database or service

```
<esi:include src="alias name" key="key name" onerror="text or path to file" timeout="time.Duration" />
```


### Basic tag

The basic tag waits on the `src` until `esi.timeout` cancels the loading. If src
contains an invalid URL or has not been specified at all, nothing happens.

An unreachable `src` gets marked as "failed" in the internal cache with an
exponential back-off strategy: 2ms, 4ms, 8ms, 16ms, 32ms ... or as defined in
the `esi.timeout` or tag based `timeout` attribute.

ESI tags are getting internally cached after they have been parsed. Changes to
ESI tags during development must some how trigger a cache clearance.

A Request URI defines the internal cache key for a set of ESI tags listed in an
HTML page. For each incoming request the ESI processor knows beforehand which
ESI tags to load and to process instead of parsing the HTML page sequentially.
From this follows that all ESI `src` will be loaded parallel and immediately
after that the page output starts.

```
<esi:include src="https://micro.service/esi/foo" />
```

### With timeout (optional)

The basic tag with the attribute `timeout` waits for the src until the timeout
occurs. After the timeout, the ESI tag gets not rendered and hence displays
nothing. The attribute `timeout` overwrites the default `esi.timeout`.

```
<esi:include src="https://micro.service/esi/foo" timeout="time.Duration" />
```

100% Support with http/s requests to backend services.

### With ttl (optional)

The basic tag with the attribute `ttl` stores the returned data from the `src`
in the specified `esi.backend`. The attribute `ttl` overwrites the default
`esi.ttl`. If `esi.backend` has not been set or `ttl` set to empty, caching is
disabled.

`esi.backend` uses the `src` as a cache key.

```
<esi:include src="https://micro.service/esi/foo" ttl="time.Duration" />
```
 
### Load local file after timeout (optional)

The basic tag with the attribute `timeout` waits for the src until the timeout
occurs. After the timeout, the ESI processor loads the local file or text from
the attribute `onerror` and renders it into the page. If the file does not
exists an ugly error gets shown.

Supported file extensions: `"html", "htm", "xml", "txt", "json"`

```
<esi:include src="https://micro.service/esi/foo" timeout="time.Duration" onerror="path/to/mylocalFile.html"/>

<esi:include src="https://micro.service/esi/foo" timeout="time.Duration" onerror="Cannot load weather service"/>
```

### Max body size to limit the size of the body returned from a backend (optional)

The basic tag with the attribute `maxbodysize="size"` takes care that the
returned body size has been limited to the provided value. Default body size is
5 MB. Other attributes can be additionally defined.
Available size identifiers: [https://github.com/dustin/go-humanize/blob/master/bytes.go#L34](https://github.com/dustin/go-humanize/blob/master/bytes.go#L34)

```
<esi:include src="https://micro.service/esi/foo" maxbodysize="3MB"/>
```

### Flip src to AJAX call after timeout (optional)

The basic tag with the attribute `timeout` waits for the src until the timeout
occurs. After the timeout, the ESI processor converts the URI in the `src` attribute
to an AJAX call in the frontend. TODO: JS code of the template ...

```
<esi:include src="https://micro.service/esi/foo" timeout="time.Duration" onerror="ajax"/>
```

### Forward all headers (optional)

The basic tag with the attribute `forwardheaders` forwards all incoming request
headers to the `src`. Other attributes can be additionally defined.

```
<esi:include src="https://micro.service/esi/foo" forwardheaders="all"/>
```

### Forward some headers (optional)

The basic tag with the attribute `forwardheaders` forwards only the specified
headers of the incoming request to the `src`. Other attributes can be
additionally defined.

```
<esi:include src="https://micro.service/esi/foo" forwardheaders="Cookie,Accept-Language,Authorization"/>
```

### Return all headers (optional)

The basic tag with the attribute `returnheaders` returns all `src` headers to
the final response. Other attributes can be additionally defined. If duplicate
headers from multiple sources occurs, they are getting appended to the response.

```
<esi:include src="https://micro.service/esi/foo" returnheaders="all"/>
```

### Return some headers (optional)

The basic tag with the attribute `returnheaders` returns the listed headers of
the `src` to the final response. Other attributes can be additionally defined. If
duplicate headers from multiple sources occurs, they are getting appended to the
response.

```
<esi:include src="https://micro.service/esi/foo" returnheaders="Set-Cookie"/>
```

### Coalesce multiple requests into one backend request (optional)

The basic tag with the attribute `coalesce="true"` takes care that for multiple
incoming requests only one backend request gets fired. Other attributes can be
additionally defined.

```
<esi:include src="https://micro.service/esi/foo" coalesce="true"/>
```

### Multiple sources

The basic ESI tag can contain multiple sources. The ESI processor tries to load
`src` attributes in its specified order. The next `src` gets called after the
`esi.timeout` or `timeout` occurs. Other attributes can be additionally defined.
Add the attribute `race="true"` to fire all requests at once and the one which
is the fastest gets served and the others dropped.

```
<esi:include 
    src="https://micro1.service/esi/foo" 
    src="http://micro2.service/esi/foo"
    src="https://micro3.service/esi/foo" 
    timeout="time.Duration" />
```

### Dynamic sources

The basic ESI tag can extend all `src` URLs with additional parameters from the
`http.Request` object. The Go `text/template` parser will be used once the ESI
processor detects curly brackets in the `src`.

```
<esi:include src="http://micro.service/search?query={{ .Req.Form.Encode }}"/>
<esi:include src="https://micro.service/catalog/product/?id={{ .Req.Form.Get productID }}"/>
```

### Conditional tag loading

The basic ESI tag can contain the attribute `condition`. A `condition` must
return the string `true` to trigger the loading of the `src`. The `condition`
attribute uses Go `text/template` logic. For now only the `http.Request` object
can be used for comparison. TODO: rethink this to optimize condition execution.

```
<esi:include src="http://micro.service/search?query={{ .Req.Form.Encode }}"
    condition="{{ if .Req.Host eq 'customer.micro.service' }}true{{end}}"/>
```

### Redis access

The ESI processor can access Redis resources which are specified in the Caddy
configuration. The `src` attribute must contain the valid alias name as defined
in configuration file. The ESI tag must contain a `key` attribute which accesses
the value in Redis. This value will be rendered unmodified into the HTML page
output. If multiple `src` tags are defined the next `src` gets used once the key
cannot be found in the previous source or the `timeout` applies. Return and
forward headers are not supported.

If the `key` attribute contains 2x curly brackets, the Go `text/template` logic
gets created.

```
<esi:include src="redisAWS1" src="redisLocal1" key="my_redis_key_x" timeout="time.Duration" />
<esi:include src="redisAWS1" key="my_redis_key_x" onerror="myLocalFile.html" timeout="time.Duration" />
<esi:include src="redis1" key="prefix_{{ .Req.Host }}_{{ .Header.Get "X-Whatever" }}" timeout="time.Duration" />
```

### Registered schemes or aliases in the src attribute

The `src` attribute can contain the following prefixes to fetch content from a
resource backend:

- http://
- https://
- sh://
- Name of the aliases as defined in the Caddyfile
- sql:// a SQL query (TODO)
- rpc:// a remote procedure call with GRPC (TODO)

#### http

`http|s` allows the usual HTTP requests to a resource.

#### sh shell

`sh` starts a shell program or any other executable script. The incoming HTTP
request plus the current arguments gets transformed into a JSON string and will
be passed as first argument to the program via stdin. If the program writes to
stderr CaddyESI recognizes that and triggers it as an usual error. Too many
errors and the circuit breaker opens. To output towards the HTTP response gets written
to stdout. The src attribute must look like:

```html
<esi:include src="sh://myGoBinary" />
<esi:include src="sh:///path/to/my/slow/php/script.php" />

illegal (for now):
<esi:include src="sh://php /path/to/my/slow/php/script.php" />
```

*ProTip:* Provide always the full path to the binary or script because the
*lookup time in the operation systems environment PATH variable will take long.
*In general shell execution is pretty slow no matter which kind of
*program/script you call.

#### sql queries TODO

`sql` Creates a prepared statement once the ESI tag has been parsed.

```html
<esi:include src="sql://SELECT value FROM myTable where key=?; r.Header.Get " />
```

#### rpc Remote Procedure Calls TODO

`rpc` queries another GRPC endpoint


```html
<esi:include src="rpc://Name" key="cart1" />
```

## Unsupported ESI Tags

All other tags, as defined in
[https://www.w3.org/TR/esi-lang](https://www.w3.org/TR/esi-lang), won't be
supported. You should switch to a server side scripting language ;-).

## Future backwards compatibility breaks

1. Due to performance reasons the template engine text/template might get
replaced by a different package with a different syntax.
2. That's it for now.

# Contribute

Send me a pull request or open an issue if you encounter a bug or something can
be improved!

Multi-time pull request senders gets collaborator access.

# License

[Cyrill Schumacher](https://github.com/SchumacherFM) - [My pgp public key](https://www.schumacher.fm/cyrill.asc)

Copyright 2016 Cyrill Schumacher All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
