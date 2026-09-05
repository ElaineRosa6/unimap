package config

import (
	"fmt"
	"reflect"
	"testing"
)

// Populate every exported field so future additions missing from Clone fail
// the snapshot fidelity test rather than being silently dropped.
func populatedConfigValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populatedConfigValue(v.Field(i))
		}
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populatedConfigValue(v.Elem())
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 2))
		for i := 0; i < 2; i++ {
			populatedConfigValue(v.Index(i))
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key := reflect.New(v.Type().Key()).Elem()
		populatedConfigValue(key)
		value := reflect.New(v.Type().Elem()).Elem()
		populatedConfigValue(value)
		v.SetMapIndex(key, value)
	case reflect.String:
		v.SetString("fixture")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(17)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(17)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.25)
	default:
		panic(fmt.Sprintf("extend config fixture for %s", v.Type()))
	}
}

func TestManagerSnapshotFidelity(t *testing.T) {
	full := &Config{}
	populatedConfigValue(reflect.ValueOf(full).Elem())
	for name, cfg := range map[string]*Config{"zero": {}, "all-fields": full} {
		t.Run(name, func(t *testing.T) {
			m := NewManager("")
			m.SetConfig(cfg)
			if !reflect.DeepEqual(cfg, m.GetConfig()) {
				t.Fatal("snapshot changed configuration fields or nil/empty representation")
			}
		})
	}
}

var snapshotBenchmarkSink *Config

func BenchmarkManagerSnapshot(b *testing.B) {
	fixture := &Config{}
	populatedConfigValue(reflect.ValueOf(fixture).Elem())
	m := NewManager("")
	m.SetConfig(fixture)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshotBenchmarkSink = m.GetConfig()
	}
}
