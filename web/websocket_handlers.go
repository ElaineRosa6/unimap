package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/metrics"
	"github.com/unimap/project/internal/model"
	"github.com/unimap/project/internal/service"
)

const (
	// pongWait is the maximum time to wait for a pong response after sending a ping.
	// Must be greater than pingInterval to allow time for the pong to arrive.
	pongWait = 60 * time.Second

	// pingInterval is the interval between protocol-level ping frames.
	// Must be less than pongWait. The connection will be closed if no pong
	// is received within pongWait after each ping.
	pingInterval = 30 * time.Second

	// writeWait is the maximum time allowed for a single write operation.
	writeWait = 10 * time.Second
)

// setWriteDeadline sets the write deadline on the connection.
// Should be called while holding the writeMu lock.
func setWriteDeadline(conn *websocket.Conn) {
	conn.SetWriteDeadline(time.Now().Add(writeWait))
}

func generateConnectionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.validateWebSocketRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil { logger.Errorf("WebSocket upgrade failed: %v", err); return }
	defer conn.Close()

	connID := generateConnectionID()
	managed := &managedConn{conn: conn}
	connCtx, cancelConn := context.WithCancel(r.Context())
	writeJSON := func(v interface{}) error {
		managed.writeMu.Lock()
		defer managed.writeMu.Unlock()
		setWriteDeadline(conn)
		return conn.WriteJSON(v)
	}
	done := make(chan struct{})

	s.connManager.mutex.Lock()
	s.connManager.connections[connID] = managed
	s.connManager.mutex.Unlock()
	metrics.IncWebSocketConnection()

	defer func() {
		cancelConn()
		close(done)
		s.connManager.mutex.Lock()
		delete(s.connManager.connections, connID)
		s.connManager.mutex.Unlock()
		metrics.DecWebSocketConnection()
		logger.Infof("WebSocket connection closed: %s", connID)
	}()

	wsSetupPingPong(conn, managed, done)
	wsMessageLoop(s, conn, connCtx, connID, writeJSON)
}

// wsSetupPingPong 设置 WebSocket ping/pong 心跳
func wsSetupPingPong(conn *websocket.Conn, managed *managedConn, done chan struct{}) {
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				managed.writeMu.Lock()
				setWriteDeadline(conn)
				err := conn.WriteMessage(websocket.PingMessage, nil)
				managed.writeMu.Unlock()
				if err != nil { logger.Errorf("WebSocket ping error: %v", err); return }
			}
		}
	}()
}

// wsMessageLoop WebSocket 消息处理循环
func wsMessageLoop(s *Server, conn *websocket.Conn, connCtx context.Context, connID string, writeJSON func(interface{}) error) {
	for {
		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("WebSocket read error: %v", err)
			}
			return
		}
		metrics.IncWebSocketMessage("inbound")
		msgType, ok := message["type"].(string)
		if !ok {
			continue
		}
		switch msgType {
		case "ping":
			metrics.IncWebSocketMessage("outbound")
			if err := writeJSON(map[string]interface{}{"type": "pong"}); err != nil {
				logger.Errorf("WebSocket write error: %v", err)
			}
		case "pong":
			conn.SetReadDeadline(time.Now().Add(pongWait))
		case "query":
			s.handleWebSocketQuery(connCtx, connID, message, writeJSON)
		}
	}
}

// validateWebSocketRequest 验证WebSocket连接请求
func (s *Server) validateWebSocketRequest(r *http.Request) bool {
	adminToken := s.adminToken()
	if adminToken == "" {
		return true // auth not configured
	}

	// 1. Session cookie (browser sends automatically)
	token := s.getSessionToken(r)
	// 2. Header (non-browser clients)
	if token == "" {
		token = r.Header.Get("X-WebSocket-Token")
	}

	if token == "" {
		logger.Warn("WebSocket connection rejected: missing token")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
		logger.Warn("WebSocket connection rejected: invalid token")
		return false
	}
	return true
}

