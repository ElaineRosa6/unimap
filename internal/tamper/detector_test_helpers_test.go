package tamper

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// allowLoopbackTestPageLoader is only for httptest fixtures. Production
// callers receive the screenshot Manager's guarded browser implementation.
func allowLoopbackTestPageLoader(d *Detector) {
	d.SetBrowserPageLoader(BrowserPageLoaderFunc(func(ctx context.Context, targetURL string) (string, string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return "", "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", "", fmt.Errorf("fixture returned HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		return "", string(body), err
	}))
}
