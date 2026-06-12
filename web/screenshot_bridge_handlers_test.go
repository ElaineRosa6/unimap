package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/unimap/project/internal/config"
)

func newBridgeTestServer(signatureRequired bool) *Server {
	cfg := &config.Config{}
	cfg.Screenshot.BaseDir = "./screenshots"
	cfg.Screenshot.Extension.PairingRequired = true
	cfg.Screenshot.Extension.CallbackSignatureRequired = signatureRequired
	cfg.Screenshot.Extension.CallbackSignatureSkewSeconds = 300
	cfg.Screenshot.Extension.CallbackNonceTTLSeconds = 600

	return &Server{
		config: cfg,
		bridge: &BridgeState{
			Mock:           newBridgeMockClient(),
			Tokens:         map[string]int64{"tok-test": time.Now().Add(5 * time.Minute).Unix()},
			CallbackNonces: make(map[string]int64),
		},
	}
}

func signedBridgeHeaders(token string, body []byte, ts int64, nonce string) map[string]string {
	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("%d\n%s\n%s", ts, nonce, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))
	return map[string]string{
		"Authorization":      "Bearer " + token,
		"X-Bridge-Timestamp": fmt.Sprintf("%d", ts),
		"X-Bridge-Nonce":     nonce,
		"X-Bridge-Signature": signature,
	}
}

func setLoopbackBridgeRequest(req *http.Request) {
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1:8448"
}

func TestBridgeMockResultRejectsMissingSignatureWhenRequired(t *testing.T) {
	s := newBridgeTestServer(true)
	body := `{"request_id":"req-1","success":true,"image_path":"c:/tmp/x.png"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok-test")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBridgeMockResultAcceptsValidSignature(t *testing.T) {
	s := newBridgeTestServer(true)
	body := []byte(`{"request_id":"req-2","success":true,"image_path":"c:/tmp/x.png"}`)
	ts := time.Now().Unix()
	headers := signedBridgeHeaders("tok-test", body, ts, "nonce-req-2")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(string(body)))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ok, _ := resp["success"].(bool); !ok {
		t.Fatalf("expected success=true, got %v", resp)
	}
}

func TestBridgeMockResultForwardsCollectedData(t *testing.T) {
	s := newBridgeTestServer(false)
	body := `{"request_id":"req-collect","success":true,"collected_data":"raw title","structured_collected_data":{"total":1,"items":[{"ip":"1.2.3.4","port":443}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok-test")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	result, err := s.bridge.Mock.AwaitResult(req.Context(), "req-collect")
	if err != nil {
		t.Fatalf("await result failed: %v", err)
	}
	if result.CollectedData != "raw title" {
		t.Fatalf("expected collected data to be forwarded, got %q", result.CollectedData)
	}
	if result.StructuredCollectedData == nil {
		t.Fatal("expected structured collected data to be forwarded")
	}
	if result.StructuredCollectedData.Total != 1 {
		t.Fatalf("expected total 1, got %v", result.StructuredCollectedData.Total)
	}
}

func TestBridgeMockResultRejectsReplayNonce(t *testing.T) {
	s := newBridgeTestServer(true)
	body := []byte(`{"request_id":"req-3","success":true,"image_path":"c:/tmp/x.png"}`)
	ts := time.Now().Unix()
	nonce := "nonce-replay-1"
	headers := signedBridgeHeaders("tok-test", body, ts, nonce)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(string(body)))
	setLoopbackBridgeRequest(firstReq)
	firstReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		firstReq.Header.Set(k, v)
	}
	firstW := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(firstW, firstReq)
	if firstW.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", firstW.Code)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(string(body)))
	setLoopbackBridgeRequest(secondReq)
	secondReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		secondReq.Header.Set(k, v)
	}
	secondW := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(secondW, secondReq)
	if secondW.Code != http.StatusUnauthorized {
		t.Fatalf("replay request expected 401, got %d", secondW.Code)
	}
}

