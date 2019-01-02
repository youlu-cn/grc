package grc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/youlu-cn/grc/backend"
	"github.com/youlu-cn/grc/backend/etcd/v3"
)

var (
	ErrUnknownProvider = errors.New("unknown provider type")
	ErrNotRegistered   = errors.New("service or config not registered")
)

type ProviderType string

const (
	EtcdV3 ProviderType = backend.EtcdV3
)

func New(ctx context.Context, typ ProviderType, endPoint, user, password string) (*RemoteConfig, error) {
	var (
		provider backend.Provider
		err      error
	)

	switch typ {
	case backend.EtcdV3:
		provider, err = etcdv3.NewProvider(endPoint, user, password)
	default:
		err = ErrUnknownProvider
	}

	if err != nil {
		return nil, err
	}
	return NewWithProvider(ctx, provider)
}

func NewWithProvider(ctx context.Context, provider backend.Provider) (*RemoteConfig, error) {
	return &RemoteConfig{
		ctx:      ctx,
		provider: provider,
		config:   make(map[string]interface{}),
		service:  make(map[string][]string),
	}, nil
}

type RemoteConfig struct {
	ctx      context.Context
	provider backend.Provider

	config  map[string]interface{}
	service map[string][]string
	sync.RWMutex
}

// Register node for service discovery
func (rc *RemoteConfig) RegisterNode(path, nodeID string, ttl time.Duration) error {
	key := fmt.Sprintf("%v/%v", path, nodeID)
	return rc.provider.KeepAlive(rc.ctx, key, ttl)
}

// Subscribe specified service
func (rc *RemoteConfig) SubscribeService(path string) error {
	ctx, cancel := context.WithCancel(rc.ctx)
	evtChan := rc.provider.Watch(ctx, path, true)
	kvs, err := rc.provider.Get(rc.ctx, path, true)
	if err != nil {
		cancel()
		return err
	}

	// parse node list
	nodes := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		node := strings.TrimPrefix(kv.Key, path+"/")
		nodes = append(nodes, node)
	}

	// sort node list
	sort.Strings(nodes)
	// store
	rc.Lock()
	rc.service[path] = nodes
	rc.Unlock()

	// watch and update
	go rc.nodeUpdated(evtChan, path, nodes)

	return nil
}

// Get specified service nodes
func (rc *RemoteConfig) GetService(path string) []string {
	rc.RLock()
	nodes, ok := rc.service[path]
	rc.RUnlock()

	if !ok {
		log.Println("service not registered", path)
		panic(ErrNotRegistered)
	}
	return nodes
}

// Subscribe remote config
func (rc *RemoteConfig) SubscribeConf(path string, val interface{}) error {
	var config interface{}
	ctx, cancel := context.WithCancel(rc.ctx)
	evtChan := rc.provider.Watch(ctx, path, false)
	kvs, err := rc.provider.Get(rc.ctx, path, false)
	if err != nil {
		cancel()
		return err
	}

	// decode config
	if len(kvs) == 0 {
		config = rc.defaultValue(val)
	} else {
		config = Decode(kvs[0].Value, val)
	}
	if config == nil {
		log.Println("decode config failed", kvs[0].Value)
		return errors.New("decode config failed:" + kvs[0].Value)
	}

	// store
	rc.Lock()
	rc.config[path] = config
	rc.Unlock()

	// watch for config updated
	go rc.configUpdated(evtChan, path, val)

	return nil
}

func (rc *RemoteConfig) GetConf(path string) interface{} {
	rc.RLock()
	conf, ok := rc.config[path]
	rc.RUnlock()

	if !ok {
		log.Println("config not registered", path)
		panic(ErrNotRegistered)
	}
	return conf
}

func (rc *RemoteConfig) defaultValue(val interface{}) interface{} {
	data := ""
	switch reflect.TypeOf(val).Kind() {
	case reflect.Array, reflect.Slice:
		data = "[]"
	default:
		data = "{}"
	}
	config := Decode(data, val)
	if config == nil {
		log.Println("decode default config failed", data)
	}
	return config
}

func (rc *RemoteConfig) configUpdated(ch backend.EventChan, path string, val interface{}) {
	for {
		select {
		case <-rc.ctx.Done():
			err := rc.provider.Close()
			log.Println("stopping.. close provider failed", err)
			return

		case evt := <-ch:
			var (
				config interface{}
				data   = evt.Value
			)
			if evt.Type == backend.Put {
				config = Decode(data, val)
			} else if evt.Type == backend.Delete {
				config = rc.defaultValue(val)
			}
			if config == nil {
				log.Println("decode config failed", data)
				continue
			}

			// update
			rc.Lock()
			rc.config[path] = config
			rc.Unlock()
		}
	}
}

func (rc *RemoteConfig) nodeUpdated(ch backend.EventChan, path string, nodes []string) {
	m := make(map[string]int)
	for _, node := range nodes {
		m[node] = 0
	}

	for {
		select {
		case <-rc.ctx.Done():
			err := rc.provider.Close()
			log.Println("stopping.. close provider failed", err)
			return

		case evt := <-ch:
			node := strings.TrimPrefix(evt.Key, path+"/")
			if evt.Type == backend.Put {
				m[node] = 0
			} else if evt.Type == backend.Delete {
				delete(m, node)
			}
		}

		nodes = make([]string, 0, len(m))
		for node, _ := range m {
			nodes = append(nodes, node)
		}
		// sort
		sort.Strings(nodes)

		// update
		rc.Lock()
		rc.service[path] = nodes
		rc.Unlock()
	}
}
