package caddyesi

import (
	"context"
	"net/http"
	"time"
)

var _ KVFetcher = (*kvFetchMock)(nil)
var _ ResourceFetcher = (*resourceMock)(nil)
var _ Cacher = (*Caches)(nil)

type kvFetchMock struct {
	getFn   func(ctx context.Context, key []byte) ([]byte, error)
	closeFn func() error
}

func (kv kvFetchMock) Close() error {
	if kv.closeFn != nil {
		return kv.closeFn()
	}
	return nil
}

func (kv kvFetchMock) Get(ctx context.Context, key []byte) ([]byte, error) {
	if kv.getFn != nil {
		return kv.getFn(ctx, key)
	}
	return nil, nil
}

type resourceMock struct {
	getFn   func(*http.Request) ([]byte, error)
	closeFn func() error
}

func (rm resourceMock) Get(r *http.Request) ([]byte, error) {
	if rm.getFn != nil {
		return rm.getFn(r)
	}
	return nil, nil
}

func (rm resourceMock) Close() error {
	if rm.closeFn != nil {
		return rm.closeFn()
	}
	return nil
}

type cacherMock struct {
	setFn func(key string, value []byte, expiration time.Duration) error
	getFn func(key string) ([]byte, error)
}

func (cm cacherMock) Set(key string, value []byte, expiration time.Duration) error {
	if cm.setFn != nil {
		return cm.setFn(key, value, expiration)
	}
	return nil
}
func (cm cacherMock) Get(key string) ([]byte, error) {
	if cm.getFn != nil {
		return cm.getFn(key)
	}
	return nil, nil
}
