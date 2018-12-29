package grc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/youlu-cn/grc/backend"
	"github.com/youlu-cn/grc/backend/etcd/v3"
)

type AtomicUpdateConfig func(interface{})
type AtomicUpdateNodes func([]string)

func NewProvider(ctx context.Context, typ backend.ProviderType, endPoint, user, password string) (backend.Provider, error) {
	switch typ {
	case backend.EtcdV3:
		return etcdv3.NewProvider(ctx, endPoint, user, password)
	default:
		return nil, nil
	}
}

func New(ctx context.Context, provider backend.Provider) *RemoteConfig {
	return &RemoteConfig{
		ctx:      ctx,
		provider: provider,
	}
}

type RemoteConfig struct {
	ctx      context.Context
	provider backend.Provider
}

// Register for service discovery
func (rc *RemoteConfig) Register(path, nodeID string, ttl time.Duration) error {
	key := fmt.Sprintf("%v/%v", path, nodeID)
	return rc.provider.KeepAlive(key, ttl)
}

// Subscribe specified service nodes
func (rc *RemoteConfig) Subscribe(path string, callback AtomicUpdateNodes) ([]string, error) {
	evtChan := rc.provider.Watch(path, true)
	kvs, err := rc.provider.Get(path, true)
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
	// watch and update
	if callback != nil {
		go rc.nodeUpdated(evtChan, path, nodes, callback)
	}

	return nodes, nil
}

// Get remote config, return value type is the same as val which is reflect.TypeOf(val).
func (rc *RemoteConfig) Get(path string, val interface{}, callback AtomicUpdateConfig) (interface{}, error) {
	var (
		config interface{}
	)

	evtChan := rc.provider.Watch(path, false)
	kvs, err := rc.provider.Get(path, false)
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
	// watch for config updated
	if callback != nil {
		go rc.configUpdated(evtChan, val, callback)
	}

	return config, err
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

func (rc *RemoteConfig) configUpdated(ch backend.EventChan, val interface{}, callback AtomicUpdateConfig) {
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
			// callback
			callback(config)
		}
	}
}

func (rc *RemoteConfig) nodeUpdated(ch backend.EventChan, path string, nodes []string, callback AtomicUpdateNodes) {
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
		// callback
		callback(nodes)
	}
}
