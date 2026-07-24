//go:build manual_browser_test
// +build manual_browser_test

// Command chrome-speedtest manually verifies Chrome startup and navigation
// latency. The build tag keeps it out of normal build and test commands.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

const defaultTimeout = 30 * time.Second

func main() {
	timeout := flag.Duration("timeout", defaultTimeout, "maximum time allowed per URL")
	flag.Parse()

	urls := flag.Args()
	if len(urls) == 0 {
		urls = []string{
			"https://www.example.com",
			"https://www.baidu.com",
			"https://www.bing.com",
		}
	}

	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	exitCode := 0
	for _, targetURL := range urls {
		startedAt := time.Now()
		navigationCtx, cancelNavigation := context.WithTimeout(browserCtx, *timeout)
		var title string
		err := chromedp.Run(navigationCtx,
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Title(&title),
		)
		cancelNavigation()

		if err != nil {
			exitCode = 1
			fmt.Fprintf(os.Stderr, "FAIL %s (%s): %v\n", targetURL, time.Since(startedAt).Round(time.Millisecond), err)
			continue
		}
		fmt.Printf("OK   %s (%s): %s\n", targetURL, time.Since(startedAt).Round(time.Millisecond), title)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
