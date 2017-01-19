#!/usr/bin/env bash

sed -i.bak '/This is where other plugins get plugged in (imported)/a\
_ "github.com/SchumacherFM/caddyesi"\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddy/caddymain/run.go

sed -i.bak '/directives that add middleware to the stack/a\
"esi",\'$'\n' $GOPATH/src/github.com/mholt/caddy/caddyhttp/httpserver/plugin.go

go build -o caddy.bin $GOPATH/src/github.com/mholt/caddy/caddy/main.go
go build -o ht.bin $GOPATH/src/github.com/SchumacherFM/caddyesi/ht/*.go

nohup ./caddy.bin -conf ./Caddyfile &
sleep 5
./ht.bin

killall caddy.bin
rm -f *.bin nohup.out
