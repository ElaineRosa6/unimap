package screenshot

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/utils"
)

// loadPageContent is a CDP-only helper used by cookie/session validation.
func (m *Manager) loadPageContent(ctx context.Context, targetURL string, cookies []Cookie, title *string, html *string) error {
	allocCtx, allocCancel, err := m.newAllocator(ctx)
	if err != nil {
		return err
	}
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	actions := []chromedp.Action{}

	// 只有在非CDP模式且提供了Cookie时才设置Cookie
	// CDP模式下浏览器已保持登录状态，无需设置Cookie
	if len(cookies) > 0 && !m.isCDPMode() {
		actions = append(actions,
			chromedp.Navigate(targetURL),
			chromedp.ActionFunc(func(ctx context.Context) error {
				for _, cookie := range cookies {
					err := network.SetCookie(cookie.Name, cookie.Value).
						WithDomain(cookie.Domain).
						WithPath(cookie.Path).
						WithHTTPOnly(cookie.HTTPOnly).
						WithSecure(cookie.Secure).
						Do(ctx)
					if err != nil {
						logger.Warnf("Failed to set cookie %s: %v", cookie.Name, err)
					}
				}
				return nil
			}),
			chromedp.Navigate(targetURL),
		)
	} else {
		if m.isCDPMode() && len(cookies) > 0 {
			logger.Infof("Using CDP mode, skipping cookie setup (browser already logged in)")
		}
		actions = append(actions, chromedp.Navigate(targetURL))
	}

	actions = append(actions,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(m.waitTime),
		chromedp.Title(title),
		chromedp.OuterHTML("html", html, chromedp.ByQuery),
	)

	return chromedp.Run(ctx, actions...)
}

func (m *Manager) buildExecAllocatorOptions(proxyOverride string) []chromedp.ExecAllocatorOption {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", m.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.WindowSize(m.windowWidth, m.windowHeight),
	)

	if m.userDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(m.userDataDir))
	}
	if m.profileDir != "" {
		opts = append(opts, chromedp.Flag("profile-directory", m.profileDir))
	}

	proxyServer := strings.TrimSpace(proxyOverride)
	if proxyServer == "" {
		proxyServer = strings.TrimSpace(m.proxyServer)
	}
	if proxyServer == "" {
		proxyServer = strings.TrimSpace(os.Getenv("UNIMAP_CHROME_PROXY_SERVER"))
	}
	if proxyServer != "" {
		opts = append(opts, chromedp.Flag("proxy-server", proxyServer))
		logger.Infof("Chrome proxy enabled: %s", proxyServer)
	}

	// 确定Chrome路径
	chromePath := m.chromePath
	if chromePath == "" {
		chromePath = os.Getenv("UNIMAP_CHROME_PATH")
	}
	if chromePath == "" {
		chromePath = findChromePath()
	}
	if chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	if userData := os.Getenv("UNIMAP_CHROME_USER_DATA_DIR"); userData != "" && m.userDataDir == "" {
		opts = append(opts, chromedp.UserDataDir(userData))
	}
	if profileDir := os.Getenv("UNIMAP_CHROME_PROFILE_DIR"); profileDir != "" && m.profileDir == "" {
		opts = append(opts, chromedp.Flag("profile-directory", profileDir))
	}

	return opts
}

// findChromePath 自动查找Chrome路径
func findChromePath() string {
	var candidates []string

	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
		if homeDir, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates,
				filepath.Join(homeDir, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"),
			)
		}
	case "linux":
		candidates = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome-beta",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/opt/google/chrome/chrome",
		}
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			logger.Infof("Found Chrome at: %s", path)
			return path
		}
	}

	return ""
}

func (m *Manager) newAllocator(ctx context.Context) (context.Context, context.CancelFunc, error) {
	return m.newAllocatorWithProxy(ctx, "")
}

func (m *Manager) newAllocatorWithProxy(ctx context.Context, proxyOverride string) (context.Context, context.CancelFunc, error) {
	// 检查是否配置了远程调试URL
	remoteURL := strings.TrimSpace(m.remoteDebugURL)
	if remoteURL == "" {
		remoteURL = strings.TrimSpace(os.Getenv("UNIMAP_CHROME_REMOTE_DEBUG_URL"))
	}

	// 如果配置了远程调试URL，先尝试连接，失败则回退到本地启动
	if remoteURL != "" {
		// 测试远程调试端口是否可用
		if isRemoteDebuggerAvailable(remoteURL) {
			logger.Infof("Using remote Chrome debugger at: %s", remoteURL)
			allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, remoteURL)
			return allocCtx, cancel, nil
		}
		logger.Warnf("Remote Chrome debugger not available at %s, falling back to local Chrome", remoteURL)
	}

	// 使用本地Chrome启动allocator
	opts := m.buildExecAllocatorOptions(proxyOverride)

	// 确保有可用的Chrome路径
	chromePath := findChromePath()
	if chromePath == "" && os.Getenv("UNIMAP_CHROME_PATH") == "" {
		return nil, nil, fmt.Errorf("chrome not found; please install Chrome or set UNIMAP_CHROME_PATH environment variable")
	}

	logger.Infof("Starting Chrome with options, chrome path: %s", chromePath)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	return allocCtx, cancel, nil
}

// NewAllocator exposes the browser allocator so other browser-driven features
// can share the same Chrome/CDP bootstrap strategy as screenshots.
func (m *Manager) NewAllocator(ctx context.Context) (context.Context, context.CancelFunc, error) {
	return m.newAllocator(ctx)
}

// NewAllocatorWithProxy creates a browser allocator with request-level proxy override.
func (m *Manager) NewAllocatorWithProxy(ctx context.Context, proxy string) (context.Context, context.CancelFunc, error) {
	return m.newAllocatorWithProxy(ctx, proxy)
}

// isRemoteDebuggerAvailable 检查远程调试端口是否可用
func isRemoteDebuggerAvailable(remoteURL string) bool {
	client := utils.FastHTTPClient()
	resp, err := client.Get(remoteURL + "/json/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

