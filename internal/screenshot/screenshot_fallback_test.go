package screenshot

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestTranscodeJPEGToPNGReturnsPNGBytes(t *testing.T) {
	var jpegBytes bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := jpeg.Encode(&jpegBytes, source, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode fixture JPEG: %v", err)
	}

	got, err := transcodeJPEGToPNG(jpegBytes.Bytes())
	if err != nil {
		t.Fatalf("transcode JPEG: %v", err)
	}
	if !bytes.HasPrefix(got, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("result magic = %x, want PNG", got[:min(len(got), 8)])
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Fatalf("decode transcoded PNG: %v", err)
	}
}

func TestTranscodeJPEGToPNGRejectsInvalidInput(t *testing.T) {
	if _, err := transcodeJPEGToPNG([]byte("not-a-jpeg")); err == nil {
		t.Fatal("invalid JPEG input must fail")
	}
}
