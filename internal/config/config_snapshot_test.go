package config

import (
	"reflect"
	"sync"
	"testing"
)

func TestManagerSnapshotOwnership(t *testing.T) {
	m := NewManager("")
	c := &Config{}
	c.System.CacheTTL = 100
	c.Cache.Engines = map[string]EngineCacheConfig{"fofa": {TTL: 10}}
	m.SetConfig(c)
	c.System.CacheTTL = 999
	if got := m.GetConfig().System.CacheTTL; got != 100 {
		t.Errorf("SetConfig retained caller pointer: %d", got)
	}
	cache := m.GetAllEngineCacheConfigs()
	cache["fofa"] = EngineCacheConfig{TTL: 999}
	if got := m.GetEngineCacheConfig("fofa").TTL; got != 10 {
		t.Errorf("cache map escaped: %d", got)
	}
	engine, err := m.GetEngineConfig("fofa")
	if err != nil {
		t.Fatal(err)
	}
	reflect.ValueOf(engine).Elem().FieldByName("Enabled").SetBool(true)
	if m.GetConfig().Engines.Fofa.Enabled {
		t.Error("engine pointer escaped")
	}
}

func TestManagerConcurrentAccessors(t *testing.T) {
	m := NewManager("")
	a, b := &Config{}, &Config{}
	a.System.CacheTTL = 100
	b.System.CacheTTL = 200
	m.SetConfig(a)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			m.SetConfig(a)
			m.SetConfig(b)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = m.IsValid()
			_, _ = m.GetEngineConfig("fofa")
			_ = m.GetEngineCacheConfig("fofa")
			_ = m.GetAllEngineCacheConfigs()
			_ = m.GetCacheTTLForEngine("fofa")
			_ = m.GetCacheMaxSizeForEngine("fofa")
			_ = m.GetCacheBackend()
			_ = m.GetRedisAddr()
			_ = m.GetRedisPassword()
			_ = m.GetRedisDB()
			_ = m.GetRedisPrefix()
		}
	}()
	wg.Wait()
}
