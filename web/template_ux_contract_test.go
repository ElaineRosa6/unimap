package web

import (
	"os"
	"strings"
	"testing"
)

func readTemplateContract(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return string(b)
}

func TestMonitorScreenshotPollingHasExplicitTimeoutState(t *testing.T) {
	template := readTemplateContract(t, "monitor.html")
	for _, want := range []string{
		"截图进度查询超时",
		"后台任务可能仍在运行",
		"clearTimeout(pollTimeout)",
	} {
		if !strings.Contains(template, want) {
			t.Errorf("monitor screenshot polling contract missing %q", want)
		}
	}
}

func TestSettingsICPHealthUsesStandardErrorMessage(t *testing.T) {
	template := readTemplateContract(t, "settings.html")
	if !strings.Contains(template, "'连接失败: ' + extractErrorMsg(data)") {
		t.Fatal("ICP health failure does not use the standard API error extractor")
	}
}
