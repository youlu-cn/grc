package debug

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/youlu-cn/grc/backend"
)

var (
	zeroTime = time.Unix(0, 0)
)

type node struct {
	k      string
	v      string
	expire time.Time
}

type watch struct {
	ch     backend.EventChan
	key    string
	prefix bool
}

type Memory struct {
	kvs map[string]*node
	ws  []*watch

	event  backend.EventChan
	ctx    context.Context
	cancel context.CancelFunc
	sync.RWMutex
}

func NewProvider() (backend.Provider, error) {
	p := &Memory{
		kvs:   make(map[string]*node),
		event: make(backend.EventChan, 10),
	}
	p.ctx, p.cancel = context.WithCancel(context.TODO())
	go p.checkTTL()
	go p.checkWatch()
	return p, nil
}

func (p *Memory) Type() string {
	return backend.Debug
}

func (p *Memory) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	p.Lock()
	defer p.Unlock()
	expire := time.Now().Add(ttl)
	if ttl == 0 {
		expire = zeroTime
	}
	p.kvs[key] = &node{
		k:      key,
		v:      value,
		expire: expire,
	}
	p.event <- &backend.WatchEvent{
		Type: backend.Put,
		KVPair: backend.KVPair{
			Key:   key,
			Value: value,
		},
	}
	return nil
}

func (p *Memory) Get(ctx context.Context, key string, withPrefix bool) (backend.KVPairs, error) {
	p.RLock()
	defer p.RUnlock()
	if !withPrefix {
		if v, ok := p.kvs[key]; !ok {
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
	for k, v := range p.kvs {
		if strings.HasPrefix(k, key) {
			kvs = append(kvs, &backend.KVPair{
				Key:   k,
				Value: v.v,
			})
		}
	}
	return kvs, nil
}

func (p *Memory) Watch(ctx context.Context, key string, withPrefix bool) backend.EventChan {
	p.Lock()
	defer p.Unlock()
	ch := make(backend.EventChan, 10)
	p.ws = append(p.ws, &watch{
		ch:     ch,
		key:    key,
		prefix: withPrefix,
	})
	return ch
}

func (p *Memory) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	v := strconv.FormatInt(time.Now().UnixNano(), 10)
	return p.Put(ctx, key, v, 0)
}

func (p *Memory) Close() error {
	p.cancel()
	return nil
}

func (p *Memory) checkTTL() {
	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case <-p.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			p.Lock()
			for k, v := range p.kvs {
				if v.expire.Sub(zeroTime) > 0 && time.Now().Sub(v.expire) > 0 {
					delete(p.kvs, k)
					p.event <- &backend.WatchEvent{
						Type: backend.Delete,
						KVPair: backend.KVPair{
							Key:   v.k,
							Value: v.v,
						},
					}
				}
			}
			p.Unlock()
		}
	}
}

func (p *Memory) checkWatch() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case evt := <-p.event:
			p.RLock()
			for _, w := range p.ws {
				if w.prefix && strings.HasPrefix(evt.Key, w.key) {
					w.ch <- evt
				}
			}
			p.RUnlock()
		}
	}
}
