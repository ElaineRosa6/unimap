package notify

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
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
	"unicode/utf8"

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

	body := DingTalkMarkdownBody{
		MsgType:  "markdown",
		Markdown: DingTalkMarkdown{Title: title, Text: markdown},
	}

	sendURL := c.baseURL
	if c.secret != "" {
		// Warn if the secret looks like an unresolved environment variable placeholder
		if len(c.secret) > 3 && c.secret[0] == '$' && c.secret[1] == '{' && c.secret[len(c.secret)-1] == '}' {
			return fmt.Errorf("dingtalk secret is an unresolved placeholder: %s — set the environment variable or use the raw value", c.secret)
		}
		ts := TimestampNow()
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
func (c *FeishuChannel) buildFeishuBody(n TaskNotification) FeishuCardBody {
	statusEmoji := map[string]string{"success": "✅", "failed": "❌", "timeout": "⏰"}
	emoji := statusEmoji[n.Status]
	template := "blue"
	if n.Status == "failed" {
		template = "red"
	} else if n.Status == "timeout" {
		template = "orange"
	}
	title := fmt.Sprintf("%s **[UniMap]** 定时任务 **[%s]** %s", emoji, n.TaskName, statusLabel(n.Status))

	elements := buildFeishuPayloadElements(n)

	return FeishuCardBody{
		MsgType: "interactive",
		Card: FeishuCard{
			Header:   FeishuCardHeader{Title: FeishuTextElement{Tag: "plain_text", Content: title}, Template: template},
			Elements: elements,
		},
	}
}

// buildFeishuPayloadElements 构建飞书卡片内容元素
func buildFeishuPayloadElements(n TaskNotification) []FeishuCardElement {
	var elements []FeishuCardElement
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
			elements = append(elements, FeishuMarkdownElement(strings.Join(lines, "\n")))
		}
	}
	elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**耗时**: %.1fs", n.Duration/1000.0)))
	if n.Result != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**执行结果**:\n%s", n.Result)))
	}
	if n.Error != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**错误**: %s", n.Error)))
	}
	return elements
}

// sendFeishuRequest 发送飞书请求并验证响应
func (c *FeishuChannel) sendFeishuRequest(ctx context.Context, body FeishuCardBody) error {
	var data []byte
	sendURL := c.url
	if c.secret != "" {
		if len(c.secret) > 3 && c.secret[0] == '$' && c.secret[1] == '{' && c.secret[len(c.secret)-1] == '}' {
			return fmt.Errorf("feishu secret is an unresolved placeholder: %s — set the environment variable or use the raw value", c.secret)
		}
		ts := TimestampNow()
		sign, err := FeishuSign(c.secret, ts)
		if err != nil {
			return fmt.Errorf("feishu sign error: %w", err)
		}
		// 飞书自定义机器人要求 timestamp + sign 作为 URL query 参数（与钉钉一致），
		// 不能放在请求 body 中，否则服务端验签失败。
		u, err := url.Parse(sendURL)
		if err != nil {
			return fmt.Errorf("parse feishu URL: %w", err)
		}
		q := u.Query()
		q.Set("timestamp", fmt.Sprintf("%d", ts))
		q.Set("sign", sign)
		u.RawQuery = q.Encode()
		sendURL = u.String()
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal feishu body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu returned status %d", resp.StatusCode)
	}
	var feishuResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&feishuResp); err == nil && feishuResp.Code != 0 {
		return fmt.Errorf("feishu api error: code=%d msg=%s", feishuResp.Code, feishuResp.Msg)
	}
	return nil
}

// WeComChannel 企业微信群机器人
type WeComChannel struct {
	id                  string
	url                 string
	enabled             bool
	client              *http.Client
	msgType             string
	mentionedList       []string
	mentionedMobileList []string
}

// WeComOptions configures a WeCom webhook channel beyond its URL.
type WeComOptions struct {
	// MsgType selects the webhook message type. Supported values: "markdown"
	// (default), "markdown_v2", "text", "image", "file" — see the group-bot
	// Webhook API doc §4. Empty means "markdown".
	MsgType string
	// MentionedList / MentionedMobileList only apply to text messages (doc
	// §4.1): userids / mobile numbers to @. "@all" mentions everyone.
	MentionedList       []string
	MentionedMobileList []string
}

