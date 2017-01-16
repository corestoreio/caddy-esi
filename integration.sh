#!/usr/bin/env bash

sed -i.bak '/This is where other plugins get plugged in (imported)/a\
_ "github.com/SchumacherFM/caddyesi"\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go

sed -i.bak '/directives that add middleware to the stack/a\
"esi",\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddyhttp/httpserver/plugin.go

go build -o caddy.bin $GOPATH/src/github.com/mholt/caddy/caddy/main.go
nohup ./caddy.bin -conf ./Caddyfile &

#curl -i 'http://127.0.0.1:2017/page01.html' # parse
#curl -i 'http://127.0.0.1:2017/page01.html' # from ESI cache

ht exec -output _result -v ./ht/...
