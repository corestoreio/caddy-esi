# ESI Tags for Caddy Server

This plugin implements ESI (Edge Side Includes) tag parsing and cache handling
for the [https://caddyserver.com](Caddy Webserver).


## High level overview

![Architectural overview](caddy-esi-archi.png)

## Plugin configuration (optional)

```
https://cyrillschumacher.local:2718 {
    ...
    other caddy directives
    ...
    esi [/path_optional] {
        timeout 5ms|100us|1m|...
        ttl 5ms|100us|1m|...
        [page_id_source [host,path,ip, etc]]
        [allowed_methods [GET,POST,DELETE]]
        [cache redis://localhost:6379/0]
        [cache redis://localhost:6380/0]
        [cache memcache://localhost:11211/2]
        [cache inmemory]
        [log_file (filename|stdout|stderr)]
        [log_level (fatal|info|debug)]

        # next 3 are used for ESI includes
        redisAWS1 redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0
        redisLocal1 redis://localhost:6381/3
        memcacheLocal2 memcache://localhost:11211/1
    }
}
```

`esi` defines the namespace in the Caddy configuration file. All keys in the
`esi` namespace which are not reserved key words will be treated as a `src` aka.
resource to load data from. The value of the not reserved keywords must be a
valid URI. Reserved keys are:

- `timeout`, default x [time.Duration](https://golang.org/pkg/time/#Duration).
Time when a request to a source should be canceled. Can only occur one time.
- `ttl`, global time-to-live in the cache for ESI data. Defaults to
zero, caching disabled. Can only occur one time.
- `cache` defines a cache service which stores the retrieved data from a e.g.
micro service but only when the ttl (within an ESI tag) has been set. Can only
occur multiple times.
- `page_id_source` optional and special directive on how to identify a
request to the same page. The following settings can be used to calculate the
hash value. Available settings: `remoteaddr`, `realip`, `scheme`, `host`,
`path`, `rawpath`, `rawquery` and `url`. Special feature to access cookies and
headers: Prefix with `cookie-` or `header-` to access the appropriate value.
Default setting: `host` and `path`. Attention: The more granular you define the
more possibility occurs that your RAM will be filled up (will be fixed ...).
- `allowed_methods` optional, defaults to GET only.
- `log_file` optional, default logging disabled. Put in here either a file name
or the wordings stderr or stdout to write to those file descriptors. If empty,
logging is disabled.
- `log_level` optional, default logging disabled. Debug is most verbose.  If
empty or unknown, logging is disabled.
- `log_format` not yet supported. Ideas?
- ... ?

## Supported ESI Tags

Implemented:

- [x] Caddy configuration parsing
- [x] ESI tag parsing
- [ ] ESI tag replacement
- [ ] Background Fetcher workers
- [ ] Basic ESI Tag
- [ ] With timeout
- [ ] With ttl
- [ ] Load local file after timeout
- [ ] Flip src to AJAX call after timeout
- [ ] Forward all headers
- [ ] Forward some headers
- [ ] Return all headers
- [ ] Return some headers
- [ ] Multiple sources
- [ ] Dynamic sources
- [ ] Conditional tag loading
- [ ] Redis access
- [ ] Handle compressed content from backends
- [ ] Coalesce multiple requests into one backend request

Implementation ideas:

ESI Tags within a pages must get replaced with a stream replacement algorithm.
Take a stream of bytes and look for the bytes “<esi:include...>” and when they
are found, replace them with the fetched content from the backend. The code
cannot assume that there are any line feeds or other delimiters in the stream
and the code must assume that the stream is of any arbitrary length. The
solution cannot meaningfully buffer to the end of the stream and then process
the replacement.


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
`esi.timeout` or `timeout`. Other attributes can be additionally defined. Add
the attribute `race="true"` to fire all requests at once and the one which is
the fastest gets served and the others dropped.

```
<esi:include 
    src="https://micro1.service/esi/foo" 
    src="https://micro2.service/esi/foo" 
    src="https://micro3.service/esi/foo" 
    timeout="time.Duration" />
```

### Dynamic sources

The basic ESI tag can extend all `src` URLs with additional parameters from the
`http.Request` object. The Go `text/template` parser will be used once the ESI
processor detects curly brackets in the `src`.

```
<esi:include src="http://micro.service/search?query={{ r.Form.Encode }}"/>
<esi:include src="https://micro.service/catalog/product/?id={{ r.Form.Get productID }}"/>
```

### Conditional tag loading

The basic ESI tag can contain the attribute `condition`. A `condition` must
return the string `true` to trigger the loading of the `src`. The `condition`
attribute uses Go `text/template` logic. For now only the `http.Request` object
can be used for comparison. TODO: rethink this to optimize condition execution.

```
<esi:include src="http://micro.service/search?query={{ r.Form.Encode }}" 
    condition="{{ if r.Host eq 'customer.micro.service' }}true{{end}}"/>
```

### Redis access

The ESI processor can access Redis resources which are specified in the Caddy
configuration. The `src` attribute must contain the valid reference name as
defined in config. The ESI tag must contain a `key` attribute which accesses the
value in Redis. This value will be rendered unmodified into the HTML page
output. If multiple `src` tags are defined the next `src` gets used once the key
cannot be found in the previous source or the `timeout` applies.

If the `key` attribute contains curly brackets, the Go `text/template` logic
gets created.

```
<esi:include src="redisAWS1" src="redisLocal1" key="my_redis_key_x" timeout="time.Duration" />
<esi:include src="redisAWS1" key="my_redis_key_x" onerror="myLocalFile.html" timeout="time.Duration" />
<esi:include src="redis1" key="prefix_{{ r.Host }}_{{ r.Header.Get "X-Whatever" }}" timeout="time.Duration" />
```

## Unsupported ESI Tags

All other tags, as defined in
[https://www.w3.org/TR/esi-lang](https://www.w3.org/TR/esi-lang), won't be
supported. You should switch to a server side scripting language ;-).

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
