package tamper

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/utils"
)

type segmentTask struct {
	name     string
	hashFunc func() SegmentHash
}

// SetAllocator sets the chromedp allocator context for the detector.
func (d *Detector) SetAllocator(ctx context.Context, allocCtx context.Context, allocCancel context.CancelFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.allocCtx = allocCtx
	d.allocCancel = allocCancel
}

// ComputePageHash computes a page hash using the configured performance mode.
func (d *Detector) ComputePageHash(ctx context.Context, targetURL string) (*PageHashResult, error) {
	cacheKey := fmt.Sprintf("%s:%s", targetURL, d.performanceMode)

	d.cacheMu.RLock()
	if entry, exists := d.cache[cacheKey]; exists {
		if time.Since(entry.timestamp) < 5*time.Minute {
			d.cacheMu.RUnlock()
			logger.CtxDebugf(ctx, "Using cached hash result for %s", targetURL)
			return entry.result, nil
		}
	}
	d.cacheMu.RUnlock()

	var html string
	var title string

	if d.performanceMode == PerformanceModeFast {
		result, err := d.computeHashWithHTTP(ctx, targetURL)
		if err == nil {
			d.runPortScanIfEnabled(ctx, targetURL, result)
			d.cacheMu.Lock()
			d.cache[cacheKey] = &cacheEntry{result: result, timestamp: time.Now()}
			d.cacheMu.Unlock()
			return result, nil
		}
		logger.CtxWarnf(ctx, "Fast mode failed, falling back to chromedp: %v", err)
	}

	runCtx := ctx
	runCancel := func() {}
	if chromedp.FromContext(runCtx) == nil {
		d.mu.Lock()
		allocCtx := d.allocCtx
		d.mu.Unlock()
		if allocCtx == nil {
			allocCtx = context.Background()
		}
		runCtx, runCancel = chromedp.NewContext(allocCtx)
	}
	defer runCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(runCtx, 45*time.Second)
	defer timeoutCancel()

	if err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("failed to load page: %w", err)
	}

	result, err := d.ComputeHashFromHTML(targetURL, title, html, nil) // chromedp 模式不传 headers
	if err == nil {
		d.runPortScanIfEnabled(ctx, targetURL, result)
		d.cacheMu.Lock()
		d.cache[cacheKey] = &cacheEntry{result: result, timestamp: time.Now()}
		d.cacheMu.Unlock()
	}

	return result, err
}

func (d *Detector) computeHashWithHTTP(ctx context.Context, targetURL string) (*PageHashResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 记录重定向链
	var redirectURLs []string

	// Create an independent client to avoid mutating the shared DefaultHTTPClient.
	// Cloning the transport and setting CheckRedirect on a fresh client prevents
	// data races with concurrent tamper checks.
	baseTransport := utils.DefaultHTTPClient().Transport.(*http.Transport).Clone()
	if d.insecureSkipVerify {
		baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: baseTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			redirectURLs = append(redirectURLs, req.URL.String())
			return nil
		},
	}

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	html := string(bodyBytes)
	finalURL := resp.Request.URL.String()

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		metrics.IncBrowserDOMParseFailure("tamper")
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	title := doc.Find("title").Text()
	if title == "" {
		title = targetURL
	}

	// 收集 HTTP 响应头用于指纹识别
	httpHeaders := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		httpHeaders[k] = resp.Header.Get(k)
	}

	result, err := d.ComputeHashFromHTML(targetURL, title, html, httpHeaders)
	if err != nil {
		return nil, err
	}
	result.FinalURL = finalURL
	result.RedirectURLs = redirectURLs
	return result, nil
}

// ComputeHashFromHTML computes a page hash from raw HTML content.
// httpHeaders 可选 — HTTP 模式传入响应头用于指纹识别，chromedp 模式可传 nil。
func (d *Detector) ComputeHashFromHTML(url, title, html string, httpHeaders map[string]string) (*PageHashResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		metrics.IncBrowserDOMParseFailure("tamper")
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	result := &PageHashResult{
		URL:         url,
		Title:       title,
		Timestamp:   time.Now().Unix(),
		HTMLLength:  len(html),
		Status:      "success",
		HTTPHeaders: httpHeaders,
	}

	if len(html) > 4096 {
		result.RawHTML = html[:4096] + "... [truncated]"
	} else {
		result.RawHTML = html
	}

	segmentHashes := d.computeSegmentHashes(doc, html)
	result.SegmentHashes = segmentHashes

	result.FullHash = d.computeFullHash(segmentHashes)
	result.SimpleMD5Hash = computeSimpleMD5Hash(html)

	// 计算规范化 HTTP 指纹
	if httpHeaders != nil {
		result.NormalizedHTTPFingerprint = computeNormalizedHTTPFingerprint(
			"HTTP/1.1 200 OK", httpHeaders, result.SimpleMD5Hash)
	}

	// 运行指纹识别引擎
	if d.fpEngine != nil && httpHeaders != nil {
		body := strings.ToLower(html)
		setCookie := httpHeaders["Set-Cookie"]
		result.Fingerprints = d.fpEngine.Match(httpHeaders, body, title, setCookie)
	}

	return result, nil
}