func TestBridgeRotateTokenRevokesOldToken(t *testing.T) {
	s := newBridgeTestServer(false)
	body := `{"revoke_old":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/token/rotate", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer tok-test")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeRotateToken(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if s.validateBridgeToken("tok-test") {
		t.Fatalf("expected old token to be revoked")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	newToken, _ := resp["token"].(string)
	if strings.TrimSpace(newToken) == "" {
		t.Fatalf("expected rotated token in response")
	}
	if !s.validateBridgeToken(newToken) {
		t.Fatalf("expected rotated token to be valid")
	}
}

func TestIsLoopbackRequestRejectsForwardedHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", nil)
	setLoopbackBridgeRequest(req)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	if isLoopbackRequest(req) {
		t.Fatalf("expected forwarded loopback request to be rejected")
	}
}

func TestIsLoopbackRequestRejectsNonLoopbackHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "example.com"

	if isLoopbackRequest(req) {
		t.Fatalf("expected non-loopback host to be rejected")
	}
}

// ============================================================
// Admin token fallback + pairing/signature coordination tests
// (covers the post-restart recovery path: a static admin token
// is accepted as a bridge credential on loopback, surviving the
// loss of in-memory pairing tokens after a server restart)
// ============================================================

func withAdminToken(s *Server, token string) *Server {
	s.config.Web.Auth.Enabled = true
	s.config.Web.Auth.AdminToken = token
	return s
}

func TestBridgeMockResultAcceptsAdminTokenWithoutSignature(t *testing.T) {
	// Even with callback signature required, the admin token is accepted on
	// loopback and the signature check is skipped (admin path returns "" token).
	s := withAdminToken(newBridgeTestServer(true), "admin-secret")
	body := `{"request_id":"req-admin","success":true,"image_path":"c:/tmp/x.png"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-secret")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin token without signature, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBridgeTaskNextAcceptsAdminToken(t *testing.T) {
	s := withAdminToken(newBridgeTestServer(true), "admin-secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/bridge/tasks/next", nil)
	setLoopbackBridgeRequest(req)
	req.Header.Set("Authorization", "Bearer admin-secret")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeTaskNext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin token task pull, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBridgeMockResultRejectsUnknownToken(t *testing.T) {
	// A token that is neither a valid bridge token nor the admin token is rejected.
	s := withAdminToken(newBridgeTestServer(true), "admin-secret")
	body := `{"request_id":"req-bad","success":true,"image_path":"c:/tmp/x.png"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer not-a-real-token")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown token, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBridgeAuthAdminTokenRejectedWhenNotLoopback(t *testing.T) {
	// The admin token must only be honored as a bridge credential on loopback.
	s := withAdminToken(newBridgeTestServer(true), "admin-secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/screenshot/bridge/tasks/next", nil)
	req.RemoteAddr = "203.0.113.7:5555"
	req.Host = "example.com:8448"
	req.Header.Set("Authorization", "Bearer admin-secret")

	_, ok := s.validateBridgeAuthIfRequired(httptest.NewRecorder(), req)
	if ok {
		t.Fatalf("expected admin token to be rejected for non-loopback request")
	}
}

func TestBridgeMockResultSkipsSignatureWhenPairingDisabled(t *testing.T) {
	// Signature requires a pairing token; when pairing is disabled the callback
	// signature requirement is also lifted so callbacks are not blocked.
	s := newBridgeTestServer(true)                        // signature required...
	s.config.Screenshot.Extension.PairingRequired = false // ...but pairing disabled
	body := `{"request_id":"req-nopair","success":true,"image_path":"c:/tmp/x.png"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/screenshot/bridge/mock/result", strings.NewReader(body))
	setLoopbackBridgeRequest(req)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	s.handleScreenshotBridgeMockResult(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when pairing disabled, got %d body=%s", w.Code, w.Body.String())
	}
}
