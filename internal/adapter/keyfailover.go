package adapter

import (
	"errors"
	"fmt"

	"github.com/unimap/project/internal/logger"
)

// keyError 表示与特定 API key 相关的失败（鉴权/欠费/限流）。
// 这类失败换用备用 key 可能成功，用于驱动主/备用 key 自动切换。
type keyError struct {
	engine string
	code   int
	msg    string
}

func (e *keyError) Error() string {
	return fmt.Sprintf("%s key error %d: %s", e.engine, e.code, e.msg)
}

// isKeyError 判断错误是否为 key 级失败。
func isKeyError(err error) bool {
	var keyErr *keyError
	return errors.As(err, &keyErr)
}

// activeKeys 返回依次尝试的 API key 列表（主 key + 备用 key）。
// 备用 key 为空或与主 key 相同时不重复。
func activeKeys(primary, backup string) []string {
	keys := []string{primary}
	if backup != "" && backup != primary {
		keys = append(keys, backup)
	}
	return keys
}

// withKeyFailover 依次用主/备用 key 执行 fn，仅在 fn 返回 key 级失败时切换 key。
// n 是可用 key 数量（无备用 key 时为 1）；fn 接收 key 索引（0=主 key，1=备用 key）。
// 网络等瞬时错误由 fn 内部重试处理，不在此处切换。
func withKeyFailover(engine string, n int, fn func(idx int) error) error {
	for i := 0; i < n; i++ {
		err := fn(i)
		if err == nil {
			return nil
		}
		if isKeyError(err) && i < n-1 {
			logger.Warnf("%s API key #%d failed (%v); trying backup key", engine, i+1, err)
			continue
		}
		return err
	}
	return fmt.Errorf("%s: all api keys exhausted", engine)
}
