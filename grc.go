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
	"github.com/youlu-cn/grc/backend/debug"
	etcdv3 "github.com/youlu-cn/grc/backend/etcd/v3"
)

var (
	ErrUnknownProvider = errors.New("unknown provider type")
	ErrNotRegistered   = errors.New("service or config not registered")
)

type ProviderType string

const (
	Debug  ProviderType = backend.Debug
	EtcdV3              = backend.EtcdV3
)

func New(ctx context.Context, typ ProviderType, endPoint, user, password string) (*RemoteConfig, error) {
	var (
		provider backend.Provider
		err      error
	)

	switch typ {
	case backend.Debug:
		provider, err = debug.NewProvider()
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
	evtChan := rc.provider.Watch(rc.ctx, path, true)
	nodes, err := rc.parseNode(path)
	if err != nil {
		return err
	}

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
	evtChan := rc.provider.Watch(rc.ctx, path, false)
	config, err := rc.parseConfig(path, val)
	if err != nil {
		return err
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

func (rc *RemoteConfig) parseConfig(path string, val interface{}) (interface{}, error) {
	var config interface{}
	ctx, cancel := context.WithCancel(rc.ctx)
	defer cancel()
	kvs, err := rc.provider.Get(ctx, path, false)
	if err != nil {
		return nil, err
	}

	// decode config
	if len(kvs) == 0 {
		config = rc.defaultValue(val)
	} else {
		config = Decode(kvs[0].Value, val)
	}
	if config == nil {
		log.Println("decode config failed", kvs[0].Value)
		return nil, errors.New("decode config failed:" + kvs[0].Value)
	}
	return config, nil
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
				err    error
				config interface{}
			)

			switch evt.Type {
			case backend.Put:
				config = Decode(evt.Value, val)
			case backend.Delete:
				config = rc.defaultValue(val)
			case backend.Reset:
				config, err = rc.parseConfig(path, val)
				if err != nil {
					continue
				}
			}

			if config == nil {
				log.Println("decode config failed", evt.Value)
				continue
			}

			// update
			rc.Lock()
			rc.config[path] = config
			rc.Unlock()
		}
	}
}

func (rc *RemoteConfig) parseNode(path string) ([]string, error) {
	ctx, cancel := context.WithCancel(rc.ctx)
	defer cancel()
	kvs, err := rc.provider.Get(ctx, path, true)
	if err != nil {
		return nil, err
	}

	// parse node list
	nodes := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		node := strings.TrimPrefix(kv.Key, path+"/")
		nodes = append(nodes, node)
	}

	// sort node list
	sort.Strings(nodes)
	return nodes, nil
}

func (rc *RemoteConfig) nodeUpdated(ch backend.EventChan, path string, nodes []string) {
	m := make(map[string]int, len(nodes))
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
			switch evt.Type {
			case backend.Put:
				m[node] = 0
			case backend.Delete:
				delete(m, node)
			case backend.Reset:
				ns, err := rc.parseNode(path)
				if err != nil {
					continue
				}
				m = make(map[string]int, len(ns))
				for _, node := range ns {
					m[node] = 0
				}
			}
		}

		nodes = make([]string, 0, len(m))
		for node := range m {
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
