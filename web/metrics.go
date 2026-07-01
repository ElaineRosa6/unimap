package web

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/unimap/project/internal/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		metrics.IncHTTPInFlight()
		defer metrics.DecHTTPInFlight()

		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)
		metrics.ObserveHTTPRequest(r.URL.Path, r.Method, recorder.statusCode, time.Since(start))
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", map[string]string{"expected": http.MethodGet})
		return
	}
	if s.config != nil && s.config.Web.Auth.Enabled {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			token = r.Header.Get("X-Admin-Token")
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken())) != 1 {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "admin token required for metrics", nil)
			return
		}
	} else if bindAddr := s.bindAddr(); bindAddr != "127.0.0.1" && bindAddr != "localhost" {
		writeAPIError(w, http.StatusForbidden, "forbidden", "metrics disabled on non-loopback without auth", nil)
		return
	}
	promhttp.Handler().ServeHTTP(w, r)
}
