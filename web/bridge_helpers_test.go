package web

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unimap/project/internal/config"
)

func TestSplitDataURL_PNG(t *testing.T) {
	header, _, err := splitDataURL("data:image/png;base64,iVBOR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(header, "image/png") {
		t.Fatalf("expected 'image/png' in header, got %q", header)
	}
}

func TestSplitDataURL_JPEG(t *testing.T) {
	header, _, err := splitDataURL("data:image/jpeg;base64,/9j/4A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(header, "image/jpeg") {
		t.Fatalf("expected 'image/jpeg' in header, got %q", header)
	}
}

func TestSplitDataURL_InvalidPrefix(t *testing.T) {
	_, _, err := splitDataURL("not-a-data-url")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestSplitDataURL_NoComma(t *testing.T) {
	_, _, err := splitDataURL("data:image/png;base64")
	if err == nil {
		t.Fatal("expected error for missing comma")
	}
}

func TestDetectImageFormat_PNG(t *testing.T) {
	ext, mime := detectImageFormat("data:image/png")
	if ext != ".png" || mime != "image/png" {
		t.Fatalf("expected .png/image/png, got %q/%q", ext, mime)
	}
}

func TestDetectImageFormat_JPEG(t *testing.T) {
	ext, mime := detectImageFormat("data:image/jpeg")
	if ext != ".jpg" || mime != "image/jpeg" {
		t.Fatalf("expected .jpg/image/jpeg, got %q/%q", ext, mime)
	}
}

func TestDetectImageFormat_WebP(t *testing.T) {
	ext, mime := detectImageFormat("data:image/webp")
	if ext != ".webp" || mime != "image/webp" {
		t.Fatalf("expected .webp/image/webp, got %q/%q", ext, mime)
	}
}

func TestDetectImageFormat_Unknown(t *testing.T) {
	ext, mime := detectImageFormat("data:image/gif")
	if ext != ".png" || mime != "image/png" {
		t.Fatalf("expected .png/image/png for unknown, got %q/%q", ext, mime)
	}
}

func TestBuildBridgeFilePath_WithRequestID(t *testing.T) {
	got := buildBridgeFilePath("/batch", ".png", "req-123", "")
	if !strings.Contains(got, "req-123.png") {
		t.Fatalf("expected req-123.png in path, got %q", got)
	}
	if !strings.Contains(got, "batch") {
		t.Fatalf("expected 'batch' in path, got %q", got)
	}
}

func TestBuildBridgeFilePath_EmptyRequestID(t *testing.T) {
	got := buildBridgeFilePath("/batch", ".png", "", "")
	if !strings.Contains(got, "bridge_") {
		t.Fatalf("expected 'bridge_' prefix for empty requestID, got %q", got)
	}
}

func TestBuildBridgeFilePath_WithURL(t *testing.T) {
	got := buildBridgeFilePath("/batch", ".jpg", "req1", "https://example.com/path")
	if !strings.Contains(got, "example.com_req1.jpg") {
		t.Fatalf("expected 'example.com_req1.jpg', got %q", got)
	}
}

func TestBuildBridgeFilePath_SanitizesSpecialChars(t *testing.T) {
	got := buildBridgeFilePath("/batch", ".png", "req/123:test", "")
	// / and : should be replaced with _
	if strings.Contains(got, "req/123:test") {
		t.Fatalf("expected special chars sanitized, got %q", got)
	}
}

func TestBuildBridgeFilePath_EmptyAll(t *testing.T) {
	got := buildBridgeFilePath("/batch", ".png", "  ", "  ")
	if !strings.Contains(got, "bridge_") {
		t.Fatalf("expected 'bridge_' for all-whitespace input, got %q", got)
	}
}

func TestAllowedExtensionIDsFromConfig_Nil(t *testing.T) {
	got := allowedExtensionIDsFromConfig(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestAllowedExtensionIDsFromConfig_Empty(t *testing.T) {
	cfg := &config.Config{}
	got := allowedExtensionIDsFromConfig(cfg)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestAllowedExtensionIDsFromConfig_WithIDs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Web.CORS.AllowedExtensionIDs = []string{"abc123", "def456"}
	got := allowedExtensionIDsFromConfig(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
}

func TestAllowedExtensionIDsFromConfig_FiltersEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.Web.CORS.AllowedExtensionIDs = []string{"abc123", "", "  ", "def456"}
	got := allowedExtensionIDsFromConfig(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 (empty filtered), got %d: %v", len(got), got)
	}
}

func TestEncodeJPEG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{255, 0, 0, 255})
	err := encodeJPEG(path, img)
	if err != nil {
		t.Fatalf("encodeJPEG: %v", err)
	}
	// Verify file exists and is valid JPEG
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	_, err = jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
}

func TestEncodePNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{0, 255, 0, 255})
	err := encodePNG(path, img)
	if err != nil {
		t.Fatalf("encodePNG: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	_, err = png.Decode(f)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
}

func TestWriteRawToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raw.bin")
	data := []byte("hello world")
	err := writeRawToFile(path, data)
	if err != nil {
		t.Fatalf("writeRawToFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(got))
	}
}

func TestEncodeOrWriteRaw_RawSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	called := false
	err := encodeOrWriteRaw(path, []byte("data"), func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected encode function to be called")
	}
}

func TestEncodeOrWriteRaw_RawFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	err := encodeOrWriteRaw(path, []byte("data"), func() error {
		return fmt.Errorf("encode failed")
	})
	if err == nil {
		t.Fatal("expected error when encode fails")
	}
}

func TestSaveImageToFile_JPEG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	// Create a minimal valid JPEG
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	raw := buf.Bytes()

	result, err := saveImageToFile(path, "image/jpeg", ".jpg", raw)
	if err != nil {
		t.Fatalf("saveImageToFile: %v", err)
	}
	if result != path {
		t.Fatalf("expected %q, got %q", path, result)
	}
}

func TestSaveImageToFile_PNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	raw := buf.Bytes()

	result, err := saveImageToFile(path, "image/png", ".png", raw)
	if err != nil {
		t.Fatalf("saveImageToFile: %v", err)
	}
	if result != path {
		t.Fatalf("expected %q, got %q", path, result)
	}
}
