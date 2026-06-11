package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/screenshot"
)

func normalizeBridgeTargetURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "http://" + trimmed
	}
	return trimmed
}

func buildSearchEngineResultURL(engine, query string) string {
	b64Query := base64.StdEncoding.EncodeToString([]byte(query))
	encodedB64 := url.QueryEscape(b64Query)
	encodedQuery := url.QueryEscape(query)

	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "fofa":
		return fmt.Sprintf("%s/result?qbase64=%s", model.FOFAOfficialWebURL, encodedB64)
	case "hunter":
		return fmt.Sprintf("https://hunter.qianxin.com/list?search=%s", encodedB64)
	case "quake":
		return fmt.Sprintf("https://quake.360.net/quake/#/searchResult?search=%s", encodedQuery)
	case "zoomeye":
		return fmt.Sprintf("https://www.zoomeye.hk/searchResult?q=%s", encodedQuery)
	default:
		return ""
	}
}

func (s *ScreenshotAppService) captureSearchEngineWithBridge(ctx context.Context, mgr *screenshot.Manager, engine, query, queryID string) (string, error) {
	if s == nil {
		metrics.IncBridgeRequest("extension", "service_unavailable")
		return "", fmt.Errorf("bridge service not initialized")
	}
	cfg := s.configSnapshot()
	if cfg.bridgeService == nil {
		metrics.IncBridgeRequest("extension", "service_unavailable")
		return "", fmt.Errorf("bridge service not initialized")
	}
	startedAt := time.Now()

	searchURL := ""
	if mgr != nil {
		searchURL = strings.TrimSpace(mgr.BuildSearchEngineURL(engine, query))
	}
	if searchURL == "" {
		searchURL = buildSearchEngineResultURL(engine, query)
	}
	if searchURL == "" {
		metrics.IncBridgeRequest("extension", "unsupported_engine")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", fmt.Errorf("unsupported engine: %s", engine)
	}

	task := screenshot.BridgeTask{
		RequestID:    fmt.Sprintf("bridge_search_%d", time.Now().UnixNano()),
		URL:          searchURL,
		BatchID:      queryID,
		WaitStrategy: "load",
	}
	result, err := cfg.bridgeService.Submit(ctx, task)
	if err != nil {
		metrics.IncBridgeRequest("extension", "submit_failed")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", err
	}
	if !result.Success {
		metrics.IncBridgeRequest("extension", "result_failed")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		if strings.TrimSpace(result.Error) != "" {
			return "", fmt.Errorf("bridge capture failed: %s", strings.TrimSpace(result.Error))
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			return "", fmt.Errorf("bridge capture failed: %s", strings.TrimSpace(result.ErrorCode))
		}
		return "", fmt.Errorf("bridge capture failed")
	}
	if strings.TrimSpace(result.ImagePath) == "" {
		metrics.IncBridgeRequest("extension", "missing_image_path")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", fmt.Errorf("bridge capture missing image path")
	}
	metrics.IncBridgeRequest("extension", "success")
	metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
	return strings.TrimSpace(result.ImagePath), nil
}

func buildTargetCaptureURL(targetURL, ip, port, protocol string) (string, error) {
	resolvedURL := strings.TrimSpace(targetURL)
	resolvedIP := strings.TrimSpace(ip)
	resolvedPort := strings.TrimSpace(port)
	resolvedProto := strings.TrimSpace(protocol)

	if resolvedURL == "" {
		if resolvedIP == "" {
			return "", fmt.Errorf("target URL or IP is required")
		}
		proto := "http"
		if resolvedProto != "" {
			proto = strings.ToLower(resolvedProto)
		} else if resolvedPort == "443" {
			proto = "https"
		}

		if resolvedPort != "" && resolvedPort != "80" && resolvedPort != "443" {
			resolvedURL = fmt.Sprintf("%s://%s:%s", proto, resolvedIP, resolvedPort)
		} else {
			resolvedURL = fmt.Sprintf("%s://%s", proto, resolvedIP)
		}
	}

	if !strings.HasPrefix(resolvedURL, "http://") && !strings.HasPrefix(resolvedURL, "https://") {
		resolvedURL = "http://" + resolvedURL
	}

	return resolvedURL, nil
}

func (s *ScreenshotAppService) captureTargetWithBridge(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	if s == nil {
		metrics.IncBridgeRequest("extension", "service_unavailable")
		return "", fmt.Errorf("bridge service not initialized")
	}
	cfg := s.configSnapshot()
	if cfg.bridgeService == nil {
		metrics.IncBridgeRequest("extension", "service_unavailable")
		return "", fmt.Errorf("bridge service not initialized")
	}
	startedAt := time.Now()

	resolvedURL, err := buildTargetCaptureURL(targetURL, ip, port, protocol)
	if err != nil {
		metrics.IncBridgeRequest("extension", "invalid_target")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", err
	}

	task := screenshot.BridgeTask{
		RequestID:    fmt.Sprintf("bridge_target_%d", time.Now().UnixNano()),
		URL:          resolvedURL,
		BatchID:      queryID,
		WaitStrategy: "load",
	}
	result, err := cfg.bridgeService.Submit(ctx, task)
	if err != nil {
		metrics.IncBridgeRequest("extension", "submit_failed")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", err
	}
	if !result.Success {
		metrics.IncBridgeRequest("extension", "result_failed")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		if strings.TrimSpace(result.Error) != "" {
			return "", fmt.Errorf("bridge capture failed: %s", strings.TrimSpace(result.Error))
		}
		if strings.TrimSpace(result.ErrorCode) != "" {
			return "", fmt.Errorf("bridge capture failed: %s", strings.TrimSpace(result.ErrorCode))
		}
		return "", fmt.Errorf("bridge capture failed")
	}
	if strings.TrimSpace(result.ImagePath) == "" {
		metrics.IncBridgeRequest("extension", "missing_image_path")
		metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
		return "", fmt.Errorf("bridge capture missing image path")
	}
	metrics.IncBridgeRequest("extension", "success")
	metrics.ObserveBridgeDuration("extension", time.Since(startedAt))
	return strings.TrimSpace(result.ImagePath), nil
}

func (s *ScreenshotAppService) resolveProvider(mgr *screenshot.Manager) (screenshot.Provider, error) {
	if s != nil && s.provider != nil {
		return s.provider, nil
	}
	if mgr != nil {
		return screenshot.NewCDPProvider(mgr), nil
	}
	return nil, fmt.Errorf("screenshot manager not initialized")
}
