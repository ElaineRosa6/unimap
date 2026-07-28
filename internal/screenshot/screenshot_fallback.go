package screenshot

import (
	"context"
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

	logger.Warnf("PNG screenshot timed out, falling back to JPEG")
	jpgCtx, jpgCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer jpgCancel()
	return chromedp.Run(jpgCtx, chromedp.ActionFunc(func(c context.Context) error {
		var e error
		*buf, e = page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatJpeg).
			WithQuality(85).
			WithCaptureBeyondViewport(false).
			Do(c)
		return e
	}))
}