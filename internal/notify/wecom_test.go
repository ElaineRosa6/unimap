package notify

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidWeComMsgType(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},            // empty means markdown
		{"markdown", true},    //
		{"MARKDOWN", true},    // normalized case
		{" markdown ", true},  // trimmed
		{"markdown_v2", true}, //
		{"text", true},        //
		{"image", true},       //
		{"file", true},        //
		{"news", false},       // API supports news but we don't implement it
		{"bogus", false},      //
	}
	for _, c := range cases {
		if got := ValidWeComMsgType(c.in); got != c.want {
			t.Errorf("ValidWeComMsgType(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewWeComChannel_InvalidMsgType(t *testing.T) {
	_, err := NewWeComChannelWithOptions("w", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", true, false, WeComOptions{MsgType: "news"})
	if err == nil {
		t.Fatal("expected error for unsupported msgtype")
	}
	if !strings.Contains(err.Error(), "unsupported message type") {
		t.Errorf("error should name the unsupported type, got: %v", err)
	}
}

// TestWeComChannel_OptionsClone verifies the mention slices are deep-copied so
// later mutation of the caller's slices cannot affect the channel.
func TestWeComChannel_OptionsClone(t *testing.T) {
	mentions := []string{"zhangsan"}
	mobile := []string{"13800000000"}
	ch, err := NewWeComChannelWithOptions("w", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", true, false, WeComOptions{
		MsgType:             "text",
		MentionedList:       mentions,
		MentionedMobileList: mobile,
	})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	mentions[0] = "lisi"
	mobile[0] = "13900000000"
	if ch.mentionedList[0] != "zhangsan" {
		t.Errorf("mentionedList mutated: got %v", ch.mentionedList)
	}
	if ch.mentionedMobileList[0] != "13800000000" {
		t.Errorf("mentionedMobileList mutated: got %v", ch.mentionedMobileList)
	}
}

func TestWeComChannel_Send_MarkdownV2(t *testing.T) {
	var received WeComMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeMarkdownV2})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	n := TaskNotification{TaskID: "t1", TaskName: "test", TaskType: "query", Status: "success", Result: "ok", Duration: 1200}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != WeComMsgTypeMarkdownV2 {
		t.Errorf("expected msgtype markdown_v2, got %q", received.MsgType)
	}
	if received.Markdown.Content == "" {
		t.Error("expected markdown content")
	}
}

func TestWeComChannel_Send_Text_WithMentions(t *testing.T) {
	var received WeComTextBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{
		MsgType:             WeComMsgTypeText,
		MentionedList:       []string{"zhangsan", "@all"},
		MentionedMobileList: []string{"13800000000"},
	})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	n := TaskNotification{TaskID: "t1", TaskName: "test", TaskType: "query", Status: "success", Result: "ok", Duration: 1200}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "text" {
		t.Errorf("expected msgtype text, got %q", received.MsgType)
	}
	if len(received.Text.MentionedList) != 2 || received.Text.MentionedList[1] != "@all" {
		t.Errorf("mentioned_list = %v, want [zhangsan @all]", received.Text.MentionedList)
	}
	if len(received.Text.MentionedMobileList) != 1 || received.Text.MentionedMobileList[0] != "13800000000" {
		t.Errorf("mentioned_mobile_list = %v", received.Text.MentionedMobileList)
	}
	if strings.Contains(received.Text.Content, "**") || strings.Contains(received.Text.Content, "> ") {
		t.Errorf("text content must not contain markdown markers, got: %q", received.Text.Content)
	}
}

// TestWeComChannel_Send_Text_Truncation builds a notification whose rendered
// text exceeds the 2048-byte limit and checks the payload is cut without
// splitting a multi-byte rune.
func TestWeComChannel_Send_Text_Truncation(t *testing.T) {
	var received WeComTextBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeText})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	// ~3000 CJK runes at 3 bytes each ≈ 9000 bytes > 2048.
	big := strings.Repeat("巡", 3000)
	n := TaskNotification{Status: "success", Result: big, Duration: 0}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received.Text.Content) > wecomTextMaxBytes {
		t.Errorf("content = %d bytes, want <= %d", len(received.Text.Content), wecomTextMaxBytes)
	}
	if !strings.HasPrefix(received.Text.Content, "[UniMap]") {
		t.Errorf("truncated content should keep the head, got prefix %q", received.Text.Content[:16])
	}
}

