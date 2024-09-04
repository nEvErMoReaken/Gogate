package strategyImpl

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"gw22-train-sam/config"
	"gw22-train-sam/logger"
	"gw22-train-sam/model"
	"time"
)

// InfluxDbStrategy 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbStrategy struct {
	client          influxdb2.Client
	batch           *model.SnapshotBatch
	publishInterval time.Duration
	stopChan        chan struct{}
	writeAPI        api.WriteAPI
	eventChan       chan struct{} // 用于通知立即发送的事件通道
}

// NewInfluxDbStrategy 构造函数
func NewInfluxDbStrategy(dbConfig config.InfluxDBConfig, stopChan chan struct{}) *InfluxDbStrategy {
	client := influxdb2.NewClientWithOptions(dbConfig.URL, dbConfig.Token, influxdb2.DefaultOptions().SetBatchSize(uint(dbConfig.BatchSize)))
	// 获取写入 API
	writeAPI := client.WriteAPI(dbConfig.ORG, dbConfig.Bucket)
	// Get errors channel
	errorsCh := writeAPI.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			logger.Log.Errorf("write error: %s\n", err.Error())
		}
	}()
	return &InfluxDbStrategy{
		batch:           model.NewSnapshotBatch(),
		publishInterval: dbConfig.Tips.Interval,
		stopChan:        stopChan,
		client:          client,
		writeAPI:        writeAPI,
		eventChan:       make(chan struct{}, 1), // 非阻塞通道，缓冲区大小为1
	}
}

// AddDevice 注：在InfluxDB案例下，即使是“立即发送”，也是先缓存到内存中，然后定期发送。其还会受到InfluxDB的批量写入机制的影响。
func (b *InfluxDbStrategy) AddDevice(device model.DeviceSnapshot) {
	b.batch.Put(device)
	if b.publishInterval == 0 {
		select {
		case b.eventChan <- struct{}{}:
			// 发送信号，通知立即发送
		default:
			// 如果 eventChan 已满，则跳过，防止阻塞
		}
	}
}

// Start 启动 InfluxDBStrategy, 如果 publishInterval 大于 0，则启动定期发送逻辑
func (b *InfluxDbStrategy) Start() {
	defer b.client.Close()
	if b.publishInterval > 0 {
		ticker := time.NewTicker(b.publishInterval)
		defer ticker.Stop()
		for {
			select {
			case <-b.stopChan:
				b.writeAPI.Flush() // 在停止时强制刷新所有数据
				return
			case <-b.eventChan:
				// 处理立即发送的逻辑
				b.Publish()
			case <-ticker.C:
				// 定期发送
				b.Publish()
			}
		}
	} else {
		// 如果不聚合，不启动任何逻辑
		// 因为 AddDevice 已经处理了立即发送
		<-b.stopChan       // 阻塞，直到收到 stop 信号
		b.writeAPI.Flush() // 在停止时强制刷新所有数据
	}
}

// Publish 存入数据库的逻辑
func (b *InfluxDbStrategy) Publish() {
	// 将数据发布到 InfluxDB 的逻辑
	// 遍历批次中的所有设备快照并创建数据点
	for _, snapshots := range b.batch.GetAll() {
		for _, snapshot := range snapshots {
			// 创建一个数据点
			p := influxdb2.NewPoint(
				snapshot.DeviceType,
				map[string]string{
					"train_id": snapshot.DeviceName,
				},
				snapshot.Fields,
				snapshot.Ts,
			)
			b.writeAPI.WritePoint(p)
		}
	}
	b.writeAPI.Flush()
}