// WeCom webhook message types.
const (
	WeComMsgTypeMarkdown   = "markdown"
	WeComMsgTypeMarkdownV2 = "markdown_v2"
	WeComMsgTypeText       = "text"
	WeComMsgTypeImage      = "image"
	WeComMsgTypeFile       = "file"
)

var validWeComMsgTypes = map[string]bool{
	WeComMsgTypeMarkdown:   true,
	WeComMsgTypeMarkdownV2: true,
	WeComMsgTypeText:       true,
	WeComMsgTypeImage:      true,
	WeComMsgTypeFile:       true,
}

func NewWeComChannel(id, rawURL string, enabled bool, allowPrivate bool) (*WeComChannel, error) {
	return NewWeComChannelWithOptions(id, rawURL, enabled, allowPrivate, WeComOptions{})
}

// NewWeComChannelWithOptions creates a WeCom webhook channel. The msgType is
// validated up front so a mistyped config value fails at load time instead of
// silently degrading every send.
func NewWeComChannelWithOptions(id, rawURL string, enabled bool, allowPrivate bool, opts WeComOptions) (*WeComChannel, error) {
	checkOpts := urlguard.CheckOptions{AllowPrivate: allowPrivate}
	if _, err := urlguard.Check(rawURL, checkOpts); err != nil {
		return nil, fmt.Errorf("urlguard blocked wecom URL: %w", err)
	}
	msgType := normalizeWeComMsgType(opts.MsgType)
	if !validWeComMsgTypes[msgType] {
		return nil, fmt.Errorf("wecom channel %q: unsupported message type %q (want one of markdown, markdown_v2, text, image, file)", id, opts.MsgType)
	}
	client := urlguard.SafeHTTPClient(checkOpts, 30*time.Second)
	return &WeComChannel{
		id:                  id,
		url:                 rawURL,
		enabled:             enabled,
		client:              client,
		msgType:             msgType,
		mentionedList:       append([]string(nil), opts.MentionedList...),
		mentionedMobileList: append([]string(nil), opts.MentionedMobileList...),
	}, nil
}

// normalizeWeComMsgType lower-cases and trims a configured message type,
// treating the empty value as the default "markdown".
func normalizeWeComMsgType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return WeComMsgTypeMarkdown
	}
	return s
}

// ValidWeComMsgType reports whether s names a supported WeCom message type.
// The empty value is valid and means "markdown".
func ValidWeComMsgType(s string) bool {
	return validWeComMsgTypes[normalizeWeComMsgType(s)]
}

func (c *WeComChannel) ID() string      { return c.id }
func (c *WeComChannel) Type() string    { return "wecom" }
func (c *WeComChannel) IsEnabled() bool { return c.enabled }
func (c *WeComChannel) Close() error    { return nil }

// WeCom webhook content limits from the API doc §4/§5.
const (
	// wecomMarkdownMaxBytes is the markdown / markdown_v2 content limit
	// (errcode 40058: "markdown.content exceed max length 4096").
	wecomMarkdownMaxBytes = 4096
	// wecomTextMaxBytes is the text content limit (doc §4.1).
	wecomTextMaxBytes = 2048
	// wecomImageMaxBytes is the image limit, measured before base64 (doc §4.4).
	wecomImageMaxBytes = 2 << 20
	// wecomFileMaxBytes is the file upload limit (doc §5).
	wecomFileMaxBytes = 20 << 20
)

// truncateUTF8 truncates s to at most maxBytes bytes without splitting a
// multi-byte UTF-8 rune, so CJK content is never corrupted mid-character.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := s[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func (c *WeComChannel) Send(ctx context.Context, n TaskNotification) error {
	if !c.enabled {
		return nil
	}
	switch c.msgType {
	case WeComMsgTypeText:
		if err := c.sendText(ctx, n); err != nil {
			return err
		}
		return c.sendAttachedFiles(ctx, n)
	case WeComMsgTypeImage:
		return c.sendImage(ctx, n)
	case WeComMsgTypeFile:
		return c.sendFile(ctx, n)
	case WeComMsgTypeMarkdownV2:
		// markdown_v2 renders pipe tables, lists and code blocks that classic
		// markdown does not (doc §4.3), so the compact query table stays a table.
		if err := c.sendMarkdown(ctx, n, WeComMsgTypeMarkdownV2); err != nil {
			return err
		}
		return c.sendAttachedFiles(ctx, n)
	default:
		if err := c.sendMarkdown(ctx, n, WeComMsgTypeMarkdown); err != nil {
			return err
		}
		return c.sendAttachedFiles(ctx, n)
	}
}