func TestWeComChannel_Send_Image(t *testing.T) {
	var received WeComImageBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeImage})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	img := []byte("\x89PNG fake image data")
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, img, 0o644); err != nil {
		t.Fatalf("write temp image: %v", err)
	}
	n := TaskNotification{Status: "success", ImagePaths: []string{path}}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "image" {
		t.Errorf("expected msgtype image, got %q", received.MsgType)
	}
	wantB64 := base64.StdEncoding.EncodeToString(img)
	if received.Image.Base64 != wantB64 {
		t.Errorf("base64 mismatch")
	}
	wantMD5 := fmt.Sprintf("%x", md5.Sum(img))
	if received.Image.MD5 != wantMD5 {
		t.Errorf("md5 = %s, want %s", received.Image.MD5, wantMD5)
	}
}

// TestWeComChannel_Send_Image_Fallback ensures an image-mode channel still
// delivers a markdown notification when no suitable image is attached, so the
// message is never silently dropped.
func TestWeComChannel_Send_Image_Fallback(t *testing.T) {
	var received WeComMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeImage})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	n := TaskNotification{Status: "success", Result: "ok"} // no ImagePaths
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "markdown" {
		t.Errorf("expected markdown fallback, got %q", received.MsgType)
	}
}

// TestWeComChannel_Send_Image_UnsupportedExtension routes a non-image path so
// it is skipped and the channel falls back to markdown.
func TestWeComChannel_Send_Image_UnsupportedExtension(t *testing.T) {
	var received WeComMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, _ := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeImage})
	path := filepath.Join(t.TempDir(), "shot.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	n := TaskNotification{Status: "success", ImagePaths: []string{path}}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "markdown" {
		t.Errorf("expected markdown fallback, got %q", received.MsgType)
	}
}

