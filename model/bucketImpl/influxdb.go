package bucketImpl

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"gw22-train-sam/config"
	"gw22-train-sam/logger"
	"gw22-train-sam/model"
	"time"
)

// InfluxDbBucket 实现将数据发布到 InfluxDB 的逻辑
type InfluxDbBucket struct {
	client          influxdb2.Client
	batch           *model.SnapshotBatch
	publishInterval time.Duration
	stopChan        chan struct{}
	writeAPI        api.WriteAPI
}

// NewInfluxDbBucket 构造函数
func NewInfluxDbBucket(dbConfig config.InfluxDBConfig, stopChan chan struct{}) *InfluxDbBucket {
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
	return &InfluxDbBucket{
		batch:           model.NewSnapshotBatch(),
		publishInterval: dbConfig.Tips.Interval,
		stopChan:        stopChan,
		client:          client,
		writeAPI:        writeAPI,
	}
}

// AddDevice 注：在InfluxDB案例下，即使是“立即发送”，也是先缓存到内存中，然后定期发送。其还会受到InfluxDB的批量写入机制的影响。
func (b *InfluxDbBucket) AddDevice(device model.DeviceSnapshot) {
	b.batch.Put(device)
	if b.publishInterval == 0 {
		b.Publish() // 不聚合时，有消息就发送
	}
}

// Start 启动 InfluxDBBucket, 如果 publishInterval 大于 0，则启动定期发送逻辑
func (b *InfluxDbBucket) Start() {
	defer b.client.Close()
	if b.publishInterval > 0 {
		ticker := time.NewTicker(b.publishInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.Publish() // 定期发送
			case <-b.stopChan:
				b.writeAPI.Flush() // 在停止时强制刷新所有数据
				return
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
func (b *InfluxDbBucket) Publish() {
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
