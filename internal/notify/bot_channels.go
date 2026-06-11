package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unimap/project/internal/utils/urlguard"
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
	client := urlguard.SafeHTTPClient(opts, 30*time.Second)
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
		// Warn if the secret looks like an unresolved environment variable placeholder
		if len(c.secret) > 3 && c.secret[0] == '$' && c.secret[1] == '{' && c.secret[len(c.secret)-1] == '}' {
			return fmt.Errorf("dingtalk secret is an unresolved placeholder: %s — set the environment variable or use the raw value", c.secret)
		}
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
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

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
	client := urlguard.SafeHTTPClient(opts, 30*time.Second)
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
	body := c.buildFeishuBody(n)
	return c.sendFeishuRequest(ctx, body)
}

// buildFeishuBody 构建飞书消息卡片 body
func (c *FeishuChannel) buildFeishuBody(n TaskNotification) map[string]interface{} {
	statusEmoji := map[string]string{"success": "✅", "failed": "❌", "timeout": "⏰"}
	emoji := statusEmoji[n.Status]
	template := "blue"
	if n.Status == "failed" { template = "red" } else if n.Status == "timeout" { template = "orange" }
	title := fmt.Sprintf("%s **[UniMap]** 定时任务 **[%s]** %s", emoji, n.TaskName, statusLabel(n.Status))

	elements := buildFeishuPayloadElements(n)

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header":   map[string]interface{}{"title": map[string]interface{}{"tag": "plain_text", "content": title}, "template": template},
			"elements": elements,
		},
	}
}

// buildFeishuPayloadElements 构建飞书卡片内容元素
func buildFeishuPayloadElements(n TaskNotification) []map[string]interface{} {
	var elements []map[string]interface{}
	if n.Payload != nil {
		payloadFields := []struct{ key, label string }{
			{"urls", "目标"}, {"query", "查询"}, {"queries", "查询"}, {"engines", "引擎"}, {"engine", "引擎"},
			{"detection_mode", "检测模式"}, {"low_threshold", "阈值"}, {"format", "格式"}, {"ports", "端口"},
			{"max_age_days", "保留天数"}, {"alert_type", "告警类型"}, {"duration_minutes", "静默时长"},
			{"task_type", "任务类型"}, {"type", "备案类型"}, {"file_pattern", "文件模式"},
		}
		var lines []string
		for _, f := range payloadFields {
			if v, ok := n.Payload[f.key]; ok {
				if f.key == "query" {
					lines = append(lines, fmt.Sprintf("**%s**: `%v`", f.label, v))
				} else {
					lines = append(lines, fmt.Sprintf("**%s**: %v", f.label, v))
				}
			}
		}
		if len(lines) > 0 {
			elements = append(elements, map[string]interface{}{"tag": "markdown", "content": strings.Join(lines, "\n")})
		}
	}
	elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**耗时**: %.1fs", n.Duration/1000.0)})
	if n.Result != "" {
		elements = append(elements, map[string]interface{}{"tag": "hr"})
		elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**执行结果**:\n%s", n.Result)})
	}
	if n.Error != "" {
		elements = append(elements, map[string]interface{}{"tag": "hr"})
		elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**错误**: %s", n.Error)})
	}
	return elements
}

