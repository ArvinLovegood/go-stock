package agent

import (
	"context"
	"fmt"
	"testing"
)

// isContextCanceledError 用于在 context 失效时跳过 React 降级重试，
// 必须能识别 eino 包装后的 GraphRunError/NodeRunError 文案。
func TestIsContextCanceledError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.DeadlineExceeded, true},
		{context.Canceled, true},
		{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), true},
		{fmt.Errorf("[GraphRunError] context has been canceled: context deadline exceeded"), true},
		{fmt.Errorf("NodeRunError, gen message failed: context canceled"), true},
		{fmt.Errorf("max_tokens above maximum value"), false},
		{fmt.Errorf("unmarshal plan error: invalid char"), false},
	}
	for i, c := range cases {
		if got := isContextCanceledError(c.err); got != c.want {
			t.Errorf("case %d: isContextCanceledError(%v) = %v, want %v", i, c.err, got, c.want)
		}
	}
}
