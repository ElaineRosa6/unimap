package distributed

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/unimap/project/internal/logger"
)

// Registry stores node liveness and runtime metadata in memory.
type Registry struct {
	nodes            map[string]*NodeRecord
	heartbeatTimeout time.Duration
	cleanupInterval  time.Duration
	mu               sync.RWMutex
	stopChan         chan struct{}
	stopped          bool
	taskQueue        *TaskQueue // Optional: for releasing tasks when node goes offline
	failoverStrategy FailoverStrategy
}

func NewRegistry(heartbeatTimeout time.Duration) *Registry {
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 30 * time.Second
	}
	r := &Registry{
		nodes:            make(map[string]*NodeRecord),
		heartbeatTimeout: heartbeatTimeout,
		cleanupInterval:  heartbeatTimeout * 2,
		stopChan:         make(chan struct{}),
		failoverStrategy: FailoverStrategyHealthBased, // 默认基于健康状态的故障转移
	}
	go r.startBackgroundCleanup()
	// Finalizer is a safety net only — GC timing is non-deterministic, so callers
	// must call Close() explicitly to guarantee cleanup.
	runtime.SetFinalizer(r, func(r *Registry) {
		r.Stop()
	})
	return r
}

// Close stops the background cleanup goroutine and clears the finalizer.
// This is the primary cleanup mechanism and must be called explicitly by callers.
func (r *Registry) Close() {
	runtime.SetFinalizer(r, nil)
	r.Stop()
}

// SetTaskQueue sets the task queue for releasing tasks when nodes go offline
func (r *Registry) SetTaskQueue(q *TaskQueue) {
	r.mu.Lock()
	r.taskQueue = q
	r.mu.Unlock()
}

// Stop stops the background cleanup goroutine
func (r *Registry) Stop() {
	r.mu.Lock()
	if !r.stopped {
		r.stopped = true
		close(r.stopChan)
	}
	r.mu.Unlock()
}

// startBackgroundCleanup periodically removes stale nodes
func (r *Registry) startBackgroundCleanup() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.cleanupStaleNodes()
		}
	}
}

// cleanupStaleNodes removes nodes that have been offline for too long
func (r *Registry) cleanupStaleNodes() {
	now := time.Now()
	// Remove nodes that have been offline for 10x heartbeat timeout
	cutoff := r.heartbeatTimeout * 10

	r.mu.Lock()
	defer r.mu.Unlock()

	for nodeID, record := range r.nodes {
		if !record.Online && now.Sub(record.LastHeartbeatAt) > cutoff {
			// Release orphaned tasks from stale node before removing record
			if r.taskQueue != nil {
				released := r.taskQueue.ReleaseNodeTasks(nodeID)
				if released > 0 {
					logger.Infof("registry: released %d orphaned task(s) from stale node %s", released, nodeID)
				}
			}
			delete(r.nodes, nodeID)
		}
	}
}

func (r *Registry) Register(req NodeRegistration) (NodeRecord, error) {
	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		return NodeRecord{}, fmt.Errorf("node_id is required")
	}
	if req.MaxConcurrency < 0 {
		return NodeRecord{}, fmt.Errorf("max_concurrency must be greater than or equal to 0")
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		record = &NodeRecord{NodeID: nodeID, RegisteredAt: now}
		r.nodes[nodeID] = record
	}

	record.Hostname = strings.TrimSpace(req.Hostname)
	record.Region = strings.TrimSpace(req.Region)
	record.Labels = cloneLabels(req.Labels)
	record.Capabilities = cloneStringSlice(req.Capabilities)
	record.Version = strings.TrimSpace(req.Version)
	record.EgressIP = strings.TrimSpace(req.EgressIP)
	if req.MaxConcurrency > 0 {
		record.MaxConcurrency = req.MaxConcurrency
	}
	if record.MaxConcurrency <= 0 {
		record.MaxConcurrency = 1
	}
	record.LastHeartbeatAt = now
	record.Online = true

	return *record, nil
}

