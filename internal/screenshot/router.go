package screenshot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/unimap/project/internal/logger"
)

// ScreenshotMode represents the active screenshot capture mode.
type ScreenshotMode string

const (
	ModeCDP       ScreenshotMode = "cdp"
	ModeExtension ScreenshotMode = "extension"
	ModeAuto      ScreenshotMode = "auto"
)

// RouterConfig holds the routing configuration.
type RouterConfig struct {
	Priority      ScreenshotMode // Primary mode to prefer
	Fallback      bool           // Whether to fall back to the other mode on failure
	ProbeInterval time.Duration  // How often to run health checks
	ProbeTimeout  time.Duration  // Timeout per probe
}

// ScreenshotRouter routes screenshot requests between CDP and Extension modes
// with automatic health-based failover.
type ScreenshotRouter struct {
	cfg       RouterConfig
	cdp       Provider
	extBridge *BridgeService
	mgr       *Manager

	cdpChecker HealthChecker
	extChecker HealthChecker

	// Current active mode (atomic for lock-free reads)
	currentMode atomic.Value // ScreenshotMode

	// Per-mode health status
	cdpHealthy atomic.Bool
	extHealthy atomic.Bool

	// Probe goroutine lifecycle
	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool

	// Metrics hooks
	onModeSwitch  func(from, to ScreenshotMode)
	onHealthCheck func(mode string, healthy bool)
}

// SetMetricsHooks registers callback functions for Prometheus metrics.
func (r *ScreenshotRouter) SetMetricsHooks(onModeSwitch func(from, to ScreenshotMode), onHealthCheck func(mode string, healthy bool)) {
	r.onModeSwitch = onModeSwitch
	r.onHealthCheck = onHealthCheck
}

// SetExtensionHealthSignals injects optional liveness and activity providers
// into the extension health checker. All parameters are optional — nil values
// are ignored and preserve the existing checker state.
func (r *ScreenshotRouter) SetExtensionHealthSignals(liveClient LiveClientProvider, lastActivity LastActivityProvider, cutoff time.Duration) {
	if r.extChecker == nil {
		return
	}
	if ext, ok := r.extChecker.(*ExtensionHealthChecker); ok {
		if liveClient != nil {
			ext.LiveClient = liveClient
		}
		if lastActivity != nil {
			ext.LastActivity = lastActivity
		}
		if cutoff > 0 {
			ext.RecentActivityCutoff = cutoff
		}
	}
}

// NewScreenshotRouter creates a new ScreenshotRouter.
func NewScreenshotRouter(cfg RouterConfig, cdp Provider, extBridge *BridgeService, mgr *Manager) *ScreenshotRouter {
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = 30 * time.Second
	}
	if cfg.ProbeTimeout <= 0 {
		cfg.ProbeTimeout = 5 * time.Second
	}

	r := &ScreenshotRouter{
		cfg:       cfg,
		cdp:       cdp,
		extBridge: extBridge,
		mgr:       mgr,
		stopCh:    make(chan struct{}),
	}

	// Initialize health checkers
	if cdp != nil {
		remoteURL := ""
		if mgr != nil {
			remoteURL = mgr.RemoteDebugURL()
		}
		r.cdpChecker = &CDPHealthChecker{RemoteDebugURL: remoteURL}
		r.cdpHealthy.Store(true) // CDP available
	} else {
		r.cdpChecker = &CDPHealthChecker{RemoteDebugURL: ""}
		r.cdpHealthy.Store(false)
	}

	r.extChecker = &ExtensionHealthChecker{BridgeService: extBridge}
	r.extHealthy.Store(extBridge != nil)

	// Set initial mode
	r.currentMode.Store(cfg.Priority)

	return r
}

// Start launches the health probe goroutine.
func (r *ScreenshotRouter) Start(ctx context.Context) {
	// Run initial probes synchronously
	r.runProbes(ctx)

	go r.probeLoop(ctx)
}

// Stop terminates the health probe goroutine.
func (r *ScreenshotRouter) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped {
		r.stopped = true
		close(r.stopCh)
	}
}

func (r *ScreenshotRouter) probeLoop(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Errorf("panic in screenshot probeLoop: %v", rec)
		}
	}()

	ticker := time.NewTicker(r.cfg.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.runProbes(ctx)
		}
	}
}

func (r *ScreenshotRouter) runProbes(ctx context.Context) {
	probeCtx, cancel := context.WithTimeout(ctx, r.cfg.ProbeTimeout)
	defer cancel()

	cdpOK, _ := r.cdpChecker.Check(probeCtx)
	r.cdpHealthy.Store(cdpOK)

	extOK, _ := r.extChecker.Check(probeCtx)
	r.extHealthy.Store(extOK)

	// Metrics
	if r.onHealthCheck != nil {
		if cdpOK {
			r.onHealthCheck("cdp", true)
		} else {
			r.onHealthCheck("cdp", false)
		}
		if extOK {
			r.onHealthCheck("extension", true)
		} else {
			r.onHealthCheck("extension", false)
		}
	}

	// Determine best mode
	current := r.loadMode()
	best := r.determineBestMode(current, cdpOK, extOK)
	if best != current {
		r.currentMode.Store(best)
		if r.onModeSwitch != nil {
			r.onModeSwitch(current, best)
		}
	}
}

