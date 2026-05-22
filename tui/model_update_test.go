package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/speedtester"
)

// TestTUIModelUpdate tests the TUI model update logic
func TestTUIModelUpdate(t *testing.T) {
	// Create a result channel
	resultChannel := make(chan *speedtester.Result, 10)

	// Create a new TUI model
	model := NewTUIModel(speedtester.SpeedModeDownload, 3, resultChannel, nil)

	// Verify initial state
	if model.mode != speedtester.SpeedModeDownload {
		t.Errorf("Expected mode to be %v, got %v", speedtester.SpeedModeDownload, model.mode)
	}
	if model.totalProxies != 3 {
		t.Errorf("Expected totalProxies to be 3, got %d", model.totalProxies)
	}
	if model.currentProxy != 0 {
		t.Errorf("Expected currentProxy to be 0, got %d", model.currentProxy)
	}
	if len(model.results) != 0 {
		t.Errorf("Expected results length to be 0, got %d", len(model.results))
	}
	if model.testing != true {
		t.Errorf("Expected testing to be true, got %v", model.testing)
	}
	if model.quitting != false {
		t.Errorf("Expected quitting to be false, got %v", model.quitting)
	}

	// Create test results
	result1 := &speedtester.Result{
		ProxyName:     "Proxy 1",
		ProxyType:     "SS",
		Latency:       100 * time.Millisecond,
		Jitter:        50 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadSpeed: 15 * 1024 * 1024, // 15 MB/s
		UploadSpeed:   8 * 1024 * 1024,  // 8 MB/s
		ProxyConfig:   map[string]any{},
	}

	result2 := &speedtester.Result{
		ProxyName:     "Proxy 2",
		ProxyType:     "Trojan",
		Latency:       200 * time.Millisecond,
		Jitter:        100 * time.Millisecond,
		PacketLoss:    15.0,
		DownloadSpeed: 8 * 1024 * 1024, // 8 MB/s
		UploadSpeed:   3 * 1024 * 1024, // 3 MB/s
		ProxyConfig:   map[string]any{},
	}

	result3 := &speedtester.Result{
		ProxyName:     "Proxy 3",
		ProxyType:     "Vmess",
		Latency:       300 * time.Millisecond,
		Jitter:        200 * time.Millisecond,
		PacketLoss:    25.0,
		DownloadSpeed: 3 * 1024 * 1024, // 3 MB/s
		UploadSpeed:   1 * 1024 * 1024, // 1 MB/s
		ProxyConfig:   map[string]any{},
	}

	// Send first result
	resultChannel <- result1
	updatedModel, cmd := model.Update(resultMsg{result: result1})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 1 {
		t.Errorf("Expected currentProxy to be 1, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 1 {
		t.Errorf("Expected results length to be 1, got %d", len(updatedModel.(tuiModel).results))
	}
	if updatedModel.(tuiModel).results[0] != result1 {
		t.Error("Expected first result to be result1")
	}

	// Send second result
	resultChannel <- result2
	updatedModel, cmd = updatedModel.(tuiModel).Update(resultMsg{result: result2})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 2 {
		t.Errorf("Expected currentProxy to be 2, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 2 {
		t.Errorf("Expected results length to be 2, got %d", len(updatedModel.(tuiModel).results))
	}

	// Send third result
	resultChannel <- result3
	updatedModel, cmd = updatedModel.(tuiModel).Update(resultMsg{result: result3})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 3 {
		t.Errorf("Expected currentProxy to be 3, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 3 {
		t.Errorf("Expected results length to be 3, got %d", len(updatedModel.(tuiModel).results))
	}

	// Flush updates to apply sorting and table refresh.
	updatedModel, _ = updatedModel.(tuiModel).Update(flushResultsMsg{})

	// Verify results are sorted by download speed (descending)
	// result1 (15 MB/s) > result2 (8 MB/s) > result3 (3 MB/s)
	if updatedModel.(tuiModel).results[0] != result1 {
		t.Error("Expected first result to be result1")
	}
	if updatedModel.(tuiModel).results[1] != result2 {
		t.Error("Expected second result to be result2")
	}
	if updatedModel.(tuiModel).results[2] != result3 {
		t.Error("Expected third result to be result3")
	}

	// Send done message
	updatedModel, cmd = updatedModel.(tuiModel).Update(doneMsg{})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil (progress update command)")
	}
	if updatedModel.(tuiModel).testing != false {
		t.Errorf("Expected testing to be false, got %v", updatedModel.(tuiModel).testing)
	}
	// Verify progress is complete by checking the percent
	if updatedModel.(tuiModel).progress.Percent() != 1.0 {
		t.Errorf("Expected progress percent to be 1.0, got %f", updatedModel.(tuiModel).progress.Percent())
	}
}

// TestTUIModelDoneMsgTriggersSaveCallback verifies that receiving doneMsg
// triggers saveCallback asynchronously, shows "正在保存..." immediately,
// and returns tea.Quit after saveCompleteMsg.
func TestTUIModelDoneMsgTriggersSaveCallback(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	saveCalled := false

	saveCallback := func() SaveResult {
		saveCalled = true
		return SaveResult{
			YamlPath: "/tmp/out.yaml",
			CsvPath:  "/tmp/out.csv",
			GistOK:   true,
		}
	}

	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel, saveCallback)

	result := &speedtester.Result{
		ProxyName:     "TestProxy",
		ProxyType:     "SS",
		Latency:       50 * time.Millisecond,
		DownloadSpeed: 10 * 1024 * 1024,
		ProxyConfig:   map[string]any{"name": "TestProxy", "server": "1.2.3.4"},
	}
	resultChannel <- result
	close(resultChannel)

	updatedModel, _ := model.Update(resultMsg{result: result})

	// doneMsg 应返回异步 cmd，不阻塞
	m := updatedModel.(tuiModel)
	updatedModel, cmd := m.Update(doneMsg{})
	afterDone := updatedModel.(tuiModel)

	// 应立即显示 "正在保存..."
	if afterDone.saveStatus != "正在保存..." {
		t.Errorf("doneMsg 后 saveStatus 应为 '正在保存...', got: %s", afterDone.saveStatus)
	}
	if afterDone.testing {
		t.Error("testing should be false after doneMsg")
	}
	// 应返回 cmd（异步保存），不是 nil
	if cmd == nil {
		t.Fatal("doneMsg 应返回非 nil cmd（异步保存）")
	}

	// 执行异步 cmd → 应得到 saveCompleteMsg
	saveMsg := cmd()
	ssm, ok := saveMsg.(saveCompleteMsg)
	if !ok {
		t.Fatalf("期望 saveCompleteMsg, got %T", saveMsg)
	}
	if !saveCalled {
		t.Error("saveCallback 未被调用")
	}
	if ssm.YamlPath != "/tmp/out.yaml" {
		t.Errorf("期望 yaml 路径 /tmp/out.yaml, got %s", ssm.YamlPath)
	}
	if !ssm.GistOK {
		t.Error("期望 GistOK 为 true")
	}

	// 处理 saveCompleteMsg → 应设置最终状态并返回 tea.Quit
	updatedModel, quitCmd := afterDone.Update(saveCompleteMsg{SaveResult: ssm.SaveResult})
	finalModel := updatedModel.(tuiModel)

	if !strings.Contains(finalModel.saveStatus, "/tmp/out.yaml") {
		t.Errorf("saveStatus 应包含 yaml 路径, got: %s", finalModel.saveStatus)
	}
	if !strings.Contains(finalModel.saveStatus, "gist 已上传") {
		t.Errorf("saveStatus 应显示 gist 已上传, got: %s", finalModel.saveStatus)
	}
	if quitCmd == nil {
		t.Fatal("saveCompleteMsg 应返回 tea.Quit")
	}
	quitMsg := quitCmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Errorf("期望 tea.QuitMsg, got %T", quitMsg)
	}

	// View() 应包含保存状态
	view := finalModel.View()
	if !strings.Contains(view, "gist 已上传") {
		t.Errorf("View() 应包含保存状态, got:\n%s", view)
	}
}

