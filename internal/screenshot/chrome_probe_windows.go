//go:build windows

package screenshot

import (
	"context"
	"debug/pe"
	"path/filepath"
	"strings"
)

// validateChromeBinary validates a Windows executable without starting it.
// Starting chrome.exe with --version can be forwarded to the user's existing
// browser session and open or activate visible windows.
func validateChromeBinary(_ context.Context, path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name != "chrome.exe" && name != "chromium.exe" && name != "chromium-browser.exe" &&
		name != "msedge.exe" && name != "google-chrome.exe" {
		return false
	}
	file, err := pe.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	characteristics := file.FileHeader.Characteristics
	return characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE != 0 && characteristics&pe.IMAGE_FILE_DLL == 0
}
