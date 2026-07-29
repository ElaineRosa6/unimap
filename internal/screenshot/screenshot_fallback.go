package screenshot

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"image/png"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/unimap/project/internal/logger"
)

// captureScreenshotWithFallback tries PNG first with a short timeout, then
// falls back to JPEG. Some complex SPA pages cause Chrome PNG compositor to
// hang indefinitely while JPEG encoding succeeds.
func captureScreenshotWithFallback(browserCtx context.Context, buf *[]byte) error {
	pngCtx, pngCancel := context.WithTimeout(browserCtx, 10*time.Second)
	defer pngCancel()
	if err := chromedp.Run(pngCtx, chromedp.CaptureScreenshot(buf)); err == nil {
		return nil
	}

	logger.Warnf("PNG screenshot timed out, falling back to JPEG capture with PNG transcoding")
	jpgCtx, jpgCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer jpgCancel()
	var jpegBytes []byte
	if err := chromedp.Run(jpgCtx, chromedp.ActionFunc(func(c context.Context) error {
		var err error
		jpegBytes, err = page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatJpeg).
			WithQuality(85).
			WithCaptureBeyondViewport(false).
			Do(c)
		return err
	})); err != nil {
		return err
	}
	pngBytes, err := transcodeJPEGToPNG(jpegBytes)
	if err != nil {
		return err
	}
	*buf = pngBytes
	return nil
}

func transcodeJPEGToPNG(jpegBytes []byte) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		return nil, fmt.Errorf("decode JPEG screenshot fallback: %w", err)
	}
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, img); err != nil {
		return nil, fmt.Errorf("encode PNG screenshot fallback: %w", err)
	}
	return pngBytes.Bytes(), nil
}
