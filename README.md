# grc

## Usage

> Define configuration structure with default values

```go
type Config struct {
	IntVal   int               `json:"int_val" default:"8080"`
	StrVal   string            `default:"test"`
	SliceVal []int             `default:"1,2,3,4,ee"`
	MapVal   map[string]string `default:"a:aa,b:bb,c,d"`
}
```

> Define configuration callback

```go
var (
	config atomic.Value
)

func configUpdated(v interface{}) {
	config.Store(v)
}
```

> Create grc instance, and subscribe configuration

```golang
rc, err := grc.New(context.TODO(), grc.EtcdV3, "127.0.0.1:2379", "username", "password")
if err != nil {
	return
}
err = rc.SubscribeConf("/test/conf", Config{}, configUpdated)
if err != nil {
	return
}
```

> Use configuration

```go
func getConfig() Config {
	return config.Load().(Config)
}
```