func computeSimpleMD5Hash(html string) string {
	headerEnd := strings.Index(strings.ToLower(html), "</head>")
	if headerEnd == -1 {
		headerEnd = strings.Index(html, "<body")
		if headerEnd == -1 {
			headerEnd = len(html) / 2
		}
	}

	headerPart := html[:headerEnd]
	bodyPart := html[headerEnd:]

	combined := headerPart + "\r\n\r\n" + bodyPart
	hash := md5.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func (d *Detector) computeSegmentHashes(doc *goquery.Document, html string) []SegmentHash {
	var tasks []segmentTask

	switch d.performanceMode {
	case PerformanceModeFast:
		tasks = []segmentTask{
			{name: SegmentScripts, hashFunc: func() SegmentHash { return d.computeScriptHash(doc) }},
			{name: SegmentJSFiles, hashFunc: func() SegmentHash { return d.computeJSFileHash(doc) }},
			{name: SegmentForms, hashFunc: func() SegmentHash { return d.computeFormHash(doc) }},
			{name: SegmentMain, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "main", SegmentMain) }},
			{name: SegmentArticle, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "article", SegmentArticle) }},
		}

	case PerformanceModeBalanced:
		tasks = []segmentTask{
			{name: SegmentScripts, hashFunc: func() SegmentHash { return d.computeScriptHash(doc) }},
			{name: SegmentJSFiles, hashFunc: func() SegmentHash { return d.computeJSFileHash(doc) }},
			{name: SegmentForms, hashFunc: func() SegmentHash { return d.computeFormHash(doc) }},
			{name: SegmentLinks, hashFunc: func() SegmentHash { return d.computeLinkHash(doc) }},
			{name: SegmentMain, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "main", SegmentMain) }},
			{name: SegmentArticle, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "article", SegmentArticle) }},
			{name: SegmentBody, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "body", SegmentBody) }},
		}

	default:
		tasks = []segmentTask{
			{name: SegmentHead, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "head", SegmentHead) }},
			{name: SegmentBody, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "body", SegmentBody) }},
			{name: SegmentHeader, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "header", SegmentHeader) }},
			{name: SegmentNav, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "nav", SegmentNav) }},
			{name: SegmentMain, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "main", SegmentMain) }},
			{name: SegmentArticle, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "article", SegmentArticle) }},
			{name: SegmentSection, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "section", SegmentSection) }},
			{name: SegmentAside, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "aside", SegmentAside) }},
			{name: SegmentFooter, hashFunc: func() SegmentHash { return d.computeElementHash(doc, "footer", SegmentFooter) }},
			{name: SegmentScripts, hashFunc: func() SegmentHash { return d.computeScriptHash(doc) }},
			{name: SegmentJSFiles, hashFunc: func() SegmentHash { return d.computeJSFileHash(doc) }},
			{name: SegmentStyles, hashFunc: func() SegmentHash { return d.computeStyleHash(doc) }},
			{name: SegmentMeta, hashFunc: func() SegmentHash { return d.computeMetaHash(doc) }},
			{name: SegmentFavicon, hashFunc: func() SegmentHash { return d.computeFaviconHash(doc) }},
			{name: SegmentLinks, hashFunc: func() SegmentHash { return d.computeLinkHash(doc) }},
			{name: SegmentImages, hashFunc: func() SegmentHash { return d.computeImageHash(doc) }},
			{name: SegmentButtons, hashFunc: func() SegmentHash { return d.computeButtonHash(doc) }},
			{name: SegmentForms, hashFunc: func() SegmentHash { return d.computeFormHash(doc) }},
		}
	}

	resultChan := make(chan SegmentHash, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t segmentTask) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("computeSegmentHashes task %s panicked: %v", t.name, r)
				}
			}()
			resultChan <- t.hashFunc()
		}(task)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var segments []SegmentHash
	for segment := range resultChan {
		segments = append(segments, segment)
	}

	if d.performanceMode == PerformanceModeComprehensive {
		cleanHTML := d.cleanHTML(html)
		fullContentHash := SegmentHash{
			Name:     SegmentFullContent,
			Hash:     computeSHA256(cleanHTML),
			Length:   len(cleanHTML),
			Elements: 1,
		}
		segments = append(segments, fullContentHash)
	}

	return segments
}

