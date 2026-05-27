package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/unimap/project/internal/utils/urlguard"
)

// NotifyChannel 通知渠道接口
type NotifyChannel interface {
	ID() string
	Type() string
	Send(ctx context.Context, n TaskNotification) error
	IsEnabled() bool
	Close() error
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
	ID             string            `yaml:"id" json:"id"`
	Type           string            `yaml:"type" json:"type"`
	Enabled        bool              `yaml:"enabled" json:"enabled"`
	WebhookURL     string            `yaml:"webhook_url" json:"webhook_url"`
	Secret         string            `yaml:"secret" json:"secret"`
	Headers        map[string]string `yaml:"headers" json:"headers"`
	AllowPrivateIP bool              `yaml:"allow_private_ip" json:"allow_private_ip"`
}

// NewChannelFromConfig 根据配置创建渠道实例
func NewChannelFromConfig(cfg ChannelConfig) (NotifyChannel, error) {
	switch cfg.Type {
	case "log":
		return NewLogChannel(cfg.ID, cfg.Enabled), nil
	case "webhook":
		return NewGenericWebhookChannel(cfg.ID, cfg.WebhookURL, cfg.Headers, cfg.Enabled, cfg.AllowPrivateIP)
	case "dingtalk":
		return NewDingTalkChannel(cfg.ID, cfg.WebhookURL, cfg.Secret, cfg.Enabled, cfg.AllowPrivateIP)
	case "feishu":
		return NewFeishuChannel(cfg.ID, cfg.WebhookURL, cfg.Secret, cfg.Enabled, cfg.AllowPrivateIP)
	case "wecom":
		return NewWeComChannel(cfg.ID, cfg.WebhookURL, cfg.Enabled, cfg.AllowPrivateIP)
	default:
		return nil, fmt.Errorf("unknown notify channel type: %s", cfg.Type)
	}
}

// LogChannel 日志通知渠道
type LogChannel struct {
	id      string
	enabled bool
}

func NewLogChannel(id string, enabled bool) *LogChannel {
	return &LogChannel{id: id, enabled: enabled}
}

func (c *LogChannel) ID() string      { return c.id }
func (c *LogChannel) Type() string    { return "log" }
func (c *LogChannel) IsEnabled() bool { return c.enabled }
func (c *LogChannel) Close() error    { return nil }

func (c *LogChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}
	msg := fmt.Sprintf("[notify] task=%s name=%s type=%s status=%s duration=%.0fms",
		n.TaskID, n.TaskName, n.TaskType, n.Status, n.Duration)
	if n.Error != "" {
		msg += " error=" + n.Error
	}
	switch n.Status {
	case "success":
		log.Printf("[INFO] %s", msg)
	case "failed":
		log.Printf("[WARN] %s", msg)
	case "timeout":
		log.Printf("[WARN] %s", msg)
	default:
		log.Printf("[INFO] %s", msg)
	}
	return nil
}

// GenericWebhookChannel 通用 JSON Webhook
type GenericWebhookChannel struct {
	id      string
	url     string
	headers map[string]string
	enabled bool
	client  *http.Client
}

func NewGenericWebhookChannel(id, rawURL string, headers map[string]string, enabled bool, allowPrivate bool) (*GenericWebhookChannel, error) {
	opts := urlguard.CheckOptions{AllowPrivate: allowPrivate}
	if _, err := urlguard.Check(rawURL, opts); err != nil {
		return nil, fmt.Errorf("urlguard blocked webhook URL: %w", err)
	}

	client := urlguard.SafeHTTPClient(opts, 10*time.Second)
	return &GenericWebhookChannel{
		id:      id,
		url:     rawURL,
		headers: headers,
		enabled: enabled,
		client:  client,
	}, nil
}

func (c *GenericWebhookChannel) ID() string      { return c.id }
func (c *GenericWebhookChannel) Type() string    { return "webhook" }
func (c *GenericWebhookChannel) IsEnabled() bool { return c.enabled }
func (c *GenericWebhookChannel) Close() error    { return nil }

func (c *GenericWebhookChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled || c.url == "" {
		return nil
	}

	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
