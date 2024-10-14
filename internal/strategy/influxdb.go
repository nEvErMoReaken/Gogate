package strategy

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"strconv"
)

// 拓展数据源步骤
func init() {
	// 注册发送策略
	Register("influxdb", NewInfluxDbStrategy)
}

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	info     InfluxDbInfo
	core     Core
	logger   *zap.Logger
}

// NewInfluxDbStrategy Step.0 构造函数
func NewInfluxDbStrategy(ctx context.Context) (Strategy, error) {
	config := pkg.ConfigFromContext(ctx)
	var info InfluxDbInfo
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "influxDB" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig, &info); err != nil {
				return nil, fmt.Errorf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
			}
		}
	}
	client := influxdb2.NewClientWithOptions(info.URL, info.Token, influxdb2.DefaultOptions().SetBatchSize(info.BatchSize))
	// 获取写入 API
	writeAPI := client.WriteAPI(info.Org, info.Bucket)
	// Get errors channel
	errorsCh := writeAPI.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			pkg.LoggerFromContext(ctx).Error("write error: %s\n", zap.Error(err))
			// 不清楚Influxdb通道内的错误是否会导致程序退出，所以这里不直接返回错误
		}
	}()
	return &InfluxDbStrategy{
		logger:   pkg.LoggerFromContext(ctx),
		client:   client,
		writeAPI: writeAPI,
		info:     info,
		core:     Core{StrategyType: "influxDB", pointChan: make(chan pkg.Point, 200), ctx: ctx},
	}, nil
}

// GetCore Step.1
func (b *InfluxDbStrategy) GetCore() Core {
	return b.core
}

// GetChan Step.2
func (b *InfluxDbStrategy) GetChan() chan pkg.Point {
	return b.core.pointChan
}

// Start Step.3
func (b *InfluxDbStrategy) Start() {
	defer b.client.Close()
	pkg.LoggerFromContext(b.core.ctx).Info("===InfluxDbStrategy started===")
	for {
		select {
		case <-b.core.ctx.Done():
			b.Stop()
			pkg.LoggerFromContext(b.core.ctx).Info("===IoTDBStrategy stopped===")
		case point := <-b.core.pointChan:
			err := b.Publish(point)
			if err != nil {
				pkg.ErrChanFromContext(b.core.ctx) <- fmt.Errorf("IoTDBStrategy error occurred: %w", err)
			}
		}
	}
}

// InfluxDbInfo InfluxDB的专属配置
type InfluxDbInfo struct {
	URL       string   `mapstructure:"url"`
	Org       string   `mapstructure:"org"`
	Token     string   `mapstructure:"token"`
	Bucket    string   `mapstructure:"bucket"`
	BatchSize uint     `mapstructure:"batch_size"`
	Tags      []string `mapstructure:"tags"`
}

func (b *InfluxDbStrategy) Publish(point pkg.Point) error {
	// ～～～将数据发布到 InfluxDB 的逻辑～～～
	b.logger.Debug("正在发送 %+v", zap.Any("point", point))

	// 创建一个新的 map[string]interface{} 来存储解引用的字段
	decodedFields := make(map[string]interface{})
	// 将 b.info.Tags 转换为一个 map，以便快速查找
	tagsSet := make(map[string]struct{})
	if b.info.Tags != nil {
		for _, tag := range b.info.Tags {
			tagsSet[tag] = struct{}{}
		}
	}
	tagsMap := make(map[string]string)
	// 遍历 point.Field
	for key, valuePtr := range point.Field {
		if valuePtr == nil {
			continue // 如果值为 nil，直接跳过
		}

		value := valuePtr
		// 判断 key 是否在 tags 中
		if _, isTag := tagsSet[key]; isTag {
			// 如果是 tags 中的字段，处理类型转换
			switch v := value.(type) {
			case int:
				tagsMap[key] = strconv.Itoa(v)
			case int64:
				tagsMap[key] = strconv.Itoa(int(v))
			case float64:
				tagsMap[key] = strconv.FormatFloat(v, 'f', -1, 64)
			case string:
				tagsMap[key] = v
			case bool:
				tagsMap[key] = strconv.FormatBool(v)
			default:
				b.logger.Warn("Unexpected type for key %s in tagsMap", zap.Any("key", key))
			}
		} else {
			// 如果不是 tags 中的字段，直接放入 decodedFields
			decodedFields[key] = value
		}
	}
	tagsMap["devName"] = point.DeviceName
	//common.Log.Debugf("正在发送 %+v", decodedFields)
	// 创建一个数据点
	p := influxdb2.NewPoint(
		point.DeviceType, // measurement
		tagsMap,          // tags
		decodedFields,    // fields (converted)
		point.Ts,         // timestamp
	)
	// 写入到 InfluxDB
	b.writeAPI.WritePoint(p)
	b.logger.Debug("InfluxDBStrategy published", zap.Any("point", point))
	return nil
}

// Stop 停止 InfluxDBStrategy
func (b *InfluxDbStrategy) Stop() {
	b.writeAPI.Flush() // 确保所有数据被写入
	b.client.Close()   // 关闭 InfluxDB 客户端
}
