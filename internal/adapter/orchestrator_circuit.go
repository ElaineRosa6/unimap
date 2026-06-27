package adapter

import (
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/logger"
)

const (
	// DefaultConcurrency 默认并发数
	DefaultConcurrency = 5
	// MaxConcurrency 最大并发数
	MaxConcurrency = 8
	// DefaultCacheTTL 默认缓存时间
	DefaultCacheTTL = 30 * time.Minute
	// DefaultRateLimitDelay 默认速率限制延迟
	DefaultRateLimitDelay = 100 * time.Millisecond
	// DefaultCircuitBreakerThreshold 熔断器失败阈值（连续N次失败）
	DefaultCircuitBreakerThreshold = 5
	// DefaultCircuitBreakerDuration 熔断器打开持续时间
	DefaultCircuitBreakerDuration = 2 * time.Minute
)

// EngineCacheTTLConfig 引擎缓存TTL配置
type EngineCacheTTLConfig struct {
	TTL     time.Duration
	Enabled bool
}

// CircuitState 熔断器状态
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // 正常状态
	CircuitOpen     CircuitState = "open"      // 熔断状态，跳过该引擎
	CircuitHalfOpen CircuitState = "half_open" // 半开状态，尝试恢复
)

// CircuitBreaker 简单熔断器
type CircuitBreaker struct {
	mu            sync.Mutex
	State         CircuitState
	Failures      int
	LastFailure   time.Time
	Threshold     int
	ResetDuration time.Duration
}

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.State == CircuitClosed {
		return true
	}
	if cb.State == CircuitOpen {
		if time.Since(cb.LastFailure) > cb.ResetDuration {
			cb.State = CircuitHalfOpen
			return true
		}
		return false
	}
	// half_open: allow one request
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.State = CircuitClosed
	cb.Failures = 0
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.Failures++
	cb.LastFailure = time.Now()
	if cb.Failures >= cb.Threshold {
		cb.State = CircuitOpen
	}
}

// GetState returns current state safely
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.State
}

// GetStats returns all stats for monitoring safely
func (cb *CircuitBreaker) GetStats() (state CircuitState, failures int, threshold int, lastFailure time.Time, resetDuration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.State, cb.Failures, cb.Threshold, cb.LastFailure, cb.ResetDuration
}

// SetCircuitBreakerConfig 设置熔断器配置
func (o *EngineOrchestrator) SetCircuitBreakerConfig(engineName string, threshold int, resetDuration time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	name := strings.ToLower(engineName)
	if _, exists := o.circuitBreakers[name]; !exists {
		o.circuitBreakers[name] = &CircuitBreaker{
			State:         CircuitClosed,
			Threshold:     threshold,
			ResetDuration: resetDuration,
		}
	} else {
		cb := o.circuitBreakers[name]
		cb.Threshold = threshold
		cb.ResetDuration = resetDuration
	}
}

// GetCircuitState 获取引擎熔断器状态
func (o *EngineOrchestrator) GetCircuitState(engineName string) CircuitState {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	cb, exists := o.circuitBreakers[strings.ToLower(engineName)]
	if !exists {
		return CircuitClosed
	}
	return cb.GetState()
}

// IsEngineCircuited 检查引擎是否被熔断（true = 应跳过）
func (o *EngineOrchestrator) IsEngineCircuited(engineName string) bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	cb, exists := o.circuitBreakers[strings.ToLower(engineName)]
	if !exists {
		return false
	}
	state, _, _, lastFailure, resetDuration := cb.GetStats()
	return state == CircuitOpen && time.Since(lastFailure) <= resetDuration
}

// RecordEngineSuccess 记录引擎成功（关闭熔断器）
func (o *EngineOrchestrator) RecordEngineSuccess(engineName string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	cb, exists := o.circuitBreakers[strings.ToLower(engineName)]
	if !exists {
		return
	}
	cb.RecordSuccess()
}

// RecordEngineFailure 记录引擎失败（可能触发熔断）
func (o *EngineOrchestrator) RecordEngineFailure(engineName string) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	name := strings.ToLower(engineName)
	if _, exists := o.circuitBreakers[name]; !exists {
		o.circuitBreakers[name] = &CircuitBreaker{
			State:         CircuitClosed,
			Threshold:     DefaultCircuitBreakerThreshold,
			ResetDuration: DefaultCircuitBreakerDuration,
		}
	}
	cb := o.circuitBreakers[name]
	cb.RecordFailure()
	state, _, threshold, _, _ := cb.GetStats()
	if state == CircuitOpen {
		logger.Warnf("Circuit breaker opened for engine %s after %d consecutive failures", engineName, threshold)
	}
}

// CircuitBreakerEntryStats is the typed stats for a single circuit breaker.
type CircuitBreakerEntryStats struct {
	State         string        `json:"state"`
	Failures      int           `json:"failures"`
	Threshold     int           `json:"threshold"`
	LastFailure   time.Time     `json:"last_failure"`
	ResetDuration time.Duration `json:"reset_duration"`
}

// GetCircuitBreakerStats 获取所有熔断器状态（用于调试/监控）
func (o *EngineOrchestrator) GetCircuitBreakerStats() map[string]CircuitBreakerEntryStats {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	stats := make(map[string]CircuitBreakerEntryStats)
	for name, cb := range o.circuitBreakers {
		state, failures, threshold, lastFailure, resetDuration := cb.GetStats()
		stats[name] = CircuitBreakerEntryStats{
			State:         string(state),
			Failures:      failures,
			Threshold:     threshold,
			LastFailure:   lastFailure,
			ResetDuration: resetDuration,
		}
	}
	return stats
}
