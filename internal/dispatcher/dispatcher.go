package dispatcher

import (
	"context"
	"fmt"
	"gateway/internal/pkg"

	"go.uber.org/zap"
)

type Dispatcher struct {
	ctx     context.Context
	SinkMap *pkg.Dispatch2SinkChan // 策略名称 -> 其附属的数据源通道
}

var New = func(ctx context.Context) *Dispatcher {
	return &Dispatcher{
		ctx: ctx,
	}
}

// Start 启动聚合器
func (dis *Dispatcher) Start(source *pkg.Parser2DispatcherChan, sinkMap *pkg.Dispatch2SinkChan) {
	logger := pkg.LoggerFromContext(dis.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("===聚合器启动===")

	handler, err := NewHandler(pkg.ConfigFromContext(dis.ctx).Strategy)
	if err != nil {
		logger.Error("error creating handler", zap.Error(err))
		return
	}
	dis.SinkMap = sinkMap
	for {
		select {
		case frame2point := <-(*source):

			// 记录接收到的点
			metrics.IncMsgReceived("aggregator")

			readyPointPackage, err := handler.Dispatch(frame2point)
			if err != nil {
				logger.Error("error dispatching point", zap.Error(err))
				return
			}
			dis.launch(readyPointPackage)
			// 释放 frame2point 的 Points
			for _, point := range frame2point.Points {
				pkg.PointPoolInstance.Put(point)
			}
		case <-dis.ctx.Done():
			return
		}
	}
}

// launch 方法用于启动聚合器的发送流程
func (dis *Dispatcher) launch(deviceMap map[string]*pkg.PointPackage) {
	logger := pkg.LoggerFromContext(dis.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Debug("launching", zap.String("sinkMap", fmt.Sprintf("%v", dis.SinkMap)))

	// 创建发送计时器
	sendTimer := metrics.NewTimer("aggregator_send")

	// 记录要发送的点总数
	pointCount := 0

	for strategy, readyPointPackage := range deviceMap {
		select {
		case (*dis.SinkMap)[strategy] <- readyPointPackage:
			pointCount += 1
		case <-dis.ctx.Done():
			return
		}

	}

	duration := sendTimer.Stop() // 停止计时器
	if pointCount > 0 {
		logger.Debug("发送完成",
			zap.Duration("duration", duration),
			zap.Int("pointCount", pointCount),
			zap.Float64("avgTime", float64(duration.Nanoseconds())/float64(pointCount)/1000000)) // 平均每点毫秒
	}
}
