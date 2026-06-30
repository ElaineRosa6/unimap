package unierror

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestError_Formatting 验证 Error() 方法的格式化输出
func TestError_Formatting(t *testing.T) {
	t.Run("without OriginalErr", func(t *testing.T) {
		err := New(ErrorTypeNetwork, ErrNetworkTimeout, "connection timed out")
		// 期望格式: "%s error: %s" (Type, Message)
		expected := fmt.Sprintf("%s error: %s", ErrorTypeNetwork, "connection timed out")
		assert.Equal(t, expected, err.Error())
	})

	t.Run("with OriginalErr", func(t *testing.T) {
		original := errors.New("dial tcp: i/o timeout")
		err := Wrap(original, ErrorTypeNetwork, ErrNetworkTimeout, "connection timed out")
		// 期望格式: "%s error: %s (original: %v)"
		expected := fmt.Sprintf("%s error: %s (original: %v)", ErrorTypeNetwork, "connection timed out", original)
		assert.Equal(t, expected, err.Error())
		// 确保包含 "(original: " 片段
		assert.Contains(t, err.Error(), "(original: ")
	})
}

// TestUnwrap 验证 Unwrap() 与 errors.Is/errors.As 的链式行为
func TestUnwrap(t *testing.T) {
	t.Run("Unwrap returns original", func(t *testing.T) {
		original := errors.New("root cause")
		err := Wrap(original, ErrorTypeAPI, ErrAPIInternalServer, "api call failed")
		assert.Same(t, original, err.Unwrap())
		assert.Same(t, original, errors.Unwrap(err))
	})

	t.Run("Unwrap returns nil when no OriginalErr", func(t *testing.T) {
		err := New(ErrorTypeAPI, ErrAPIInternalServer, "api call failed")
		assert.Nil(t, err.Unwrap())
	})

	t.Run("errors.Is walks the chain", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		wrapped := Wrap(sentinel, ErrorTypeNetwork, ErrNetworkConnection, "wrapped sentinel")
		assert.True(t, errors.Is(wrapped, sentinel))
		assert.False(t, errors.Is(wrapped, errors.New("different")))
	})

	t.Run("errors.As extracts *UnimapError", func(t *testing.T) {
		original := errors.New("boom")
		wrapped := Wrap(original, ErrorTypeConfig, ErrConfigInvalid, "bad config")
		var target *UnimapError
		assert.True(t, errors.As(wrapped, &target))
		require.NotNil(t, target)
		assert.Equal(t, ErrorTypeConfig, target.Type)
		assert.Equal(t, ErrConfigInvalid, target.Code)
		assert.Equal(t, "bad config", target.Message)
		assert.Same(t, original, target.OriginalErr)
	})

	t.Run("errors.Is matches a *UnimapError against itself via As chain", func(t *testing.T) {
		// 多层包装: 原 *UnimapError 被 Wrap 时, errors.As 仍能提取到 *UnimapError
		inner := New(ErrorTypeValidation, ErrValidationRequired, "Field name is required")
		outer := Wrap(inner, ErrorTypeBusiness, ErrBusinessOperationFailed, "operation failed due to validation")
		var target *UnimapError
		assert.True(t, errors.As(outer, &target))
		// errors.As 找到的应该是链上第一个 *UnimapError (即 outer 本身)
		assert.Equal(t, ErrorTypeBusiness, target.Type)
		assert.Equal(t, ErrBusinessOperationFailed, target.Code)
	})
}

// TestNew 验证 New() 在有/无格式化参数时的语义
func TestNew(t *testing.T) {
	t.Run("without args: Details equals message", func(t *testing.T) {
		err := New(ErrorTypeNetwork, ErrNetworkTimeout, "connection timed out")
		assert.Equal(t, "connection timed out", err.Message)
		assert.Equal(t, "connection timed out", err.Details)
		assert.Equal(t, ErrorTypeNetwork, err.Type)
		assert.Equal(t, ErrNetworkTimeout, err.Code)
		assert.Nil(t, err.OriginalErr)
	})

	t.Run("with args: Details formatted, Message stays raw", func(t *testing.T) {
		err := New(ErrorTypeAPI, ErrAPIBadRequest, "bad request for host %s port %d", "example.com", 8080)
		// Message 保持原始格式串
		assert.Equal(t, "bad request for host %s port %d", err.Message)
		// Details 是格式化后的结果
		assert.Equal(t, "bad request for host example.com port 8080", err.Details)
		assert.Equal(t, ErrorTypeAPI, err.Type)
		assert.Equal(t, ErrAPIBadRequest, err.Code)
	})
}

