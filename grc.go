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

var (
	ErrUnknownProvider = errors.New("unknown provider type")
	ErrEmptyCallback   = errors.New("invalid callback")
)

type AtomicUpdateConfig func(interface{})
type AtomicUpdateNodes func([]string)

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

	return &RemoteConfig{
		ctx:      ctx,
		provider: provider,
	}, nil
}

type RemoteConfig struct {
	ctx      context.Context
	provider backend.Provider
}

// Register node for service discovery
func (rc *RemoteConfig) RegisterNode(path, nodeID string, ttl time.Duration) error {
	key := fmt.Sprintf("%v/%v", path, nodeID)
	return rc.provider.KeepAlive(rc.ctx, key, ttl)
}

// Subscribe specified service nodes
func (rc *RemoteConfig) SubscribeNodes(path string, callback AtomicUpdateNodes) error {
	if callback == nil {
		return ErrEmptyCallback
	}

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
	// callback
	callback(nodes)
	// watch and update
	go rc.nodeUpdated(evtChan, path, nodes, callback)

	return nil
}

// Subscribe remote config, return value type is the same as val which is reflect.TypeOf(val).
func (rc *RemoteConfig) SubscribeConf(path string, val interface{}, callback AtomicUpdateConfig) error {
	if callback == nil {
		return ErrEmptyCallback
	}

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
	// callback
	callback(config)
	// watch for config updated
	go rc.configUpdated(evtChan, val, callback)

	return nil
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
