package notify

import (
	"fmt"
	"sync"

	"github.com/unimap/project/internal/logger"
)

// NotifyGlobalCfg 全局通知配置
type NotifyGlobalCfg struct {
	Enabled        bool `json:"enabled"`
	SendTimeoutSec int  `json:"send_timeout_sec"`
	MaxRetries     int  `json:"max_retries"`
}

// Registry 通知渠道注册表
type Registry struct {
	mu       sync.RWMutex
	channels map[string]NotifyChannel
}

// NewRegistry 创建空的注册表
func NewRegistry() *Registry {
	return &Registry{
		channels: make(map[string]NotifyChannel),
	}
}

// Register 注册一个渠道
func (r *Registry) Register(ch NotifyChannel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.channels[ch.ID()]; exists {
		return fmt.Errorf("notify channel %q already registered", ch.ID())
	}
	r.channels[ch.ID()] = ch
	return nil
}

// Get 根据 ID 获取渠道
func (r *Registry) Get(id string) NotifyChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels[id]
}

// Remove 移除渠道
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.channels[id]; ok {
		ch.Close()
		delete(r.channels, id)
	}
}

// List 返回所有已注册渠道的 ID 列表
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.channels))
	for id, ch := range r.channels {
		if ch.IsEnabled() {
			ids = append(ids, id)
		}
	}
	return ids
}

// ChannelInfo 返回渠道元信息（脱敏）
type ChannelInfo struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// ListAllInfos 返回所有渠道的元信息
func (r *Registry) ListAllInfos() []ChannelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]ChannelInfo, 0, len(r.channels))
	for id, ch := range r.channels {
		infos = append(infos, ChannelInfo{
			ID:      id,
			Type:    ch.Type(),
			Enabled: ch.IsEnabled(),
		})
	}
	return infos
}

// Reload 根据新配置重新注册渠道
func (r *Registry) Reload(chanCfgs []ChannelConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 找出需要移除的渠道
	existing := make(map[string]bool)
	for _, cfg := range chanCfgs {
		existing[cfg.ID] = true
	}
	for id, ch := range r.channels {
		if !existing[id] {
			ch.Close()
			delete(r.channels, id)
			logger.Infof("notify channel %q removed", id)
		}
	}

	// 注册/更新渠道
	for _, cfg := range chanCfgs {
		if !cfg.Enabled {
			continue
		}
		ch, err := NewChannelFromConfig(cfg)
		if err != nil {
			logger.Warnf("notify channel %q failed to create: %v", cfg.ID, err)
			continue
		}
		r.channels[cfg.ID] = ch
	}
}
