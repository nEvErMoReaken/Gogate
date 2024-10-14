package pkg

import (
	"context"
)

// 定义一个不导出的 key 类型，避免 context key 冲突
type errChanKey struct{}

// WithErrChan 将带有模块信息的 zap.Logger 存入 context 中
func WithErrChan(ctx context.Context, errChan chan error) context.Context {
	return context.WithValue(ctx, errChanKey{}, errChan)
}

// ErrChanFromContext 从 context 中提取配置指针
func ErrChanFromContext(ctx context.Context) chan<- error {
	if errChan, ok := ctx.Value(errChanKey{}).(chan error); ok {
		return errChan
	}
	return nil
}
