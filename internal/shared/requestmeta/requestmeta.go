// Package requestmeta 保存请求范围元数据。
package requestmeta

import "context"

type requestIDKey struct{}

// WithRequestID 返回带请求标识的上下文。
func WithRequestID(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, value)
}

// RequestID 返回请求标识；空值不视为有效标识。
func RequestID(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(requestIDKey{}).(string)
	return value, ok && value != ""
}
