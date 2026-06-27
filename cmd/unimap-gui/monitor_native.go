//go:build gui
// +build gui

package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/unimap/project/internal/screenshot"
	"github.com/unimap/project/internal/tamper"
)

type monitorTarget struct {
	InputURL         string
	NormalizedURL    string
	FormatValid      bool
	Reachable        bool
	StatusCode       int
	ReasonType       string
	Reason           string
	BaselineExists   bool
	BaselineStatus   string
	TamperStatus     string
	Tampered         bool
	TamperedSegments []string
	Changes          []tamper.SegmentChange
	LastCheckedAt    int64
	ScreenshotPath   string
	ScreenshotError  string
}

type historyURLItem struct {
	URL         string
	HasBaseline bool
	RecordCount int
	LastCheckAt int64
}

type screenshotBatchItem struct {
	Name      string
	Path      string
	FileCount int
	UpdatedAt time.Time
}

type screenshotFileItem struct {
	Name       string
	Path       string
	PreviewURL string
	Size       int64
	UpdatedAt  time.Time
}

type monitorTabCtx struct {
	window          fyne.Window
	appState        *AppState
	targets         []monitorTarget
	baselines       []string
	selectedTarget  int
	selectedBaseURL int
	actionButtons   []*widget.Button

	urlEntry         *widget.Entry
	concurrencyEntry *widget.Entry
	statusLabel      *widget.Label
	summaryLabel     *widget.Label
	detailEntry      *widget.Entry
	baselineDetail   *widget.Entry
	targetList       *widget.List
	baselineList     *widget.List

	probeBtn           *widget.Button
	baselineBtn        *widget.Button
	tamperBtn          *widget.Button
	screenshotBtn      *widget.Button
	refreshBaselineBtn *widget.Button
	retryBtn           *widget.Button
	openScreenshotBtn  *widget.Button
	copyURLBtn         *widget.Button
	appendBaselineBtn  *widget.Button
	deleteBaselineBtn  *widget.Button
}

func newMonitorTabCtx(window fyne.Window, state *AppState) *monitorTabCtx {
	c := &monitorTabCtx{
		window:          window,
		appState:        state,
		selectedTarget:  -1,
		selectedBaseURL: -1,
	}
	c.urlEntry = widget.NewMultiLineEntry()
	c.urlEntry.SetMinRowsVisible(8)
	c.urlEntry.SetPlaceHolder("每行一个 URL，支持不带协议的域名或主机:端口")

	c.concurrencyEntry = widget.NewEntry()
	c.concurrencyEntry.SetText("5")
	c.concurrencyEntry.SetPlaceHolder("5")

	c.statusLabel = widget.NewLabel("就绪")
	c.summaryLabel = widget.NewLabel("总数 0 | 格式合法 0 | 可达 0 | 不可达 0")

	c.detailEntry = widget.NewMultiLineEntry()
	c.detailEntry.Disable()
	c.detailEntry.SetMinRowsVisible(18)
	c.baselineDetail = widget.NewMultiLineEntry()
	c.baselineDetail.Disable()
	c.baselineDetail.SetMinRowsVisible(7)
	return c
}

func createMonitorTab(window fyne.Window, state *AppState) fyne.CanvasObject {
	c := newMonitorTabCtx(window, state)
	c.buildTargetList()
	c.buildBaselineList()
	c.newBatchButtons()
	c.newTargetActionButtons()
	c.newBaselineActionButtons()
	c.actionButtons = []*widget.Button{
		c.probeBtn, c.baselineBtn, c.tamperBtn, c.screenshotBtn, c.refreshBaselineBtn,
		c.retryBtn, c.openScreenshotBtn, c.copyURLBtn, c.appendBaselineBtn, c.deleteBaselineBtn,
	}
	c.refreshBaselines()
	c.refreshDetails()
	return c.buildLayout()
}

func (c *monitorTabCtx) refreshDetails() {
	if c.selectedTarget < 0 || c.selectedTarget >= len(c.targets) {
		c.detailEntry.SetText("选择左侧 URL 查看探活、基线、篡改和截图详情。")
	} else {
		c.detailEntry.SetText(formatMonitorDetail(c.targets[c.selectedTarget]))
	}
	if c.selectedBaseURL < 0 || c.selectedBaseURL >= len(c.baselines) {
		c.baselineDetail.SetText("选择基线 URL 查看管理信息。")
	} else {
		baselineURL := c.baselines[c.selectedBaseURL]
		records, _ := c.appState.TamperStorage.LoadCheckRecords(baselineURL, 5)
		c.baselineDetail.SetText(formatBaselineDetail(c.appState, baselineURL, records))
	}
}