func (r *ScreenshotRouter) determineBestMode(current ScreenshotMode, cdpOK, extOK bool) ScreenshotMode {
	// Auto mode: pick the healthiest provider
	if current == ModeAuto {
		if cdpOK && extOK {
			return ModeCDP // prefer CDP when both healthy
		}
		if cdpOK {
			return ModeCDP
		}
		if extOK {
			return ModeExtension
		}
		return ModeAuto // neither healthy, stay auto
	}

	// Forced mode: respect the configured mode, fall back only if configured
	switch current {
	case ModeCDP:
		if cdpOK {
			return ModeCDP
		}
		if r.cfg.Fallback && extOK {
			return ModeExtension
		}
		return ModeCDP
	case ModeExtension:
		if extOK {
			return ModeExtension
		}
		if r.cfg.Fallback && cdpOK {
			return ModeCDP
		}
		return ModeExtension
	}
	return current
}

// loadMode safely reads the current screenshot mode from atomic.Value.
func (r *ScreenshotRouter) loadMode() ScreenshotMode {
	v := r.currentMode.Load()
	if v == nil {
		return ModeAuto
	}
	mode, ok := v.(ScreenshotMode)
	if !ok {
		return ModeAuto
	}
	return mode
}

// ActiveMode returns the current active screenshot mode.
func (r *ScreenshotRouter) ActiveMode() ScreenshotMode {
	return r.loadMode()
}

// HealthStatus returns the health status of both modes.
func (r *ScreenshotRouter) HealthStatus() (cdpHealthy, extHealthy bool) {
	return r.cdpHealthy.Load(), r.extHealthy.Load()
}

// Config returns the router configuration.
func (r *ScreenshotRouter) Config() RouterConfig {
	return r.cfg
}

// SetMode sets the active screenshot execution mode.
// ModeAuto delegates to the health probe loop to pick the best mode.
// ModeCDP and ModeExtension are forced modes — they will not auto-switch
// unless fallback is enabled and the forced provider is unhealthy.
func (r *ScreenshotRouter) SetMode(mode ScreenshotMode) {
	if mode != ModeCDP && mode != ModeExtension && mode != ModeAuto {
		return
	}
	old := r.loadMode()
	if old == mode {
		return
	}
	r.currentMode.Store(mode)
	if r.onModeSwitch != nil {
		r.onModeSwitch(old, mode)
	}
}

// CurrentMode returns the active screenshot execution mode.
func (r *ScreenshotRouter) CurrentMode() ScreenshotMode {
	return r.loadMode()
}

// CaptureSearchEngineResult captures a search engine result using the active mode.
func (r *ScreenshotRouter) CaptureSearchEngineResult(ctx context.Context, engine, query, queryID string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.CaptureSearchEngineResult(ctx, engine, query, queryID)
}

// CaptureTargetWebsite captures a target website using the active mode.
func (r *ScreenshotRouter) CaptureTargetWebsite(ctx context.Context, targetURL, ip, port, protocol, queryID string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.CaptureTargetWebsite(ctx, targetURL, ip, port, protocol, queryID)
}

// CaptureBatchURLs captures a batch of URLs using the active mode.
func (r *ScreenshotRouter) CaptureBatchURLs(ctx context.Context, urls []string, batchID string, concurrency int) ([]BatchScreenshotResult, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return nil, err
	}
	return provider.CaptureBatchURLs(ctx, urls, batchID, concurrency)
}

// GetScreenshotDirectory returns the screenshot base directory.
func (r *ScreenshotRouter) GetScreenshotDirectory() string {
	if r.mgr != nil {
		return r.mgr.GetScreenshotDirectory()
	}
	return ""
}

// OpenSearchEngineResult opens a search engine result page using the active mode.
func (r *ScreenshotRouter) OpenSearchEngineResult(ctx context.Context, engine, query string) (string, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return "", err
	}
	return provider.OpenSearchEngineResult(ctx, engine, query)
}

// CollectSearchEngineResult collects structured data from a search engine result using the active mode.
func (r *ScreenshotRouter) CollectSearchEngineResult(ctx context.Context, engine, query, queryID string) ([]CollectResult, error) {
	provider, err := r.resolveProvider(r.loadMode())
	if err != nil {
		return nil, err
	}
	return provider.CollectSearchEngineResult(ctx, engine, query, queryID)
}

// resolveProvider returns the best available Provider based on current health and fallback config.
func (r *ScreenshotRouter) resolveProvider(primaryMode ScreenshotMode) (Provider, error) {
	mode := r.determineBestMode(primaryMode, r.cdpHealthy.Load(), r.extHealthy.Load())

	// Try the determined mode first
	if provider := r.providerForMode(mode); provider != nil {
		return provider, nil
	}

	// Fallback to the other mode
	other := ModeCDP
	if mode == ModeCDP {
		other = ModeExtension
	}
	if r.cfg.Fallback {
		if provider := r.providerForMode(other); provider != nil {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no screenshot provider available (cdp=%v, extension=%v, mode=%s, fallback=%v)",
		r.cdp != nil, r.extBridge != nil, mode, r.cfg.Fallback)
}

// providerForMode returns the Provider for the given mode, or nil if unavailable.
func (r *ScreenshotRouter) providerForMode(mode ScreenshotMode) Provider {
	switch mode {
	case ModeCDP:
		return r.cdp
	case ModeExtension:
		if r.extBridge == nil {
			return nil
		}
		return NewExtensionProvider(r.extBridge, r.mgr)
	default:
		return nil
	}
}

