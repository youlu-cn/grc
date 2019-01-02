package grc

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/youlu-cn/grc/backend"
)

var (
	provider = NewTestProvider()
	rc, _    = NewWithProvider(context.TODO(), provider)
)

type Value struct {
	k      string
	v      string
	expire time.Time
}

type Watch struct {
	ch     backend.EventChan
	key    string
	prefix bool
}

type TestProvider struct {
	m    map[string]*Value
	w    []*Watch
	stop chan bool
	sync.RWMutex
}

func NewTestProvider() backend.Provider {
	p := &TestProvider{
		m:    make(map[string]*Value),
		stop: make(chan bool),
	}
	go p.checkTTL()
	return p
}

func (p *TestProvider) Type() string {
	return "test"
}

func (p *TestProvider) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl == 0 {
		ttl = time.Hour * 24
	}
	p.Lock()
	defer p.Unlock()
	p.m[key] = &Value{
		k:      key,
		v:      value,
		expire: time.Now().Add(ttl),
	}
	p.checkWatch(key, value, backend.Put)
	return nil
}

func (p *TestProvider) Get(ctx context.Context, key string, withPrefix bool) (backend.KVPairs, error) {
	p.RLock()
	defer p.RUnlock()
	if !withPrefix {
		if v, ok := p.m[key]; !ok {
			return backend.KVPairs{}, nil
		} else {
			return backend.KVPairs{
				{
					Key:   key,
					Value: v.v,
				},
			}, nil
		}
	}
	//
	var kvs backend.KVPairs
	for k, v := range p.m {
		if strings.HasPrefix(k, key) {
			kvs = append(kvs, &backend.KVPair{
				Key:   k,
				Value: v.v,
			})
		}
	}
	return kvs, nil
}

func (p *TestProvider) Watch(ctx context.Context, key string, withPrefix bool) backend.EventChan {
	p.Lock()
	defer p.Unlock()
	ch := make(backend.EventChan, 100)
	p.w = append(p.w, &Watch{
		ch:     ch,
		key:    key,
		prefix: withPrefix,
	})
	return ch
}

func (p *TestProvider) checkWatch(key string, value string, typ string) {
	// check watch
	for _, w := range p.w {
		if w.prefix && strings.HasPrefix(key, w.key) {
			w.ch <- &backend.WatchEvent{
				Type: typ,
				KVPair: backend.KVPair{
					Key:   key,
					Value: value,
				},
			}
		}
	}
}

func (p *TestProvider) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	v := strconv.FormatInt(time.Now().UnixNano(), 10)
	return p.Put(ctx, key, v, ttl)
}

func (p *TestProvider) checkTTL() {
	ticker := time.NewTicker(time.Millisecond * 50)

	for {
		select {
		case <-p.stop:
			ticker.Stop()
			return
		case <-ticker.C:
			p.Lock()
			for k, v := range p.m {
				if time.Now().Sub(v.expire) > 0 {
					delete(p.m, k)
					p.checkWatch(v.k, v.v, backend.Delete)
				}
			}
			p.Unlock()
		}
	}
}

func (p *TestProvider) Close() error {
	p.stop <- true
	return nil
}

type TestConfig struct {
	IntVal   int                 `json:"int_val" default:"8080"`
	StrVal   string              `default:"test"`
	SliceVal []int               `json:"slice_val" default:"1,2,3,tt"` // tt will be 0
	MapVal   map[string]struct{} `default:"a,b,c,d,a"`                 // will be only one "a"
}

var (
	defVal = TestConfig{
		IntVal:   8080,
		StrVal:   "test",
		SliceVal: []int{1, 2, 3, 0},
		MapVal: map[string]struct{}{
			"a": {},
			"b": {},
			"c": {},
			"d": {},
		},
	}
)

func TestRemoteConfig_GetDefault(t *testing.T) {
	path := "/test/conf/TestRemoteConfig_GetDefault"
	err := rc.SubscribeConf(path, TestConfig{})
	if err != nil {
		t.Fatal("get config fail", err)
	}
	if config, ok := rc.GetConf(path).(TestConfig); !ok || !reflect.DeepEqual(config, defVal) {
		t.Fatal("parse err", config, defVal, ok)
	}

	err = rc.SubscribeConf(path, &TestConfig{})
	if err != nil {
		t.Fatal("get config ptr fail", err)
	}
	if pc, ok := rc.GetConf(path).(*TestConfig); !ok || !reflect.DeepEqual(pc, &defVal) {
		t.Fatal("parse err", pc, defVal, ok)
	}
}

func TestRemoteConfig_Get(t *testing.T) {
	path := "/test/conf/TestRemoteConfig_Get"
	_ = provider.Put(context.TODO(), path, `{"int_val":0,"MapVal":{"key":{}}}`, time.Minute)

	conf := TestConfig{
		IntVal:   0,
		StrVal:   "test",            // use default
		SliceVal: []int{1, 2, 3, 0}, // use default
		MapVal: map[string]struct{}{
			"key": {},
		},
	}

	err := rc.SubscribeConf(path, TestConfig{})
	if err != nil {
		t.Fatal("get config fail", err)
	}
	if config, ok := rc.GetConf(path).(TestConfig); !ok || !reflect.DeepEqual(config, conf) {
		t.Fatal("parse err", config, conf, ok)
	}
}

func TestRemoteConfig_RegisterSubscribe(t *testing.T) {
	path := "/test/nodes/TestRemoteConfig_RegisterSubscribe"
	nodes := []string{
		"test_node_1",
		"test_node_3",
		"test_node_2",
		"test_node_0",
	}

	for i, node := range nodes {
		_ = rc.RegisterNode(path, node, time.Second*time.Duration(i+1))
	}
	err := rc.SubscribeService(path)
	if err != nil || len(rc.GetService(path)) != 4 {
		t.Fatal("subscribe failed", rc.GetService(path), err)
	}
	sort.Strings(nodes)
	if !reflect.DeepEqual(rc.GetService(path), nodes) {
		t.Fatal("invalid subscribe vals")
	}

	time.Sleep(time.Millisecond * 1200)
	if len(rc.GetService(path)) != 3 {
		t.Fatal("subscribe failed", rc.GetService(path), err)
	}
}
