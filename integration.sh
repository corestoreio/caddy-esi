#!/usr/bin/env bash

sed -i.bak '/This is where other plugins get plugged in (imported)/a\
_ "github.com/SchumacherFM/caddyesi"\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go

sed -i.bak '/directives that add middleware to the stack/a\
"esi",\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddyhttp/httpserver/plugin.go

redis-cli SET "product_001" "Catalog Product 001"
redis-cli SET "category_tree" "Catalog Category Tree"

go build -o caddy.bin $GOPATH/src/github.com/mholt/caddy/caddy/main.go

nohup ./caddy.bin -conf ./Caddyfile &
sleep 5
go run $GOPATH/src/github.com/SchumacherFM/caddyesi/ht/*.go

echo "temporary debugging cURL requests"
curl -s 'http://127.0.0.1:2017/page02.html' | wc
curl -i 'http://127.0.0.1:2017/page02.html'

killall caddy.bin
rm -f *.bin nohup.out