func (c *WeComChannel) sendMarkdown(ctx context.Context, n TaskNotification, msgType string) error {
	markdown := truncateUTF8(buildWeComMarkdown(n), wecomMarkdownMaxBytes)
	return c.postWeCom(ctx, WeComMarkdownBody{
		MsgType:  msgType,
		Markdown: WeComMarkdown{Content: markdown},
	})
}

func (c *WeComChannel) sendText(ctx context.Context, n TaskNotification) error {
	content := truncateUTF8(buildWeComText(n), wecomTextMaxBytes)
	return c.postWeCom(ctx, WeComTextBody{
		MsgType: WeComMsgTypeText,
		Text: WeComText{
			Content:             content,
			MentionedList:       c.mentionedList,
			MentionedMobileList: c.mentionedMobileList,
		},
	})
}

// sendImage sends the first suitable screenshot as an image message (doc
// §4.4). If no suitable image is attached the notification falls back to
// markdown so the text content is still delivered.
func (c *WeComChannel) sendImage(ctx context.Context, n TaskNotification) error {
	imagePath := firstSuitablePath(n.ImagePaths, wecomImageMaxBytes, isSupportedImage)
	if imagePath == "" {
		return c.sendMarkdown(ctx, n, WeComMsgTypeMarkdown)
	}
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("read wecom image %s: %w", imagePath, err)
	}
	sum := md5.Sum(imageData)
	body := WeComImageBody{
		MsgType: WeComMsgTypeImage,
		Image: WeComImage{
			Base64: base64.StdEncoding.EncodeToString(imageData),
			MD5:    fmt.Sprintf("%x", sum),
		},
	}
	return c.postWeCom(ctx, body)
}

// sendFile sends the markdown body first, then the first attached file (doc
// §4.6 + §5, up to 20MB). With no attachment it is markdown-only, so a single
// wecom channel covers both text and Excel/screenshot delivery.
func (c *WeComChannel) sendFile(ctx context.Context, n TaskNotification) error {
	if err := c.sendMarkdown(ctx, n, WeComMsgTypeMarkdown); err != nil {
		return err
	}
	return c.sendAttachedFiles(ctx, n)
}

// sendAttachedFiles uploads and sends the first deliverable path as a file
// message. It is a no-op when nothing is attached.
func (c *WeComChannel) sendAttachedFiles(ctx context.Context, n TaskNotification) error {
	filePath := firstSuitablePath(n.ImagePaths, wecomFileMaxBytes, nil)
	if filePath == "" {
		return nil
	}
	mediaID, err := c.uploadMedia(ctx, filePath)
	if err != nil {
		return err
	}
	return c.postWeCom(ctx, WeComFileBody{
		MsgType: WeComMsgTypeFile,
		File:    WeComFile{MediaID: mediaID},
	})
}

// firstSuitablePath returns the first path that exists, is not a directory,
// is at most maxBytes bytes, and matches the optional accept filter.
func firstSuitablePath(paths []string, maxBytes int64, accept func(string) bool) string {
	for _, p := range paths {
		if accept != nil && !accept(p) {
			continue
		}
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			continue
		}
		if maxBytes > 0 && info.Size() > maxBytes {
			continue
		}
		return p
	}
	return ""
}

func isSupportedImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}

