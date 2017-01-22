#!/usr/bin/env bash
set -o pipefail

sed -i.bak '/This is where other plugins get plugged in (imported)/a\
_ "github.com/SchumacherFM/caddyesi"\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go

sed -i.bak '/directives that add middleware to the stack/a\
"esi",\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddyhttp/httpserver/plugin.go

redis-cli SET "product_001" "Catalog Product 001"
redis-cli SET "category_tree" "Catalog Category Tree"

go build -o caddy.bin $GOPATH/src/github.com/mholt/caddy/caddy/main.go
# go run $GOPATH/src/github.com/mholt/caddy/caddy/main.go -conf ./Caddyfile

nohup ./caddy.bin -conf ./Caddyfile &
sleep 6
go run $GOPATH/src/github.com/SchumacherFM/caddyesi/ht/*.go

