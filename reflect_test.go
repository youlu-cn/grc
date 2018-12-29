package grc

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/bitly/go-simplejson"
)

func TestDecode(t *testing.T) {
	type Embedded struct {
		UIntVal  uint        `json:"u_int_val" default:"77777"`
		FloatVal float64     `default:"4e10"`
		SliceVal []int       `default:"1,2,3,4,5,6,ttt"` // ttt will be 0
		MapVal   map[int]int `default:"1:10,2:20,3:30,4,5"`
	}
	type Test struct {
		IntVal          int            `json:"int_val" default:"8888"`
		StrVal          string         `default:"test"`
		BoolVal         bool           `json:"bool_val" default:"true"`
		SliceVal        []string       `default:"a,b,c,d"`
		MapVal          map[int]string `json:"map_val" default:"1:a,2:b,3:c,4,5"`
		AnotherEmbedded Embedded       `json:"another_embedded"`
		StructurePtr    *Embedded      `json:"structure_ptr"`
	}

	tt := Test{
		IntVal:   1,
		StrVal:   "a",
		BoolVal:  false,
		SliceVal: []string{"tt"},
		MapVal:   map[int]string{1: "aa"},
		AnotherEmbedded: Embedded{
			UIntVal:  321,
			FloatVal: 2e-3,
			SliceVal: []int{3, 2, 3},
			MapVal:   map[int]int{10: 1, 20: 2},
		},
		StructurePtr: &Embedded{
			UIntVal:  321,
			FloatVal: 2e-3,
			SliceVal: []int{3, 2, 3},
			MapVal:   map[int]int{10: 1, 20: 2},
		},
	}
	v, err := json.Marshal(tt)
	if err != nil {
		t.Fatal("fail", err)
	}
	sjson, err := simplejson.NewJson(v)
	if err != nil {
		t.Fatal("fail", err)
	}

	val := decode(reflect.TypeOf(Test{}), sjson)
	if vv, ok := val.Interface().(Test); !ok || !reflect.DeepEqual(tt, vv) {
		t.Fatal("fail", tt, vv, ok)
	}

	def := Test{
		IntVal:   8888,
		StrVal:   "test",
		BoolVal:  true,
		SliceVal: []string{"a", "b", "c", "d"},
		MapVal:   map[int]string{1: "a", 2: "b", 3: "c", 4: "", 5: ""},
		AnotherEmbedded: Embedded{
			UIntVal:  77777,
			FloatVal: 4e10,
			SliceVal: []int{1, 2, 3, 4, 5, 6, 0},
			MapVal:   map[int]int{1: 10, 2: 20, 3: 30, 4: 0, 5: 0},
		},
		StructurePtr: &Embedded{
			UIntVal:  77777,
			FloatVal: 4e10,
			SliceVal: []int{1, 2, 3, 4, 5, 6, 0},
			MapVal:   map[int]int{1: 10, 2: 20, 3: 30, 4: 0, 5: 0},
		},
	}
	sjson, err = simplejson.NewJson([]byte("{}"))
	if err != nil {
		t.Fatal("fail", err)
	}
	val = decode(reflect.TypeOf(Test{}), sjson)
	if vv, ok := val.Interface().(Test); !ok || !reflect.DeepEqual(def, vv) {
		t.Fatal("fail", def, vv, ok)
	}
}

func TestDecode_StructureSlice(t *testing.T) {
	type Base struct {
		IntVal int    `json:"int_val" default:"123"`
		StrVal string `default:"abc"`
	}
	type Test struct {
		MapVal []Base `default:"a,b,c"`
	}

	def := Test{
		MapVal: []Base{
			{
				IntVal: 123,
				StrVal: "abc",
			}, {
				IntVal: 123,
				StrVal: "abc",
			}, {
				IntVal: 123,
				StrVal: "abc",
			},
		},
	}
	sjson, err := simplejson.NewJson([]byte("{}"))
	if err != nil {
		t.Fatal("fail", err)
	}
	val := decode(reflect.TypeOf(Test{}), sjson)
	if tt, ok := val.Interface().(Test); !ok || !reflect.DeepEqual(tt, def) {
		t.Fatal("fail", tt, ok)
	}
}

