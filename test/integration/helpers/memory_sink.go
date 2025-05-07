package helpers

import (
	"context"
	"gateway/internal/pkg"
	"gateway/internal/sink"
	"sync"

	"go.uber.org/zap"
)

// MemorySink 是一个用于测试的内存型Sink，实现了sink.Template接口
type MemorySink struct {
	received []*pkg.PointPackage // 存储接收到的数据
	mu       sync.Mutex          // 保护received
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *zap.Logger
}

// RegisterMemorySink 注册 MemorySink 到 sink 工厂
func RegisterMemorySink() {
	sink.Register("memory_sink", NewMemorySink)
}

// NewMemorySink 创建一个新的MemorySink实例
func NewMemorySink(ctx context.Context) (sink.Template, error) {
	logger := pkg.LoggerFromContext(ctx)
	childCtx, cancel := context.WithCancel(ctx)

	return &MemorySink{
		received: make([]*pkg.PointPackage, 0),
		ctx:      childCtx,
		cancel:   cancel,
		logger:   logger.With(zap.String("sink", "memory")),
	}, nil
}

// GetType 返回sink的类型
func (m *MemorySink) GetType() string {
	return "memory_sink"
}

// Start 开始监听接收点数据
func (m *MemorySink) Start(pointChan chan *pkg.PointPackage) {
	m.logger.Info("MemorySink已启动")

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				m.logger.Info("MemorySink正在停止")
				return
			case point, ok := <-pointChan:
				if !ok {
					m.logger.Info("点通道已关闭")
					return
				}

				if point != nil {
					m.mu.Lock()
					m.received = append(m.received, point)
					m.mu.Unlock()
					m.logger.Debug("MemorySink接收到点数据",
						zap.Int("点数量", len(point.Points)),
						zap.Time("时间戳", point.Ts))
				}
			}
		}
	}()
}

// Stop 停止MemorySink
func (m *MemorySink) Stop() {
	m.logger.Info("请求停止MemorySink")
	m.cancel()
}

// GetReceived 返回接收到的所有点数据
func (m *MemorySink) GetReceived() []*pkg.PointPackage {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 返回副本以避免数据竞争
	result := make([]*pkg.PointPackage, len(m.received))
	copy(result, m.received)
	return result
}

// Reset 清空接收到的数据
func (m *MemorySink) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received = make([]*pkg.PointPackage, 0)
}
