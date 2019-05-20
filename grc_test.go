package grc

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/youlu-cn/grc/backend/debug"
)

var (
	provider, _ = debug.NewProvider()
	rc, _       = NewWithProvider(context.TODO(), provider)
)

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
}
