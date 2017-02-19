#!/usr/bin/env bash
set -o pipefail
set -e

sed -i.bak '/This is where other plugins get plugged in (imported)/a\
_ "github.com/SchumacherFM/caddyesi"\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go

sed -i.bak '/directives that add middleware to the stack/a\
"esi",\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddyhttp/httpserver/plugin.go

redis-cli -n 0 SET "product_001" "Catalog Product 001"
redis-cli -n 0 SET "category_tree" "Catalog Category Tree"
redis-cli -n 1 SET "checkout_cart" "You have 10 items in your cart"
# redis-cli MGET "category_tree" "product_001"
# redis-cli -n 1 GET "checkout_cart"

go build -o esigrpc.bin www_root/grpc_server_integration.go
go build -tags esiall -race -o caddy.bin $GOPATH/src/github.com/mholt/caddy/caddy/main.go
# go run -tags esiall $GOPATH/src/github.com/mholt/caddy/caddy/main.go -conf ./Caddyfile

./esigrpc.bin &
./caddy.bin -conf ./Caddyfile &
sleep 6
go run $GOPATH/src/github.com/SchumacherFM/caddyesi/ht/*.go
