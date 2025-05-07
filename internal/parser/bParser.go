package parser

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

const (
	maxNodes = 50
)

type byteParserConfig struct {
	ProtoFile string                 `mapstructure:"protoFile"`
	GlobalMap map[string]interface{} `mapstructure:"globalMap"`
}

/* ---------- 状态定义 ---------- */

// ByteState 是离散解析的上下文
type ByteState struct {
	Data     []byte
	Cursor   int
	Env      *BEnv
	LabelMap map[string]int
	Nodes    []BProcessor
}

func NewByteState(env *BEnv, labelMap map[string]int, nodes []BProcessor) *ByteState {

	return &ByteState{
		Cursor:   0,
		Env:      env,
		LabelMap: labelMap,
		Nodes:    nodes,
	}
}

func (s *ByteState) Reset() {
	s.Cursor = 0
	s.Env.Reset()
}

// StreamState 用于管理状态，在运行期间可能会被反复创建
type StreamState struct {
	ring     *pkg.RingBuffer
	Env      *BEnv
	LabelMap map[string]int
	Nodes    []BProcessor
}

func NewStreamState(ring *pkg.RingBuffer, labelMap map[string]int, nodes []BProcessor) *StreamState {
	return &StreamState{
		ring:     ring,
		LabelMap: labelMap,
		Nodes:    nodes,
	}
}

func (s *StreamState) Reset() {
	s.Env.Reset()
}

/* ---------- ByteParser 定义 ---------- */

// ByteParser 用于解析二进制数据流
type ByteParser struct {
	// Section 链表的头节点
	Nodes    []BProcessor
	LabelMap map[string]int
	ctx      context.Context
	Env      *BEnv
}

func NewByteParser(ctx context.Context) (*ByteParser, error) {

	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var c byteParserConfig

	// --- Start: Validate input Para map BEFORE decoding ---
	if v.Parser.Para == nil {
		msg := "配置文件解析失败: parser.config 为空"
		pkg.LoggerFromContext(ctx).Error(msg)
		return nil, errors.New(msg)
	}
	protoFileValue, protoFileExists := v.Parser.Para["protoFile"]
	if !protoFileExists {
		msg := "配置文件解析失败: parser.config 缺少 'protoFile' 配置项"
		pkg.LoggerFromContext(ctx).Error(msg, zap.Any("para", v.Parser.Para))
		return nil, fmt.Errorf("%s", msg)
	}
	_, ok := protoFileValue.(string)
	if !ok {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败: protoFile 类型错误", zap.Any("para", v.Parser.Para), zap.String("actualType", fmt.Sprintf("%T", protoFileValue)))
		return nil, fmt.Errorf("配置文件解析失败: parser.config 的 'protoFile' 必须是字符串, 实际类型: %T", protoFileValue)
	}
	// --- End: Input Validation ---

	// Now decode, knowing protoFile exists and is likely a string
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &c,
		TagName:          "mapstructure",
		WeaklyTypedInput: false,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("创建 mapstructure 解码器失败", zap.Error(err))
		return nil, fmt.Errorf("内部错误: 创建解码器失败: %w", err)
	}
	err = decoder.Decode(v.Parser.Para)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败 (mapstructure)", zap.Error(err), zap.Any("para", v.Parser.Para))
		return nil, fmt.Errorf("配置文件解析失败: %w", err)
	}

	// Check if the decoded ProtoFile is empty AFTER successful decode
	if c.ProtoFile == "" {
		msg := "配置文件解析失败: parser.config 的 'protoFile' 值不能为空字符串"
		pkg.LoggerFromContext(ctx).Error(msg, zap.Any("para", v.Parser.Para))
		return nil, fmt.Errorf("%s", msg)
	}

	// 2. 初始化协议配置文件
	sectionConfig, exist := v.Others[c.ProtoFile]
	if !exist {
		pkg.LoggerFromContext(ctx).Error("未找到协议文件", zap.String("ProtoFile", c.ProtoFile))
		return nil, fmt.Errorf("未找到协议文件:%s", c.ProtoFile)
	}
	pkg.LoggerFromContext(ctx).Debug("协议文件原始数据", zap.Any("data", sectionConfig))

	rawSections, ok := sectionConfig.([]map[string]any)
	if !ok {
		pkg.LoggerFromContext(ctx).Error("协议文件根格式错误，期望 []interface{}", zap.String("ProtoFile", c.ProtoFile), zap.Any("actualData", sectionConfig))
		return nil, fmt.Errorf("协议文件格式错误: %s 不是一个列表/数组", c.ProtoFile)
	}
	pkg.LoggerFromContext(ctx).Debug("Section文件列表", zap.Any("list", rawSections))

	// 3. 初始化 Env
	env := BEnv{
		GlobalMap: c.GlobalMap,
		Vars:      make(map[string]interface{}), // 初始化 Vars map
		Fields:    make(map[string]interface{}), // 初始化 Fields map
	}

	// 4. 初始化 Section 链表
	nodes, labelMap, err := BuildSequence(rawSections)
	if err != nil {
		return nil, fmt.Errorf("初始化ByteParser失败: %w", err)
	}

	byteParser := &ByteParser{
		ctx:      ctx,
		Env:      &env,
		Nodes:    nodes,
		LabelMap: labelMap,
	}
	return byteParser, nil
}

