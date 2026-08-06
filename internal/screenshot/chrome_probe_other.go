//go:build !windows

package screenshot

import (
	"context"
	"os/exec"
	"strings"
)

func validateChromeBinary(ctx context.Context, path string) bool {
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return false
	}
	version := strings.ToLower(string(out))
	return strings.Contains(version, "chrome") || strings.Contains(version, "chromium") || strings.Contains(version, "edge")
}