// TestWrap 验证 Wrap() 在有/无格式化参数时的语义
func TestWrap(t *testing.T) {
	original := errors.New("underlying failure")

	t.Run("without args: Details equals message, OriginalErr set", func(t *testing.T) {
		err := Wrap(original, ErrorTypeRuntime, ErrRuntimePanic, "runtime panic occurred")
		assert.Equal(t, "runtime panic occurred", err.Message)
		assert.Equal(t, "runtime panic occurred", err.Details)
		assert.Equal(t, ErrorTypeRuntime, err.Type)
		assert.Equal(t, ErrRuntimePanic, err.Code)
		assert.Same(t, original, err.OriginalErr)
	})

	t.Run("with args: Details formatted, Message stays raw, OriginalErr set", func(t *testing.T) {
		err := Wrap(original, ErrorTypeAPI, ErrAPITimeout, "request to %s timed out after %d seconds", "api.example.com", 30)
		assert.Equal(t, "request to %s timed out after %d seconds", err.Message)
		assert.Equal(t, "request to api.example.com timed out after 30 seconds", err.Details)
		assert.Equal(t, ErrorTypeAPI, err.Type)
		assert.Equal(t, ErrAPITimeout, err.Code)
		assert.Same(t, original, err.OriginalErr)
	})

	t.Run("wrapping nil original keeps OriginalErr nil but still format args", func(t *testing.T) {
		err := Wrap(nil, ErrorTypeConfig, ErrConfigInvalid, "config %s invalid", "timeout")
		assert.Nil(t, err.OriginalErr)
		assert.Equal(t, "config %s invalid", err.Message)
		assert.Equal(t, "config timeout invalid", err.Details)
		// OriginalErr 为 nil 时 Error() 不应包含 (original:)
		assert.NotContains(t, err.Error(), "(original:")
	})
}

// TestConvenienceConstructors 验证便捷构造函数返回正确的 Type 与 Code
func TestConvenienceConstructors(t *testing.T) {
	t.Run("Network constructors", func(t *testing.T) {
		to := NetworkTimeout("timeout occurred")
		assert.Equal(t, ErrorTypeNetwork, to.Type)
		assert.Equal(t, ErrNetworkTimeout, to.Code)
		assert.Equal(t, "timeout occurred", to.Message)

		conn := NetworkConnection("connection refused")
		assert.Equal(t, ErrorTypeNetwork, conn.Type)
		assert.Equal(t, ErrNetworkConnection, conn.Code)
		assert.Equal(t, "connection refused", conn.Message)
	})

	t.Run("API constructors", func(t *testing.T) {
		unauth := APIUnauthorized("token expired")
		assert.Equal(t, ErrorTypeAPI, unauth.Type)
		assert.Equal(t, ErrAPIUnauthorized, unauth.Code)
		assert.Equal(t, "token expired", unauth.Message)

		forbidden := APIForbidden("no permission")
		assert.Equal(t, ErrorTypeAPI, forbidden.Type)
		assert.Equal(t, ErrAPIForbidden, forbidden.Code)
		assert.Equal(t, "no permission", forbidden.Message)

		rate := APIRateLimit("too many requests")
		assert.Equal(t, ErrorTypeAPI, rate.Type)
		assert.Equal(t, ErrAPIRateLimit, rate.Code)
		assert.Equal(t, "too many requests", rate.Message)
	})

	t.Run("Config constructors", func(t *testing.T) {
		invalid := ConfigInvalid("bad value")
		assert.Equal(t, ErrorTypeConfig, invalid.Type)
		assert.Equal(t, ErrConfigInvalid, invalid.Code)
		assert.Equal(t, "bad value", invalid.Message)

		missing := ConfigMissing("missing key")
		assert.Equal(t, ErrorTypeConfig, missing.Type)
		assert.Equal(t, ErrConfigMissing, missing.Code)
		assert.Equal(t, "missing key", missing.Message)
	})

	t.Run("Business constructor", func(t *testing.T) {
		nf := BusinessNotFound("user not found")
		assert.Equal(t, ErrorTypeBusiness, nf.Type)
		assert.Equal(t, ErrBusinessNotFound, nf.Code)
		assert.Equal(t, "user not found", nf.Message)
	})

	t.Run("Validation constructors", func(t *testing.T) {
		req := ValidationRequired("email")
		assert.Equal(t, ErrorTypeValidation, req.Type)
		assert.Equal(t, ErrValidationRequired, req.Code)
		// Message 保持原始格式串
		assert.Equal(t, "Field %s is required", req.Message)
		// Details 是格式化结果
		assert.Equal(t, "Field email is required", req.Details)

		fmtErr := ValidationFormat("email", "RFC 5322")
		assert.Equal(t, ErrorTypeValidation, fmtErr.Type)
		assert.Equal(t, ErrValidationFormat, fmtErr.Code)
		assert.Equal(t, "Field %s must match format: %s", fmtErr.Message)
		assert.Equal(t, "Field email must match format: RFC 5322", fmtErr.Details)
	})
}

