package sink

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"strconv"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// 拓展数据源步骤
func init() {
	// 注册发送策略
	Register("influxdb", NewInfluxDbStrategy)
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

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	info     InfluxDbInfo
	ctx      context.Context
	logger   *zap.Logger
}

// NewInfluxDbStrategy Step.0 构造函数
func NewInfluxDbStrategy(ctx context.Context) (Template, error) {
	config := pkg.ConfigFromContext(ctx)

	var info InfluxDbInfo
	for _, strategyConfig := range config.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "influxdb" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				return nil, fmt.Errorf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
			}
		}
	}
	// 检查 BatchSize 是否为零或未设置，如果是，使用默认值 否则会出现 /0 的panic
	if info.BatchSize == 0 {
		info.BatchSize = 100 // 使用默认的批处理大小
	}
	pkg.LoggerFromContext(ctx).Debug("InfluxDB配置", zap.Any("info", info))
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
		ctx:      ctx,
	}, nil
}

// GetType Step.1
func (b *InfluxDbStrategy) GetType() string {
	return "influxdb"
}

// Start Step.2
func (b *InfluxDbStrategy) Start(sink chan *pkg.PointPackage) {
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	defer b.client.Close()
	b.logger.Debug("===InfluxDbStrategy started===")
	for {
		select {
		case <-b.ctx.Done():
			b.Stop()
			b.logger.Debug("===InfluxDbStrategy stopped===")
			return
		case point := <-sink:
			// 创建发布计时器
			publishTimer := metrics.NewTimer("influxdb_publish")

			// 记录接收的点
			metrics.IncMsgReceived("influxdb")

			err := b.Publish(point)

			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("influxdb")
				pkg.ErrChanFromContext(b.ctx) <- fmt.Errorf("InfluxDbStrategy error occurred: %w", err)
			} else {
				// 记录成功处理的点
				metrics.IncMsgProcessed("influxdb")
			}

			publishTimer.StopAndLog(b.logger)
		}
	}
}

func (b *InfluxDbStrategy) Publish(pointPackage *pkg.PointPackage) error {
	// ～～～将数据发布到 InfluxDB 的逻辑～～～
	b.logger.Debug("正在发送 %+v", zap.Any("point", pointPackage))

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
	for _, point := range pointPackage.Points {
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
		tagsMap["devName"] = point.Device

		//common.Log.Debugf("正在发送 %+v", decodedFields)
		// 创建一个数据点
		p := influxdb2.NewPoint(
			point.Device,    // measurement
			tagsMap,         // tags
			decodedFields,   // fields (converted)
			pointPackage.Ts, // timestamp
		)
		// 写入到 InfluxDB
		b.writeAPI.WritePoint(p)

		b.logger.Info("InfluxDBStrategy published", zap.Any("point", point), zap.String("frameId", pointPackage.FrameId))

	}

	return nil
}

// Stop 停止 InfluxDBStrategy
func (b *InfluxDbStrategy) Stop() {
	b.writeAPI.Flush() // 确保所有数据被写入
	b.client.Close()   // 关闭 InfluxDB 客户端
}
