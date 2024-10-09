package strategy

import (
	"gateway/common"
	"gateway/model"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/mitchellh/mapstructure"
	"strconv"
)

// 拓展数据源步骤
// init Step.1
func init() {
	// 注册发送策略
	model.RegisterStrategy("influxdb", NewInfluxDbStrategy)
}

// GetChan Step.2
func (b *InfluxDbStrategy) GetChan() chan model.Point {
	return b.pointChan
}

// Start Step.3
func (b *InfluxDbStrategy) Start() {
	defer b.client.Close()
	common.Log.Info("InfluxDBStrategy started")
	for {
		select {
		case <-b.stopChan:
			b.writeAPI.Flush() // 在停止时强制刷新所有数据
			return
		case point := <-b.pointChan:
			b.Publish(point)
		}
	}
}

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client    influxdb2.Client
	pointChan chan model.Point
	stopChan  chan struct{}
	writeAPI  api.WriteAPI
	info      InfluxDbInfo
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

// NewInfluxDbStrategy 构造函数
func NewInfluxDbStrategy(dbConfig *common.StrategyConfig, stopChan chan struct{}) model.SendStrategy {
	var info InfluxDbInfo
	// 将 map 转换为结构体
	if err := mapstructure.Decode(dbConfig.Config, &info); err != nil {
		common.Log.Fatalf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
	}

	client := influxdb2.NewClientWithOptions(info.URL, info.Token, influxdb2.DefaultOptions().SetBatchSize(info.BatchSize))
	// 获取写入 API
	writeAPI := client.WriteAPI(info.Org, info.Bucket)
	// Get errors channel
	errorsCh := writeAPI.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			common.Log.Errorf("write error: %s\n", err.Error())
		}
	}()
	return &InfluxDbStrategy{
		pointChan: make(chan model.Point, 200), // 容量为200的通道
		stopChan:  stopChan,
		client:    client,
		writeAPI:  writeAPI,
		info:      info,
	}
}

func (b *InfluxDbStrategy) Publish(point model.Point) {
	// ～～～将数据发布到 InfluxDB 的逻辑～～～
	common.Log.Debugf("正在发送 %+v", point)

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
				common.Log.Warnf("Unexpected type for key %s in tagsMap", key)
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
	common.Log.Debugf("正在发送:\n , %+v, %+v, %+v, %+v", point.DeviceType, point.DeviceName, decodedFields, point.Ts)
	// 写入到 InfluxDB
	b.writeAPI.WritePoint(p)

}

// Stop 停止 InfluxDBStrategy
func (b *InfluxDbStrategy) Stop() {
	b.writeAPI.Flush() // 确保所有数据被写入
	b.client.Close()   // 关闭 InfluxDB 客户端
}
