# grc

## Usage

### Remote Configuration

> Define the configuration structure with default values

```go
type TestConfig struct {
	IntVal   int               `json:"int_val" default:"8080"`
	StrVal   string            `default:"test"`
	SliceVal []int             `default:"1,2,3,4,ee"`
	MapVal   map[string]string `default:"a:aa,b:bb,c,d"`
}
```

> Create grc instance

```go
rc, err := grc.New(context.TODO(), grc.EtcdV3, "127.0.0.1:2379", "username", "password")
if err != nil {
	//return
}
```

> Subscribe configuration

```go
path := "/test/conf"
err := rc.SubscribeConf(path, TestConfig{})
if err != nil {
	//return
}
```

> Use configuration

```go
conf := rc.GetConf(path).(TestConfig)
```

* The return type of `rc.GetConf(path)` is the same as the second parameter passed into `rc.SubscribeConf`. 
* The type can also be a pointor:

```go
err := rc.SubscribeConf(path, &TestConfig{})
if err != nil {
	//return
}

conf := rc.GetConf(path).(*TestConfig)
```

### Service Discovery

> Register node

```go
node := "127.0.0.1:6600"
path := "/test/service/a"
err := rc.RegisterNode(path, node, time.Second*5)
if err != nil {
	//return
}
```

> Get service nodes

```go
nodes := rc.GetService(path)
```