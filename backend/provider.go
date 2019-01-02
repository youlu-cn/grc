package backend

import (
	"context"
	"time"
)

const (
	EtcdV3 = "etcdv3"
)

const (
	DialTimeout  = time.Second * 3
	ReadTimeout  = time.Second * 3
	WriteTimeout = time.Second * 3
)

const (
	Put    = "put"
	Delete = "delete"
)

type KVPair struct {
	Key   string
	Value string
}

type KVPairs []*KVPair

type WatchEvent struct {
	KVPair
	Type string
}

type EventChan chan *WatchEvent

const (
	DefaultChanLen = 100
)

type Provider interface {
	// Return provider type
	Type() string

	// Put value with the specified key
	Put(ctx context.Context, key, value string, ttl time.Duration) error

	// Get value of the specified key or path
	Get(ctx context.Context, key string, withPrefix bool) (KVPairs, error)

	// Watch for changes of the specified key or path
	Watch(ctx context.Context, key string, withPrefix bool) EventChan

	// Keep alive to backend provider
	KeepAlive(ctx context.Context, key string, ttl time.Duration) error

	// Close the connection
	Close() error
}
