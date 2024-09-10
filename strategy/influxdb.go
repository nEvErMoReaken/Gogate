package strategy

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/mitchellh/mapstructure"
	"gw22-train-sam/common"
	"gw22-train-sam/dataSource/byteType/tcpServer"
	"gw22-train-sam/model"
	"log"
)

// 拓展数据源步骤
// init Step.1
func init() {
	// 注册发送策略
	Register("influxdb", NewInfluxDbStrategy)
}

// GetChan Step.2
func (b *InfluxDbStrategy) GetChan() chan model.Point {
	return b.pointChan
}

// Start Step.3
func (b *InfluxDbStrategy) Start() {
	defer b.client.Close()
	for {
		select {
		case <-b.stopChan:
			b.writeAPI.Flush() // 在停止时强制刷新所有数据
			return
		case point := <-b.pointChan:
			b.Publish(&point)
		}
	}
}

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client    influxdb2.Client
	pointChan chan model.Point
	stopChan  chan struct{}
	writeAPI  api.WriteAPI
}

// infoType InfluxDB的专属配置
type infoType struct {
	URL       string `mapstructure:"url"`
	Org       string `mapstructure:"org"`
	Token     string `mapstructure:"token"`
	Bucket    string `mapstructure:"bucket"`
	BatchSize uint   `mapstructure:"batch_size"`
}

// NewInfluxDbStrategy 构造函数
func NewInfluxDbStrategy(dbConfig tcpServer.StrategyConfig, stopChan chan struct{}) SendStrategy {
	var info infoType
	// 将 map 转换为结构体
	if err := mapstructure.Decode(dbConfig.Config, &info); err != nil {
		log.Fatalf("[NewInfluxDbStrategy] Error decoding map to struct: %v", err)
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
	}
}

// Publish 存入数据库的逻辑
func (b *InfluxDbStrategy) Publish(point *model.Point) {
	// 将数据发布到 InfluxDB 的逻辑
	// 遍历批次中的所有设备快照并创建数据点
	// 创建一个数据点
	p := influxdb2.NewPoint(
		point.DeviceType,
		map[string]string{
			"train_id": point.DeviceName,
		},
		point.Field,
		point.Ts,
	)
	b.writeAPI.WritePoint(p)
}

// Stop 停止 InfluxDBStrategy
func (b *InfluxDbStrategy) Stop() {
	b.writeAPI.Flush() // 确保所有数据被写入
	b.client.Close()   // 关闭 InfluxDB 客户端
}