// TestTUIModelDoneMsgNilCallback verifies that when saveCallback is nil,
// doneMsg returns tea.Quit directly without panic.
func TestTUIModelDoneMsgNilCallback(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel, nil)

	result := &speedtester.Result{
		ProxyName:   "TestProxy",
		ProxyType:   "SS",
		Latency:     50 * time.Millisecond,
		ProxyConfig: map[string]any{"name": "TestProxy", "server": "1.2.3.4"},
	}
	resultChannel <- result
	close(resultChannel)

	updatedModel, _ := model.Update(resultMsg{result: result})
	updatedModel, cmd := updatedModel.(tuiModel).Update(doneMsg{})

	m := updatedModel.(tuiModel)
	if m.testing {
		t.Error("testing should be false after doneMsg")
	}
	if m.saveStatus != "" {
		t.Error("无 saveCallback 时 saveStatus 应为空")
	}
	if cmd == nil {
		t.Fatal("doneMsg 应返回 tea.Quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("期望 tea.QuitMsg, got %T", msg)
	}
}

// TestTUIModelSaveStatusMsgGistError verifies error reporting in save status.
func TestTUIModelSaveStatusMsgGistError(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel, nil)

	ssm := saveStatusMsg{
		YamlPath: "/tmp/out.yaml",
		CsvPath:  "/tmp/out.csv",
		GistErr:  "401 Unauthorized",
	}
	updatedModel, _ := model.Update(ssm)
	m := updatedModel.(tuiModel)

	if !strings.Contains(m.saveStatus, "gist 上传失败") {
		t.Errorf("saveStatus 应显示 gist 上传失败, got: %s", m.saveStatus)
	}
	if !strings.Contains(m.saveStatus, "401 Unauthorized") {
		t.Errorf("saveStatus 应包含错误详情, got: %s", m.saveStatus)
	}
}