// TestWeComChannel_Send_File drives the two-step flow: upload_media (multipart)
// then the file message carrying the returned media_id.
func TestWeComChannel_Send_File(t *testing.T) {
	var (
		uploadSeen bool
		fileName   string
		fileBody   []byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/webhook/upload_media":
			uploadSeen = true
			if got := r.URL.Query().Get("type"); got != "file" {
				t.Errorf("upload type param = %q, want file", got)
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Errorf("upload Content-Type = %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(1 << 24); err != nil {
				t.Errorf("parse multipart form: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f, fh, err := r.FormFile("media")
			if err != nil {
				t.Errorf("form file media: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer f.Close()
			fileName = filepath.Base(fh.Filename)
			fileBody, _ = io.ReadAll(f)
			json.NewEncoder(w).Encode(map[string]interface{}{"errcode": 0, "media_id": "MEDIA_123", "type": "file"})
		case "/webhook/send":
			if !uploadSeen {
				t.Error("send called before upload")
			}
			var received WeComFileBody
			json.NewDecoder(r.Body).Decode(&received)
			if received.MsgType != "file" {
				t.Errorf("send msgtype = %q, want file", received.MsgType)
			}
			if received.File.MediaID != "MEDIA_123" {
				t.Errorf("media_id = %q, want MEDIA_123", received.File.MediaID)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL+"/webhook/send", true, true, WeComOptions{MsgType: WeComMsgTypeFile})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	payload := []byte("report.pdf bytes")
	path := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	n := TaskNotification{Status: "success", ImagePaths: []string{path}}
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !uploadSeen {
		t.Error("upload_media was never called")
	}
	if fileName != "report.pdf" {
		t.Errorf("uploaded filename = %q, want report.pdf", fileName)
	}
	if !bytes.Equal(fileBody, payload) {
		t.Error("uploaded bytes differ")
	}
}

func TestWeComChannel_Send_File_Fallback(t *testing.T) {
	var received WeComMarkdownBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL, true, true, WeComOptions{MsgType: WeComMsgTypeFile})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	n := TaskNotification{Status: "success"} // no ImagePaths
	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received.MsgType != "markdown" {
		t.Errorf("expected markdown fallback, got %q", received.MsgType)
	}
}

func TestWeComChannel_UploadAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webhook/upload_media" {
			json.NewEncoder(w).Encode(map[string]interface{}{"errcode": 41006, "errmsg": "media data missing"})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch, err := NewWeComChannelWithOptions("w", server.URL+"/webhook/send", true, true, WeComOptions{MsgType: WeComMsgTypeFile})
	if err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}
	path := filepath.Join(t.TempDir(), "a.pdf")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	n := TaskNotification{Status: "success", ImagePaths: []string{path}}
	if err := ch.Send(context.Background(), n); err == nil {
		t.Fatal("expected upload API error to propagate")
	} else if !strings.Contains(err.Error(), "41006") {
		t.Errorf("error should surface errcode 41006, got: %v", err)
	}
}

func TestWeComAPIError_Hints(t *testing.T) {
	cases := []struct {
		code  int
		want  string
		exact bool // when true, hint must equal want exactly (e.g. empty for undocumented)
	}{
		{45009, "频率限制", false},
		{93000, "key 无效", false},
		{40058, "4096", false},
		{0, "", true},     // errcode 0 is success, no hint
		{99999, "", true}, // undocumented code, no hint
	}
	for _, c := range cases {
		got := wecomErrorHint(c.code)
		if c.exact {
			if got != c.want {
				t.Errorf("wecomErrorHint(%d) = %q, want %q", c.code, got, c.want)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("wecomErrorHint(%d) = %q, want it to contain %q", c.code, got, c.want)
		}
	}
}

func TestWeComAPIError_Format(t *testing.T) {
	err := wecomAPIError(45009, "api freq limit")
	msg := err.Error()
	if !strings.Contains(msg, "45009") {
		t.Errorf("formatted error should embed errcode, got: %v", err)
	}
	if !strings.Contains(msg, "频率限制") {
		t.Errorf("formatted error should append the hint text, got: %v", err)
	}
}

func TestWeComUploadURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc",
			"https://qyapi.weixin.qq.com/cgi-bin/webhook/upload_media?key=abc&type=file",
		},
		{
			"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc&type=image",
			"https://qyapi.weixin.qq.com/cgi-bin/webhook/upload_media?key=abc&type=file",
		},
		{
			// Path not matching /webhook/send is left untouched except type=file.
			"http://127.0.0.1:8080/hook?key=abc",
			"http://127.0.0.1:8080/hook?key=abc&type=file",
		},
	}
	for _, c := range cases {
		if got := wecomUploadURL(c.in); got != c.want {
			t.Errorf("wecomUploadURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFirstSuitablePath(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.png")
	large := filepath.Join(dir, "large.png")
	if err := os.WriteFile(small, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(large, bytes.Repeat([]byte("x"), 100), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := firstSuitablePath([]string{large, small}, 50, nil); got != small {
		t.Errorf("oversize path should be skipped, got %q", got)
	}
	if got := firstSuitablePath([]string{large, small}, 50, isSupportedImage); got != small {
		t.Errorf("expected small.png, got %q", got)
	}
	// Non-image extension rejected by filter.
	txt := filepath.Join(dir, "a.txt")
	os.WriteFile(txt, []byte("t"), 0o644)
	if got := firstSuitablePath([]string{txt}, 50, isSupportedImage); got != "" {
		t.Errorf("txt should be rejected, got %q", got)
	}
	if got := firstSuitablePath([]string{filepath.Join(dir, "missing.png")}, 50, nil); got != "" {
		t.Errorf("missing file should return empty, got %q", got)
	}
}