func (r *Registry) Heartbeat(hb NodeHeartbeat) (NodeRecord, error) {
	nodeID := strings.TrimSpace(hb.NodeID)
	if nodeID == "" {
		return NodeRecord{}, fmt.Errorf("node_id is required")
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		record = &NodeRecord{NodeID: nodeID, RegisteredAt: now, MaxConcurrency: 1}
		r.nodes[nodeID] = record
	}

	if hb.CurrentLoad >= 0 {
		record.CurrentLoad = hb.CurrentLoad
	}
	if hb.MaxConcurrency > 0 {
		record.MaxConcurrency = hb.MaxConcurrency
	}
	if record.MaxConcurrency <= 0 {
		record.MaxConcurrency = 1
	}
	if hb.AvgLatencyMS >= 0 {
		record.AvgLatencyMS = hb.AvgLatencyMS
	}
	if hb.SuccessRate5m >= 0 {
		record.SuccessRate5m = hb.SuccessRate5m
	}
	if v := strings.TrimSpace(hb.Version); v != "" {
		record.Version = v
	}
	if egress := strings.TrimSpace(hb.EgressIP); egress != "" {
		record.EgressIP = egress
	}

	// 更新健康检查指标
	if hb.CPUUsage >= 0 {
		record.CPUUsage = hb.CPUUsage
	}
	if hb.MemoryUsage >= 0 {
		record.MemoryUsage = hb.MemoryUsage
	}
	if hb.DiskUsage >= 0 {
		record.DiskUsage = hb.DiskUsage
	}
	if hb.NetworkLatency >= 0 {
		record.NetworkLatency = hb.NetworkLatency
	}
	if hb.ErrorRate >= 0 {
		record.ErrorRate = hb.ErrorRate
	}
	if hb.ActiveTasks >= 0 {
		record.ActiveTasks = hb.ActiveTasks
	}
	if hb.HealthChecks != nil && len(hb.HealthChecks) > 0 {
		if record.HealthChecks == nil {
			record.HealthChecks = make(map[string]bool)
		}
		for k, v := range hb.HealthChecks {
			record.HealthChecks[k] = v
		}
	}

	record.LastHeartbeatAt = now
	record.Online = true

	// 计算健康评分和状态
	r.calculateHealthScore(record)

	return *record, nil
}

func (r *Registry) Snapshot() NodeStatusSnapshot {
	now := time.Now()
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]NodeRecord, 0, len(r.nodes))
	online := 0
	offline := 0
	for _, record := range r.nodes {
		item := *record
		if now.Sub(item.LastHeartbeatAt) > r.heartbeatTimeout {
			item.Online = false
		}
		if item.Online {
			online++
		} else {
			offline++
		}
		nodes = append(nodes, item)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})

	return NodeStatusSnapshot{
		Total:   len(nodes),
		Online:  online,
		Offline: offline,
		Nodes:   nodes,
	}
}

// Get retrieves a single node by ID
func (r *Registry) Get(nodeID string) (*NodeRecord, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		return nil, nil
	}

	copyRec := *record
	// Check if online based on heartbeat
	if time.Since(copyRec.LastHeartbeatAt) > r.heartbeatTimeout {
		copyRec.Online = false
	}
	return &copyRec, nil
}

// MarkOffline marks a node as offline and optionally releases its tasks
func (r *Registry) MarkOffline(nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return fmt.Errorf("node_id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found")
	}

	wasOnline := record.Online
	record.Online = false
	record.HealthStatus = "offline"

	// 更新故障转移统计
	if wasOnline {
		record.FailoverCount++
		record.LastFailoverAt = time.Now()
	}

	// Release tasks if node was online and we have a task queue
	if wasOnline && r.taskQueue != nil {
		recoveredTasks := r.taskQueue.ReleaseNodeTasks(nodeID)
		record.TaskRecoveryCount += recoveredTasks
	}

	return nil
}

// SetFailoverStrategy 设置故障转移策略
func (r *Registry) SetFailoverStrategy(strategy FailoverStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failoverStrategy = strategy
}

// GetFailoverStrategy 获取当前故障转移策略
func (r *Registry) GetFailoverStrategy() FailoverStrategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failoverStrategy
}

// GetHealthyNodes 获取健康节点列表（返回深拷贝，防止外部修改）
func (r *Registry) GetHealthyNodes() []*NodeRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var healthyNodes []*NodeRecord
	for _, node := range r.nodes {
		if node.Online && node.HealthStatus != "critical" && node.HealthStatus != "offline" {
			healthyNodes = append(healthyNodes, copyNodeRecord(node))
		}
	}

	// 根据故障转移策略排序节点
	switch r.failoverStrategy {
	case FailoverStrategyHealthBased:
		// 按健康评分降序排序
		sort.Slice(healthyNodes, func(i, j int) bool {
			return healthyNodes[i].HealthScore > healthyNodes[j].HealthScore
		})
	case FailoverStrategyLoadBalanced:
		// 按负载（活跃任务数/最大并发数）升序排序
		sort.Slice(healthyNodes, func(i, j int) bool {
			loadI := float64(healthyNodes[i].ActiveTasks) / float64(healthyNodes[i].MaxConcurrency+1)
			loadJ := float64(healthyNodes[j].ActiveTasks) / float64(healthyNodes[j].MaxConcurrency+1)
			return loadI < loadJ
		})
	case FailoverStrategyPriorityBased:
		// 按区域优先级和健康评分排序
		sort.Slice(healthyNodes, func(i, j int) bool {
			if healthyNodes[i].Region != healthyNodes[j].Region {
				// 假设某些区域有更高优先级
				priorityRegions := map[string]int{"primary": 3, "secondary": 2, "backup": 1}
				prioI := priorityRegions[healthyNodes[i].Region]
				prioJ := priorityRegions[healthyNodes[j].Region]
				return prioI > prioJ
			}
			return healthyNodes[i].HealthScore > healthyNodes[j].HealthScore
		})
	}

	return healthyNodes
}