// uploadMedia uploads a file to the upload_media endpoint and returns its
// media_id (doc §5). The media_id is valid for 3 days and only usable by the
// same webhook key.
func (c *WeComChannel) uploadMedia(ctx context.Context, filePath string) (string, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read wecom file %s: %w", filePath, err)
	}
	if len(fileData) > wecomFileMaxBytes {
		return "", fmt.Errorf("wecom file %s exceeds %d byte limit", filePath, wecomFileMaxBytes)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("create wecom upload form: %w", err)
	}
	if _, wErr := part.Write(fileData); wErr != nil {
		return "", fmt.Errorf("write wecom upload form: %w", wErr)
	}
	if cErr := writer.Close(); cErr != nil {
		return "", fmt.Errorf("close wecom upload form: %w", cErr)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", wecomUploadURL(c.url), &buf)
	if err != nil {
		return "", fmt.Errorf("create wecom upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wecom upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("wecom upload returned status %d", resp.StatusCode)
	}

	var result wecomMediaUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode wecom upload response: %w", err)
	}
	if result.Errcode != 0 {
		return "", wecomAPIError(result.Errcode, result.Errmsg)
	}
	if result.MediaID == "" {
		return "", fmt.Errorf("wecom upload response missing media_id")
	}
	return result.MediaID, nil
}

// wecomUploadURL derives the upload_media endpoint from the send webhook URL,
// preserving the key credential and forcing type=file.
func wecomUploadURL(sendURL string) string {
	u, err := url.Parse(sendURL)
	if err != nil {
		return sendURL
	}
	if strings.HasSuffix(u.Path, "/webhook/send") {
		u.Path = strings.TrimSuffix(u.Path, "/webhook/send") + "/webhook/upload_media"
	}
	q := u.Query()
	q.Set("type", "file")
	u.RawQuery = q.Encode()
	return u.String()
}

// postWeCom POSTs a JSON body to the webhook and verifies the response,
// decoding the errcode/errmsg envelope WeCom returns with HTTP 200 (doc §7.1).
func (c *WeComChannel) postWeCom(ctx context.Context, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal wecom body: %w", err)
	}
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

	var wcResp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wcResp); err == nil && wcResp.Errcode != 0 {
		return wecomAPIError(wcResp.Errcode, wcResp.Errmsg)
	}
	return nil
}

// wecomAPIError builds an error for a non-zero errcode, appending an
// actionable hint for documented codes (doc §7.2).
func wecomAPIError(errcode int, errmsg string) error {
	if hint := wecomErrorHint(errcode); hint != "" {
		return fmt.Errorf("wecom api error: errcode=%d errmsg=%s（%s）", errcode, errmsg, hint)
	}
	return fmt.Errorf("wecom api error: errcode=%d errmsg=%s", errcode, errmsg)
}

// wecomErrorHints maps documented webhook error codes to remediation hints.
// Codes without an entry fall through to the raw errmsg.
var wecomErrorHints = map[int]string{
	-1:    "系统繁忙，稍后重试（重试不超过 3 次）",
	40001: "不合法的 key，检查 Webhook 地址中的 key",
	40003: "无效的 UserID，检查 mentioned_list",
	40004: "不合法的媒体文件类型",
	40006: "不合法的文件大小",
	40007: "不合法的 media_id（已过期或不属于当前机器人）",
	40008: "不合法的 msgtype",
	40013: "不合法的 CorpID",
	40033: "不合法的请求字符（不能包含 \\uxxxx 转义）",
	40035: "不合法的参数",
	40058: "消息内容超过最大长度（markdown 4096 / text 2048 字节）",
	40063: "参数为空，检查必填参数",
	41006: "缺少 media_id 参数",
	42001: "access_token 已过期",
	44001: "多媒体文件为空",
	44004: "文本消息 content 参数为空",
	45001: "多媒体文件大小超过限制",
	45002: "消息内容大小超过限制",
	45007: "语音播放时间超过 60 秒",
	45008: "图文消息文章数量超过 8 条",
	45009: "超过 20 条/分钟频率限制，请合并消息或稍后重试",
	45033: "接口并发调用超过限制，请降低并发",
	45034: "url 必须有 http:// 或 https:// 协议头",
	46004: "指定的用户不存在",
	48002: "API 接口无权限调用",
	93000: "Webhook key 无效，请核对地址（泄露后需删除机器人重建）",
}

func wecomErrorHint(errcode int) string {
	return wecomErrorHints[errcode]
}