// sendFeishuRequest 发送飞书请求并验证响应
func (c *FeishuChannel) sendFeishuRequest(ctx context.Context, body map[string]interface{}) error {
	if c.secret != "" {
		if len(c.secret) > 3 && c.secret[0] == '$' && c.secret[1] == '{' && c.secret[len(c.secret)-1] == '}' {
			return fmt.Errorf("feishu secret is an unresolved placeholder: %s — set the environment variable or use the raw value", c.secret)
		}
		ts := TimestampNow() / 1000
		sign, err := FeishuSign(c.secret, ts)
		if err != nil { return fmt.Errorf("feishu sign error: %w", err) }
		body["timestamp"] = fmt.Sprintf("%d", ts)
		body["sign"] = sign
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(data))
	if err != nil { return fmt.Errorf("create feishu request: %w", err) }
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.client.Do(req)
	if err != nil { return fmt.Errorf("send feishu: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("feishu returned status %d", resp.StatusCode) }
	var feishuResp struct { Code int `json:"code"`; Msg string `json:"msg"` }
	if err := json.NewDecoder(resp.Body).Decode(&feishuResp); err == nil && feishuResp.Code != 0 {
		return fmt.Errorf("feishu api error: code=%d msg=%s", feishuResp.Code, feishuResp.Msg)
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
	client := urlguard.SafeHTTPClient(opts, 30*time.Second)
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
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

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

// FeishuAppChannel 飞书应用机器人（支持图片上传）
type FeishuAppChannel struct {
	appID     string
	appSecret string
	chatID    string
	enabled   bool
	client    *http.Client
	token     string
	tokenExp  time.Time
}

// NewFeishuAppChannel 创建飞书应用渠道
func NewFeishuAppChannel(appID, appSecret, chatID string, enabled bool) *FeishuAppChannel {
	// Custom transport to work around Go HTTP client DNS/connection issues on Windows
	// when connecting to open.feishu.cn. Forces IPv4-first dial and shorter timeouts.
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Force IPv4 to avoid IPv6 resolution hangs on some Windows environments.
			dialer := &net.Dialer{
				Timeout:   8 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		MaxIdleConns:           10,
		MaxIdleConnsPerHost:    5,
		IdleConnTimeout:        90 * time.Second,
	}
	return &FeishuAppChannel{
		appID:     appID,
		appSecret: appSecret,
		chatID:    chatID,
		enabled:   enabled,
		client:    &http.Client{Timeout: 20 * time.Second, Transport: transport},
	}
}

func (c *FeishuAppChannel) ID() string      { return "feishu_app" }
func (c *FeishuAppChannel) Type() string    { return "feishu_app" }
func (c *FeishuAppChannel) IsEnabled() bool { return c.enabled }
func (c *FeishuAppChannel) Close() error    { return nil }

// getToken 获取 tenant_access_token
func (c *FeishuAppChannel) getToken(ctx context.Context) (string, error) {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	body := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	data, _ := json.Marshal(body)

	const tokenURL = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("token error: code=%d msg=%s", result.Code, result.Msg)
	}

	c.token = result.TenantAccessToken
	c.tokenExp = time.Now().Add(time.Duration(result.Expire-60) * time.Second) // 提前 60 秒过期
	return c.token, nil
}

// uploadImage 上传图片获取 image_key
func (c *FeishuAppChannel) uploadImage(ctx context.Context, imagePath string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	// 读取图片文件
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	// 构建 multipart 请求
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加 image_type 字段
	writer.WriteField("image_type", "message")

	// 添加图片文件
	part, err := writer.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	part.Write(imageData)
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/images",
		&buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload image: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("upload error: code=%d msg=%s", result.Code, result.Msg)
	}

	return result.Data.ImageKey, nil
}

// sendMessage 发送消息到群
func (c *FeishuAppChannel) sendMessage(ctx context.Context, body map[string]interface{}) error {
	token, err := c.getToken(ctx)
	if err != nil {
		return err
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create message request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode message response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("message error: code=%d msg=%s", result.Code, result.Msg)
	}

	return nil
}

// Send 发送通知（支持图片）
func (c *FeishuAppChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}
	elements := buildFeishuAppPayloadElements(ctx, c, n)
	title := buildFeishuAppTitle(n)
	template := "blue"
	if n.Status == "failed" { template = "red" } else if n.Status == "timeout" { template = "orange" }
	card := map[string]interface{}{
		"header":   map[string]interface{}{"title": map[string]interface{}{"tag": "plain_text", "content": title}, "template": template},
		"elements": elements,
	}
	cardJSON, _ := json.Marshal(card)
	return c.sendMessage(ctx, map[string]interface{}{
		"receive_id": c.chatID, "msg_type": "interactive", "content": string(cardJSON),
	})
}

func buildFeishuAppTitle(n TaskNotification) string {
	statusEmoji := map[string]string{"success": "✅", "failed": "❌", "timeout": "⏰"}
	return fmt.Sprintf("%s [UniMap] 定时任务 [%s] %s", statusEmoji[n.Status], n.TaskName, statusLabel(n.Status))
}

func buildFeishuAppPayloadElements(ctx context.Context, c *FeishuAppChannel, n TaskNotification) []map[string]interface{} {
	var elements []map[string]interface{}
	if n.Payload != nil {
		fields := []string{"urls", "query", "queries", "engines", "engine", "detection_mode", "low_threshold", "format", "ports", "max_age_days", "alert_type", "duration_minutes", "task_type", "type", "file_pattern"}
		var lines []string
		for _, f := range fields {
			if v, ok := n.Payload[f]; ok { lines = append(lines, fmt.Sprintf("**%s**: %v", f, v)) }
		}
		if len(lines) > 0 { elements = append(elements, map[string]interface{}{"tag": "markdown", "content": strings.Join(lines, "\n")}) }
	}
	elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**耗时**: %.1fs", n.Duration/1000.0)})
	if len(n.ImagePaths) > 0 {
		elements = append(elements, map[string]interface{}{"tag": "hr"})
		elements = append(elements, map[string]interface{}{"tag": "markdown", "content": "**截图预览**:"})
		for i, imgPath := range n.ImagePaths {
			imageKey, err := c.uploadImage(ctx, imgPath)
			if err != nil {
				elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("⚠️ 截图 #%d 上传失败", i+1)})
				continue
			}
			elements = append(elements, map[string]interface{}{"tag": "img", "img_key": imageKey, "alt": map[string]interface{}{"tag": "plain_text", "content": fmt.Sprintf("截图 #%d", i+1)}})
		}
	}
	if n.Result != "" {
		elements = append(elements, map[string]interface{}{"tag": "hr"})
		elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**执行结果**:\n%s", n.Result)})
	}
	if n.Error != "" {
		elements = append(elements, map[string]interface{}{"tag": "hr"})
		elements = append(elements, map[string]interface{}{"tag": "markdown", "content": fmt.Sprintf("**错误**: %s", n.Error)})
	}
	return elements
}
