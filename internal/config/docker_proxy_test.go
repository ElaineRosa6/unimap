package config

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDockerProxyFallback(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	var proxy string
	for _, line := range strings.Split(string(data), "\n") {
		for _, prefix := range []string{"ARG GOPROXY=", "ENV GOPROXY="} {
			if strings.HasPrefix(line, prefix) {
				value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), "\"")
				if !strings.Contains(value, "$") {
					proxy = value
				}
			}
		}
	}
	if proxy == "" {
		t.Fatal("missing Docker GOPROXY default")
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, body := range map[string]string{"go.mod": "module fixture.invalid/fallback\ngo 1.20\n", "fixture.go": "package fallback\n"} {
		w, err := zw.Create("fixture.invalid/fallback@v1.0.0/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name              string
		first, second     int
		success, fallback bool
	}{
		{"primary-healthy", 200, 504, true, false},
		{"primary-504", 504, 200, true, true},
		{"primary-404", 404, 200, true, true},
		{"both-fail", 504, 504, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var secondCalls atomic.Int32
			handler := func(status int, count *atomic.Int32) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if count != nil {
						count.Add(1)
					}
					if status != 200 {
						http.Error(w, "fixture failure", status)
						return
					}
					switch r.URL.Path {
					case "/fixture.invalid/fallback/@v/v1.0.0.info":
						fmt.Fprint(w, `{"Version":"v1.0.0","Time":"2020-01-01T00:00:00Z"}`)
					case "/fixture.invalid/fallback/@v/v1.0.0.mod":
						fmt.Fprint(w, "module fixture.invalid/fallback\ngo 1.20\n")
					case "/fixture.invalid/fallback/@v/v1.0.0.zip":
						_, _ = w.Write(archive.Bytes())
					default:
						http.NotFound(w, r)
					}
				}
			}
			first := httptest.NewServer(handler(tc.first, nil))
			defer first.Close()
			second := httptest.NewServer(handler(tc.second, &secondCalls))
			defer second.Close()
			// Preserve the Docker policy's delimiters, replacing network destinations
			// with local fixtures. A third destination is off, never real VCS access.
			index := 0
			localProxy := regexp.MustCompile(`[^,|]+`).ReplaceAllStringFunc(proxy, func(string) string {
				index++
				if index == 1 {
					return first.URL
				}
				if index == 2 {
					return second.URL
				}
				return "off"
			})
			if index < 2 {
				t.Fatal("Docker proxy needs a fallback")
			}
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", "fixture.invalid/fallback@v1.0.0")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOPROXY="+localProxy, "GOSUMDB=off", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local", "GO111MODULE=on", "GOMODCACHE="+filepath.Join(dir, "cache"), "GOPATH="+filepath.Join(dir, "gopath"))
			// Checksum lookup is disabled only for this synthetic local module;
			// production Docker retains the normal checksum database.
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success {
				t.Errorf("download success=%v, want %v: %s", err == nil, tc.success, out)
			}
			if (secondCalls.Load() > 0) != tc.fallback {
				t.Errorf("fallback calls=%d, want fallback=%v", secondCalls.Load(), tc.fallback)
			}
		})
	}
}