// handleWebSocketQuery 处理WebSocket查询请求
func (s *Server) handleWebSocketQuery(ctx context.Context, connID string, message map[string]interface{}, writeJSON func(interface{}) error) {
	query, engines, apiEngines, pageSize, browserQuery, browserAction, err := parseWSQueryParams(message, s.orchestrator)
	if err != nil {
		if wErr := writeJSON(map[string]interface{}{"type": "query_error", "error": err.Error()}); wErr != nil {
			logger.Errorf("WebSocket write error: %v", wErr)
		}
		return
	}

	queryID := fmt.Sprintf("%d", time.Now().UnixNano())
	status := &QueryStatus{
		ID: queryID, Query: query, Engines: engines, Status: "running",
		Results: []model.UnifiedAsset{}, Errors: []string{}, StartTime: time.Now(),
	}
	s.queryMutex.Lock()
	s.queryStatus[queryID] = status
	s.queryMutex.Unlock()

	if wErr := writeJSON(map[string]interface{}{
		"type": "query_start", "query_id": queryID, "status": status,
	}); wErr != nil {
		logger.Errorf("WebSocket write error: %v", wErr)
	}

	go s.executeWSQueryAsync(ctx, connID, queryID, query, engines, apiEngines, pageSize, browserQuery, browserAction, writeJSON)
}

// parseWSQueryParams 解析并验证 WebSocket 查询参数
func parseWSQueryParams(message map[string]interface{}, orch interface{ ListAdapters() []string }) (
	query string, engines []string, apiEngines []string, pageSize int, browserQuery bool, browserAction string, err error,
) {
	query, _ = message["query"].(string)
	query = strings.TrimSpace(query)
	if err = validateQueryInput(query); err != nil {
		return
	}
	pageSize = parseWSInt(message["page_size"], 50)
	browserQuery = parseWSBool(message["browser_query"])
	if ba, ok := message["browser_action"].(string); ok {
		browserAction = strings.TrimSpace(ba)
	}
	engines = parseWSStringList(message["engines"])
	if len(engines) == 0 {
		defaultEngines := orch.ListAdapters()
		if len(defaultEngines) > 0 {
			engines = []string{defaultEngines[0]}
		}
	}
	if len(engines) == 0 {
		err = fmt.Errorf("no engines configured/registered. Please set API keys in configs/config.yaml and enable at least one engine.")
		return
	}
	// api_engines: 浏览器查询模式下，API 只查询有 Key 的引擎
	apiEngines = parseWSStringList(message["api_engines"])
	if len(apiEngines) == 0 {
		apiEngines = engines
	}
	return
}

// executeWSQueryAsync 异步执行 WebSocket 查询
func (s *Server) executeWSQueryAsync(ctx context.Context, connID, queryID, query string, engines []string, apiEngines []string, pageSize int, browserQuery bool, browserAction string, writeJSON func(interface{}) error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("WebSocket query panic for %s: %v", queryID, r)
			s.sendToConn(connID, map[string]interface{}{
				"type": "query_error", "error": fmt.Sprintf("internal error: query %s failed", queryID),
			})
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}

	// API 查询和浏览器查询独立超时，互不拖累
	apiCtx, apiCancel := context.WithTimeout(ctx, 60*time.Second)
	defer apiCancel()

	if browserQuery {
		s.updateQueryProgress(connID, queryID, 5)
	}
	browserQueryCh := s.runBrowserQueryAsync(ctx, query, engines, browserQuery, browserAction, queryID, func(done, total int, engine string, err error) {
		if total <= 0 {
			return
		}
		progress := 5 + (float64(done)/float64(total))*45
		if progress > 50 {
			progress = 50
		}
		s.updateQueryProgress(connID, queryID, progress)
	})

	// API 查询在独立 goroutine 中执行
	type apiResult struct {
		resp *service.QueryResponse
		err  error
	}
	apiCh := make(chan apiResult, 1)
	go func() {
		resp, err := s.service.Query(apiCtx, service.QueryRequest{
			Query: query, Engines: apiEngines, PageSize: pageSize, ProcessData: true,
		})
		apiCh <- apiResult{resp, err}
	}()

	// 等待两个查询都完成（或超时）
	var resp *service.QueryResponse
	var queryErr error
	var browserOutcome browserQueryOutcome

	apiDone := false
	browserDone := browserQueryCh == nil
	for !apiDone || !browserDone {
		select {
		case r := <-apiCh:
			apiDone = true
			resp = r.resp
			queryErr = r.err
		case outcome := <-browserQueryCh:
			browserDone = true
			browserOutcome = outcome
		}
	}
	if queryErr == nil && apiCtx.Err() != nil {
		queryErr = fmt.Errorf("query timeout after 60s: %v", apiCtx.Err())
	}

	statusCopy := s.finalizeWSQueryStatus(queryID, query, engines, queryErr, resp, browserOutcome, browserAction)
	s.scheduleWSQueryCleanup(queryID)

	var errMsg string
	if queryErr != nil {
		errMsg = fmt.Sprintf("Query failed: %v", queryErr)
	}
	resultsPayload := buildQueryAPIPayload(query, engines, resp, browserOutcome, browserAction, errMsg)
	if errMsg != "" {
		resultsPayload["error"] = errMsg
	}
	if wErr := writeJSON(map[string]interface{}{
		"type": "query_complete", "query_id": queryID, "status": statusCopy, "results": resultsPayload,
	}); wErr != nil {
		logger.Errorf("WebSocket write error: %v", wErr)
	}
}

