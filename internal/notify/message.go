package notify

import "time"

// TaskNotification 定时任务通知消息体
type TaskNotification struct {
	TaskID     string                 `json:"task_id"`
	TaskName   string                 `json:"task_name"`
	TaskType   string                 `json:"task_type"`
	Status     string                 `json:"status"` // success, failed, timeout
	Result     string                 `json:"result"`
	Error      string                 `json:"error,omitempty"`
	Duration   float64                `json:"duration_ms"`
	Timestamp  time.Time              `json:"timestamp"`
	Payload    map[string]interface{} `json:"payload,omitempty"`     // 原始任务参数
	ImagePaths []string               `json:"image_paths,omitempty"` // 截图文件路径，用于飞书图片推送
}
