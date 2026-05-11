package web

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/unimap-icp-hunter/project/internal/logger"
	"github.com/unimap-icp-hunter/project/internal/metrics"
	"github.com/unimap-icp-hunter/project/internal/model"
	"github.com/unimap-icp-hunter/project/internal/service"
)

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 验证WebSocket连接请求
	if !s.validateWebSocketRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// 为连接生成唯一ID
	connID := fmt.Sprintf("%d", time.Now().UnixNano())
	managed := &managedConn{conn: conn}
	connCtx, cancelConn := context.WithCancel(r.Context())

	writeJSON := func(v interface{}) error {
		managed.writeMu.Lock()
		defer managed.writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	done := make(chan struct{})

	// 添加到连接管理器
	s.connManager.mutex.Lock()
	s.connManager.connections[connID] = managed
	s.connManager.mutex.Unlock()
	metrics.IncWebSocketConnection()

	// 连接关闭时从管理器中移除
	defer func() {
		cancelConn()
		close(done)
		s.connManager.mutex.Lock()
		delete(s.connManager.connections, connID)
		s.connManager.mutex.Unlock()
		metrics.DecWebSocketConnection()
		logger.Infof("WebSocket connection closed: %s", connID)
	}()

	// 设置连接读取超时
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 启动ping协程
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := writeJSON(map[string]interface{}{"type": "ping"}); err != nil {
					logger.Errorf("WebSocket ping error: %v", err)
					return
				}
			}
		}
	}()

	// 处理WebSocket消息
	for {
		var message map[string]interface{}
		err := conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("WebSocket read error: %v", err)
			}
			break
		}

		metrics.IncWebSocketMessage("inbound")

		// 处理不同类型的消息
		if messageType, ok := message["type"].(string); ok {
			switch messageType {
			case "ping":
				// 回复ping消息
				metrics.IncWebSocketMessage("outbound")
				if err := writeJSON(map[string]interface{}{"type": "pong"}); err != nil {
					logger.Errorf("WebSocket write error: %v", err)
					break
				}
			case "pong":
				// 收到pong消息，重置读取超时
				conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			case "query":
				// 处理查询请求
				s.handleWebSocketQuery(connCtx, message, writeJSON)
			}
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
	// 2. Query parameter (fallback for non-browser clients)
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	// 3. Header
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
func (s *Server) handleWebSocketQuery(ctx context.Context, message map[string]interface{}, writeJSON func(interface{}) error) {
	// 解析查询参数
	query, _ := message["query"].(string)
	query = strings.TrimSpace(query)

	if err := validateQueryInput(query); err != nil {
		if err := writeJSON(map[string]interface{}{
			"type":  "query_error",
			"error": err.Error(),
		}); err != nil {
			logger.Errorf("WebSocket write error: %v", err)
		}
		return
	}

	pageSize := parseWSInt(message["page_size"], 50)
	browserQuery := parseWSBool(message["browser_query"])
	browserAction := ""
	if ba, ok := message["browser_action"].(string); ok {
		browserAction = strings.TrimSpace(ba)
	}

	engines := parseWSStringList(message["engines"])
	if len(engines) == 0 {
		// 如果没有选择引擎，使用默认引擎
		defaultEngines := s.orchestrator.ListAdapters()
		if len(defaultEngines) > 0 {
			engines = []string{defaultEngines[0]}
		}
	}

	if len(engines) == 0 {
		// 发送查询错误消息
		if err := writeJSON(map[string]interface{}{
			"type":  "query_error",
			"error": "no engines configured/registered. Please set API keys in configs/config.yaml and enable at least one engine.",
		}); err != nil {
			logger.Errorf("WebSocket write error: %v", err)
		}
		return
	}

	// 生成查询ID
	queryID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 创建查询状态
	status := &QueryStatus{
		ID:         queryID,
		Query:      query,
		Engines:    engines,
		Status:     "running",
		Progress:   0,
		Results:    []model.UnifiedAsset{},
		TotalCount: 0,
		Errors:     []string{},
		StartTime:  time.Now(),
	}

	// 保存查询状态
	s.queryMutex.Lock()
	s.queryStatus[queryID] = status
	s.queryMutex.Unlock()

	// 发送查询开始消息
	if err := writeJSON(map[string]interface{}{
		"type":     "query_start",
		"query_id": queryID,
		"status":   status,
	}); err != nil {
		logger.Errorf("WebSocket write error: %v", err)
	}

	// 异步执行查询，带有独立的超时上下文
	go func() {
		// 为查询创建带超时的上下文（默认 60 秒查询超时）
		if ctx == nil {
			ctx = context.Background()
		}
		queryCtx, queryCancel := context.WithTimeout(ctx, 60*time.Second)
		defer queryCancel()

		if browserQuery {
			s.updateQueryProgress(queryID, 5)
		}
		browserQueryCh := s.runBrowserQueryAsync(queryCtx, query, engines, browserQuery, browserAction, queryID, func(done, total int, engine string, err error) {
			if total <= 0 {
				return
			}
			progress := 5 + (float64(done)/float64(total))*45
			if progress > 50 {
				progress = 50
			}
			s.updateQueryProgress(queryID, progress)
		})

		// 执行查询
		req := service.QueryRequest{
			Query:       query,
			Engines:     engines,
			PageSize:    pageSize,
			ProcessData: true,
		}

		resp, queryErr := s.service.Query(queryCtx, req)
		var browserOutcome browserQueryOutcome
		if browserQueryCh != nil {
			select {
			case browserOutcome = <-browserQueryCh:
			case <-queryCtx.Done():
				// Timeout while waiting for browser query
			}
		}

		// Check if query timed out
		if queryErr == nil && queryCtx.Err() != nil {
			queryErr = fmt.Errorf("query timeout after 60s: %v", queryCtx.Err())
		}

		endTime := time.Now()

		// 更新查询状态（在锁内修改，避免并发读写竞态）
		s.queryMutex.Lock()
		st := s.queryStatus[queryID]
		if st != nil {
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
				if errors, ok := payload["errors"].([]string); ok {
					st.Errors = errors
				} else {
					st.Errors = resp.Errors
				}
				st.Status = "completed"
			}
			st.Progress = 100
			st.EndTime = endTime
		}
		var statusCopy QueryStatus
		if st != nil {
			statusCopy = *st
		}
		s.queryMutex.Unlock()

		// 延迟清理查询状态，允许客户端在一段时间内查询已完成任务的状态
		go func() {
			select {
			case <-time.After(5 * time.Minute):
				s.queryMutex.Lock()
				delete(s.queryStatus, queryID)
				s.queryMutex.Unlock()
			case <-s.shutdownCtx.Done():
				// Server is shutting down, cleanup immediately
				s.queryMutex.Lock()
				delete(s.queryStatus, queryID)
				s.queryMutex.Unlock()
			}
		}()

		// 发送查询完成消息（发副本，避免边编码边被修改）
		var errMsg string
		if queryErr != nil {
			errMsg = fmt.Sprintf("Query failed: %v", queryErr)
		}
		resultsPayload := buildQueryAPIPayload(query, engines, resp, browserOutcome, browserAction, errMsg)
		if errMsg != "" {
			resultsPayload["error"] = errMsg
		}

		if err := writeJSON(map[string]interface{}{
			"type":     "query_complete",
			"query_id": queryID,
			"status":   statusCopy,
			"results":  resultsPayload,
		}); err != nil {
			logger.Errorf("WebSocket write error: %v", err)
		}
	}()
}

// 广播消息给所有WebSocket连接
func (s *Server) broadcastMessage(message interface{}) {
	s.connManager.mutex.RLock()
	defer s.connManager.mutex.RUnlock()

	for _, managed := range s.connManager.connections {
		managed.writeMu.Lock()
		err := managed.conn.WriteJSON(message)
		managed.writeMu.Unlock()
		if err != nil {
			logger.Errorf("WebSocket broadcast error: %v", err)
		}
	}
}

// 更新查询进度并广播
func (s *Server) updateQueryProgress(queryID string, progress float64) {
	shouldBroadcast := false

	s.queryMutex.Lock()
	if status, exists := s.queryStatus[queryID]; exists {
		if progress < status.Progress {
			progress = status.Progress
		}
		status.Progress = progress
		s.queryStatus[queryID] = status
		shouldBroadcast = true
	}
	s.queryMutex.Unlock()

	if shouldBroadcast {
		// 广播进度更新
		s.broadcastMessage(map[string]interface{}{
			"type":     "progress_update",
			"query_id": queryID,
			"progress": progress,
		})
	}
}
