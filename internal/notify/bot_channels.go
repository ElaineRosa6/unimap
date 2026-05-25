package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/unimap-icp-hunter/project/internal/utils/urlguard"
)

// DingTalkChannel 钉钉群机器人
type DingTalkChannel struct {
	id      string
	baseURL string
	secret  string
	enabled bool
	client  *http.Client
}

func NewDingTalkChannel(id, rawURL, secret string, enabled bool, allowPrivate bool) (*DingTalkChannel, error) {
	opts := urlguard.CheckOptions{AllowPrivate: allowPrivate}
	if _, err := urlguard.Check(rawURL, opts); err != nil {
		return nil, fmt.Errorf("urlguard blocked dingtalk URL: %w", err)
	}
	client := urlguard.SafeHTTPClient(opts, 10*time.Second)
	return &DingTalkChannel{
		id:      id,
		baseURL: rawURL,
		secret:  secret,
		enabled: enabled,
		client:  client,
	}, nil
}

func (c *DingTalkChannel) ID() string      { return c.id }
func (c *DingTalkChannel) Type() string    { return "dingtalk" }
func (c *DingTalkChannel) IsEnabled() bool { return c.enabled }
func (c *DingTalkChannel) Close() error    { return nil }

func (c *DingTalkChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}

	statusEmoji := map[string]string{
		"success": "✅",
		"failed":  "❌",
		"timeout": "⏰",
	}
	emoji := statusEmoji[n.Status]
	title := fmt.Sprintf("%s [UniMap] 定时任务 [%s] %s", emoji, n.TaskName, statusLabel(n.Status))

	markdown := fmt.Sprintf(
		"**%s**\n\n- 类型: %s\n- 耗时: %.1fs\n- 结果: %s",
		title, n.TaskType, n.Duration/1000.0, n.Result,
	)
	if n.Error != "" {
		markdown += fmt.Sprintf("\n- 错误: %s", n.Error)
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  markdown,
		},
	}

	sendURL := c.baseURL
	if c.secret != "" {
		ts := TimestampNow() / 1000
		sign, err := DingTalkSign(c.secret, ts)
		if err != nil {
			return fmt.Errorf("dingtalk sign error: %w", err)
		}
		u, err := url.Parse(sendURL)
		if err != nil {
			return fmt.Errorf("parse dingtalk URL: %w", err)
		}
		q := u.Query()
		q.Set("timestamp", fmt.Sprintf("%d", ts))
		q.Set("sign", sign)
		u.RawQuery = q.Encode()
		sendURL = u.String()
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}

	// DingTalk returns HTTP 200 with errcode on failure.
	var dtResp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dtResp); err == nil {
		if dtResp.Errcode != 0 {
			return fmt.Errorf("dingtalk api error: errcode=%d errmsg=%s", dtResp.Errcode, dtResp.Errmsg)
		}
	}
	return nil
}

// FeishuChannel 飞书群机器人
type FeishuChannel struct {
	id      string
	url     string
	secret  string
	enabled bool
	client  *http.Client
}

func NewFeishuChannel(id, rawURL, secret string, enabled bool, allowPrivate bool) (*FeishuChannel, error) {
	opts := urlguard.CheckOptions{AllowPrivate: allowPrivate}
	if _, err := urlguard.Check(rawURL, opts); err != nil {
		return nil, fmt.Errorf("urlguard blocked feishu URL: %w", err)
	}
	client := urlguard.SafeHTTPClient(opts, 10*time.Second)
	return &FeishuChannel{
		id:      id,
		url:     rawURL,
		secret:  secret,
		enabled: enabled,
		client:  client,
	}, nil
}

func (c *FeishuChannel) ID() string      { return c.id }
func (c *FeishuChannel) Type() string    { return "feishu" }
func (c *FeishuChannel) IsEnabled() bool { return c.enabled }
func (c *FeishuChannel) Close() error    { return nil }

func (c *FeishuChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}

	statusEmoji := map[string]string{
		"success": "✅",
		"failed":  "❌",
		"timeout": "⏰",
	}
	emoji := statusEmoji[n.Status]
	title := fmt.Sprintf("%s **[UniMap]** 定时任务 **[%s]** %s", emoji, n.TaskName, statusLabel(n.Status))

	text := fmt.Sprintf("**类型**: %s\n\n**耗时**: %.1fs\n\n**结果**: %s", n.TaskType, n.Duration/1000.0, n.Result)
	if n.Error != "" {
		text += fmt.Sprintf("\n\n**错误**: %s", n.Error)
	}

	body := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": title,
				},
				"template": "blue",
			},
			"elements": []map[string]interface{}{
				{
					"tag":      "markdown",
					"content":  text,
					"text_align": "left",
				},
			},
		},
	}

	if c.secret != "" {
		ts := TimestampNow() / 1000
		sign, err := FeishuSign(c.secret, ts)
		if err != nil {
			return fmt.Errorf("feishu sign error: %w", err)
		}
		body["timestamp"] = fmt.Sprintf("%d", ts)
		body["sign"] = sign
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu returned status %d", resp.StatusCode)
	}

	// Feishu returns HTTP 200 even on API errors — check the response body.
	var feishuResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&feishuResp); err == nil {
		if feishuResp.Code != 0 {
			return fmt.Errorf("feishu api error: code=%d msg=%s", feishuResp.Code, feishuResp.Msg)
		}
	}
	return nil
}

// WeComChannel 企业微信群机器人
type WeComChannel struct {
	id      string
	url     string
	enabled bool
	client  *http.Client
}

func NewWeComChannel(id, rawURL string, enabled bool, allowPrivate bool) (*WeComChannel, error) {
	opts := urlguard.CheckOptions{AllowPrivate: allowPrivate}
	if _, err := urlguard.Check(rawURL, opts); err != nil {
		return nil, fmt.Errorf("urlguard blocked wecom URL: %w", err)
	}
	client := urlguard.SafeHTTPClient(opts, 10*time.Second)
	return &WeComChannel{
		id:      id,
		url:     rawURL,
		enabled: enabled,
		client:  client,
	}, nil
}

func (c *WeComChannel) ID() string      { return c.id }
func (c *WeComChannel) Type() string    { return "wecom" }
func (c *WeComChannel) IsEnabled() bool { return c.enabled }
func (c *WeComChannel) Close() error    { return nil }

func (c *WeComChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}

	title := fmt.Sprintf("[UniMap] 定时任务 [%s] %s", n.TaskName, statusLabel(n.Status))
	markdown := fmt.Sprintf(
		"**%s**\n> 类型: %s\n> 耗时: %.1fs\n> 结果: %s",
		title, n.TaskType, n.Duration/1000.0, n.Result,
	)
	if n.Error != "" {
		markdown += fmt.Sprintf("\n> 错误: %s", n.Error)
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": markdown,
		},
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create wecom request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send wecom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wecom returned status %d", resp.StatusCode)
	}

	// WeCom returns HTTP 200 with errcode on failure.
	var wcResp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wcResp); err == nil {
		if wcResp.Errcode != 0 {
			return fmt.Errorf("wecom api error: errcode=%d errmsg=%s", wcResp.Errcode, wcResp.Errmsg)
		}
	}
	return nil
}

func statusLabel(status string) string {
	switch status {
	case "success":
		return "执行成功"
	case "failed":
		return "执行失败"
	case "timeout":
		return "执行超时"
	default:
		return status
	}
}
