package etcdv3

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/youlu-cn/grc/backend"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

type EtcdV3 struct {
	*clientv3.Client
}

func NewProvider(endPoint, user, password string) (backend.Provider, error) {
	endPoints := strings.Split(endPoint, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endPoints,
		DialTimeout: backend.DialTimeout,
		Username:    user,
		Password:    password,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdV3{cli}, nil
}

func (v3 *EtcdV3) Type() string {
	return backend.EtcdV3
}

func (v3 *EtcdV3) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	var options []clientv3.OpOption
	if ttl > 0 {
		ctx, cancel := context.WithTimeout(ctx, backend.WriteTimeout)
		lease, err := v3.Grant(ctx, int64(ttl.Seconds()))
		cancel()
		if err != nil {
			return err
		}
		options = append(options, clientv3.WithLease(lease.ID))
	}

	ctx, cancel := context.WithTimeout(ctx, backend.WriteTimeout)
	defer cancel()
	_, err := v3.Client.Put(ctx, key, value, options...)
	return err
}

func (v3 *EtcdV3) Get(ctx context.Context, key string, withPrefix bool) (backend.KVPairs, error) {
	var options []clientv3.OpOption
	if withPrefix {
		options = append(options, clientv3.WithPrefix())
	}

	ctx, cancel := context.WithTimeout(ctx, backend.ReadTimeout)
	defer cancel()
	resp, err := v3.Client.Get(ctx, key, options...)
	if err != nil {
		return nil, err
	}
	kvs := make(backend.KVPairs, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		kvs = append(kvs, &backend.KVPair{
			Key:   string(kv.Key),
			Value: string(kv.Value),
		})
	}
	return kvs, nil
}

func (v3 *EtcdV3) Watch(ctx context.Context, key string, withPrefix bool) backend.EventChan {
	var options []clientv3.OpOption
	if withPrefix {
		options = append(options, clientv3.WithPrefix())
	}

	eventsChan := make(backend.EventChan, backend.DefaultChanLen)
	etcdChan := v3.Client.Watch(ctx, key, options...)

	go func() {
		for {
			resp := <-etcdChan
			if resp.Canceled {
				log.Println("etcd watching canceled", resp.Err())
				return
			}
			for _, evt := range resp.Events {
				wEvent := &backend.WatchEvent{
					KVPair: backend.KVPair{
						Key:   string(evt.Kv.Key),
						Value: string(evt.Kv.Value),
					},
				}
				if evt.Type == mvccpb.PUT {
					wEvent.Type = backend.Put
				} else {
					wEvent.Type = backend.Delete
				}

				eventsChan <- wEvent
			}
		}
	}()

	return eventsChan
}

func (v3 *EtcdV3) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	// grant lease
	ctx, cancel := context.WithTimeout(ctx, backend.WriteTimeout)
	lease, err := v3.Grant(ctx, int64(ttl.Seconds()))
	cancel()
	if err != nil {
		return err
	}

	// put value with lease
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	ctx, cancel = context.WithTimeout(ctx, backend.WriteTimeout)
	_, err = v3.Client.Put(ctx, key, ts, clientv3.WithLease(lease.ID))
	cancel()
	if err != nil {
		return err
	}

	// keep alive to etcd
	ch, err := v3.Client.KeepAlive(ctx, lease.ID)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ch:
				// do nothing
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (v3 *EtcdV3) Close() error {
	return v3.Client.Close()
}