func TestDecode_StructureMap(t *testing.T) {
	type Base struct {
		IntVal int    `json:"int_val" default:"123"`
		StrVal string `default:"abc"`
	}
	type Test struct {
		MapVal map[string]*Base `default:"a,b,c"`
	}

	def := Test{
		MapVal: map[string]*Base{
			"a": {
				IntVal: 123,
				StrVal: "abc",
			},
			"b": {
				IntVal: 123,
				StrVal: "abc",
			},
			"c": {
				IntVal: 123,
				StrVal: "abc",
			},
		},
	}
	sjson, err := simplejson.NewJson([]byte("{}"))
	if err != nil {
		t.Fatal("fail", err)
	}
	val := decode(reflect.TypeOf(Test{}), sjson)
	if tt, ok := val.Interface().(Test); !ok || !reflect.DeepEqual(tt, def) {
		t.Fatal("fail", tt, ok)
	}
}

func TestDecode_Structure(t *testing.T) {
	type Test struct {
		IntVal int    `json:"int_val" default:"123"`
		StrVal string `default:"abc"`
	}

	var (
		tt  Test
		ptt *Test
	)

	sjson, err := simplejson.NewJson([]byte("{}"))
	if err != nil {
		t.Fatal("json err", err)
	}

	val := decode(reflect.TypeOf(tt), sjson)
	if tt, ok := val.Interface().(Test); !ok || tt.IntVal != 123 || tt.StrVal != "abc" {
		t.Fatal("fail", tt, ok)
	}
	val = decode(reflect.TypeOf(ptt), sjson)
	if ptt, ok := val.Interface().(*Test); !ok || ptt.IntVal != 123 || ptt.StrVal != "abc" {
		t.Fatal("fail", ptt, ok)
	}

	sjson, err = simplejson.NewJson([]byte(`{"int_val":456}`))
	if err != nil {
		t.Fatal("json err", err)
	}

	val = decode(reflect.TypeOf(tt), sjson)
	if tt, ok := val.Interface().(Test); !ok || tt.IntVal != 456 || tt.StrVal != "abc" {
		t.Fatal("fail", tt, ok)
	}
	val = decode(reflect.TypeOf(ptt), sjson)
	if ptt, ok := val.Interface().(*Test); !ok || ptt.IntVal != 456 || ptt.StrVal != "abc" {
		t.Fatal("fail", ptt, ok)
	}

	sjson, err = simplejson.NewJson([]byte(`{"int_val":0,"StrVal":""}`))
	if err != nil {
		t.Fatal("json err", err)
	}

	val = decode(reflect.TypeOf(tt), sjson)
	if tt, ok := val.Interface().(Test); !ok || tt.IntVal != 0 || tt.StrVal != "" {
		t.Fatal("fail", tt, ok)
	}
	val = decode(reflect.TypeOf(ptt), sjson)
	if ptt, ok := val.Interface().(*Test); !ok || ptt.IntVal != 0 || ptt.StrVal != "" {
		t.Fatal("fail", ptt, ok)
	}
}

func TestDecode_Structure2(t *testing.T) {
	type Parent struct {
		UIntVal  uint        `json:"u_int_val" default:"77777"`
		FloatVal float64     `default:"4e10"`
		SliceVal []int       `default:"1,2,3,4,5,6,ttt"` // ttt will be 0
		MapVal   map[int]int `default:"1:10,2:20,3:30,4,5"`
	}
	type Test struct {
		IntVal int    `default:"666"`
		StrVal string `default:"ttt"`
		Parent
	}

	def := &Test{
		IntVal: 666,
		StrVal: "ttt",
		Parent: Parent{
			UIntVal:  77777,
			FloatVal: 4e10,
			SliceVal: []int{1, 2, 3, 4, 5, 6, 0},
			MapVal:   map[int]int{1: 10, 2: 20, 3: 30, 4: 0, 5: 0},
		},
	}

	sjson, err := simplejson.NewJson([]byte("{}"))
	if err != nil {
		t.Fatal("fail", err)
	}
	val := decode(reflect.TypeOf(&Test{}), sjson)
	if vv, ok := val.Interface().(*Test); !ok || !reflect.DeepEqual(def, vv) {
		t.Fatal("fail", def, vv)
	}
}

