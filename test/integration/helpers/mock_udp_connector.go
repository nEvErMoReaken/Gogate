package helpers

import (
	"context"
	"errors"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MockUDPConnector 实现了connector.Template接口，但不实际监听网络
// 而是允许测试代码直接"注入"UDP数据
type MockUDPConnector struct {
	ctx          context.Context
	dataChannel  chan []byte // 用于接收模拟的UDP数据
	outputSink   *pkg.Parser2DispatcherChan
	isStarted    bool
	startedMutex sync.Mutex
	logger       *zap.Logger
	parser       *parser.ByteParser // 添加ByteParser引用
}

// NewMockUDPConnector 创建新的MockUDPConnector实例
func NewMockUDPConnector(ctx context.Context) (connector.Template, error) {
	logger := pkg.LoggerFromContext(ctx)

	// 确保错误通道被初始化
	errChan := make(chan error, 100)
	ctx = pkg.WithErrChan(ctx, errChan)

	// 创建ByteParser
	bp, err := parser.NewByteParser(ctx)
	if err != nil {
		logger.Error("创建ByteParser失败", zap.Error(err))
		return nil, err
	}

	return &MockUDPConnector{
		ctx:         ctx,
		dataChannel: make(chan []byte, 100), // 缓冲区大小可以根据需要调整
		isStarted:   false,
		logger:      logger.With(zap.String("component", "mock_udp_connector")),
		parser:      bp,
	}, nil
}

// Start 实现Template接口，启动模拟UDP连接器
func (m *MockUDPConnector) Start(sink *pkg.Parser2DispatcherChan) error {
	m.startedMutex.Lock()
	defer m.startedMutex.Unlock()

	if m.isStarted {
		return nil // 已经启动，无需重复操作
	}

	m.outputSink = sink
	m.isStarted = true
	m.logger.Info("MockUDPConnector已启动")

	// 启动处理模拟数据的goroutine
	go m.processIncomingData()

	return nil
}

// processIncomingData 处理模拟的UDP数据
func (m *MockUDPConnector) processIncomingData() {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("MockUDPConnector正在停止")
			return
		case data, ok := <-m.dataChannel:
			if !ok {
				m.logger.Info("数据通道已关闭")
				return
			}

			// 记录接收的消息
			metrics.IncMsgReceived("mock_udp")

			// 创建ByteState处理数据
			state := createTestByteState(data)

			// 使用ByteParser处理数据
			for i, chunk := range m.parser.Chunks {
				err := chunk.ProcessWithBytes(m.ctx, state)
				if err != nil {
					metrics.IncErrorCount()
					metrics.IncMsgErrors("mock_udp_process")
					m.logger.Error("处理模拟UDP数据失败",
						zap.Error(err),
						zap.Int("chunk_index", i))
					break
				}
			}

			// 发送处理结果到输出通道
			if len(state.Out) > 0 {
				frameId := time.Now().Format("150405.000")
				pointPkg := &pkg.PointPackage{
					FrameId: frameId,
					Ts:      time.Now(),
					Points:  state.Out,
				}

				select {
				case (*m.outputSink) <- pointPkg:
					metrics.IncMsgProcessed("mock_udp")
					m.logger.Debug("模拟UDP数据已处理并转发",
						zap.Int("points", len(state.Out)),
						zap.Int("data_size", len(data)))
				default:
					m.logger.Warn("输出通道已满，丢弃数据包")
				}
			}
		}
	}
}

// InjectData 允许测试代码注入模拟的UDP数据
func (m *MockUDPConnector) InjectData(data []byte) error {
	m.startedMutex.Lock()
	started := m.isStarted
	m.startedMutex.Unlock()

	if !started {
		return errors.New("MockUDPConnector未启动")
	}

	select {
	case m.dataChannel <- data:
		return nil
	default:
		return errors.New("数据通道已满")
	}
}

// RegisterMockConnector 注册MockUDPConnector到connector工厂
func RegisterMockConnector() {
	connector.Register("mock_udp", NewMockUDPConnector)
}

// createTestByteState 为测试数据创建ByteState
func createTestByteState(data []byte) *parser.ByteState {
	return &parser.ByteState{
		Data:   data,
		Cursor: 0,
		Out:    make([]*pkg.Point, 0),
	}
}