// TestTUIModelSaveStatusMsgRepo verifies repo upload status.
func TestTUIModelSaveStatusMsgRepo(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel, nil)

	ssm := saveStatusMsg{
		YamlPath: "/tmp/out.yaml",
		CsvPath:  "/tmp/out.csv",
		RepoOK:   true,
	}
	updatedModel, _ := model.Update(ssm)
	m := updatedModel.(tuiModel)

	if !strings.Contains(m.saveStatus, "repo 已上传") {
		t.Errorf("saveStatus 应显示 repo 已上传, got: %s", m.saveStatus)
	}
}

// TestTUIModelUpdateFastMode tests the TUI model update logic in fast mode
func TestTUIModelUpdateFastMode(t *testing.T) {
	// Create a result channel
	resultChannel := make(chan *speedtester.Result, 10)

	// Create a new TUI model in fast mode
	model := NewTUIModel(speedtester.SpeedModeFast, 3, resultChannel, nil)

	// Verify initial state
	if model.mode != speedtester.SpeedModeFast {
		t.Errorf("Expected mode to be %v, got %v", speedtester.SpeedModeFast, model.mode)
	}

	// Create test results (only latency matters in fast mode)
	result1 := &speedtester.Result{
		ProxyName:   "Proxy 1",
		ProxyType:   "SS",
		Latency:     300 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	result2 := &speedtester.Result{
		ProxyName:   "Proxy 2",
		ProxyType:   "Trojan",
		Latency:     100 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	result3 := &speedtester.Result{
		ProxyName:   "Proxy 3",
		ProxyType:   "Vmess",
		Latency:     200 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	// Send results
	resultChannel <- result1
	updatedModel, _ := model.Update(resultMsg{result: result1})

	resultChannel <- result2
	updatedModel, _ = updatedModel.(tuiModel).Update(resultMsg{result: result2})

	resultChannel <- result3
	updatedModel, _ = updatedModel.(tuiModel).Update(resultMsg{result: result3})

	updatedModel, _ = updatedModel.(tuiModel).Update(flushResultsMsg{})

	// Verify results are sorted by latency (ascending)
	// result2 (100ms) < result3 (200ms) < result1 (300ms)
	if updatedModel.(tuiModel).results[0] != result2 {
		t.Error("Expected first result to be result2")
	}
	if updatedModel.(tuiModel).results[1] != result3 {
		t.Error("Expected second result to be result3")
	}
	if updatedModel.(tuiModel).results[2] != result1 {
		t.Error("Expected third result to be result1")
	}
}
