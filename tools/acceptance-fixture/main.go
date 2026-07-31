// Command acceptance-fixture provides the controlled external test environment
// required by the UniMap cloud security acceptance runbook. It serves:
//
//   - A stable target page for DNS rebinding egress validation.
//   - A mutable page for tamper-evidence notification validation.
//   - Control API endpoints for DNS record flipping (via Cloudflare),
//     private-sink hit counting, and page state mutation.
//   - A private sink listener that counts leaked connections.
//
// Deploy behind a reverse proxy (Caddy) with automatic HTTPS on two domains:
//   - Target domain: hosts the target page and mutable page.
//   - Control domain: hosts the control API (stable, never DNS-flipped).
//
// See README.md in this directory for deployment instructions.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	var cfg Config
	flag.StringVar(&cfg.ControlAddr, "control-addr", envOr("FIXTURE_CONTROL_ADDR", ":9090"),
		"Control API listen address")
	flag.StringVar(&cfg.TargetAddr, "target-addr", envOr("FIXTURE_TARGET_ADDR", ":8081"),
		"Target page listen address (DNS rebinding fixture)")
	flag.StringVar(&cfg.MutableAddr, "mutable-addr", envOr("FIXTURE_MUTABLE_ADDR", ":8082"),
		"Mutable page listen address (tamper fixture)")
	flag.StringVar(&cfg.SinkAddr, "sink-addr", envOr("FIXTURE_SINK_ADDR", ""),
		"Private sink listen address (e.g. 10.0.0.1:8083); empty disables")
	flag.StringVar(&cfg.ControlToken, "token", envOr("FIXTURE_CONTROL_TOKEN", ""),
		"Bearer token required by all control API endpoints")
	flag.StringVar(&cfg.CFToken, "cf-token", envOr("CLOUDFLARE_API_TOKEN", ""),
		"Cloudflare API token with DNS edit permission")
	flag.StringVar(&cfg.CFZoneID, "cf-zone-id", envOr("CLOUDFLARE_ZONE_ID", ""),
		"Cloudflare zone ID for the target domain")
	flag.StringVar(&cfg.CFRecordName, "cf-record-name", envOr("CLOUDFLARE_RECORD_NAME", ""),
		"Full DNS record name to flip (e.g. rebind.example.com)")
	flag.StringVar(&cfg.PublicIP, "public-ip", envOr("FIXTURE_PUBLIC_IP", ""),
		"Public IP to set on DNS reset")
	flag.StringVar(&cfg.PrivateIP, "private-ip", envOr("FIXTURE_PRIVATE_IP", "127.0.0.1"),
		"Private/loopback IP to set on DNS flip")
	flag.Parse()

	if cfg.ControlToken == "" {
		log.Fatal("FIXTURE_CONTROL_TOKEN or -token is required")
	}

	var dnsProvider DNSProvider
	if cfg.CFToken != "" && cfg.CFZoneID != "" && cfg.CFRecordName != "" {
		dnsProvider = NewCloudflareProvider(cfg.CFToken, cfg.CFZoneID, cfg.CFRecordName)
		log.Printf("dns: cloudflare zone=%s record=%s", cfg.CFZoneID, cfg.CFRecordName)
	} else {
		log.Printf("dns: no Cloudflare config; DNS flip/reset endpoints return 501")
	}

	s := &Server{
		cfg:         cfg,
		dns:         dnsProvider,
		publicIP:    cfg.PublicIP,
		privateIP:   cfg.PrivateIP,
		pageState:   pageOriginal,
		mutableBody: pageOriginal,
	}

	// Start private sink (counts leaked connections from DNS rebinding).
	if cfg.SinkAddr != "" {
		ln, err := net.Listen("tcp", cfg.SinkAddr)
		if err != nil {
			log.Fatalf("private sink listen %s: %v", cfg.SinkAddr, err)
		}
		go s.serveSink(ln)
		log.Printf("sink: listening on %s", cfg.SinkAddr)
	}

	// Start target page (DNS rebinding fixture target).
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.handleTargetPage)
		srv := &http.Server{Addr: cfg.TargetAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
		log.Printf("target: listening on %s", cfg.TargetAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("target server: %v", err)
		}
	}()

	// Start mutable page (tamper fixture target).
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.handleMutablePage)
		srv := &http.Server{Addr: cfg.MutableAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
		log.Printf("mutable: listening on %s", cfg.MutableAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("mutable server: %v", err)
		}
	}()

	// Start control API.
	controlMux := http.NewServeMux()
	// DNS rebinding fixture endpoints.
	controlMux.HandleFunc("POST /dns/reset", s.auth(s.handleDNSReset))
	controlMux.HandleFunc("POST /dns/flip", s.auth(s.handleDNSFlip))
	controlMux.HandleFunc("GET /dns/private-hits", s.auth(s.handlePrivateHits))
	// Tamper fixture endpoints.
	controlMux.HandleFunc("POST /page/reset", s.auth(s.handlePageReset))
	controlMux.HandleFunc("POST /page/mutate", s.auth(s.handlePageMutate))
	// Health check (no auth).
	controlMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	controlSrv := &http.Server{
		Addr:              cfg.ControlAddr,
		Handler:           controlMux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("control: listening on %s", cfg.ControlAddr)
		if err := controlSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("control server: %v", err)
		}
	}()

	log.Printf("acceptance fixture ready (public_ip=%s private_ip=%s)", s.publicIP, s.privateIP)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down")
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// Config holds all runtime configuration for the acceptance fixture.
type Config struct {
	ControlAddr  string
	TargetAddr   string
	MutableAddr  string
	SinkAddr     string
	ControlToken string
	CFToken      string
	CFZoneID     string
	CFRecordName string
	PublicIP     string
	PrivateIP    string
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server is the acceptance fixture control server.
type Server struct {
	cfg         Config
	dns         DNSProvider
	publicIP    string
	privateIP   string
	sinkHits    atomic.Int64
	mu          sync.RWMutex
	pageState   string // pageOriginal or pageMutated
	mutableBody string
}

const (
	pageOriginal = `<html>
<head><title>Acceptance Fixture</title></head>
<body>
<main>
<h1>Controlled Acceptance Page</h1>
<p id="content">This is the original baseline content for tamper detection verification.</p>
<p>Integrity marker: ALPHA-7742</p>
</main>
</body>
</html>`

	pageMutated = `<html>
<head><title>Acceptance Fixture</title></head>
<body>
<main>
<h1>Controlled Acceptance Page</h1>
<p id="content">TAMPERED: This content has been deliberately modified for acceptance testing.</p>
<p>Integrity marker: BRAVO-9917</p>
</main>
</body>
</html>`
)

// ---------------------------------------------------------------------------
// Auth middleware
// ---------------------------------------------------------------------------

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != s.cfg.ControlToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// ---------------------------------------------------------------------------
// DNS rebinding fixture handlers
// ---------------------------------------------------------------------------

// handleDNSReset sets the target domain to the public IP and zeros the sink counter.
func (s *Server) handleDNSReset(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "no DNS provider configured"})
		return
	}
	if s.publicIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "FIXTURE_PUBLIC_IP not set"})
		return
	}
	if err := s.dns.SetRecord(r.Context(), s.publicIP); err != nil {
		log.Printf("dns reset failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "DNS update failed"})
		return
	}
	s.sinkHits.Store(0)
	log.Printf("dns: reset to %s, sink zeroed", s.publicIP)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ip": s.publicIP, "hits_reset": true})
}

