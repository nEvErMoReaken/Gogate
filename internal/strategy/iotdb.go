package strategy

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
	"strings"

	"github.com/apache/iotdb-client-go/client"
	"github.com/apache/iotdb-client-go/rpc"
	"github.com/mitchellh/mapstructure"
)

// 初始化函数，注册 IoTDB 策略
func init() {
	// 注册发送策略
	Register("iotdb", NewIoTDBStrategy)
}

// IoTDBStrategy 实现将数据发布到 IoTDB 的逻辑
type IoTDBStrategy struct {
	sessionPool SessionPoolInterface
	info        IotDBInfo
	ctx         context.Context
	logger      *zap.Logger
}

// SessionInterface 定义了我们需要使用的 iotdbclient.Session 方法
type SessionInterface interface {
	InsertAlignedRecordsOfOneDevice(deviceId string, timestamps []int64, measurements [][]string, dataTypes [][]client.TSDataType, values [][]interface{}, isAligned bool) (*rpc.TSStatus, error)
	Close()
}

// SessionPoolInterface 定义了 SessionPool 的行为
type SessionPoolInterface interface {
	GetSession() (SessionInterface, error)
	PutBack(session SessionInterface)
	Close()
}

// Session 包装了 iotdbclient.Session
type Session struct {
	session client.Session
}

func (s *Session) InsertAlignedRecordsOfOneDevice(deviceId string, timestamps []int64, measurements [][]string, dataTypes [][]client.TSDataType, values [][]interface{}, isAligned bool) (*rpc.TSStatus, error) {
	return s.session.InsertAlignedRecordsOfOneDevice(deviceId, timestamps, measurements, dataTypes, values, isAligned)
}

func (s *Session) Close() {
	_, err := s.session.Close()
	if err != nil {
		return
	}
}

// SessionPool 包装了 iotdbclient.SessionPool
type SessionPool struct {
	pool *client.SessionPool
}

func (sp *SessionPool) GetSession() (SessionInterface, error) {
	session, err := sp.pool.GetSession()
	if err != nil {
		return nil, err
	}
	return &Session{session: session}, nil
}

func (sp *SessionPool) PutBack(session SessionInterface) {
	if s, ok := session.(*Session); ok {
		sp.pool.PutBack(s.session)
	}
}

func (sp *SessionPool) Close() {
	sp.pool.Close()
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

func NewIoTDBStrategy(ctx context.Context) (Template, error) {
	c := pkg.ConfigFromContext(ctx)
	var info IotDBInfo
	for _, strategyConfig := range c.Strategy {
		if strategyConfig.Enable && strategyConfig.Type == "iotdb" {
			// 将 map 转换为结构体
			if err := mapstructure.Decode(strategyConfig.Para, &info); err != nil {
				return nil, fmt.Errorf("[NewIoTDBStrategy] Error decoding map to struct: %v", err)
			}
		}
	}

	var sessionPool SessionPoolInterface
	if info.mode == "cluster" {
		// 集群模式
		config := &client.PoolConfig{
			NodeUrls:        strings.Split(info.URL, ","),
			UserName:        info.UserName,
			Password:        info.Password,
			FetchSize:       info.BatchSize,
			TimeZone:        "Asia/Shanghai",
			ConnectRetryMax: 5,
		}
		pool := client.NewSessionPool(config, 3, 60000, 60000, false)
		sessionPool = &SessionPool{pool: &pool}
	} else {
		// 单机模式
		config := &client.PoolConfig{
			Host:            info.Host,
			Port:            info.Port,
			UserName:        info.UserName,
			Password:        info.Password,
			FetchSize:       info.BatchSize,
			TimeZone:        "Asia/Shanghai",
			ConnectRetryMax: 5,
		}
		pool := client.NewSessionPool(config, 3, 60000, 60000, false)
		sessionPool = &SessionPool{pool: &pool}
	}
	pkg.LoggerFromContext(ctx).Debug("IoTDB 配置", zap.Any("info", info))
	return &IoTDBStrategy{
		logger:      pkg.LoggerFromContext(ctx),
		sessionPool: sessionPool,
		info:        info,
		ctx:         ctx,
	}, nil
}

// GetChan Step.1
func (b *IoTDBStrategy) GetType() string {
	return "iotdb"
}

// Start Step.2 启动策略
func (b *IoTDBStrategy) Start(pointChan chan pkg.Point) {
	b.logger.Info("===IoTDBStrategy started===")
	for {
		select {
		case <-b.ctx.Done():
			b.Stop()
			b.logger.Info("===IoTDBStrategy stopped===")
		case point := <-pointChan:
			err := b.Publish(point)
			if err != nil {
				pkg.ErrChanFromContext(b.ctx) <- fmt.Errorf("IoTDBStrategy error occurred: %w", err)
			}
		}
	}
}

// Publish 将数据发布到 IoTDB
func (b *IoTDBStrategy) Publish(point pkg.Point) error {
	log := b.logger // 避免每次都要强转一次
	// 日志记录
	log.Debug("正在发送 %+v", zap.Any("point", point))
	session, err := b.sessionPool.GetSession()
	defer b.sessionPool.PutBack(session)
	if err != nil {
		return fmt.Errorf("failed to get session %+v", err)
	}

	var (
		deviceId     string // 设备 ID
		measurements = [][]string{make([]string, 0)}
		values       = [][]interface{}{make([]interface{}, 0)}
		dataTypes    = [][]client.TSDataType{make([]client.TSDataType, 0)}
	)
	// 遍历字段
	for key, valuePtr := range point.Field {
		if valuePtr == nil {
			continue // 跳过 nil 值
		}

		value := valuePtr

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
			log.Warn("Unsupported data type",
				zap.String("key", key), // key 的值可以根据实际情况调整类型
				zap.Any("value", v),    // v 的值可以是任意类型
			)
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
	err = checkError(session.InsertAlignedRecordsOfOneDevice(deviceId, timestamp, measurements, dataTypes, values, false))
	if err != nil {
		return err
	}
	return nil
}

// Stop 停止策略
func (b *IoTDBStrategy) Stop() {
	b.sessionPool.Close()
}

func checkError(status *rpc.TSStatus, err error) error {
	if err != nil {
		return err
	}
	if status != nil {
		if err = client.VerifySuccess(status); err != nil {
			return err
		}
	}
	return nil
}