func TestDecode_Slice(t *testing.T) {
	var (
		arr []string
	)

	sjson, err := simplejson.NewJson([]byte(`["a","b","c"]`))
	if err != nil {
		t.Fatal("json err", err)
	}

	val := decode(reflect.TypeOf(arr), sjson)
	if arr, ok := val.Interface().([]string); !ok || !reflect.DeepEqual(arr, []string{"a", "b", "c"}) {
		t.Fatal("fail", arr, ok)
	}
}

func TestDecode_Map(t *testing.T) {
	var (
		mss map[string]string
		msi map[string]int
	)

	sjson, err := simplejson.NewJson([]byte(`{"k1":"v1","k2":"v2"}`))
	if err != nil {
		t.Fatal("json err", err)
	}

	val := decode(reflect.TypeOf(mss), sjson)
	if mss, ok := val.Interface().(map[string]string); !ok || !reflect.DeepEqual(mss, map[string]string{"k1": "v1", "k2": "v2"}) {
		t.Fatal("fail", mss, ok)
	}

	sjson, err = simplejson.NewJson([]byte(`{"k1":1,"k2":2,"k3":"ttt"}`))
	if err != nil {
		t.Fatal("json err", err)
	}

	val = decode(reflect.TypeOf(msi), sjson)
	if msi, ok := val.Interface().(map[string]int); !ok || !reflect.DeepEqual(msi, map[string]int{"k1": 1, "k2": 2, "k3": 0}) {
		t.Fatal("fail", msi, ok)
	}
}

func TestDefaultValue(t *testing.T) {
	var (
		s   string
		b   bool
		i   int
		i8  int8
		i16 int16
		ui  uint
		u32 uint32
		u64 uint64
		f32 float32
		f64 float64
	)

	val := defaultValue(reflect.TypeOf(s), "abc")
	if s, ok := val.Interface().(string); !ok || s != "abc" {
		t.Fatal("fail", s, ok)
	}

	val = defaultValue(reflect.TypeOf(b), "true")
	if b, ok := val.Interface().(bool); !ok || !b {
		t.Fatal("fail", b, ok)
	}

	val = defaultValue(reflect.TypeOf(i), "30000")
	if i, ok := val.Interface().(int); !ok || i != 30000 {
		t.Fatal("fail", i, ok)
	}
	val = defaultValue(reflect.TypeOf(i8), "-128")
	if i8, ok := val.Interface().(int8); !ok || i8 != int8(-128) {
		t.Fatal("fail", i8, ok)
	}
	val = defaultValue(reflect.TypeOf(i16), "32767")
	if i16, ok := val.Interface().(int16); !ok || i16 != int16(32767) {
		t.Fatal("fail", i16, ok)
	}

	val = defaultValue(reflect.TypeOf(ui), "127")
	if ui, ok := val.Interface().(uint); !ok || ui != uint(127) {
		t.Fatal("fail", ui, ok)
	}
	val = defaultValue(reflect.TypeOf(u32), "2147483648")
	if u32, ok := val.Interface().(uint32); !ok || u32 != 2147483648 {
		t.Fatal("fail", u32, ok)
	}
	val = defaultValue(reflect.TypeOf(u64), "9223372036854775808")
	if u64, ok := val.Interface().(uint64); !ok || u64 != 9223372036854775808 {
		t.Fatal("fail", u64, ok)
	}

	val = defaultValue(reflect.TypeOf(f32), "123.0001")
	if f32, ok := val.Interface().(float32); !ok || f32 != 123.0001 {
		t.Fatal("fail", f32, ok)
	}
	val = defaultValue(reflect.TypeOf(f64), "1e23")
	if f64, ok := val.Interface().(float64); !ok || f64 != 1e23 {
		t.Fatal("fail", f64, ok)
	}
}

