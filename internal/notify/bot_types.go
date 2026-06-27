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

// WeComMarkdownBody is the JSON body for WeCom markdown messages.
type WeComMarkdownBody struct {
	MsgType  string        `json:"msgtype"`
	Markdown WeComMarkdown `json:"markdown"`
}

// WeComMarkdown holds the markdown content.
type WeComMarkdown struct {
	Content string `json:"content"`
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
