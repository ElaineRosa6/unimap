package appversion

import (
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	// Short 返回 Version 变量
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "1.2.3"
	if got := Short(); got != "1.2.3" {
		t.Errorf("Short() = %q, want %q", got, "1.2.3")
	}

	Version = "dev"
	if got := Short(); got != "dev" {
		t.Errorf("Short() = %q, want %q", got, "dev")
	}
}

func TestFull(t *testing.T) {
	// Full 返回 "Version (commit=GitCommit, built=BuildTime)" 格式
	origV, origC, origB := Version, GitCommit, BuildTime
	t.Cleanup(func() {
		Version, GitCommit, BuildTime = origV, origC, origB
	})

	Version, GitCommit, BuildTime = "2.0.0", "abc123", "2026-06-30"
	got := Full()
	want := "2.0.0 (commit=abc123, built=2026-06-30)"
	if got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}

	// 验证包含所有三个组件
	if !strings.Contains(got, "2.0.0") {
		t.Error("Full() should contain Version")
	}
	if !strings.Contains(got, "abc123") {
		t.Error("Full() should contain GitCommit")
	}
	if !strings.Contains(got, "2026-06-30") {
		t.Error("Full() should contain BuildTime")
	}
}

func TestFull_DefaultValues(t *testing.T) {
	// 使用默认值验证格式
	origV, origC, origB := Version, GitCommit, BuildTime
	t.Cleanup(func() {
		Version, GitCommit, BuildTime = origV, origC, origB
	})

	Version, GitCommit, BuildTime = "dev", "unknown", "unknown"
	got := Full()
	want := "dev (commit=unknown, built=unknown)"
	if got != want {
		t.Errorf("Full() with defaults = %q, want %q", got, want)
	}
}
