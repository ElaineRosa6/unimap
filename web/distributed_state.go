package web

import (
	"github.com/unimap/project/internal/distributed"
)

// DistributedState holds the distributed node registry and task queue.
type DistributedState struct {
	NodeRegistry  *distributed.Registry
	NodeTaskQueue *distributed.TaskQueue
}
