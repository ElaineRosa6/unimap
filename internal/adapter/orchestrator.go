package adapter

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/utils"
)

// EngineOrchestrator 引擎编排器
type EngineOrchestrator struct {
	adapters        map[string]EngineAdapter
	mutex           sync.RWMutex
	cache           utils.QueryCache
	concurrency     int
	engineCacheTTL  map[string]EngineCacheTTLConfig // 按引擎的缓存TTL配置
	defaultCacheTTL time.Duration                   // 默认缓存TTL
	circuitBreakers map[string]*CircuitBreaker      // 按引擎的熔断器
}

// NewEngineOrchestrator 创建引擎编排器
func NewEngineOrchestrator() *EngineOrchestrator {
	return NewEngineOrchestratorWithConfig(false, "", "", 0)
}

// NewEngineOrchestratorWithConfig 使用配置创建引擎编排器
func NewEngineOrchestratorWithConfig(useRedis bool, redisAddr, redisPassword string, redisDB int) *EngineOrchestrator {
	cache := utils.NewCache(
		useRedis,
		redisAddr,
		redisPassword,
		redisDB,
		"unimap:",
		1000,
		5*time.Minute,
	)

	return &EngineOrchestrator{
		adapters:        make(map[string]EngineAdapter),
		cache:           cache,
		concurrency:     DefaultConcurrency,
		engineCacheTTL:  make(map[string]EngineCacheTTLConfig),
		defaultCacheTTL: DefaultCacheTTL,
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// SetEngineCacheTTL 设置引擎缓存TTL配置
func (o *EngineOrchestrator) SetEngineCacheTTL(engineName string, ttl time.Duration, enabled bool) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.engineCacheTTL[strings.ToLower(engineName)] = EngineCacheTTLConfig{
		TTL:     ttl,
		Enabled: enabled,
	}
}

// SetEngineCacheTTLFromConfig 从配置map设置引擎缓存TTL
// configMap 格式: map[engineName]{ttl_seconds, enabled}
func (o *EngineOrchestrator) SetEngineCacheTTLFromConfig(configMap map[string]struct {
	TTL     int
	Enabled bool
}) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	for engine, cfg := range configMap {
		o.engineCacheTTL[strings.ToLower(engine)] = EngineCacheTTLConfig{
			TTL:     time.Duration(cfg.TTL) * time.Second,
			Enabled: cfg.Enabled,
		}
	}
}

// GetEngineCacheTTL 获取引擎缓存TTL
func (o *EngineOrchestrator) GetEngineCacheTTL(engineName string) (time.Duration, bool) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	if cfg, exists := o.engineCacheTTL[strings.ToLower(engineName)]; exists && cfg.Enabled {
		return cfg.TTL, true
	}
	return o.defaultCacheTTL, true
}

// IsCacheEnabledForEngine 检查引擎是否启用缓存
func (o *EngineOrchestrator) IsCacheEnabledForEngine(engineName string) bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	if cfg, exists := o.engineCacheTTL[strings.ToLower(engineName)]; exists {
		return cfg.Enabled
	}
	return true // 默认启用
}

// SetDefaultCacheTTL 设置默认缓存TTL
func (o *EngineOrchestrator) SetDefaultCacheTTL(ttl time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.defaultCacheTTL = ttl
}

// SetConcurrency 设置并发数
func (o *EngineOrchestrator) SetConcurrency(concurrency int) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	if concurrency > MaxConcurrency {
		concurrency = MaxConcurrency
	}
	o.concurrency = concurrency
}

// GetConcurrency 获取当前并发设置。
func (o *EngineOrchestrator) GetConcurrency() int {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	return o.concurrency
}

// RegisterAdapter 注册引擎适配器
func (o *EngineOrchestrator) RegisterAdapter(adapter EngineAdapter) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.adapters[adapter.Name()] = adapter
}

// UnregisterAdapter 移除指定引擎适配器
func (o *EngineOrchestrator) UnregisterAdapter(name string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	delete(o.adapters, name)
}