// HandleNodeFailure 处理节点故障
func (r *Registry) HandleNodeFailure(nodeID string) error {
	// 标记节点离线
	err := r.MarkOffline(nodeID)
	if err != nil {
		return err
	}

	// 获取健康节点
	healthyNodes := r.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		return fmt.Errorf("no healthy nodes available for failover")
	}

	// 故障转移成功，可以在这里添加额外的处理逻辑
	// 例如：发送告警、记录日志等

	return nil
}

// Deregister removes a node from the registry
func (r *Registry) Deregister(nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return fmt.Errorf("node_id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found")
	}

	// Release tasks before removing
	if r.taskQueue != nil {
		r.taskQueue.ReleaseNodeTasks(nodeID)
	}

	delete(r.nodes, nodeID)
	_ = record // suppress unused variable warning
	return nil
}

func cloneLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// calculateHealthScore 计算节点健康评分并设置健康状态
func (r *Registry) calculateHealthScore(record *NodeRecord) {
	totalScore := 0.20*calculateResourceScore(record.CPUUsage, 80, 90) +
		0.20*calculateResourceScore(record.MemoryUsage, 80, 90) +
		0.15*calculateResourceScore(record.DiskUsage, 85, 95) +
		0.15*calculateNetworkScore(record.NetworkLatency) +
		0.15*calculateErrorRateScore(record.ErrorRate) +
		0.10*record.SuccessRate5m*100 +
		0.05*calculateLoadScore(record.ActiveTasks, record.MaxConcurrency)

	record.HealthScore = totalScore
	record.HealthStatus = healthStatusFromScore(record.Online, totalScore)
}

func calculateNetworkScore(latencyMs float64) float64 {
	switch {
	case latencyMs <= 0 || latencyMs <= 200:
		return 100
	case latencyMs <= 500:
		return 75
	case latencyMs <= 1000:
		return 50
	default:
		return 0
	}
}

func calculateErrorRateScore(rate float64) float64 {
	switch {
	case rate <= 0:
		return 100
	case rate <= 0.1:
		return 100
	case rate <= 0.2:
		return 75
	case rate <= 0.5:
		return 50
	default:
		return 0
	}
}

func calculateLoadScore(activeTasks, maxConcurrency int) float64 {
	if maxConcurrency <= 0 {
		return 100
	}
	ratio := float64(activeTasks) / float64(maxConcurrency)
	switch {
	case ratio <= 0.7:
		return 100
	case ratio <= 0.9:
		return 75
	case ratio <= 1.2:
		return 50
	default:
		return 0
	}
}

func healthStatusFromScore(online bool, score float64) string {
	if !online {
		return "offline"
	}
	switch {
	case score >= 80:
		return "healthy"
	case score >= 60:
		return "warning"
	default:
		return "critical"
	}
}

// calculateResourceScore 计算资源使用率评分
func calculateResourceScore(usage, warningThreshold, criticalThreshold float64) float64 {
	if usage >= criticalThreshold {
		return 0
	} else if usage >= warningThreshold {
		return 50
	} else {
		return 100
	}
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// copyNodeRecord creates a deep copy of a NodeRecord to prevent external mutation.
func copyNodeRecord(src *NodeRecord) *NodeRecord {
	cp := *src
	if src.Labels != nil {
		cp.Labels = make(map[string]string, len(src.Labels))
		for k, v := range src.Labels {
			cp.Labels[k] = v
		}
	}
	if src.Capabilities != nil {
		cp.Capabilities = make([]string, len(src.Capabilities))
		copy(cp.Capabilities, src.Capabilities)
	}
	if src.HealthChecks != nil {
		cp.HealthChecks = make(map[string]bool, len(src.HealthChecks))
		for k, v := range src.HealthChecks {
			cp.HealthChecks[k] = v
		}
	}
	return &cp
}