func buildWeComMarkdown(n TaskNotification) string {
	title := fmt.Sprintf("[UniMap] 定时任务 [%s] %s", n.TaskName, statusLabel(n.Status))
	markdown := fmt.Sprintf(
		"**%s**\n> 类型: %s\n> 耗时: %.1fs\n> 结果: %s",
		title, n.TaskType, n.Duration/1000.0, n.Result,
	)
	if n.Error != "" {
		markdown += fmt.Sprintf("\n> 错误: %s", n.Error)
	}
	return markdown
}

// buildWeComText renders the same notification without markdown markers, so a
// text message does not display raw ** / > characters (doc §4.1).
func buildWeComText(n TaskNotification) string {
	title := fmt.Sprintf("[UniMap] 定时任务 [%s] %s", n.TaskName, statusLabel(n.Status))
	text := fmt.Sprintf(
		"%s\n类型: %s\n耗时: %.1fs\n结果: %s",
		title, n.TaskType, n.Duration/1000.0, n.Result,
	)
	if n.Error != "" {
		text += fmt.Sprintf("\n错误: %s", n.Error)
	}
	return text
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
	// when connecting to open.feishu.cn. Forces IPv4-first dial and generous timeouts
	// to accommodate slow DNS resolution (12s+ observed on some Windows environments).
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Force IPv4 to avoid IPv6 resolution hangs on some Windows environments.
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
	}
	return &FeishuAppChannel{
		appID:     appID,
		appSecret: appSecret,
		chatID:    chatID,
		enabled:   enabled,
		client:    &http.Client{Timeout: 45 * time.Second, Transport: transport},
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
	writer.WriteField("image_type", "message") //nolint:errcheck

	// 添加图片文件
	part, err := writer.CreateFormFile("image", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	part.Write(imageData) //nolint:errcheck
	writer.Close()        //nolint:errcheck

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
func (c *FeishuAppChannel) sendMessage(ctx context.Context, body FeishuAppMessage) error {
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
	if n.Status == "failed" {
		template = "red"
	} else if n.Status == "timeout" {
		template = "orange"
	}
	card := FeishuCard{
		Header:   FeishuCardHeader{Title: FeishuTextElement{Tag: "plain_text", Content: title}, Template: template},
		Elements: elements,
	}
	cardJSON, _ := json.Marshal(card)
	return c.sendMessage(ctx, FeishuAppMessage{
		ReceiveID: c.chatID,
		MsgType:   "interactive",
		Content:   string(cardJSON),
	})
}

func buildFeishuAppTitle(n TaskNotification) string {
	statusEmoji := map[string]string{"success": "✅", "failed": "❌", "timeout": "⏰"}
	return fmt.Sprintf("%s [UniMap] 定时任务 [%s] %s", statusEmoji[n.Status], n.TaskName, statusLabel(n.Status))
}

func buildFeishuAppPayloadElements(ctx context.Context, c *FeishuAppChannel, n TaskNotification) []FeishuCardElement {
	var elements []FeishuCardElement
	if n.Payload != nil {
		fields := []string{"urls", "query", "queries", "engines", "engine", "detection_mode", "low_threshold", "format", "ports", "max_age_days", "alert_type", "duration_minutes", "task_type", "type", "file_pattern"}
		var lines []string
		for _, f := range fields {
			if v, ok := n.Payload[f]; ok {
				lines = append(lines, fmt.Sprintf("**%s**: %v", f, v))
			}
		}
		if len(lines) > 0 {
			elements = append(elements, FeishuMarkdownElement(strings.Join(lines, "\n")))
		}
	}
	elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**耗时**: %.1fs", n.Duration/1000.0)))
	if len(n.ImagePaths) > 0 {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement("**截图预览**:"))
		for i, imgPath := range n.ImagePaths {
			imageKey, err := c.uploadImage(ctx, imgPath)
			if err != nil {
				elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("⚠️ 截图 #%d 上传失败", i+1)))
				continue
			}
			elements = append(elements, FeishuImageElement(imageKey, fmt.Sprintf("截图 #%d", i+1)))
		}
	}
	if n.Result != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**执行结果**:\n%s", n.Result)))
	}
	if n.Error != "" {
		elements = append(elements, FeishuHRElement())
		elements = append(elements, FeishuMarkdownElement(fmt.Sprintf("**错误**: %s", n.Error)))
	}
	return elements
}
