package screenshot

import (
	"context"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/logger"
)

func setCookieActions(cookies []Cookie, targetURL string) []chromedp.Action {
	if len(cookies) == 0 {
		return nil
	}
	targetURL = strings.TrimSpace(targetURL)
	return []chromedp.Action{
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, cookie := range cookies {
				name := strings.TrimSpace(cookie.Name)
				if name == "" {
					continue
				}
				path := strings.TrimSpace(cookie.Path)
				if path == "" {
					path = "/"
				}
				params := network.SetCookie(name, cookie.Value).
					WithPath(path).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSecure(cookie.Secure)
				if targetURL != "" {
					params = params.WithURL(targetURL)
				}
				if domain := strings.TrimSpace(cookie.Domain); domain != "" {
					params = params.WithDomain(domain)
				}
				if err := params.Do(ctx); err != nil {
					logger.Warnf("Failed to set cookie %s: %v", name, err)
				}
			}
			return nil
		}),
	}
}
