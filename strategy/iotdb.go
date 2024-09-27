package strategy

import (
	"fmt"
	"gateway/common"
	"gateway/model"
	"github.com/apache/iotdb-client-go/client"
	"github.com/apache/iotdb-client-go/rpc"
	"github.com/mitchellh/mapstructure"
	"strings"
)

// 初始化函数，注册 IoTDB 策略
func init() {
	// 注册发送策略
	model.RegisterStrategy("iotdb", NewIoTDBStrategy)
}

// IoTDBStrategy 实现将数据发布到 IoTDB 的逻辑
type IoTDBStrategy struct {
	sessionPool *client.SessionPool
	pointChan   chan model.Point
	stopChan    chan struct{}
	info        IotDBInfo
}

// IotDBInfo IoTDB 的专属配置
type IotDBInfo struct {
	Host      string `mapstructure:"host"`
	Port      string `mapstructure:"port"`
	mode      string `mapstructure:"mode"`
	URL       string `mapstructure:"url"`
	UserName  string `mapstructure:"username"`
	Password  string `mapstructure:"password"`
	BatchSize int32  `mapstructure:"batch_size"`
}

// NewIoTDBStrategy 构造函数
func NewIoTDBStrategy(dbConfig *common.StrategyConfig, stopChan chan struct{}) model.SendStrategy {
	var info IotDBInfo
	// 将 map 转换为结构体
	if err := mapstructure.Decode(dbConfig.Config, &info); err != nil {
		common.Log.Fatalf("[NewIoTDBStrategy] Error decoding map to struct: %v", err)
	}
	var sessionPool client.SessionPool
	if info.mode == "cluster" {
		// 集群模式
		config := &client.PoolConfig{
			NodeUrls: strings.Split(info.URL, ","),
			UserName: info.UserName,
			Password: info.Password,
			// 可选配置
			FetchSize:       info.BatchSize,
			TimeZone:        "Asia/Shanghai",
			ConnectRetryMax: 5,
		}
		sessionPool = client.NewSessionPool(config, 3, 60000, 60000, false)
	} else {
		// 单机模式
		config := &client.PoolConfig{
			Host:     info.Host,
			Port:     info.Port,
			UserName: info.UserName,
			Password: info.Password,
			// 可选配置
			FetchSize:       info.BatchSize,
			TimeZone:        "Asia/Shanghai",
			ConnectRetryMax: 5,
		}
		sessionPool = client.NewSessionPool(config, 3, 60000, 60000, false)
	}

	return &IoTDBStrategy{
		pointChan:   make(chan model.Point, 200), // 容量为 200 的通道
		stopChan:    stopChan,
		sessionPool: &sessionPool,
		info:        info,
	}
}

// GetChan 返回通道
func (b *IoTDBStrategy) GetChan() chan model.Point {
	return b.pointChan
}

// Start 启动策略
func (b *IoTDBStrategy) Start() {

	common.Log.Info("IoTDBStrategy started")
	for {
		select {
		case <-b.stopChan:
			// 在停止时处理必要的清理
			return
		case point := <-b.pointChan:
			b.Publish(point)
		}
	}
}

// Publish 将数据发布到 IoTDB
func (b *IoTDBStrategy) Publish(point model.Point) {
	// 日志记录
	common.Log.Debugf("正在发送 %+v", point)
	session, err := b.sessionPool.GetSession()
	defer b.sessionPool.PutBack(session)
	if err == nil {
		common.Log.Error(err)
		return
	}

	var (
		deviceId     string // 设备 ID
		measurements = make([][]string, 0)
		values       = make([][]interface{}, 0)
		dataTypes    = make([][]client.TSDataType, 0) //dataTypes 切片
	)

	// 遍历字段
	for key, valuePtr := range point.Field {
		if valuePtr == nil {
			continue // 跳过 nil 值
		}

		value := *valuePtr

		// 添加到测量和值列表
		measurements[0] = append(measurements[0], key)
		values[0] = append(values[0], value)
		// 根据值的类型生成对应的 dataType
		switch v := value.(type) {
		case int8, int16, int32:
			dataTypes[0] = append(dataTypes[0], client.INT32)
		case int, int64:
			dataTypes[0] = append(dataTypes[0], client.INT64)
		case float32:
			dataTypes[0] = append(dataTypes[0], client.FLOAT)
		case float64:
			dataTypes[0] = append(dataTypes[0], client.DOUBLE)
		case bool:
			dataTypes[0] = append(dataTypes[0], client.BOOLEAN)
		case string:
			dataTypes[0] = append(dataTypes[0], client.TEXT)
		default:
			common.Log.Warnf("Unsupported data type for key %s: %T", key, v)
			// 可以选择跳过该值，或者返回错误
			// 此处选择跳过
			continue
		}
	}

	if point.DeviceType != "" {
		deviceId = fmt.Sprintf("root.%s.%s", point.DeviceType, point.DeviceName)
	} else {
		deviceId = fmt.Sprintf("root.%s", point.DeviceName)
	}

	// 设置时间戳（毫秒）
	timestamp := []int64{point.Ts.UnixNano() / 1e6}

	// 插入记录
	checkError(session.InsertAlignedRecordsOfOneDevice(deviceId, timestamp, measurements, dataTypes, values, false))

}

// Stop 停止策略
func (b *IoTDBStrategy) Stop() {
	b.sessionPool.Close()
	common.Log.Infof("IoTDBStrategy stopped")
	//if err := b.sessionPool.Close(); err != nil {
	//	common.Log.Errorf("Failed to close IoTDB session: %v", err)
	//}
}

func checkError(status *rpc.TSStatus, err error) {
	if err != nil {
		common.Log.Fatal(err)
	}
	if status != nil {
		if err = client.VerifySuccess(status); err != nil {
			common.Log.Fatal(err)
		}
	}
}
