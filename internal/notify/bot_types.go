package notify

// --- DingTalk ---

// DingTalkMarkdownBody is the JSON body for DingTalk markdown messages.
type DingTalkMarkdownBody struct {
	MsgType  string           `json:"msgtype"`
	Markdown DingTalkMarkdown `json:"markdown"`
}

// DingTalkMarkdown holds the markdown content fields.
type DingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// --- WeCom ---

// WeComMarkdownBody is the JSON body for WeCom markdown / markdown_v2 messages.
type WeComMarkdownBody struct {
	MsgType  string        `json:"msgtype"`
	Markdown WeComMarkdown `json:"markdown"`
}

// WeComMarkdown holds the markdown content.
type WeComMarkdown struct {
	Content string `json:"content"`
}

// WeComTextBody is the JSON body for WeCom text messages (doc §4.1).
type WeComTextBody struct {
	MsgType string    `json:"msgtype"`
	Text    WeComText `json:"text"`
}

// WeComText holds a plain-text message with optional @ mentions.
// MentionedList and MentionedMobileList may each contain "@all" to mention
// everyone in the group.
type WeComText struct {
	Content             string   `json:"content"`
	MentionedList       []string `json:"mentioned_list,omitempty"`
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
}

// WeComImageBody is the JSON body for WeCom image messages (doc §4.4).
type WeComImageBody struct {
	MsgType string     `json:"msgtype"`
	Image   WeComImage `json:"image"`
}

// WeComImage holds a base64-encoded image plus its raw (pre-base64) MD5.
type WeComImage struct {
	Base64 string `json:"base64"`
	MD5    string `json:"md5"`
}

// WeComFileBody is the JSON body for WeCom file messages (doc §4.6).
type WeComFileBody struct {
	MsgType string    `json:"msgtype"`
	File    WeComFile `json:"file"`
}

// WeComFile holds a media_id obtained from the upload_media endpoint.
type WeComFile struct {
	MediaID string `json:"media_id"`
}

// wecomMediaUploadResponse is the response of the upload_media endpoint (§5).
type wecomMediaUploadResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	Type    string `json:"type"`
	MediaID string `json:"media_id"`
}

// --- Feishu ---

// FeishuCardBody is the top-level Feishu interactive card body.
type FeishuCardBody struct {
	MsgType string     `json:"msg_type"`
	Card    FeishuCard `json:"card"`
}

// FeishuCard represents a Feishu message card.
type FeishuCard struct {
	Header   FeishuCardHeader    `json:"header"`
	Elements []FeishuCardElement `json:"elements"`
}

// FeishuCardHeader is the card header.
type FeishuCardHeader struct {
	Title    FeishuTextElement `json:"title"`
	Template string            `json:"template"`
}

// FeishuTextElement is a Feishu plain_text element.
type FeishuTextElement struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

// FeishuCardElement is a union type for card elements (markdown, hr, img).
type FeishuCardElement struct {
	Tag     string             `json:"tag"`
	Content string             `json:"content,omitempty"`
	ImgKey  string             `json:"img_key,omitempty"`
	Alt     *FeishuTextElement `json:"alt,omitempty"`
}

// FeishuMarkdownElement creates a markdown card element.
func FeishuMarkdownElement(content string) FeishuCardElement {
	return FeishuCardElement{Tag: "markdown", Content: content}
}

// FeishuHRElement creates a horizontal rule card element.
func FeishuHRElement() FeishuCardElement {
	return FeishuCardElement{Tag: "hr"}
}

// FeishuImageElement creates an image card element.
func FeishuImageElement(imgKey, altText string) FeishuCardElement {
	return FeishuCardElement{
		Tag:    "img",
		ImgKey: imgKey,
		Alt:    &FeishuTextElement{Tag: "plain_text", Content: altText},
	}
}

// FeishuAppMessage is the body for Feishu App API send message.
type FeishuAppMessage struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
}