func TestDefault_SliceValue(t *testing.T) {
	var (
		ss []string
		nn []int
		ii []interface{}
	)

	val := defaultValue(reflect.TypeOf(ss), "a,b,c,d,e")
	if sv, ok := val.Interface().([]string); !ok || !reflect.DeepEqual(sv, []string{"a", "b", "c", "d", "e"}) {
		t.Fatal("fail", sv, ok)
	}

	val = defaultValue(reflect.TypeOf(nn), "1,2,3,4,5")
	if iv, ok := val.Interface().([]int); !ok || !reflect.DeepEqual(iv, []int{1, 2, 3, 4, 5}) {
		t.Fatal("fail", iv, ok)
	}

	val = defaultValue(reflect.TypeOf(ii), "a,2,c,4")
	if sv, ok := val.Interface().([]interface{}); !ok || !reflect.DeepEqual(sv, []interface{}{"a", "2", "c", "4"}) {
		t.Fatal("fail", sv, ok)
	}
}

func TestDefault_MapValue(t *testing.T) {
	var (
		ssm  map[string]string
		sim  map[string]int
		ism  map[int]string
		siim map[string]interface{}
		sssm map[string]struct{}
	)

	val := defaultValue(reflect.TypeOf(ssm), "k1:v1,k2:v2,k3")
	if nssm, ok := val.Interface().(map[string]string); !ok || !reflect.DeepEqual(nssm, map[string]string{"k1": "v1", "k2": "v2", "k3": ""}) {
		t.Fatal("fail", nssm, ok)
	}

	val = defaultValue(reflect.TypeOf(sim), "k1:11,k2:22,k3")
	if nsim, ok := val.Interface().(map[string]int); !ok || !reflect.DeepEqual(nsim, map[string]int{"k1": 11, "k2": 22, "k3": 0}) {
		t.Fatal("fail", nsim, ok)
	}

	val = defaultValue(reflect.TypeOf(ism), "1:v1,2:v2,3")
	if nism, ok := val.Interface().(map[int]string); !ok || !reflect.DeepEqual(nism, map[int]string{1: "v1", 2: "v2", 3: ""}) {
		t.Fatal("fail", nism, ok)
	}

	val = defaultValue(reflect.TypeOf(siim), "k1:1,k2:v2,k3")
	if nsiim, ok := val.Interface().(map[string]interface{}); !ok || !reflect.DeepEqual(nsiim, map[string]interface{}{"k1": "1", "k2": "v2", "k3": ""}) {
		t.Fatal("fail", nsiim, ok)
	}

	val = defaultValue(reflect.TypeOf(sssm), "k1,k2,k3")
	if nsssm, ok := val.Interface().(map[string]struct{}); !ok || len(nsssm) != 3 {
		t.Fatal(nsssm, ok)
	}
}

func TestDefault_StructureValue(t *testing.T) {
	type Test struct {
		IntVal int    `default:"100"`
		StrVal string `default:"abc"`
	}

	var (
		tt  Test
		ptt *Test
	)

	val := defaultValue(reflect.TypeOf(tt), "")
	if nt, ok := val.Interface().(Test); !ok || nt.IntVal != 100 || nt.StrVal != "abc" {
		t.Fatal("fail", nt, ok)
	}
	val = defaultValue(reflect.TypeOf(ptt), "")
	if npt, ok := val.Interface().(*Test); !ok || npt.IntVal != 100 || npt.StrVal != "abc" {
		t.Fatal("fail", npt, ok)
	}
}