// handleDNSFlip sets the target domain to a private/loopback IP.
func (s *Server) handleDNSFlip(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "no DNS provider configured"})
		return
	}
	if err := s.dns.SetRecord(r.Context(), s.privateIP); err != nil {
		log.Printf("dns flip failed: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "DNS update failed"})
		return
	}
	log.Printf("dns: flipped to %s", s.privateIP)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ip": s.privateIP})
}

// handlePrivateHits returns the number of connections received by the private sink.
func (s *Server) handlePrivateHits(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"hits": s.sinkHits.Load()})
}

// ---------------------------------------------------------------------------
// Tamper fixture handlers
// ---------------------------------------------------------------------------

// handlePageReset restores the mutable page to its original baseline content.
func (s *Server) handlePageReset(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.pageState = pageOriginal
	s.mutableBody = pageOriginal
	s.mu.Unlock()
	log.Println("page: reset to original")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": "original"})
}

// handlePageMutate modifies the mutable page content deterministically.
func (s *Server) handlePageMutate(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.pageState = pageMutated
	s.mutableBody = pageMutated
	s.mu.Unlock()
	log.Println("page: mutated")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": "mutated"})
}

// ---------------------------------------------------------------------------
// Page servers
// ---------------------------------------------------------------------------

func (s *Server) handleTargetPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<html><head><title>DNS Target</title></head><body><h1>OK</h1><p>dns-fixture-target</p></body></html>`)
}

func (s *Server) handleMutablePage(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	body := s.mutableBody
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, body)
}

// ---------------------------------------------------------------------------
// Private sink
// ---------------------------------------------------------------------------

func (s *Server) serveSink(ln net.Listener) {
	srv := &http.Server{
		Handler:           http.HandlerFunc(s.handleSinkRequest),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.Serve(ln); err != nil {
		log.Printf("private sink: %v", err)
	}
}

func (s *Server) handleSinkRequest(w http.ResponseWriter, r *http.Request) {
	n := s.sinkHits.Add(1)
	log.Printf("SINK LEAK #%d from %s %s %s", n, r.RemoteAddr, r.Method, r.URL.Path)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "sink")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
