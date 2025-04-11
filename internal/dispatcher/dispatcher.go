package dispatcher

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"regexp"

	"go.uber.org/zap"
)

type Dispatcher struct {
	ctx     context.Context
	SinkMap *pkg.Dispatch2DataSourceChan // 策略名称 -> 其附属的数据源通道
	Road    map[string][]string     // 设备名称 -> 其对应的策略名称
}

var New = func(ctx context.Context) *Dispatcher {
	return &Dispatcher{
		ctx:  ctx,
		Road: make(map[string][]string),
	}
}

// Start 启动聚合器
func (dis *Dispatcher) Start(source *pkg.Parser2DispatcherChan, sinkMap *pkg.Dispatch2DataSourceChan) {
	logger := pkg.LoggerFromContext(dis.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("===聚合器启动===")
	dis.SinkMap = sinkMap
	for {
		select {
		case deviceList := <-(*source):
			// 创建point处理计时器
			pointTimer := metrics.NewTimer("aggregator_point")
			for _,  frame:= range deviceList {
				logger.Debug("received point", zap.Any("point", frame))
				err := dis.launch(frame)
			// 记录接收到的点
			metrics.IncMsgReceived("aggregator")


			if err != nil {
				// 记录错误
				metrics.IncErrorCount()
				metrics.IncMsgErrors("aggregator")
				logger.Error("error adding point", zap.Error(err))
			} else {
				// 记录成功处理
				metrics.IncMsgProcessed("aggregator")
			}

			pointTimer.StopAndLog(logger) // 停止计时器并记录
		default:
		}

	}
}

func (dis *Dispatcher) InitDataSink(deviceName string) error {
	strategies := pkg.ConfigFromContext(agg.ctx).Strategy
	for _, strategy := range strategies {
		if strategy.Filter == nil {
			// 如果策略没有过滤条件，则默认接收所有字段
			strategy.Filter = []string{".*"}
		}
		for _, filter := range strategy.Filter {
			// 遍历字段，判断是否符合策略过滤条件
			if ok, err := checkFilter(deviceName, filter); ok && (err == nil) {
				// 检查 DataSink 是否已经存在该策略对应的 Point
				if _, exists := agg.Road[deviceName]; !exists {
					// 不存在则初始化数组并添加
					agg.Road[deviceName] = []string{strategy.Type}
				} else {
					// 如果 Road 已存在，更新其字段引用
					agg.Road[deviceName] = append(agg.Road[deviceName], strategy.Type)
				}
			} else if err != nil {
				// 如果过滤条件不符合预期语法
				return fmt.Errorf("error compiling regex: %v", err)
			}
		}
	}
	return nil
}

// checkFilter 根据filter正则推断Strategies遥测名称的匹配
func checkFilter(device, filter string) (bool, error) {
	// 解析过滤语法，语法为：xagx.vobc.vobc0001.speed.v

	// 编译设备类型、设备名称和遥测名称的正则表达式
	deviceRe, err := regexp.Compile(filter)

	// 检查正则表达式编译错误
	if err != nil {
		return false, fmt.Errorf("error compiling regex: %v", err)
	}
	// 分别匹配设备类型、设备名称和遥测名称
	return deviceRe.MatchString(device), nil
}

// launch 方法用于启动聚合器的发送流程
func (dis *Dispatcher) launch(device map[string]*pkg.Point) {
	logger := pkg.LoggerFromContext(dis.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Debug("launching", zap.Any("dis.Road", dis.Road), zap.String("sinkMap", fmt.Sprintf("%v", dis.SinkMap)))

	// 创建发送计时器
	sendTimer := metrics.NewTimer("aggregator_send")

	// 记录要发送的点总数
	pointCount := 0

	// 发之前先判断是否存在Road




	duration := sendTimer.Stop() // 停止计时器
	if pointCount > 0 {
		logger.Debug("发送完成",
			zap.Duration("duration", duration),
			zap.Int("pointCount", pointCount),
			zap.Float64("avgTime", float64(duration.Nanoseconds())/float64(pointCount)/1000000)) // 平均每点毫秒
	}
}