func (d *Detector) computeElementHash(doc *goquery.Document, selector, segmentName string) SegmentHash {
	selection := doc.Find(selector)
	content, _ := selection.Html()

	cleanContent := d.cleanHTML(content)
	hash := computeSHA256(cleanContent)

	elementCount := selection.Length()

	return SegmentHash{
		Name:     segmentName,
		Hash:     hash,
		Length:   len(cleanContent),
		Elements: elementCount,
	}
}

func (d *Detector) computeScriptHash(doc *goquery.Document) SegmentHash {
	var scripts []string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		src = normalizeAssetURL(src)
		integrity, _ := s.Attr("integrity")
		async, _ := s.Attr("async")
		deferAttr, _ := s.Attr("defer")
		content := normalizeInlineScript(s.Text())
		scripts = append(scripts, strings.Join([]string{src, integrity, async, deferAttr, content}, ":"))
	})

	sort.Strings(scripts)
	combined := strings.Join(scripts, "|")

	return SegmentHash{
		Name:     SegmentScripts,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(scripts),
	}
}

func (d *Detector) computeJSFileHash(doc *goquery.Document) SegmentHash {
	var jsFiles []string
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		src = normalizeAssetURL(src)
		integrity, _ := s.Attr("integrity")
		crossorigin, _ := s.Attr("crossorigin")
		referrerpolicy, _ := s.Attr("referrerpolicy")
		jsFiles = append(jsFiles, strings.Join([]string{src, integrity, crossorigin, referrerpolicy}, ":"))
	})

	sort.Strings(jsFiles)
	combined := strings.Join(jsFiles, "|")

	return SegmentHash{
		Name:     SegmentJSFiles,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(jsFiles),
	}
}

func (d *Detector) computeStyleHash(doc *goquery.Document) SegmentHash {
	var styles []string
	doc.Find("style").Each(func(i int, s *goquery.Selection) {
		styles = append(styles, s.Text())
	})
	doc.Find("link[rel='stylesheet']").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = normalizeAssetURL(href)
		styles = append(styles, href)
	})

	sort.Strings(styles)
	combined := strings.Join(styles, "|")

	return SegmentHash{
		Name:     SegmentStyles,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(styles),
	}
}

func (d *Detector) computeMetaHash(doc *goquery.Document) SegmentHash {
	var metas []string
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		content, _ := s.Attr("content")
		property, _ := s.Attr("property")
		metas = append(metas, fmt.Sprintf("%s:%s:%s", name, property, content))
	})

	sort.Strings(metas)
	combined := strings.Join(metas, "|")

	return SegmentHash{
		Name:     SegmentMeta,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(metas),
	}
}

func (d *Detector) computeFaviconHash(doc *goquery.Document) SegmentHash {
	var icons []string
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		rel, _ := s.Attr("rel")
		relLower := strings.ToLower(rel)
		if !strings.Contains(relLower, "icon") {
			return
		}
		href, _ := s.Attr("href")
		href = normalizeAssetURL(href)
		typ, _ := s.Attr("type")
		sizes, _ := s.Attr("sizes")
		icons = append(icons, strings.Join([]string{relLower, href, typ, sizes}, ":"))
	})

	sort.Strings(icons)
	combined := strings.Join(icons, "|")

	return SegmentHash{
		Name:     SegmentFavicon,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(icons),
	}
}

func (d *Detector) computeLinkHash(doc *goquery.Document) SegmentHash {
	var links []string
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := s.Text()
		links = append(links, fmt.Sprintf("%s:%s", href, text))
	})

	sort.Strings(links)
	combined := strings.Join(links, "|")

	return SegmentHash{
		Name:     SegmentLinks,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(links),
	}
}

func (d *Detector) computeImageHash(doc *goquery.Document) SegmentHash {
	var images []string
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		srcset, _ := s.Attr("srcset")
		alt, _ := s.Attr("alt")
		width, _ := s.Attr("width")
		height, _ := s.Attr("height")
		loading, _ := s.Attr("loading")
		decoding, _ := s.Attr("decoding")
		images = append(images, strings.Join([]string{src, srcset, alt, width, height, loading, decoding}, ":"))
	})

	sort.Strings(images)
	combined := strings.Join(images, "|")

	return SegmentHash{
		Name:     SegmentImages,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(images),
	}
}

