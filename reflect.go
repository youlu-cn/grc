package grc

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/bitly/go-simplejson"
)

func Decode(data string, item interface{}) interface{} {
	json, err := simplejson.NewJson([]byte(data))
	if err != nil {
		return nil
	}
	val := decode(reflect.TypeOf(item), json)
	if !val.IsValid() {
		return nil
	}
	return val.Interface()
}

func decode(t reflect.Type, json *simplejson.Json) (val reflect.Value) {
	switch t.Kind() {
	case reflect.Ptr:
		el := decode(t.Elem(), json)
		val = reflect.New(t).Elem()
		val.Set(el.Addr())
	case reflect.String:
		s, _ := json.String()
		val = reflect.New(t).Elem()
		val.SetString(s)
	case reflect.Bool:
		b, _ := json.Bool()
		val = reflect.New(t).Elem()
		val.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64, _ := json.Int64()
		val = reflect.New(t).Elem()
		val.SetInt(i64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ui64, _ := json.Uint64()
		val = reflect.New(t).Elem()
		val.SetUint(ui64)
	case reflect.Float32, reflect.Float64:
		f, _ := json.Float64()
		val = reflect.New(t).Elem()
		val.SetFloat(f)
	case reflect.Slice, reflect.Array:
		arr, err := json.Array()
		if err != nil {
			val = reflect.MakeSlice(t, 0, 0)
			break
		}
		val = reflect.MakeSlice(t, 0, len(arr))
		for i := 0; i < len(arr); i++ {
			val = reflect.Append(val, decode(t.Elem(), json.GetIndex(i)))
		}
	case reflect.Map:
		m, err := json.Map()
		if err != nil {
			val = reflect.MakeMapWithSize(reflect.MapOf(t.Key(), t.Elem()), 0)
			break
		}
		val = reflect.MakeMapWithSize(reflect.MapOf(t.Key(), t.Elem()), len(m))
		for k, _ := range m {
			val.SetMapIndex(defaultValue(t.Key(), k), decode(t.Elem(), json.Get(k)))
		}
	case reflect.Struct:
		val = reflect.New(t).Elem()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" {
				jsonTag = field.Name
			}
			// set json value
			if json, ok := json.CheckGet(jsonTag); ok {
				v := decode(field.Type, json)
				val.Field(i).Set(v)
				continue
			}
			// set default value
			defaultTag := field.Tag.Get("default")
			if defaultTag != "" || field.Type.Kind() == reflect.Ptr || field.Type.Kind() == reflect.Struct {
				val.Field(i).Set(defaultValue(field.Type, defaultTag))
			}
		}

	//case reflect.Complex64, reflect.Complex128:
	//case reflect.Interface:
	default:
	}

	return
}

func defaultValue(t reflect.Type, v string) (val reflect.Value) {
	switch t.Kind() {
	case reflect.Ptr:
		el := defaultValue(t.Elem(), v)
		val = reflect.New(t).Elem()
		val.Set(el.Addr())
	case reflect.Bool:
		b, _ := strconv.ParseBool(v)
		val = reflect.New(t).Elem()
		val.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64, _ := strconv.ParseInt(v, 10, 64)
		val = reflect.New(t).Elem()
		val.SetInt(i64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ui64, _ := strconv.ParseUint(v, 10, 64)
		val = reflect.New(t).Elem()
		val.SetUint(ui64)
	case reflect.Float32, reflect.Float64:
		f, _ := strconv.ParseFloat(v, 64)
		val = reflect.New(t).Elem()
		val.SetFloat(f)
	case reflect.Slice, reflect.Array:
		vs := strings.Split(v, ",")
		val = reflect.MakeSlice(t, 0, len(vs))
		for _, s := range vs {
			val = reflect.Append(val, defaultValue(t.Elem(), s))
		}
	case reflect.Map:
		kvs := strings.Split(v, ",")
		val = reflect.MakeMapWithSize(reflect.MapOf(t.Key(), t.Elem()), len(kvs))
		for _, s := range kvs {
			kv := strings.Split(s, ":")
			key, value := kv[0], ""
			if len(kv) > 1 {
				value = kv[1]
			}
			val.SetMapIndex(defaultValue(t.Key(), key), defaultValue(t.Elem(), value))
		}
	case reflect.Struct:
		val = reflect.New(t).Elem()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			val.Field(i).Set(defaultValue(field.Type, field.Tag.Get("default")))
		}

	//case reflect.Complex64, reflect.Complex128:
	//case reflect.Interface:
	//case reflect.String:
	default:
		val = reflect.New(t).Elem()
		val.Set(reflect.ValueOf(v))
	}

	return
}
