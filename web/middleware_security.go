package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

type cspNonceKey struct{}

func cspNonceFromContext(ctx context.Context) string {
	if v := ctx.Value(cspNonceKey{}); v != nil {
		s, ok := v.(string)
		if ok {
			return s
		}
	}
	return ""
}

func generateCSPNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// securityMiddleware 添加安全响应头的中间件
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateCSPNonce()
		ctx := context.WithValue(r.Context(), cspNonceKey{}, nonce)
		r = r.WithContext(ctx)

		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy",
			fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s' 'unsafe-hashes'; style-src 'self' 'nonce-%s' https://fonts.googleapis.com; img-src 'self' data: https:; font-src 'self' data: https://fonts.gstatic.com;", nonce, nonce))
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}
