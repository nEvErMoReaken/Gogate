package aggregator

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
	"regexp"
)

type Aggregator struct {
	ctx     context.Context
	SinkMap *pkg.StrategyDataSource // 策略名称 -> 其附属的数据源通道
	Road    map[string][]string     // 设备名称 -> 其对应的策略名称
	Cache   map[string]pkg.Point    // 设备名称 -> 其对应的Point
}

var New = func(ctx context.Context) *Aggregator {
	return &Aggregator{
		ctx:   ctx,
		Road:  make(map[string][]string),
		Cache: make(map[string]pkg.Point),
	}
}

// Start 启动聚合器
func (agg *Aggregator) Start(source *pkg.AggregatorDataSource, sinkMap *pkg.StrategyDataSource) {
	agg.SinkMap = sinkMap
	for {
		select {
		case <-source.EndChan:
			agg.launch()
		case <-agg.ctx.Done():
			return
		default:
			select {
			case point := <-source.PointChan:
				pkg.LoggerFromContext(agg.ctx).Debug("received point", zap.Any("point", point))
				err := agg.add(point)
				if err != nil {
					pkg.LoggerFromContext(agg.ctx).Error("error adding point", zap.Error(err))
				}
			default:
			}
		}
	}
}

// add 向聚合器加入Point
func (agg *Aggregator) add(point pkg.Point) error {
	// 如果 Road 中没有该设备的策略，则初始化数据源（根据正则策略）
	if _, exist := agg.Road[point.Device]; !exist {
		err := agg.InitDataSink(point.Device)
		if err != nil {
			return fmt.Errorf("error initializing data sink: %v", err)
		}
	}
	// 如果 Cache 中没有该设备的 Point，则直接加入
	if _, exist := agg.Cache[point.Device]; !exist {
		agg.Cache[point.Device] = point
	} else {
		// 如果 Cache 中已经有该设备的 Point，则合并
		p := agg.Cache[point.Device]
		p.Merge(point)
	}
	return nil
}

func (agg *Aggregator) InitDataSink(deviceName string) error {
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
func (agg *Aggregator) launch() {
	pkg.LoggerFromContext(agg.ctx).Debug("launching", zap.Any("agg.Road", agg.Road), zap.Any("agg.Cache", agg.Cache), zap.String("sinkMap", fmt.Sprintf("%v", agg.SinkMap)))
	for device, cache := range agg.Cache {
		strategyList := agg.Road[device]
		for _, strategy := range strategyList {
			// 发送数据
			agg.SinkMap.PointChan[strategy] <- cache
		}
	}
	// 清空缓存
	agg.Cache = make(map[string]pkg.Point)
}