// GetAdapter 获取指定引擎适配器
func (o *EngineOrchestrator) GetAdapter(name string) (EngineAdapter, bool) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	adapter, exists := o.adapters[name]
	return adapter, exists
}

// ListAdapters 列出所有适配器
func (o *EngineOrchestrator) ListAdapters() []string {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	names := make([]string, 0, len(o.adapters))
	for name := range o.adapters {
		names = append(names, name)
	}
	return names
}

// SetWebOnlyBrowserBackend wires a browser query backend into all WebOnly
// adapters that support it (via the SetBrowserBackend method).
func (o *EngineOrchestrator) SetWebOnlyBrowserBackend(backend BrowserQueryBackend) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	for _, a := range o.adapters {
		if w, ok := a.(interface{ SetBrowserBackend(BrowserQueryBackend) }); ok {
			w.SetBrowserBackend(backend)
		}
	}
}

// TranslateQuery 将UQL转换为各引擎查询
func (o *EngineOrchestrator) TranslateQuery(ast *model.UQLAST, engineNames []string) ([]model.EngineQuery, error) {
	if ast == nil {
		return nil, fmt.Errorf("AST cannot be nil")
	}
	if len(engineNames) == 0 {
		return nil, fmt.Errorf("engine names cannot be empty")
	}

	queries := []model.EngineQuery{}
	translateErrs := []string{}

	for _, name := range engineNames {
		// 跳过熔断的引擎
		if o.IsEngineCircuited(name) {
			logger.Warnf("Engine %s circuit breaker open, skipping translation", name)
			continue
		}

		adapter, exists := o.GetAdapter(name)
		if !exists {
			translateErrs = append(translateErrs, fmt.Sprintf("adapter %s not found", name))
			continue
		}

		query, err := adapter.Translate(ast)
		if err != nil {
			translateErrs = append(translateErrs, fmt.Sprintf("failed to translate for %s: %v", name, err))
			continue
		}

		queries = append(queries, model.EngineQuery{
			EngineName: name,
			Query:      query,
		})
	}

	if len(translateErrs) > 0 {
		logger.Warnf("query translation had partial failures: %s", strings.Join(translateErrs, "; "))
	}

	if len(queries) == 0 {
		if len(translateErrs) > 0 {
			return nil, fmt.Errorf("no translatable engines: %s", strings.Join(translateErrs, "; "))
		}
		return nil, fmt.Errorf("no translatable engines available")
	}

	return queries, nil
}

// NormalizeResults 标准化引擎结果
func (o *EngineOrchestrator) NormalizeResults(engineResults []*model.EngineResult) ([]model.UnifiedAsset, error) {
	assets := []model.UnifiedAsset{}

	for _, result := range engineResults {
		if result == nil || result.Error != "" {
			continue
		}

		if result.Cached && len(result.NormalizedData) > 0 {
			assets = append(assets, result.NormalizedData...)
			continue
		}

		adapter, exists := o.GetAdapter(result.EngineName)
		if !exists {
			continue
		}

		normalized, err := adapter.Normalize(result)
		if err != nil {
			continue
		}

		assets = append(assets, normalized...)
	}

	return assets, nil
}

// ExecuteUnifiedQuery 执行统一查询（完整流程）
func (o *EngineOrchestrator) ExecuteUnifiedQuery(ast *model.UQLAST, engineNames []string, pageSize, maxPages int) ([]model.UnifiedAsset, error) {
	// 1. 翻译查询
	queries, err := o.TranslateQuery(ast, engineNames)
	if err != nil {
		return nil, fmt.Errorf("translate error: %w", err)
	}

	// 2. 并行搜索
	engineResults, err := o.SearchEnginesWithPagination(queries, pageSize, maxPages)
	if err != nil {
		return nil, fmt.Errorf("search error: %w", err)
	}

	// 3. 标准化
	assets, err := o.NormalizeResults(engineResults)
	if err != nil {
		return nil, fmt.Errorf("normalize error: %w", err)
	}

	return assets, nil
}
