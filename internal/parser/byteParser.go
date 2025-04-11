package parser

import (
	"context"
	"encoding/hex"
	"fmt"
	"gateway/internal/pkg"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type byteParserConfig struct {
	Dir        string `mapstructure:"dir"`
	ProtoFile  string `mapstructure:"protoFile"`
	BufferSize int    `mapstructure:"bufferSize"`
}

// ByteParser 用于解析二进制数据流
type ByteParser struct {
	Chunks []Chunk `mapstructure:"chunks"`
	ctx    context.Context
}

func NewByteParser(ctx context.Context) (Template, error) {

	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var c byteParserConfig
	err := mapstructure.Decode(v.Parser.Para, &c)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	// 2. 初始化协议配置文件
	chunksConfig, exist := pkg.ConfigFromContext(ctx).Others[c.ProtoFile]
	if !exist {
		pkg.LoggerFromContext(ctx).Error("未找到协议文件", zap.String("ProtoFile", c.ProtoFile))
		return nil, fmt.Errorf("未找到协议文件:%s", c.ProtoFile)
	}
	pkg.LoggerFromContext(ctx).Debug("协议文件", zap.Any("chunks", chunksConfig))
	chunks, ok := chunksConfig.(map[string]interface{})
	if !ok {
		pkg.LoggerFromContext(ctx).Error("协议文件格式错误", zap.Any("chunks", chunksConfig))
		return nil, fmt.Errorf("协议文件格式错误")
	}
	// 3. 初始化 ByteParser
	byteParser, err := CreateByteParser(ctx, c, chunks["chunks"].([]interface{}))
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("未找到协议文件", zap.Error(err))
		return nil, fmt.Errorf("初始化ByteParser失败: %s", err)
	}
	return byteParser, nil
}

func (r *ByteParser) GetType() string {
	return "stream"
}

// Parse 方法用于解析数据的纯函数
func (r *ByteParser) Parse(buffer []byte) (points []*pkg.Point, err error) {

}

// StartWithRingBuffer 方法用于启动一个基于RingBuffer的ByteParser
func (r *ByteParser) StartWithRingBuffer(ring *pkg.RingBuffer, sink chan *pkg.Frame2Point) {
	logger := pkg.LoggerFromContext(r.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("===ByteParser 开始处理数据===")

	go func() {
		select {
		case <-r.ctx.Done():
			return
		default:
			ctx := r.ctx
			// 4.1 绑定默认时间, 协议中有可能覆盖
			ctx = context.WithValue(ctx, "ts", time.Now())

			// 4.2 记录接收消息
			metrics.IncMsgReceived("byteParser")
			rawPointList := make([]*pkg.Point, 0)
			start := ring.ReadPos() // 记录帧起始位置
			// ** 此处是完整的一帧的开始 **
			for index, chunk := range r.Chunks {
				// 4.2.2 处理Chunk
				ctx, err = chunk.Process(ctx, ring, rawPointList)
				if err != nil {
					// 记录错误
					metrics.IncErrorCount()
					metrics.IncMsgErrors("byteParser")
					pkg.ErrChanFromContext(r.ctx) <- fmt.Errorf("解析第 %d 个 Chunk 失败: %s", index+1, err) // 其他错误，终止连接
					return                                                                             // 解析失败时终止处理
				}
			}
			end := ring.ReadPos() // 记录帧结束位置
			// ** 此处是完整的一帧的结束 **
			// 4.3聚合数据点
			aggregatedPoints, err := pkg.AggregatePoints(rawPointList)
			if err != nil {
				logger.Error("聚合数据点失败", zap.Error(err))
				metrics.IncMsgErrors("byteParser")
				return
			}
			// 4.4 自增计数，获取计数，生成帧ID
			frameId := fmt.Sprintf("%06X", metrics.IncMsgProcessed("byteParser"))
			// 4.5 发送聚合后的数据点
			sink <- &pkg.Frame2Point{
				FrameId: frameId,
				Point:   aggregatedPoints,
			}

			raw := ring.Snapshot(start, end)
			hexRaw := hex.EncodeToString(raw)
			// 4.6 打印原始报文
			logger.Info("Frame",
				zap.String("count", frameId), // 使用 6 位 16 进制数格式化 count
				zap.String("frame", hexRaw))  // frame 转为16进制字符串
		}
	}()
}

// ByteParser 的 String 方法
func (r *ByteParser) String() string {
	result := "IoReader:"
	for i, chunk := range r.Chunks {
		result += fmt.Sprintf("  Chunk %d: %s", i+1, chunk.String()) // 调用每个 Chunk 的 String 方法
	}
	return result
}
