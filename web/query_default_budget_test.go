package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/adapter"
	"github.com/unimap/project/internal/collection"
	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/service"
)

type queryBudgetProvider struct {
	successfulScreenshotProvider
	budgets chan time.Duration
}

func (p *queryBudgetProvider) CollectSearchEngineResult(ctx context.Context, _, _, _ string) ([]collection.CollectResult, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		p.budgets <- 0
	} else {
		p.budgets <- time.Until(deadline)
	}
	return nil, errors.New("fixture collection completed with error")
}

func TestHTTPQueryDefaultBrowserBudget(t *testing.T) {
	for _, action := range []string{"", "  ", "collect_and_capture", "collect"} {
		t.Run("action="+action, func(t *testing.T) {
			provider := &queryBudgetProvider{budgets: make(chan time.Duration, 1)}
			router := screenshot.NewScreenshotRouter(screenshot.RouterConfig{Priority: screenshot.ModeCDP}, provider, nil, nil)
			s := &Server{queryApp: service.NewQueryAppService(nil, adapter.NewEngineOrchestrator()), screenshotRouter: router}
			form := url.Values{"query": {`port="443"`}, "engines": {"fofa"}, "browser_query": {"true"}, "browser_action": {action}}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "http://localhost:8448")
			w := httptest.NewRecorder()
			s.handleAPIQuery(w, req)
			select {
			case got := <-provider.budgets:
				want := service.BrowserCollectAndCaptureWaitTimeout
				if action == "collect" {
					want = service.BrowserQueryWaitTimeout
				}
				if got > want || got < want-5*time.Second {
					t.Errorf("browser deadline remaining=%v want approximately %v", got, want)
				}
			default:
				t.Fatalf("browser provider not reached: HTTP %d %s", w.Code, w.Body.String())
			}
		})
	}
}