// StartWithChan 方法用于启动一个基于Channel的ByteParser
func (r *ByteParser) StartWithChan(dataChan chan []byte, sink pkg.Parser2DispatcherChan) error {
	logger := pkg.LoggerFromContext(r.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("===ByteParser StartWithChan goroutine started===", zap.Int("maxNodesPerFrame", maxNodes))
	byteState := NewByteState(r.Env, r.LabelMap, r.Nodes)
	for {
		select {
		case <-r.ctx.Done():
			logger.Info("StartWithChan goroutine exiting due to context done") // 添加退出日志
			return nil
		case data := <-dataChan:
			logger.Info("StartWithChan received data", zap.Int("len", len(data)), zap.String("hex", hex.EncodeToString(data))) // 添加接收日志
			// 1. 重置状态
			byteState.Reset()
			out := make([]*pkg.Point, 0, 10) // 修正：初始 len 为 0，cap 为 10
			byteState.Data = data
			processedNodeCount := 0 // 重置计数器

			current := r.Nodes[0]
			nodeIndex := 0 // 添加索引用于日志
			for current != nil {
				// --- 死循环检测 ---
				if processedNodeCount >= maxNodes {
					logger.Error("死循环防护触发：处理节点数超过最大限制",
						zap.Int("maxNodes", maxNodes),
						zap.Int("processedCount", processedNodeCount),
						zap.Int("lastNodeIndex", nodeIndex),
						zap.Stringer("lastNode", current))
					// 可以选择是仅中断当前帧处理，还是返回错误使整个 parser 停止
					// 这里选择仅中断当前帧，记录错误，然后继续等待下一帧
					// 如果需要停止整个 parser，则 return ErrMaxNodesExceeded
					return errors.New("死循环防护触发：处理节点数超过最大限制") // 跳出内部 for 循环，处理下一帧
				}
				processedNodeCount++ // 增加计数
				// --- 检测结束 ---

				logger.Info("StartWithChan processing node", zap.Int("index", nodeIndex), zap.Stringer("node", current)) // 添加节点处理日志
				var tmp BProcessor                                                                                       // 声明 tmp 变量
				var err error                                                                                            // 声明 err 变量
				// 调用 ProcessWithBytes 并接收返回的 out, tmp, err
				out, tmp, err = current.ProcessWithBytes(r.ctx, byteState, out)
				if err != nil {
					// *** 记录详细错误信息 ***
					logger.Error("ProcessWithBytes returned error, exiting StartWithChan",
						zap.Int("nodeIndex", nodeIndex),
						zap.Stringer("node", current),
						zap.Error(err))
					return err // 返回错误，导致 goroutine 退出
				}
				logger.Info("StartWithChan node processed successfully", zap.Int("index", nodeIndex)) // 添加成功日志
				current = tmp
				nodeIndex++
			}

			// --- 如果是因为错误或超出限制而 break，则不发送数据 ---
			if current != nil { // 如果循环不是正常结束(current != nil)，说明是 break 跳出的
				logger.Warn("当前数据帧处理被中断(错误或超出节点限制)，不发送数据", zap.String("frameHex", hex.EncodeToString(data)))
				pkg.BytesPoolInstance.Put(data) // 仍然需要释放缓冲区
				byteState.Reset()               // 重置状态以备下一帧
				continue                        // 继续外层 for 循环等待下一数据
			}
			// --- 循环正常结束 ---
			logger.Info("StartWithChan finished processing loop, preparing to send to sink", zap.Int("points_count", len(out))) // 添加发送前日志
			frameId := fmt.Sprintf("%06X", metrics.IncMsgProcessed("byteParser"))

			sink <- &pkg.PointPackage{
				FrameId: frameId,
				Points:  out, // 发送最终的 out slice
				Ts:      time.Now(),
			}
			logger.Info("StartWithChan sent result to sink", zap.String("frameId", frameId)) // 添加发送后日志

			hexRaw := hex.EncodeToString(data)
			logger.Info("Frame",
				zap.String("count", frameId), // 使用 6 位 16 进制数格式化 count
				zap.String("frame", hexRaw))  // frame 转为16进制字符串
			pkg.BytesPoolInstance.Put(data) // 释放缓冲区

		}
	}
}

// StartWithRingBuffer 方法用于启动一个基于RingBuffer的ByteParser
func (r *ByteParser) StartWithRingBuffer(ring *pkg.RingBuffer, sink pkg.Parser2DispatcherChan) error {
	logger := pkg.LoggerFromContext(r.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("===ByteParser 开始处理数据===")
	state := NewStreamState(ring, r.LabelMap, r.Nodes)
	for {
		select {
		case <-r.ctx.Done():
			return nil
		default:
			out := make([]*pkg.Point, 10)
			metrics.IncMsgReceived("byteParser")

			// 1. 记录帧起始位置
			start := ring.ReadPos()

			// 2. 处理帧
			current := r.Nodes[0]
			processedNodeCount := 0 // 重置计数器
			for current != nil {
				// --- 死循环检测 ---
				if processedNodeCount >= maxNodes {
					logger.Error("死循环防护触发：处理节点数超过最大限制",
						zap.Int("maxNodes", maxNodes),
						zap.Int("processedCount", processedNodeCount),
						zap.Stringer("lastNode", current))
					// 如果需要停止整个 parser，则 return ErrMaxNodesExceeded
					return errors.New("死循环防护触发：处理节点数超过最大限制") // 跳出内部 for 循环，处理下一帧
				}
				tmp, err := current.ProcessWithRing(r.ctx, state, out)
				if err != nil {
					return err
				}
				current = tmp
				processedNodeCount++ // 增加计数
			}

			end := ring.ReadPos() // 记录帧结束位置
			// ** 此处是完整的一帧的结束 **

			// 3. 自增计数，获取计数，生成帧ID
			frameId := fmt.Sprintf("%06X", metrics.IncMsgProcessed("byteParser"))
			// 4. 发送聚合后的数据点
			sink <- &pkg.PointPackage{
				FrameId: frameId,
				Points:  out,
				Ts:      time.Now(),
			}

			raw := ring.Snapshot(start, end)
			hexRaw := hex.EncodeToString(raw)
			// 5. 打印原始报文
			logger.Info("Frame",
				zap.String("count", frameId), // 使用 6 位 16 进制数格式化 count
				zap.String("frame", hexRaw))  // frame 转为16进制字符串
			// 6. 重置状态
			state.Reset()
		}
	}
}
