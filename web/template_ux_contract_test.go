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

func TestSettingsEngineBaseURLHintsMatchAdapterDefaults(t *testing.T) {
	template := readTemplateContract(t, "settings.html")
	for _, want := range []string{
		`id="hunter-base-url" placeholder="https://hunter.qianxin.com"`,
		`id="quake-base-url" placeholder="https://quake.360.net/api"`,
	} {
		if !strings.Contains(template, want) {
			t.Errorf("settings engine base URL contract missing %q", want)
		}
	}
	for _, stale := range []string{
		`id="hunter-base-url" placeholder="https://hunter.qianxin.com/api"`,
		`id="quake-base-url" placeholder="https://quake.360.net/api/v1"`,
	} {
		if strings.Contains(template, stale) {
			t.Errorf("settings retains stale engine base URL %q", stale)
		}
	}
}

func TestMainScreenshotPollingFinalizesFailedJobsAndCancelsTimeout(t *testing.T) {
	script := readTemplateContract(t, "../static/js/main.js")
	for _, want := range []string{
		"clearTimeout(screenshotPollTimeout)",
		"截图任务失败",
		"后台任务可能仍在运行",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("main screenshot polling contract missing %q", want)
		}
	}
	if strings.Contains(script, "if (job.error) return;") {
		t.Fatal("failed screenshot jobs are ignored before their terminal status is handled")
	}
}

func TestMainQueryLifecycleAvoidsAuditedInnerHTMLSinks(t *testing.T) {
	script := readTemplateContract(t, "../static/js/main.js")
	for _, forbidden := range []string{
		"resultBox.innerHTML = ''",
		"resultsContent.innerHTML = `",
		"resultsPage.innerHTML = `",
		"main.innerHTML = ''",
	} {
		if strings.Contains(script, forbidden) {
			t.Errorf("audited DOM sink remains in main query lifecycle: %q", forbidden)
		}
	}
	for _, required := range []string{
		"resultBox.replaceChildren()",
		"resultsContent.replaceChildren(statusPanel)",
		"main.replaceChildren(resultsPage)",
		"browserSummary.textContent",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("safe query DOM contract missing %q", required)
		}
	}
}

func TestAccountUserCreationChecksRolePromotionAndFormatsDeleteErrors(t *testing.T) {
	template := readTemplateContract(t, "account.html")
	for _, want := range []string{
		"if (!promoteRes.ok)",
		"用户已创建为 readonly",
		"extractAccountError",
	} {
		if !strings.Contains(template, want) {
			t.Errorf("account interaction contract missing %q", want)
		}
	}
	if strings.Contains(template, "msg = d.error || msg") {
		t.Fatal("delete user still passes an object-shaped API error directly to the toast")
	}
}

func TestSchedulerHistoryRejectsErrorObjectsBeforeRendering(t *testing.T) {
	template := readTemplateContract(t, "scheduler.html")
	start := strings.Index(template, "async function loadHistory()")
	end := strings.Index(template, "async function createTask()")
	if start < 0 || end <= start {
		t.Fatal("could not locate scheduler history function")
	}
	historyFunction := template[start:end]
	for _, want := range []string{"if (!res.ok)", "Array.isArray(records)", "extractErr"} {
		if !strings.Contains(historyFunction, want) {
			t.Errorf("scheduler history contract missing %q", want)
		}
	}
}

func TestSettingsNotificationEditPreservesOmittedCredentials(t *testing.T) {
	template := readTemplateContract(t, "settings.html")
	for _, want := range []string{
		"preserve_existing: !!editingChannelId",
		"document.getElementById('nch-private-ip').checked = !!ch.allow_private_ip",
		"!editingChannelId && !appSecret",
		"document.getElementById('nch-type').disabled = true",
	} {
		if !strings.Contains(template, want) {
			t.Errorf("notification edit contract missing %q", want)
		}
	}
}