func (d *Detector) computeButtonHash(doc *goquery.Document) SegmentHash {
	var buttons []string

	doc.Find("button").Each(func(i int, s *goquery.Selection) {
		typ, _ := s.Attr("type")
		id, _ := s.Attr("id")
		class, _ := s.Attr("class")
		name, _ := s.Attr("name")
		ariaLabel, _ := s.Attr("aria-label")
		text := strings.TrimSpace(s.Text())
		buttons = append(buttons, strings.Join([]string{"button", typ, id, class, name, ariaLabel, text}, ":"))
	})

	doc.Find("input[type='button'], input[type='submit'], input[type='reset']").Each(func(i int, s *goquery.Selection) {
		typ, _ := s.Attr("type")
		id, _ := s.Attr("id")
		class, _ := s.Attr("class")
		name, _ := s.Attr("name")
		value, _ := s.Attr("value")
		buttons = append(buttons, strings.Join([]string{"input", typ, id, class, name, value}, ":"))
	})

	doc.Find("a[role='button']").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		id, _ := s.Attr("id")
		class, _ := s.Attr("class")
		ariaLabel, _ := s.Attr("aria-label")
		text := strings.TrimSpace(s.Text())
		buttons = append(buttons, strings.Join([]string{"anchor", href, id, class, ariaLabel, text}, ":"))
	})

	sort.Strings(buttons)
	combined := strings.Join(buttons, "|")

	return SegmentHash{
		Name:     SegmentButtons,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(buttons),
	}
}

func (d *Detector) computeFormHash(doc *goquery.Document) SegmentHash {
	var forms []string
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		action, _ := s.Attr("action")
		method, _ := s.Attr("method")
		forms = append(forms, fmt.Sprintf("%s:%s", action, method))

		s.Find("input, select, textarea").Each(func(i int, field *goquery.Selection) {
			name, _ := field.Attr("name")
			inputType, _ := field.Attr("type")
			forms = append(forms, fmt.Sprintf("field:%s:%s", name, inputType))
		})
	})

	sort.Strings(forms)
	combined := strings.Join(forms, "|")

	return SegmentHash{
		Name:     SegmentForms,
		Hash:     computeSHA256(combined),
		Length:   len(combined),
		Elements: len(forms),
	}
}

func (d *Detector) computeFullHash(segments []SegmentHash) string {
	var hashes []string
	for _, seg := range segments {
		hashes = append(hashes, fmt.Sprintf("%s:%s", seg.Name, seg.Hash))
	}
	sort.Strings(hashes)
	return computeSHA256(strings.Join(hashes, "|"))
}

func (d *Detector) cleanHTML(html string) string {
	if html == "" {
		return ""
	}

	html = reMultipleSpaces.ReplaceAllString(html, " ")
	html = reComments.ReplaceAllString(html, "")
	html = reDataImages.ReplaceAllString(html, "DATA_IMAGE_REMOVED")
	html = reNonce.ReplaceAllString(html, "")
	html = reCSRFToken.ReplaceAllString(html, "")
	html = reTimestamp.ReplaceAllString(html, "TIMESTAMP")
	html = reUUID.ReplaceAllString(html, "UUID")
	html = strings.TrimSpace(html)

	return html
}

// normalizeAssetURL 归一化版本化文件名和 cache-busting 参数
func normalizeAssetURL(raw string) string {
	raw = reVersionedFile.ReplaceAllString(raw, "HASH.$2")
	raw = reCacheBust.ReplaceAllString(raw, "")
	return raw
}

// normalizeInlineScript 剥离 SSR 水合数据和分析/统计脚本
func normalizeInlineScript(content string) string {
	content = reSSRHydration.ReplaceAllString(content, "__SSR_HYDRATION__")
	content = reAnalyticsScript.ReplaceAllString(content, "__ANALYTICS__")
	return content
}

// computeNormalizedHTTPFingerprint 计算规范化 HTTP 指纹
//
// 这不是 urlive.py 的原始响应 MD5。这是规范化版本：
//
//	MD5(状态行 + 排序规范化头 + 空行 + SimpleMD5Hash)
//
// 移除 Date/Age/Expires 等易变头，确保同一页面两次请求的指纹一致。
func computeNormalizedHTTPFingerprint(statusLine string, headers map[string]string, bodyHash string) string {
	if headers == nil {
		return ""
	}

	// 收集排序后的规范头（排除易变头）
	var normalized []string
	for k, v := range headers {
		if _, skip := volatileHTTPHeaders[k]; skip {
			continue
		}
		normalized = append(normalized, k+": "+v)
	}
	sort.Strings(normalized)

	combined := statusLine + "\r\n" + strings.Join(normalized, "\r\n") + "\r\n\r\n" + bodyHash
	hash := md5.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func computeSHA256(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