func (c *monitorTabCtx) buildTargetList() {
	c.targetList = widget.NewList(
		func() int { return len(c.targets) },
		func() fyne.CanvasObject {
			primary := widget.NewLabel("")
			primary.TextStyle = fyne.TextStyle{Bold: true}
			secondary := widget.NewLabel("")
			secondary.Wrapping = fyne.TextWrapWord
			return container.NewVBox(primary, secondary)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := c.targets[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			primary, ok := box.Objects[0].(*widget.Label)
			if ok {
				primary.SetText(monitorTargetTitle(item))
			}
			secondary, ok := box.Objects[1].(*widget.Label)
			if ok {
				secondary.SetText(monitorTargetSubtitle(item))
			}
		},
	)
	c.targetList.OnSelected = func(id widget.ListItemID) {
		c.selectedTarget = id
		c.refreshDetails()
	}
}

func (c *monitorTabCtx) buildBaselineList() {
	c.baselineList = widget.NewList(
		func() int { return len(c.baselines) },
		func() fyne.CanvasObject {
			primary := widget.NewLabel("")
			primary.TextStyle = fyne.TextStyle{Bold: true}
			secondary := widget.NewLabel("")
			return container.NewVBox(primary, secondary)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			baselineURL := c.baselines[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			primary, ok := box.Objects[0].(*widget.Label)
			if ok {
				primary.SetText(baselineURL)
			}
			secondary, ok := box.Objects[1].(*widget.Label)
			if ok {
				secondary.SetText(baselineMetaText(c.appState, baselineURL))
			}
		},
	)
	c.baselineList.OnSelected = func(id widget.ListItemID) {
		c.selectedBaseURL = id
		c.refreshDetails()
	}
}

func (c *monitorTabCtx) refreshBaselines() {
	list, source, err := listBaselinesPreferAPI(context.Background(), c.appState)
	if err != nil {
		c.statusLabel.SetText("读取基线失败: " + err.Error())
		return
	}
	c.baselines = list
	syncBaselineFlags(c.targets, c.baselines)
	if c.selectedBaseURL >= len(c.baselines) {
		c.selectedBaseURL = -1
	}
	c.baselineList.Refresh()
	c.targetList.Refresh()
	c.refreshDetails()
	c.statusLabel.SetText(fmt.Sprintf("已加载 %d 条基线（来源: %s）", len(c.baselines), source))
}

func (c *monitorTabCtx) updateSummary() {
	total := len(c.targets)
	valid := 0
	reachable := 0
	for _, item := range c.targets {
		if item.FormatValid {
			valid++
		}
		if item.Reachable {
			reachable++
		}
	}
	c.summaryLabel.SetText(fmt.Sprintf("总数 %d | 格式合法 %d | 可达 %d | 不可达 %d", total, valid, reachable, total-valid+valid-reachable))
}

func (c *monitorTabCtx) setTargets(items []monitorTarget) {
	c.targets = items
	syncBaselineFlags(c.targets, c.baselines)
	if c.selectedTarget >= len(c.targets) {
		c.selectedTarget = -1
	}
	c.updateSummary()
	c.targetList.Refresh()
	c.refreshDetails()
}

func (c *monitorTabCtx) parseConcurrency() int {
	value := strings.TrimSpace(c.concurrencyEntry.Text)
	if value == "" {
		return 5
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 5
	}
	if parsed > 20 {
		return 20
	}
	return parsed
}

func (c *monitorTabCtx) setBusy(busy bool, text string) {
	if busy {
		c.statusLabel.SetText(text)
	} else if text != "" {
		c.statusLabel.SetText(text)
	}
	for _, button := range c.actionButtons {
		if button == nil {
			continue
		}
		if busy {
			button.Disable()
		} else {
			button.Enable()
		}
	}
}

func (c *monitorTabCtx) runBatchAction(name string, handler func(context.Context, []monitorTarget, int) ([]monitorTarget, string, error)) {
	parsed := parseMonitorTargets(c.urlEntry.Text)
	if len(parsed) == 0 {
		dialog.ShowError(fmt.Errorf("请输入至少一个 URL"), c.window)
		return
	}
	concurrency := c.parseConcurrency()
	c.setBusy(true, name+"中...")
	go func() {
		ctx := context.Background()
		probed := probeMonitorTargets(ctx, parsed, concurrency)
		resultTargets, statusText, err := handler(ctx, probed, concurrency)
		if err != nil {
			c.setTargets(probed)
			c.refreshBaselines()
			c.setBusy(false, statusText)
			dialog.ShowError(err, c.window)
			return
		}
		c.setTargets(resultTargets)
		c.refreshBaselines()
		c.setBusy(false, statusText)
	}()
}

func (c *monitorTabCtx) baselineHandler(ctx context.Context, probed []monitorTarget, concurrency int) ([]monitorTarget, string, error) {
	reachable := reachableURLs(probed)
	if len(reachable) == 0 {
		annotateUnreachableBaseline(probed)
		return probed, "无可达 URL 可设置基线", nil
	}
	results, source, err := runSetBaselinePreferAPI(ctx, c.appState, reachable, concurrency)
	if err != nil {
		return probed, "基线设置失败", err
	}
	applyBaselineResults(probed, results)
	return probed, fmt.Sprintf("%s（来源: %s）", summarizeBaselineStatus(probed), source), nil
}

func (c *monitorTabCtx) tamperHandler(ctx context.Context, probed []monitorTarget, concurrency int) ([]monitorTarget, string, error) {
	reachable := reachableURLs(probed)
	if len(reachable) == 0 {
		annotateUnreachableTamper(probed)
		return probed, "无可达 URL 可检测", nil
	}
	results, source, err := runTamperCheckPreferAPI(ctx, c.appState, reachable, concurrency)
	if err != nil {
		return probed, "篡改检测失败", err
	}
	applyTamperResults(probed, results)
	return probed, fmt.Sprintf("%s（来源: %s）", summarizeTamperStatus(probed), source), nil
}

func (c *monitorTabCtx) screenshotHandler(ctx context.Context, probed []monitorTarget, concurrency int) ([]monitorTarget, string, error) {
	reachable := reachableURLs(probed)
	if len(reachable) == 0 {
		annotateUnreachableScreenshot(probed)
		return probed, "无可达 URL 可截图", nil
	}
	batchID := time.Now().Format("20060102-150405")
	results, source, err := runBatchScreenshotPreferAPI(ctx, c.appState, reachable, batchID, concurrency)
	if err != nil {
		return probed, "批量截图失败", err
	}
	applyScreenshotResults(probed, results)
	return probed, fmt.Sprintf("%s（来源: %s）", summarizeScreenshotStatus(probed), source), nil
}

func (c *monitorTabCtx) newBatchButtons() {
	c.probeBtn = widget.NewButtonWithIcon("探活", theme.ViewRefreshIcon(), func() {
		c.runBatchAction("探活", func(_ context.Context, probed []monitorTarget, _ int) ([]monitorTarget, string, error) {
			return probed, summarizeProbeStatus(probed), nil
		})
	})
	c.baselineBtn = widget.NewButtonWithIcon("设置基线", theme.DocumentCreateIcon(), func() {
		c.runBatchAction("设置基线", c.baselineHandler)
	})
	c.tamperBtn = widget.NewButtonWithIcon("篡改检测", theme.WarningIcon(), func() {
		c.runBatchAction("篡改检测", c.tamperHandler)
	})
	c.screenshotBtn = widget.NewButtonWithIcon("批量截图", theme.DocumentIcon(), func() {
		if c.appState.ScreenshotMgr == nil {
			dialog.ShowError(fmt.Errorf("截图功能未启用，请在配置中开启 screenshot.enabled 并配置 Chrome"), c.window)
			return
		}
		c.runBatchAction("批量截图", c.screenshotHandler)
	})
	c.refreshBaselineBtn = widget.NewButtonWithIcon("刷新基线", theme.ViewRefreshIcon(), func() {
		c.refreshBaselines()
		c.statusLabel.SetText(fmt.Sprintf("已加载 %d 条基线", len(c.baselines)))
	})
}

func (c *monitorTabCtx) newTargetActionButtons() {
	c.retryBtn = widget.NewButton("重试所选探活", func() {
		if c.selectedTarget < 0 || c.selectedTarget >= len(c.targets) {
			dialog.ShowInformation("提示", "请先选择一个 URL", c.window)
			return
		}
		item := c.targets[c.selectedTarget]
		c.setBusy(true, "重试探活中...")
		go func() {
			refreshed := probeMonitorTargets(context.Background(), []monitorTarget{item}, 1)
			if len(refreshed) == 1 {
				c.targets[c.selectedTarget] = refreshed[0]
				syncBaselineFlags(c.targets, c.baselines)
			}
			c.updateSummary()
			c.targetList.Refresh()
			c.refreshDetails()
			c.setBusy(false, "所选 URL 已完成重试探活")
		}()
	})
	c.openScreenshotBtn = widget.NewButton("打开所选截图", func() {
		if c.selectedTarget < 0 || c.selectedTarget >= len(c.targets) {
			dialog.ShowInformation("提示", "请先选择一个 URL", c.window)
			return
		}
		path := strings.TrimSpace(c.targets[c.selectedTarget].ScreenshotPath)
		if path == "" {
			dialog.ShowInformation("提示", "所选 URL 还没有截图结果", c.window)
			return
		}
		if err := openPathInSystem(path); err != nil {
			dialog.ShowError(err, c.window)
		}
	})
	c.copyURLBtn = widget.NewButton("复制所选 URL", func() {
		if c.selectedTarget < 0 || c.selectedTarget >= len(c.targets) {
			dialog.ShowInformation("提示", "请先选择一个 URL", c.window)
			return
		}
		c.window.Clipboard().SetContent(selectedMonitorURL(c.targets[c.selectedTarget]))
		c.statusLabel.SetText("已复制所选 URL")
	})
}

func (c *monitorTabCtx) newBaselineActionButtons() {
	c.appendBaselineBtn = widget.NewButton("导入所选基线", func() {
		if c.selectedBaseURL < 0 || c.selectedBaseURL >= len(c.baselines) {
			dialog.ShowInformation("提示", "请先选择一个基线 URL", c.window)
			return
		}
		baseURL := c.baselines[c.selectedBaseURL]
		text := strings.TrimSpace(c.urlEntry.Text)
		if text == "" {
			c.urlEntry.SetText(baseURL)
		} else {
			c.urlEntry.SetText(text + "\n" + baseURL)
		}
	})
	c.deleteBaselineBtn = widget.NewButton("删除所选基线", func() {
		if c.selectedBaseURL < 0 || c.selectedBaseURL >= len(c.baselines) {
			dialog.ShowInformation("提示", "请先选择一个基线 URL", c.window)
			return
		}
		baselineURL := c.baselines[c.selectedBaseURL]
		dialog.ShowConfirm("删除基线", "确认删除该 URL 的基线？", func(confirm bool) {
			if !confirm {
				return
			}
			source, err := runDeleteBaselinePreferAPI(context.Background(), c.appState, baselineURL)
			if err != nil {
				dialog.ShowError(err, c.window)
				return
			}
			c.refreshBaselines()
			c.statusLabel.SetText("基线已删除（来源: " + source + "）")
		}, c.window)
	})
}

func (c *monitorTabCtx) buildLayout() fyne.CanvasObject {
	controls := container.NewHBox(
		c.probeBtn, c.baselineBtn, c.tamperBtn, c.screenshotBtn, c.refreshBaselineBtn,
		layout.NewSpacer(),
		widget.NewLabel("并发:"),
		container.NewGridWrap(fyne.NewSize(70, c.concurrencyEntry.MinSize().Height), c.concurrencyEntry),
	)
	leftPane := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("监控 URL 输入", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			c.urlEntry, widget.NewSeparator(), controls, widget.NewSeparator(), c.summaryLabel,
		), nil, nil, nil,
		container.NewBorder(widget.NewLabelWithStyle("探活与检测结果", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, c.targetList),
	)
	rightPane := container.NewVSplit(
		container.NewBorder(
			container.NewVBox(widget.NewLabelWithStyle("所选 URL 详情", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				container.NewHBox(c.copyURLBtn, c.retryBtn, c.openScreenshotBtn), widget.NewSeparator()),
			nil, nil, nil, c.detailEntry,
		),
		container.NewBorder(
			container.NewVBox(widget.NewLabelWithStyle("基线管理", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				container.NewHBox(c.appendBaselineBtn, c.deleteBaselineBtn), widget.NewSeparator()),
			nil, nil, nil, container.NewVSplit(c.baselineList, c.baselineDetail),
		),
	)
	rightPane.Offset = 0.58
	content := container.NewHSplit(leftPane, rightPane)
	content.Offset = 0.56
	return container.NewBorder(
		container.NewVBox(
			container.NewHBox(widget.NewIcon(theme.ComputerIcon()),
				widget.NewLabelWithStyle("原生 URL 监控", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				layout.NewSpacer(), c.statusLabel),
			widget.NewSeparator(),
		), nil, nil, nil, content,
	)
}

func createHistoryTab(window fyne.Window, state *AppState) fyne.CanvasObject {
	h := &historyTabState{state: state, recordsByURL: make(map[string][]*tamper.CheckRecord)}

	h.statsLabel = widget.NewLabel("就绪")
	h.urlDetail = widget.NewMultiLineEntry()
	h.urlDetail.Disable()
	h.urlDetail.SetMinRowsVisible(6)
	h.recordDetail = widget.NewMultiLineEntry()
	h.recordDetail.Disable()
	h.recordDetail.SetMinRowsVisible(16)

	h.urlList = buildHistoryURLList(h)
	h.recordList = buildHistoryRecordList(h)
	h.urlList.OnSelected = func(id widget.ListItemID) { h.selectedURL = id; h.loadRecords() }
	h.recordList.OnSelected = func(id widget.ListItemID) { h.selectedRecord = id; h.refreshDetail() }

	h.refreshURLs()

	left := container.NewBorder(container.NewVBox(widget.NewLabelWithStyle("监控目标", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator()), nil, nil, nil, h.urlList)
	middle := container.NewBorder(container.NewVBox(widget.NewLabelWithStyle("检测记录", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator()), nil, nil, nil, h.recordList)
	right := container.NewVSplit(h.urlDetail, h.recordDetail)
	right.Offset = 0.33
	content := container.NewHSplit(container.NewHSplit(left, middle), right)
	content.Offset = 0.58
	if s, ok := content.Leading.(*container.Split); ok {
		s.Offset = 0.42
	}

	toolbar := buildHistoryToolbar(window, h)
	return container.NewBorder(
		container.NewVBox(
			container.NewHBox(widget.NewIcon(theme.HistoryIcon()), widget.NewLabelWithStyle("检测历史", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layout.NewSpacer(), h.statsLabel),
			widget.NewSeparator(), toolbar, widget.NewSeparator(),
		), nil, nil, nil, content,
	)
}

type historyTabState struct {
	state          *AppState
	urlItems       []historyURLItem
	records        []*tamper.CheckRecord
	recordsByURL   map[string][]*tamper.CheckRecord
	selectedURL    int
	selectedRecord int
	statsLabel     *widget.Label
	urlDetail      *widget.Entry
	recordDetail   *widget.Entry
	urlList        *widget.List
	recordList     *widget.List
}

func (h *historyTabState) refreshDetail() {
	if h.selectedURL < 0 || h.selectedURL >= len(h.urlItems) {
		h.urlDetail.SetText("选择左侧 URL 查看统计信息。")
	} else {
		item := h.urlItems[h.selectedURL]
		stats, _ := h.state.TamperStorage.GetCheckStats(item.URL)
		h.urlDetail.SetText(formatHistoryURLDetail(item, stats))
	}
	if h.selectedRecord < 0 || h.selectedRecord >= len(h.records) {
		h.recordDetail.SetText("选择中间的检测记录查看变更详情。")
	} else {
		h.recordDetail.SetText(formatCheckRecordDetail(h.records[h.selectedRecord]))
	}
}

func (h *historyTabState) loadRecords() {
	if h.selectedURL < 0 || h.selectedURL >= len(h.urlItems) {
		h.records = nil
		h.selectedRecord = -1
	} else {
		h.records = h.recordsByURL[h.urlItems[h.selectedURL].URL]
		if h.selectedRecord >= len(h.records) {
			h.selectedRecord = -1
		}
	}
	h.recordList.Refresh()
	h.refreshDetail()
}

func (h *historyTabState) refreshURLs() {
	h.recordsByURL = make(map[string][]*tamper.CheckRecord)
	baselineURLs, baselineSource, baselineErr := listBaselinesPreferAPI(context.Background(), h.state)
	if baselineErr != nil {
		h.statsLabel.SetText("读取基线失败: " + baselineErr.Error())
		return
	}
	baselineSet := make(map[string]bool, len(baselineURLs))
	for _, item := range baselineURLs {
		baselineSet[item] = true
	}

	byURL := make(map[string]*historyURLItem)
	source, apiRecords, apiErr := "API", []apiTamperHistoryItem{}, error(nil)
	apiRecords, apiErr = listTamperHistoryViaAPI(context.Background(), 1000)
	if apiErr == nil {
		h.mergeHistoryFromAPI(baselineURLs, baselineSet, byURL, apiRecords)
		for urlText := range h.recordsByURL {
			sort.Slice(h.recordsByURL[urlText], func(i, j int) bool {
				return h.recordsByURL[urlText][i].Timestamp > h.recordsByURL[urlText][j].Timestamp
			})
		}
	} else {
		source = "本地"
		allRecords, err := h.state.TamperStorage.ListAllCheckRecords()
		if err != nil {
			h.statsLabel.SetText("读取历史索引失败: " + err.Error())
			return
		}
		h.mergeHistoryFromLocal(baselineURLs, baselineSet, byURL, allRecords)
	}

	h.urlItems = h.urlItems[:0]
	for _, item := range byURL {
		h.urlItems = append(h.urlItems, *item)
	}
	sort.Slice(h.urlItems, func(i, j int) bool {
		if h.urlItems[i].LastCheckAt == h.urlItems[j].LastCheckAt {
			return h.urlItems[i].URL < h.urlItems[j].URL
		}
		return h.urlItems[i].LastCheckAt > h.urlItems[j].LastCheckAt
	})
	if h.selectedURL >= len(h.urlItems) {
		h.selectedURL = -1
	}
	h.urlList.Refresh()
	h.loadRecords()
	h.statsLabel.SetText(fmt.Sprintf("已加载 %d 个监控目标（历史来源: %s，基线来源: %s）", len(h.urlItems), source, baselineSource))
}

func (h *historyTabState) mergeHistoryFromAPI(baselineURLs []string, baselineSet map[string]bool, byURL map[string]*historyURLItem, apiRecords []apiTamperHistoryItem) {
	for _, item := range baselineURLs {
		if _, ok := byURL[item]; !ok {
			byURL[item] = &historyURLItem{URL: item, HasBaseline: true}
		}
	}
	for _, rec := range apiRecords {
		record := rec.toCheckRecord()
		if record == nil || strings.TrimSpace(record.URL) == "" {
			continue
		}
		h.recordsByURL[record.URL] = append(h.recordsByURL[record.URL], record)
		info, ok := byURL[record.URL]
		if !ok {
			info = &historyURLItem{URL: record.URL}
			byURL[record.URL] = info
		}
		info.RecordCount++
		if record.Timestamp > info.LastCheckAt {
			info.LastCheckAt = record.Timestamp
		}
		if baselineSet[record.URL] {
			info.HasBaseline = true
		}
	}
}

func (h *historyTabState) mergeHistoryFromLocal(baselineURLs []string, baselineSet map[string]bool, byURL map[string]*historyURLItem, allRecords map[string][]*tamper.CheckRecord) {
	for _, item := range baselineURLs {
		if _, ok := byURL[item]; !ok {
			byURL[item] = &historyURLItem{URL: item, HasBaseline: true}
		}
	}
	for _, list := range allRecords {
		for _, record := range list {
			if strings.TrimSpace(record.URL) == "" {
				continue
			}
			h.recordsByURL[record.URL] = append(h.recordsByURL[record.URL], record)
			info, ok := byURL[record.URL]
			if !ok {
				info = &historyURLItem{URL: record.URL}
				byURL[record.URL] = info
			}
			info.RecordCount++
			if record.Timestamp > info.LastCheckAt {
				info.LastCheckAt = record.Timestamp
			}
			if baselineSet[record.URL] {
				info.HasBaseline = true
			}
		}
	}
}

func buildHistoryURLList(h *historyTabState) *widget.List {
	return widget.NewList(
		func() int { return len(h.urlItems) },
		func() fyne.CanvasObject {
			primary := widget.NewLabel("")
			primary.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewVBox(primary, widget.NewLabel(""))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := h.urlItems[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			if primary, ok := box.Objects[0].(*widget.Label); ok {
				primary.SetText(item.URL)
			}
			if secondary, ok := box.Objects[1].(*widget.Label); ok {
				secondary.SetText(fmt.Sprintf("记录 %d | 基线 %s | 最近 %s", item.RecordCount, yesNo(item.HasBaseline), formatTimestamp(item.LastCheckAt)))
			}
		},
	)
}

func buildHistoryRecordList(h *historyTabState) *widget.List {
	return widget.NewList(
		func() int { return len(h.records) },
		func() fyne.CanvasObject {
			primary := widget.NewLabel("")
			primary.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewVBox(primary, widget.NewLabel(""))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			record := h.records[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			if primary, ok := box.Objects[0].(*widget.Label); ok {
				primary.SetText(fmt.Sprintf("%s | %s", record.CheckType, formatTimestamp(record.Timestamp)))
			}
			if secondary, ok := box.Objects[1].(*widget.Label); ok {
				secondary.SetText(historyRecordSummary(record))
			}
		},
	)
}

func buildHistoryToolbar(window fyne.Window, h *historyTabState) *fyne.Container {
	refreshBtn := widget.NewButtonWithIcon("刷新历史", theme.ViewRefreshIcon(), func() { h.refreshURLs() })
	copyBtn := widget.NewButton("复制所选 URL", func() {
		if h.selectedURL < 0 || h.selectedURL >= len(h.urlItems) {
			dialog.ShowInformation("提示", "请先选择一个 URL", window)
			return
		}
		window.Clipboard().SetContent(h.urlItems[h.selectedURL].URL)
		h.statsLabel.SetText("已复制所选 URL")
	})
	deleteHistoryBtn := widget.NewButton("删除该 URL 历史", func() {
		if h.selectedURL < 0 || h.selectedURL >= len(h.urlItems) {
			dialog.ShowInformation("提示", "请先选择一个 URL", window)
			return
		}
		selected := h.urlItems[h.selectedURL]
		dialog.ShowConfirm("删除历史", "确认删除该 URL 的全部检测记录？", func(confirm bool) {
			if !confirm {
				return
			}
			source, err := runDeleteHistoryPreferAPI(context.Background(), h.state, selected.URL)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			h.refreshURLs()
			h.statsLabel.SetText("历史已删除（来源: " + source + "）")
		}, window)
	})
	deleteBaselineBtn := widget.NewButton("删除该 URL 基线", func() {
		if h.selectedURL < 0 || h.selectedURL >= len(h.urlItems) {
			dialog.ShowInformation("提示", "请先选择一个 URL", window)
			return
		}
		selected := h.urlItems[h.selectedURL]
		source, err := runDeleteBaselinePreferAPI(context.Background(), h.state, selected.URL)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		h.refreshURLs()
		h.statsLabel.SetText("基线已删除（来源: " + source + "）")
	})
	return container.NewHBox(refreshBtn, copyBtn, deleteHistoryBtn, deleteBaselineBtn)
}

func createScreenshotTab(window fyne.Window, state *AppState) fyne.CanvasObject {
	if state.ScreenshotMgr == nil {
		return container.NewCenter(widget.NewLabel("截图功能未启用。请在配置中开启 screenshot.enabled，并配置 Chrome 路径或远程调试地址。"))
	}
	st := &screenshotTabState{state: state, baseDir: state.ScreenshotMgr.GetScreenshotDirectory(), selectedBatch: -1, selectedFile: -1}
	st.statusLabel = widget.NewLabel("就绪")
	st.detailEntry = widget.NewMultiLineEntry()
	st.detailEntry.Disable()
	st.detailEntry.SetMinRowsVisible(18)
	st.fileList = buildScreenshotFileList(st)
	st.batchList = buildScreenshotBatchList(st)
	st.fileList.OnSelected = func(id widget.ListItemID) { st.selectedFile = id; st.refreshDetail() }
	st.batchList.OnSelected = func(id widget.ListItemID) { st.selectedBatch = id; st.loadFiles() }
	st.refreshBatches()

	content := container.NewHSplit(
		container.NewBorder(widget.NewLabelWithStyle("截图批次", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, st.batchList),
		container.NewHSplit(
			container.NewBorder(widget.NewLabelWithStyle("批次文件", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, st.fileList),
			container.NewBorder(widget.NewLabelWithStyle("详情", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, st.detailEntry),
		),
	)
	content.Offset = 0.34
	if s, ok := content.Trailing.(*container.Split); ok {
		s.Offset = 0.45
	}
	st.refreshDetail()
	toolbar := buildScreenshotToolbar(window, st)
	return container.NewBorder(
		container.NewVBox(
			container.NewHBox(widget.NewIcon(theme.FolderIcon()), widget.NewLabelWithStyle("截图管理", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), layout.NewSpacer(), st.statusLabel),
			widget.NewLabel("截图根目录: "+st.baseDir), widget.NewSeparator(), toolbar, widget.NewSeparator(),
		), nil, nil, nil, content,
	)
}

type screenshotTabState struct {
	state         *AppState
	baseDir       string
	batches       []screenshotBatchItem
	files         []screenshotFileItem
	selectedBatch int
	selectedFile  int
	statusLabel   *widget.Label
	detailEntry   *widget.Entry
	fileList      *widget.List
	batchList     *widget.List
}

func (st *screenshotTabState) refreshDetail() {
	if st.selectedFile >= 0 && st.selectedFile < len(st.files) {
		st.detailEntry.SetText(formatScreenshotFileDetail(st.files[st.selectedFile]))
	} else if st.selectedBatch >= 0 && st.selectedBatch < len(st.batches) {
		st.detailEntry.SetText(formatScreenshotBatchDetail(st.batches[st.selectedBatch]))
	} else {
		st.detailEntry.SetText("选择左侧批次或中间文件查看详情。")
	}
}

func (st *screenshotTabState) loadFiles() {
	st.files = nil
	st.selectedFile = -1
	if st.selectedBatch < 0 || st.selectedBatch >= len(st.batches) {
		st.fileList.Refresh()
		st.refreshDetail()
		return
	}
	batchName := st.batches[st.selectedBatch].Name
	apiFiles, source, err := listScreenshotBatchFilesPreferAPI(context.Background(), batchName, st.baseDir)
	if err != nil {
		st.statusLabel.SetText("读取截图批次失败: " + err.Error())
		return
	}
	st.files = apiFiles
	st.fileList.Refresh()
	st.refreshDetail()
	st.statusLabel.SetText(fmt.Sprintf("已加载批次 %s 的 %d 个文件（来源: %s）", batchName, len(st.files), source))
}

func (st *screenshotTabState) refreshBatches() {
	st.batches = nil
	apiBatches, source, err := listScreenshotBatchesPreferAPI(context.Background(), st.baseDir)
	if err != nil {
		st.statusLabel.SetText("读取截图目录失败: " + err.Error())
		return
	}
	st.batches = apiBatches
	if st.selectedBatch >= len(st.batches) {
		st.selectedBatch = -1
	}
	st.batchList.Refresh()
	st.loadFiles()
	st.statusLabel.SetText(fmt.Sprintf("已加载 %d 个截图批次（来源: %s）", len(st.batches), source))
}

func buildScreenshotBatchList(st *screenshotTabState) *widget.List {
	return widget.NewList(
		func() int { return len(st.batches) },
		func() fyne.CanvasObject {
			p := widget.NewLabel("")
			p.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewVBox(p, widget.NewLabel(""))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := st.batches[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			if p, ok := box.Objects[0].(*widget.Label); ok {
				p.SetText(item.Name)
			}
			if s, ok := box.Objects[1].(*widget.Label); ok {
				s.SetText(fmt.Sprintf("文件 %d | %s", item.FileCount, item.UpdatedAt.Format("2006-01-02 15:04:05")))
			}
		},
	)
}

func buildScreenshotFileList(st *screenshotTabState) *widget.List {
	return widget.NewList(
		func() int { return len(st.files) },
		func() fyne.CanvasObject {
			p := widget.NewLabel("")
			p.TextStyle = fyne.TextStyle{Bold: true}
			return container.NewVBox(p, widget.NewLabel(""))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			item := st.files[id]
			box, ok := obj.(*fyne.Container)
			if !ok || len(box.Objects) < 2 {
				return
			}
			if p, ok := box.Objects[0].(*widget.Label); ok {
				p.SetText(item.Name)
			}
			if s, ok := box.Objects[1].(*widget.Label); ok {
				s.SetText(fmt.Sprintf("%s | %s", formatFileSize(item.Size), item.UpdatedAt.Format("2006-01-02 15:04:05")))
			}
		},
	)
}

func buildScreenshotToolbar(window fyne.Window, st *screenshotTabState) *fyne.Container {
	refreshBtn := widget.NewButtonWithIcon("刷新截图", theme.ViewRefreshIcon(), func() { st.refreshBatches() })
	openRootBtn := widget.NewButton("打开根目录", func() {
		if err := openPathInSystem(st.baseDir); err != nil {
			dialog.ShowError(err, window)
		}
	})
	openBatchBtn := widget.NewButton("打开所选批次", func() {
		if st.selectedBatch < 0 || st.selectedBatch >= len(st.batches) {
			dialog.ShowInformation("提示", "请先选择一个批次", window)
			return
		}
		if err := openPathInSystem(st.batches[st.selectedBatch].Path); err != nil {
			dialog.ShowError(err, window)
		}
	})
	openFileBtn := widget.NewButton("打开所选文件", func() {
		if st.selectedFile < 0 || st.selectedFile >= len(st.files) {
			dialog.ShowInformation("提示", "请先选择一个截图文件", window)
			return
		}
		if err := openPathInSystem(st.files[st.selectedFile].Path); err != nil {
			dialog.ShowError(err, window)
		}
	})
	deleteFileBtn := widget.NewButton("删除所选文件", func() {
		if st.selectedBatch < 0 || st.selectedBatch >= len(st.batches) || st.selectedFile < 0 || st.selectedFile >= len(st.files) {
			dialog.ShowInformation("提示", "请先选择批次和截图文件", window)
			return
		}
		batchName, fileName, filePath := st.batches[st.selectedBatch].Name, st.files[st.selectedFile].Name, st.files[st.selectedFile].Path
		dialog.ShowConfirm("删除截图文件", "确认删除所选截图文件？", func(confirm bool) {
			if !confirm {
				return
			}
			source, err := runDeleteScreenshotFilePreferAPI(context.Background(), batchName, fileName, filePath)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			st.loadFiles()
			st.statusLabel.SetText("截图文件已删除（来源: " + source + "）")
		}, window)
	})
	deleteBatchBtn := widget.NewButton("删除所选批次", func() {
		if st.selectedBatch < 0 || st.selectedBatch >= len(st.batches) {
			dialog.ShowInformation("提示", "请先选择一个批次", window)
			return
		}
		batchName, batchPath := st.batches[st.selectedBatch].Name, st.batches[st.selectedBatch].Path
		dialog.ShowConfirm("删除截图批次", "确认删除该批次及其全部截图文件？", func(confirm bool) {
			if !confirm {
				return
			}
			source, err := runDeleteScreenshotBatchPreferAPI(context.Background(), batchName, batchPath)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			st.refreshBatches()
			st.statusLabel.SetText("截图批次已删除（来源: " + source + "）")
		}, window)
	})
	return container.NewHBox(refreshBtn, openRootBtn, openBatchBtn, openFileBtn, deleteFileBtn, deleteBatchBtn)
}

func parseMonitorTargets(raw string) []monitorTarget {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	seen := make(map[string]bool)
	items := make([]monitorTarget, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, monitorTarget{InputURL: value})
	}
	return items
}

func probeMonitorTargets(ctx context.Context, items []monitorTarget, concurrency int) []monitorTarget {
	if concurrency <= 0 {
		concurrency = 5
	}
	results := make([]monitorTarget, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for index, item := range items {
		wg.Add(1)
		go func(i int, target monitorTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := target
			normalized, err := normalizeMonitorURL(target.InputURL)
			if err != nil {
				result.FormatValid = false
				result.ReasonType = "invalid_format"
				result.Reason = err.Error()
				results[i] = result
				return
			}
			result.FormatValid = true
			result.NormalizedURL = normalized
			reachable, statusCode, reasonType, reason := probeURLReachability(ctx, normalized)
			result.Reachable = reachable
			result.StatusCode = statusCode
			result.ReasonType = reasonType
			result.Reason = reason
			if reachable {
				result.BaselineStatus = "可设置基线"
			}
			results[i] = result
		}(index, item)
	}
	wg.Wait()
	return results
}

func normalizeMonitorURL(rawURL string) (string, error) {
	urlText := strings.TrimSpace(rawURL)
	if urlText == "" {
		return "", fmt.Errorf("empty URL")
	}
	if !strings.HasPrefix(urlText, "http://") && !strings.HasPrefix(urlText, "https://") {
		urlText = "https://" + urlText
	}
	parsed, err := url.ParseRequestURI(urlText)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("missing host")
	}
	return parsed.String(), nil
}

type guiTamperAPIResponse struct {
	Success bool                       `json:"success"`
	Mode    string                     `json:"mode"`
	Summary map[string]int             `json:"summary"`
	Results []tamper.TamperCheckResult `json:"results"`
}

type guiBaselineAPIResponse struct {
	Success bool                    `json:"success"`
	Summary map[string]int          `json:"summary"`
	Results []tamper.PageHashResult `json:"results"`
}

type guiBaselinesListResponse struct {
	Success bool     `json:"success"`
	URLs    []string `json:"urls"`
}

type guiTamperHistoryResponse struct {
	Success bool                     `json:"success"`
	Records []guiTamperHistoryRecord `json:"records"`
}

type guiTamperHistoryRecord struct {
	URL              string   `json:"url"`
	CheckType        string   `json:"check_type"`
	Tampered         bool     `json:"tampered"`
	TamperedSegments []string `json:"tampered_segments"`
	Timestamp        int64    `json:"timestamp"`
	CurrentFullHash  string   `json:"current_full_hash"`
	BaselineFullHash string   `json:"baseline_full_hash"`
}

func (r guiTamperHistoryRecord) toCheckRecord() *tamper.CheckRecord {
	rec := &tamper.CheckRecord{
		URL:              strings.TrimSpace(r.URL),
		CheckType:        strings.TrimSpace(r.CheckType),
		Tampered:         r.Tampered,
		TamperedSegments: r.TamperedSegments,
		Timestamp:        r.Timestamp,
	}
	if strings.TrimSpace(r.CurrentFullHash) != "" {
		rec.CurrentHash = &tamper.PageHashResult{FullHash: r.CurrentFullHash}
	}
	if strings.TrimSpace(r.BaselineFullHash) != "" {
		rec.BaselineHash = &tamper.PageHashResult{FullHash: r.BaselineFullHash}
	}
	if rec.URL == "" {
		return nil
	}
	if rec.CheckType == "" {
		rec.CheckType = "check"
	}
	return rec
}

type guiScreenshotBatchesResponse struct {
	Success bool                    `json:"success"`
	Batches []guiScreenshotBatchDTO `json:"batches"`
}

type guiScreenshotBatchDTO struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
	UpdatedAt int64  `json:"updated_at"`
}

type guiScreenshotFilesResponse struct {
	Success bool                   `json:"success"`
	Files   []guiScreenshotFileDTO `json:"files"`
}

type guiScreenshotFileDTO struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	UpdatedAt  int64  `json:"updated_at"`
	PreviewURL string `json:"preview_url"`
}

func resolveGUIAPIBase() string {
	if raw := strings.TrimSpace(os.Getenv("UNIMAP_API_BASE")); raw != "" {
		return strings.TrimRight(raw, "/")
	}
	return "http://127.0.0.1:8448"
}

func runTamperCheckViaAPI(ctx context.Context, urls []string, concurrency int) ([]tamper.TamperCheckResult, error) {
	base := resolveGUIAPIBase()
	type tamperCheckRequest struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
		Mode        string   `json:"mode"`
	}
	body, err := json.Marshal(tamperCheckRequest{
		URLs: urls, Concurrency: concurrency, Mode: tamper.DetectionModeRelaxed,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/tamper/check", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiTamperAPIResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("tamper api returned unsuccessful response")
	}
	return decoded.Results, nil
}

func runTamperCheckPreferAPI(ctx context.Context, state *AppState, urls []string, concurrency int) ([]tamper.TamperCheckResult, string, error) {
	apiResults, apiErr := runTamperCheckViaAPI(ctx, urls, concurrency)
	if apiErr == nil {
		return apiResults, "API", nil
	}

	localResults, localErr := state.Detector.BatchCheckTampering(ctx, urls, concurrency)
	if localErr == nil {
		return localResults, "本地", nil
	}

	return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func runSetBaselineViaAPI(ctx context.Context, urls []string, concurrency int) ([]tamper.PageHashResult, error) {
	base := resolveGUIAPIBase()
	type setBaselineRequest struct {
		URLs        []string `json:"urls"`
		Concurrency int      `json:"concurrency"`
	}
	body, err := json.Marshal(setBaselineRequest{URLs: urls, Concurrency: concurrency})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/tamper/baseline", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiBaselineAPIResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("baseline api returned unsuccessful response")
	}
	return decoded.Results, nil
}

func runSetBaselinePreferAPI(ctx context.Context, state *AppState, urls []string, concurrency int) ([]tamper.PageHashResult, string, error) {
	apiResults, apiErr := runSetBaselineViaAPI(ctx, urls, concurrency)
	if apiErr == nil {
		return apiResults, "API", nil
	}

	localResults, localErr := state.Detector.BatchSetBaseline(ctx, urls, concurrency)
	if localErr == nil {
		return localResults, "本地", nil
	}

	return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func listBaselinesViaAPI(ctx context.Context) ([]string, error) {
	base := resolveGUIAPIBase()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/tamper/baseline/list", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiBaselinesListResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("baseline list api returned unsuccessful response")
	}
	return decoded.URLs, nil
}

func listBaselinesPreferAPI(ctx context.Context, state *AppState) ([]string, string, error) {
	apiURLs, apiErr := listBaselinesViaAPI(ctx)
	if apiErr == nil {
		return apiURLs, "API", nil
	}

	localURLs, localErr := state.Detector.ListBaselines()
	if localErr == nil {
		return localURLs, "本地", nil
	}

	return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func runDeleteBaselineViaAPI(ctx context.Context, targetURL string) error {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/tamper/baseline/delete?url=%s", base, url.QueryEscape(targetURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func runDeleteBaselinePreferAPI(ctx context.Context, state *AppState, targetURL string) (string, error) {
	apiErr := runDeleteBaselineViaAPI(ctx, targetURL)
	if apiErr == nil {
		return "API", nil
	}

	localErr := state.Detector.DeleteBaseline(targetURL)
	if localErr == nil {
		return "本地", nil
	}

	return "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func listTamperHistoryViaAPI(ctx context.Context, limit int) ([]guiTamperHistoryRecord, error) {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/tamper/history?limit=%d", base, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiTamperHistoryResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("history api returned unsuccessful response")
	}
	return decoded.Records, nil
}

func runDeleteHistoryViaAPI(ctx context.Context, targetURL string) error {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/tamper/history/delete?url=%s", base, url.QueryEscape(targetURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func runDeleteHistoryPreferAPI(ctx context.Context, state *AppState, targetURL string) (string, error) {
	apiErr := runDeleteHistoryViaAPI(ctx, targetURL)
	if apiErr == nil {
		return "API", nil
	}

	localErr := state.TamperStorage.DeleteCheckRecords(targetURL)
	if localErr == nil {
		return "本地", nil
	}

	return "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func listScreenshotBatchesViaAPI(ctx context.Context, baseDir string) ([]screenshotBatchItem, error) {
	base := resolveGUIAPIBase()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/screenshot/batches", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiScreenshotBatchesResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("screenshot batches api returned unsuccessful response")
	}

	items := make([]screenshotBatchItem, 0, len(decoded.Batches))
	for _, item := range decoded.Batches {
		items = append(items, screenshotBatchItem{
			Name:      item.Name,
			Path:      filepath.Join(baseDir, item.Name),
			FileCount: item.FileCount,
			UpdatedAt: time.Unix(item.UpdatedAt, 0),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func listScreenshotBatchesPreferAPI(ctx context.Context, baseDir string) ([]screenshotBatchItem, string, error) {
	apiItems, apiErr := listScreenshotBatchesViaAPI(ctx, baseDir)
	if apiErr == nil {
		return apiItems, "API", nil
	}

	entries, localErr := os.ReadDir(baseDir)
	if localErr != nil {
		if os.IsNotExist(localErr) {
			return []screenshotBatchItem{}, "本地", nil
		}
		return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
	}

	items := make([]screenshotBatchItem, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fileCount := 0
		children, err := os.ReadDir(filepath.Join(baseDir, entry.Name()))
		if err == nil {
			for _, child := range children {
				if !child.IsDir() {
					fileCount++
				}
			}
		}
		items = append(items, screenshotBatchItem{
			Name:      entry.Name(),
			Path:      filepath.Join(baseDir, entry.Name()),
			FileCount: fileCount,
			UpdatedAt: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, "本地", nil
}

func listScreenshotBatchFilesViaAPI(ctx context.Context, batchName, baseDir string) ([]screenshotFileItem, error) {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/screenshot/batches/files?batch=%s", base, url.QueryEscape(batchName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded guiScreenshotFilesResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	if !decoded.Success {
		return nil, fmt.Errorf("screenshot files api returned unsuccessful response")
	}

	items := make([]screenshotFileItem, 0, len(decoded.Files))
	for _, item := range decoded.Files {
		items = append(items, screenshotFileItem{
			Name:       item.Name,
			Path:       filepath.Join(baseDir, batchName, item.Name),
			PreviewURL: item.PreviewURL,
			Size:       item.Size,
			UpdatedAt:  time.Unix(item.UpdatedAt, 0),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func listScreenshotBatchFilesPreferAPI(ctx context.Context, batchName, baseDir string) ([]screenshotFileItem, string, error) {
	apiItems, apiErr := listScreenshotBatchFilesViaAPI(ctx, batchName, baseDir)
	if apiErr == nil {
		return apiItems, "API", nil
	}

	batchPath := filepath.Join(baseDir, batchName)
	entries, localErr := os.ReadDir(batchPath)
	if localErr != nil {
		return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
	}

	items := make([]screenshotFileItem, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, screenshotFileItem{
			Name:      entry.Name(),
			Path:      filepath.Join(batchPath, entry.Name()),
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, "本地", nil
}

func runDeleteScreenshotBatchViaAPI(ctx context.Context, batchName string) error {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/screenshot/batches/delete?batch=%s", base, url.QueryEscape(batchName))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func runDeleteScreenshotBatchPreferAPI(ctx context.Context, batchName, batchPath string) (string, error) {
	apiErr := runDeleteScreenshotBatchViaAPI(ctx, batchName)
	if apiErr == nil {
		return "API", nil
	}

	localErr := os.RemoveAll(batchPath)
	if localErr == nil {
		return "本地", nil
	}

	return "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func runDeleteScreenshotFileViaAPI(ctx context.Context, batchName, fileName string) error {
	base := resolveGUIAPIBase()
	endpoint := fmt.Sprintf("%s/api/v1/screenshot/file/delete?batch=%s&file=%s", base, url.QueryEscape(batchName), url.QueryEscape(fileName))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func runDeleteScreenshotFilePreferAPI(ctx context.Context, batchName, fileName, filePath string) (string, error) {
	apiErr := runDeleteScreenshotFileViaAPI(ctx, batchName, fileName)
	if apiErr == nil {
		return "API", nil
	}

	localErr := os.Remove(filePath)
	if localErr == nil {
		return "本地", nil
	}

	return "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func runBatchScreenshotViaAPI(ctx context.Context, urls []string, batchID string, concurrency int) ([]screenshot.BatchScreenshotResult, error) {
	base := resolveGUIAPIBase()
	type batchScreenshotRequest struct {
		URLs        []string `json:"urls"`
		BatchID     string   `json:"batch_id"`
		Concurrency int      `json:"concurrency"`
	}
	body, err := json.Marshal(batchScreenshotRequest{
		URLs: urls, BatchID: batchID, Concurrency: concurrency,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/screenshot/batch-urls", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Results []screenshot.BatchScreenshotResult `json:"results"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, err
	}
	return decoded.Results, nil
}

func runBatchScreenshotPreferAPI(ctx context.Context, state *AppState, urls []string, batchID string, concurrency int) ([]screenshot.BatchScreenshotResult, string, error) {
	apiResults, apiErr := runBatchScreenshotViaAPI(ctx, urls, batchID, concurrency)
	if apiErr == nil {
		return apiResults, "API", nil
	}

	localResults, localErr := state.ScreenshotMgr.CaptureBatchURLs(ctx, urls, batchID, concurrency)
	if localErr == nil {
		return localResults, "本地", nil
	}

	return nil, "", fmt.Errorf("API 模式失败: %v; 本地模式失败: %v", apiErr, localErr)
}

func probeURLReachability(ctx context.Context, targetURL string) (bool, int, string, string) {
	client := &http.Client{Timeout: 8 * time.Second}
	var headErr error

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		errType, reason := classifyReachabilityError(err)
		return false, 0, errType, reason
	}

	headResp, err := client.Do(headReq)
	if err == nil {
		defer headResp.Body.Close()
		if headResp.StatusCode != http.StatusMethodNotAllowed {
			return true, headResp.StatusCode, "http_status", fmt.Sprintf("HTTP %d", headResp.StatusCode)
		}
	} else {
		headErr = err
	}

	getReq, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if reqErr != nil {
		if headErr != nil {
			errType, reason := classifyReachabilityError(headErr)
			return false, 0, errType, reason
		}
		errType, reason := classifyReachabilityError(reqErr)
		return false, 0, errType, reason
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		if headErr != nil {
			errType, reason := classifyReachabilityError(headErr)
			return false, 0, errType, reason
		}
		errType, reason := classifyReachabilityError(err)
		return false, 0, errType, reason
	}
	defer getResp.Body.Close()
	return true, getResp.StatusCode, "http_status", fmt.Sprintf("HTTP %d", getResp.StatusCode)
}

func classifyReachabilityError(err error) (string, string) {
	if err == nil {
		return "unknown", "unknown error"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns", dnsErr.Error()
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", netErr.Error()
	}
	var certErr x509.UnknownAuthorityError
	if errors.As(err, &certErr) {
		return "tls", err.Error()
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "tls") || strings.Contains(msg, "certificate") || strings.Contains(msg, "ssl"):
		return "tls", err.Error()
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "connrefused"):
		return "connection_refused", err.Error()
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "timed out"):
		return "timeout", err.Error()
	case strings.Contains(msg, "name not resolved") || strings.Contains(msg, "no such host") || strings.Contains(msg, "dns"):
		return "dns", err.Error()
	default:
		return "network", err.Error()
	}
}

func syncBaselineFlags(items []monitorTarget, baselines []string) {
	baselineSet := make(map[string]bool, len(baselines))
	for _, item := range baselines {
		baselineSet[item] = true
	}
	for index := range items {
		urlText := selectedMonitorURL(items[index])
		items[index].BaselineExists = baselineSet[urlText]
		if items[index].BaselineExists && items[index].BaselineStatus == "" {
			items[index].BaselineStatus = "已有基线"
		}
		if !items[index].BaselineExists && items[index].BaselineStatus == "已有基线" {
			items[index].BaselineStatus = ""
		}
	}
}

func reachableURLs(items []monitorTarget) []string {
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if item.Reachable {
			urls = append(urls, selectedMonitorURL(item))
		}
	}
	return urls
}

func annotateUnreachableBaseline(items []monitorTarget) {
	for index := range items {
		if !items[index].Reachable {
			items[index].BaselineStatus = "目标不可达"
		}
	}
}

func annotateUnreachableTamper(items []monitorTarget) {
	for index := range items {
		if !items[index].Reachable {
			items[index].TamperStatus = "目标不可达"
		}
	}
}

func annotateUnreachableScreenshot(items []monitorTarget) {
	for index := range items {
		if !items[index].Reachable {
			items[index].ScreenshotError = "目标不可达"
		}
	}
}

func applyBaselineResults(items []monitorTarget, results []tamper.PageHashResult) {
	resultMap := make(map[string]tamper.PageHashResult, len(results))
	for _, result := range results {
		resultMap[result.URL] = result
	}
	for index := range items {
		if !items[index].Reachable {
			items[index].BaselineStatus = "目标不可达"
			continue
		}
		result, ok := resultMap[selectedMonitorURL(items[index])]
		if !ok {
			items[index].BaselineStatus = "未返回结果"
			continue
		}
		if strings.HasPrefix(result.Status, "error") {
			items[index].BaselineStatus = result.Status
			continue
		}
		items[index].BaselineExists = true
		items[index].BaselineStatus = "基线已保存"
		items[index].Reason = fmt.Sprintf("标题: %s", result.Title)
	}
}

func applyTamperResults(items []monitorTarget, results []tamper.TamperCheckResult) {
	resultMap := make(map[string]tamper.TamperCheckResult, len(results))
	for _, result := range results {
		resultMap[result.URL] = result
	}
	for index := range items {
		if !items[index].Reachable {
			items[index].TamperStatus = "目标不可达"
			continue
		}
		result, ok := resultMap[selectedMonitorURL(items[index])]
		if !ok {
			items[index].TamperStatus = "未返回结果"
			continue
		}
		items[index].Tampered = result.Tampered
		items[index].TamperedSegments = result.TamperedSegments
		items[index].Changes = result.Changes
		items[index].LastCheckedAt = result.Timestamp
		switch result.Status {
		case "no_baseline":
			items[index].TamperStatus = "无可比对基线"
		case "unreachable":
			items[index].TamperStatus = fmt.Sprintf("不可达: %s", strings.TrimSpace(result.ErrorMessage))
		case "tampered":
			items[index].TamperStatus = "检测到页面变化"
		case "normal":
			items[index].TamperStatus = "页面正常"
		default:
			items[index].TamperStatus = result.Status
		}
	}
}

func applyScreenshotResults(items []monitorTarget, results []screenshot.BatchScreenshotResult) {
	resultMap := make(map[string]screenshot.BatchScreenshotResult, len(results))
	for _, result := range results {
		resultMap[result.URL] = result
	}
	for index := range items {
		if !items[index].Reachable {
			items[index].ScreenshotError = "目标不可达"
			continue
		}
		result, ok := resultMap[selectedMonitorURL(items[index])]
		if !ok {
			items[index].ScreenshotError = "未返回截图结果"
			continue
		}
		if result.Success {
			items[index].ScreenshotPath = result.FilePath
			items[index].ScreenshotError = ""
			continue
		}
		items[index].ScreenshotPath = ""
		items[index].ScreenshotError = result.Error
	}
}

func summarizeProbeStatus(items []monitorTarget) string {
	reachable := 0
	invalid := 0
	for _, item := range items {
		if item.Reachable {
			reachable++
		}
		if !item.FormatValid {
			invalid++
		}
	}
	return fmt.Sprintf("探活完成: 可达 %d / %d，格式非法 %d", reachable, len(items), invalid)
}

func summarizeBaselineStatus(items []monitorTarget) string {
	saved := 0
	failed := 0
	for _, item := range items {
		switch {
		case item.BaselineStatus == "基线已保存":
			saved++
		case item.BaselineStatus != "" && item.BaselineStatus != "目标不可达":
			failed++
		}
	}
	return fmt.Sprintf("基线设置完成: 成功 %d，失败 %d", saved, failed)
}

func summarizeTamperStatus(items []monitorTarget) string {
	tampered := 0
	normal := 0
	noBaseline := 0
	for _, item := range items {
		switch item.TamperStatus {
		case "检测到页面变化":
			tampered++
		case "页面正常":
			normal++
		case "无可比对基线":
			noBaseline++
		}
	}
	return fmt.Sprintf("篡改检测完成: 正常 %d，变化 %d，无基线 %d", normal, tampered, noBaseline)
}

func summarizeScreenshotStatus(items []monitorTarget) string {
	success := 0
	for _, item := range items {
		if strings.TrimSpace(item.ScreenshotPath) != "" {
			success++
		}
	}
	return fmt.Sprintf("批量截图完成: 成功 %d / %d", success, len(items))
}

func selectedMonitorURL(item monitorTarget) string {
	if strings.TrimSpace(item.NormalizedURL) != "" {
		return item.NormalizedURL
	}
	return strings.TrimSpace(item.InputURL)
}

func monitorTargetTitle(item monitorTarget) string {
	return selectedMonitorURL(item)
}

func monitorTargetSubtitle(item monitorTarget) string {
	probeStatus := "格式非法"
	if item.FormatValid {
		if item.Reachable {
			probeStatus = fmt.Sprintf("可达 (%d)", item.StatusCode)
		} else {
			probeStatus = "不可达"
		}
	}
	baselineStatus := item.BaselineStatus
	if baselineStatus == "" {
		if item.BaselineExists {
			baselineStatus = "已有基线"
		} else {
			baselineStatus = "未设置"
		}
	}
	tamperStatus := item.TamperStatus
	if tamperStatus == "" {
		tamperStatus = "未检测"
	}
	return fmt.Sprintf("探活: %s | 基线: %s | 篡改: %s", probeStatus, baselineStatus, tamperStatus)
}

func formatMonitorDetail(item monitorTarget) string {
	lines := []string{
		"URL: " + selectedMonitorURL(item),
		"探活状态: " + monitorTargetSubtitle(item),
	}
	if item.ReasonType != "" {
		lines = append(lines, "失败类型: "+formatReasonType(item.ReasonType))
	}
	if item.Reason != "" {
		lines = append(lines, "说明: "+item.Reason)
	}
	if item.LastCheckedAt > 0 {
		lines = append(lines, "最近检测: "+formatTimestamp(item.LastCheckedAt))
	}
	if len(item.TamperedSegments) > 0 {
		lines = append(lines, "变更段落: "+strings.Join(item.TamperedSegments, ", "))
	}
	if len(item.Changes) > 0 {
		lines = append(lines, "变更详情:")
		for _, change := range item.Changes {
			lines = append(lines, fmt.Sprintf("- %s | %s | %s", change.Segment, change.ChangeType, change.Description))
		}
	}
	if item.ScreenshotPath != "" {
		lines = append(lines, "截图文件: "+item.ScreenshotPath)
	}
	if item.ScreenshotError != "" {
		lines = append(lines, "截图状态: "+item.ScreenshotError)
	}
	return strings.Join(lines, "\n")
}

func formatBaselineDetail(state *AppState, baselineURL string, records []*tamper.CheckRecord) string {
	stats, _ := state.TamperStorage.GetCheckStats(baselineURL)
	lines := []string{"URL: " + baselineURL}
	if stats.TotalChecks > 0 {
		lines = append(lines, fmt.Sprintf("检测总数: %d", stats.TotalChecks))
		lines = append(lines, fmt.Sprintf("篡改次数: %d", stats.TamperedCount))
		lines = append(lines, fmt.Sprintf("安全次数: %d", stats.SafeCount))
	}
	if len(records) == 0 {
		lines = append(lines, "最近记录: 暂无")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "最近记录:")
	for _, record := range records {
		lines = append(lines, fmt.Sprintf("- %s | %s | tampered=%s", formatTimestamp(record.Timestamp), record.CheckType, yesNo(record.Tampered)))
	}
	return strings.Join(lines, "\n")
}

func baselineMetaText(state *AppState, baselineURL string) string {
	stats, err := state.TamperStorage.GetCheckStats(baselineURL)
	if err != nil || stats.TotalChecks == 0 {
		return "已保存基线"
	}
	return fmt.Sprintf("检测 %d 次 | 最近 %s", stats.TotalChecks, formatAnyTimestamp(stats.LastCheckTime))
}

func formatHistoryURLDetail(item historyURLItem, stats tamper.CheckStats) string {
	lines := []string{
		"URL: " + item.URL,
		"是否有基线: " + yesNo(item.HasBaseline),
		fmt.Sprintf("历史记录数: %d", item.RecordCount),
		"最近检测: " + formatTimestamp(item.LastCheckAt),
	}
	if stats.TotalChecks > 0 {
		lines = append(lines,
			fmt.Sprintf("篡改次数: %d", stats.TamperedCount),
			fmt.Sprintf("安全次数: %d", stats.SafeCount),
			fmt.Sprintf("首次检测次数: %d", stats.FirstCheckCount),
		)
	}
	return strings.Join(lines, "\n")
}

func historyRecordSummary(record *tamper.CheckRecord) string {
	parts := []string{"tampered=" + yesNo(record.Tampered)}
	if len(record.TamperedSegments) > 0 {
		parts = append(parts, "segments="+strings.Join(record.TamperedSegments, ","))
	}
	return strings.Join(parts, " | ")
}

func formatCheckRecordDetail(record *tamper.CheckRecord) string {
	lines := []string{
		"URL: " + record.URL,
		"检测类型: " + record.CheckType,
		"时间: " + formatTimestamp(record.Timestamp),
		"是否篡改: " + yesNo(record.Tampered),
	}
	if record.CurrentHash != nil {
		lines = append(lines, "当前标题: "+record.CurrentHash.Title)
		lines = append(lines, "当前哈希: "+record.CurrentHash.FullHash)
	}
	if record.BaselineHash != nil {
		lines = append(lines, "基线哈希: "+record.BaselineHash.FullHash)
	}
	if len(record.TamperedSegments) > 0 {
		lines = append(lines, "变更段落: "+strings.Join(record.TamperedSegments, ", "))
	}
	if len(record.Changes) > 0 {
		lines = append(lines, "变更详情:")
		for _, change := range record.Changes {
			lines = append(lines, fmt.Sprintf("- %s | %s | %s", change.Segment, change.ChangeType, change.Description))
		}
	}
	return strings.Join(lines, "\n")
}

func formatScreenshotBatchDetail(item screenshotBatchItem) string {
	return strings.Join([]string{
		"批次: " + item.Name,
		"目录: " + item.Path,
		fmt.Sprintf("文件数: %d", item.FileCount),
		"更新时间: " + item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, "\n")
}

func formatScreenshotFileDetail(item screenshotFileItem) string {
	return strings.Join([]string{
		"文件: " + item.Name,
		"路径: " + item.Path,
		"大小: " + formatFileSize(item.Size),
		"更新时间: " + item.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, "\n")
}

func formatReasonType(reasonType string) string {
	switch reasonType {
	case "invalid_format":
		return "格式非法"
	case "dns":
		return "DNS 解析失败"
	case "timeout":
		return "连接超时"
	case "tls":
		return "TLS/证书错误"
	case "connection_refused":
		return "连接被拒绝"
	case "http_status":
		return "HTTP 状态"
	case "network":
		return "网络错误"
	default:
		return reasonType
	}
}

func formatTimestamp(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func formatAnyTimestamp(value interface{}) string {
	switch v := value.(type) {
	case int64:
		return formatTimestamp(v)
	case int:
		return formatTimestamp(int64(v))
	case float64:
		return formatTimestamp(int64(v))
	default:
		return "-"
	}
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func openPathInSystem(path string) error {
	cleanPath := filepath.Clean(path)
	if _, err := os.Stat(cleanPath); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", cleanPath)
	case "darwin":
		cmd = exec.Command("open", cleanPath)
	default:
		cmd = exec.Command("xdg-open", cleanPath)
	}
	return cmd.Start()
}