// finalizeWSQueryStatus 在锁内更新查询状态并返回副本
func (s *Server) finalizeWSQueryStatus(queryID, query string, engines []string, queryErr error, resp *service.QueryResponse, browserOutcome browserQueryOutcome, browserAction string) QueryStatus {
	s.queryMutex.Lock()
	defer s.queryMutex.Unlock()
	st := s.queryStatus[queryID]
	if st == nil {
		return QueryStatus{}
	}
	if queryErr != nil {
		st.Errors = append(st.Errors, fmt.Sprintf("Query failed: %v", queryErr))
		st.Errors = appendUniqueStrings(st.Errors, browserOutcome.Errors)
		st.Errors = appendUniqueStrings(st.Errors, browserOutcome.AutoCaptureErrors)
		st.Status = "error"
	} else {
		payload := buildQueryAPIPayload(query, engines, resp, browserOutcome, browserAction)
		if assets, ok := payload["assets"].([]model.UnifiedAsset); ok {
			st.Results = assets
		} else {
			st.Results = resp.Assets
		}
		if totalCount, ok := payload["totalCount"].(int); ok {
			st.TotalCount = totalCount
		} else {
			st.TotalCount = resp.TotalCount
		}
		if errs, ok := payload["errors"].([]string); ok {
			st.Errors = errs
		} else {
			st.Errors = resp.Errors
		}
		st.Status = "completed"
	}
	st.Progress = 100
	st.EndTime = time.Now()

	statusCopy := *st
	if st.Results != nil {
		statusCopy.Results = append([]model.UnifiedAsset(nil), st.Results...)
	}
	if st.Errors != nil {
		statusCopy.Errors = append([]string(nil), st.Errors...)
	}
	return statusCopy
}

// scheduleWSQueryCleanup 延迟清理查询状态
func (s *Server) scheduleWSQueryCleanup(queryID string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("WebSocket query cleanup panic for %s: %v", queryID, r)
			}
		}()
		select {
		case <-time.After(5 * time.Minute):
			s.queryMutex.Lock()
			delete(s.queryStatus, queryID)
			s.queryMutex.Unlock()
		case <-s.shutdownCtx.Done():
			s.queryMutex.Lock()
			delete(s.queryStatus, queryID)
			s.queryMutex.Unlock()
		}
	}()
}

// 广播消息给所有WebSocket连接
func (s *Server) broadcastMessage(message interface{}) {
	s.connManager.mutex.RLock()
	defer s.connManager.mutex.RUnlock()

	for _, managed := range s.connManager.connections {
		managed.writeMu.Lock()
		setWriteDeadline(managed.conn)
		err := managed.conn.WriteJSON(message)
		managed.writeMu.Unlock()
		if err != nil {
			logger.Errorf("WebSocket broadcast error: %v", err)
		}
	}
}

// sendToConn 发送消息给指定连接
func (s *Server) sendToConn(connID string, message interface{}) {
	s.connManager.mutex.RLock()
	managed, ok := s.connManager.connections[connID]
	s.connManager.mutex.RUnlock()

	if !ok {
		return
	}

	managed.writeMu.Lock()
	defer managed.writeMu.Unlock()
	setWriteDeadline(managed.conn)
	if err := managed.conn.WriteJSON(message); err != nil {
		logger.Errorf("WebSocket sendToConn error for %s: %v", connID, err)
	}
}

// updateQueryProgress 更新查询进度并仅发送给发起该查询的连接
func (s *Server) updateQueryProgress(connID string, queryID string, progress float64) {
	shouldSend := false

	s.queryMutex.Lock()
	if status, exists := s.queryStatus[queryID]; exists {
		if progress < status.Progress {
			progress = status.Progress
		}
		status.Progress = progress
		s.queryStatus[queryID] = status
		shouldSend = true
	}
	s.queryMutex.Unlock()

	if shouldSend {
		s.sendToConn(connID, map[string]interface{}{
			"type":     "progress_update",
			"query_id": queryID,
			"progress": progress,
		})
	}
}