// TestHelper_Is 验证 Is() 在 *UnimapError 与普通 error 上的行为
func TestHelper_Is(t *testing.T) {
	t.Run("matching type returns true", func(t *testing.T) {
		err := New(ErrorTypeNetwork, ErrNetworkTimeout, "timeout")
		assert.True(t, Is(err, ErrorTypeNetwork))
	})

	t.Run("non-matching type returns false", func(t *testing.T) {
		err := New(ErrorTypeNetwork, ErrNetworkTimeout, "timeout")
		assert.False(t, Is(err, ErrorTypeAPI))
		assert.False(t, Is(err, ErrorTypeConfig))
	})

	t.Run("non-*UnimapError returns false", func(t *testing.T) {
		plain := errors.New("plain error")
		assert.False(t, Is(plain, ErrorTypeNetwork))
		assert.False(t, Is(plain, ErrorTypeAPI))
	})

	t.Run("nil-safe: wrapped error still checks the outer type", func(t *testing.T) {
		original := errors.New("root")
		wrapped := Wrap(original, ErrorTypeBusiness, ErrBusinessNotFound, "missing")
		// Is 仅检查外层类型, 不做链式类型匹配 (与实现一致)
		assert.True(t, Is(wrapped, ErrorTypeBusiness))
	})
}

// TestHelper_GetCode 验证 GetCode() 在 *UnimapError 与普通 error 上的行为
func TestHelper_GetCode(t *testing.T) {
	t.Run("returns Code for *UnimapError", func(t *testing.T) {
		err := New(ErrorTypeAPI, ErrAPIForbidden, "forbidden")
		assert.Equal(t, ErrAPIForbidden, GetCode(err))
	})

	t.Run("returns 0 for plain error", func(t *testing.T) {
		plain := errors.New("plain error")
		assert.Equal(t, 0, GetCode(plain))
	})
}

// TestHelper_GetMessage 验证 GetMessage() 在 *UnimapError 与普通 error 上的行为
func TestHelper_GetMessage(t *testing.T) {
	t.Run("returns Message for *UnimapError", func(t *testing.T) {
		err := New(ErrorTypeConfig, ErrConfigMissing, "missing config")
		assert.Equal(t, "missing config", GetMessage(err))
	})

	t.Run("returns err.Error() for plain error", func(t *testing.T) {
		plain := errors.New("plain error")
		assert.Equal(t, "plain error", GetMessage(plain))
	})
}

// TestHelper_GetDetails 验证 GetDetails() 在 *UnimapError 与普通 error 上的行为
func TestHelper_GetDetails(t *testing.T) {
	t.Run("returns Details for *UnimapError", func(t *testing.T) {
		err := New(ErrorTypeAPI, ErrAPIBadRequest, "bad request for %s", "host")
		assert.Equal(t, "bad request for host", GetDetails(err))
	})

	t.Run("returns err.Error() for plain error", func(t *testing.T) {
		plain := errors.New("plain error")
		assert.Equal(t, "plain error", GetDetails(plain))
	})
}

// TestGetStackTrace 验证 getStackTrace 的内容
func TestGetStackTrace(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		stack := getStackTrace()
		assert.NotEmpty(t, stack)
	})

	t.Run("does not contain internal/error/ lines", func(t *testing.T) {
		stack := getStackTrace()
		assert.NotContains(t, stack, "internal/error/")
		// 进一步确认每一行都不包含该路径
		for _, line := range strings.Split(stack, "\n") {
			assert.NotContains(t, line, "internal/error/")
		}
	})
}
