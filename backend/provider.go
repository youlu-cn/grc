package backend

import (
	"time"
)

type ProviderType string

const (
	EtcdV3 ProviderType = "etcdv3"
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
	// Return provider
	Type() ProviderType

	// Put value with the specified key
	Put(key, value string, ttl time.Duration) error

	// Get value of the specified key or path
	Get(key string, withPrefix bool) (KVPairs, error)

	// Watch for changes of the specified key or path
	Watch(key string, withPrefix bool) EventChan

	// Keep alive to backend provider
	KeepAlive(key string, ttl time.Duration) error

	// Close the connection
	Close() error
}
