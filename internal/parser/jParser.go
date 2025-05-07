package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// jParserConfig 定义 JParser 的配置结构
type jParserConfig struct {
	// Fields 定义了输出字段名到表达式的映射。
	// 表达式应该访问解析后的 JSON 数据，通常命名为 'Data'。
	// 例如: "temperature": "Data['sensorA']['temp']"
	Fields map[string]string `mapstructure:"fields"`
	// Device 指定生成的 Point 的设备名称。
	Device string `mapstructure:"device"`
	// GlobalMap (可选) 存储全局变量，可在表达式中访问。
	GlobalMap map[string]interface{} `mapstructure:"globalMap"`
}

// JEnv 是 JSON 处理的表达式执行环境。
type JEnv struct {
	// Data 存储解组后的 JSON 数据 (map[string]interface{})。
	Data map[string]interface{}
	// Fields 存储由 F() 函数设置的输出字段。
	Fields map[string]interface{}
	// GlobalMap 存储全局配置变量。
	GlobalMap map[string]interface{}
}

// Reset 清空 JEnv 的 Data 和 Fields，以便复用。
func (e *JEnv) Reset() {
	// 清空 map 最高效的方式是重新创建
	e.Data = make(map[string]interface{})
	e.Fields = make(map[string]interface{})
	// GlobalMap 不需要重置，它是共享的
}

// F 在 Fields 映射中设置一个键值对，并返回 nil。
// 这是 expr 表达式中用于设置输出字段的函数。
func (e *JEnv) F(key string, val interface{}) any {
	e.Fields[key] = val
	return nil
}

// JEnvPool 是 JEnv 对象的 sync.Pool，用于复用。
type JEnvPool struct {
	sync.Pool
}

// NewJEnvPool 创建一个新的 JEnvPool。
func NewJEnvPool(globalMap map[string]interface{}) *JEnvPool {
	return &JEnvPool{
		Pool: sync.Pool{
			New: func() any {
				// 初始化时创建空的 map
				return &JEnv{
					Data:      make(map[string]interface{}),
					Fields:    make(map[string]interface{}),
					GlobalMap: globalMap, // 共享全局 map
				}
			},
		},
	}
}

// Get 从池中获取一个 JEnv 实例。
func (p *JEnvPool) Get() *JEnv {
	return p.Pool.Get().(*JEnv)
}

// Put 将一个 JEnv 实例放回池中。
func (p *JEnvPool) Put(e *JEnv) {
	e.Reset() // 重置状态
	p.Pool.Put(e)
}

// BuildJExprOptions 返回用于编译 JSON 处理表达式的 expr 选项。
// 环境设置为 *JEnv，允许访问 Data 和调用 F()。
func BuildJExprOptions() []expr.Option {
	options := []expr.Option{
		expr.Env(&JEnv{}), // 环境是 JEnv 指针
		// 可以选择性地添加 helpers，如果需要的话
		// expr.Function(...)
	}
	// 可以添加全局辅助函数，如果它们也适用于 JSON 处理
	// options = append(options, helpers...)
	return options
}

// JParser 用于解析 JSON 数据并根据表达式提取字段。
type JParser struct {
	ctx           context.Context
	jParserConfig jParserConfig
	jEnvPool      *JEnvPool   // 重命名以避免与类型冲突
	program       *vm.Program // 单一编译后的程序
}

// NewJsonParser 创建一个新的 JParser 实例。
func NewJsonParser(ctx context.Context) (*JParser, error) {
	logger := pkg.LoggerFromContext(ctx)
	logger.Info("Initializing JParser...")

	// 1. 加载和解码配置
	v := pkg.ConfigFromContext(ctx)
	var jC jParserConfig
	if err := mapstructure.Decode(v.Parser.Para, &jC); err != nil {
		logger.Error("Failed to decode jParser config", zap.Any("config", v.Parser.Para), zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %w", err)
	}
	logger.Debug("jParser config loaded", zap.Any("config", jC))

	// 2. 验证配置
	if jC.Device == "" {
		err := errors.New("jParser config requires a non-empty 'device' field")
		logger.Error(err.Error())
		return nil, err
	}
	if len(jC.Fields) == 0 {
		err := errors.New("jParser config requires a non-empty 'fields' map")
		logger.Error(err.Error())
		return nil, err
	}
	logger.Info("jParser config validated", zap.String("device", jC.Device), zap.Int("field_count", len(jC.Fields)))

	// 3. 构建组合的表达式源码
	var calls []string
	for fieldName, expression := range jC.Fields {
		calls = append(calls, fmt.Sprintf("F(%q, %s)", fieldName, expression))
	}
	// 添加最终的 nil 返回值，确保表达式总有返回值
	source := strings.Join(calls, "; ") + "; nil"
	logger.Debug("Compiled jParser source", zap.String("source", source))

	// 4. 编译表达式
	program, err := expr.Compile(source, BuildJExprOptions()...)
	if err != nil {
		logger.Error("Failed to compile jParser expression", zap.String("source", source), zap.Error(err))
		return nil, fmt.Errorf("编译表达式失败: %w", err)
	}
	logger.Info("jParser expression compiled successfully")

	// 5. 创建 JParser 实例
	parser := &JParser{
		ctx:           ctx,
		jParserConfig: jC,
		jEnvPool:      NewJEnvPool(jC.GlobalMap), // 使用新的 Pool 名称
		program:       program,
	}
	logger.Info("JParser initialized successfully", zap.String("device", parser.jParserConfig.Device))
	return parser, nil
}

// Start 启动 JSON 解析器，监听输入通道并将结果发送到输出通道。
func (j *JParser) Start(rawChan chan []byte, sink chan *pkg.PointPackage) {
	logger := pkg.LoggerFromContext(j.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("=== jParser started processing data ===")

	for {
		select {
		case <-j.ctx.Done():
			logger.Info("jParser stopping due to context done.")
			return
		case data := <-rawChan:
			ts := time.Now() // 记录处理开始时间
			metrics.IncMsgReceived("jParser")
			logger.Debug("Received JSON data", zap.Int("bytes", len(data)))

			// 处理 JSON 数据
			processTimer := metrics.NewTimer("jParser_process")
			pointList, err := j.process(data)
			processTimer.Stop()

			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("jParser_process") // 使用更具体的指标名称
				logger.Error("Failed to process JSON data", zap.Error(err), zap.ByteString("raw_data", data))
				// 考虑是否需要释放 data (如果来自对象池)
				continue // 继续处理下一条消息
			}

			// 如果没有生成任何点 (例如，表达式结果为空或过滤掉了)，则不发送
			if len(pointList) == 0 {
				logger.Debug("No points generated for this JSON frame", zap.ByteString("raw_data", data))
				// 考虑是否需要释放 data
				continue
			}

			// 使用 metrics 或 sequence 生成 FrameId
			frameId := fmt.Sprintf("%06X", metrics.IncMsgProcessed("jParser"))

			// 发送结果
			sink <- &pkg.PointPackage{
				FrameId: frameId,
				Points:  pointList,
				Ts:      ts, // 使用处理开始时的时间戳
			}

			// 打印成功处理的信息和原始数据 (截断长数据)
			logData := data
			if len(logData) > 256 { // Limit logged data size
				logData = logData[:256]
			}
			logger.Info("Successfully processed JSON frame",
				zap.String("frameId", frameId),
				zap.Int("points_generated", len(pointList)),
				zap.ByteString("data_prefix", logData))
			// 考虑是否需要释放 data
		}
	}
}

// process 处理单条 JSON 数据。
func (j *JParser) process(js []byte) ([]*pkg.Point, error) { // 返回切片和错误
	logger := pkg.LoggerFromContext(j.ctx)
	// metrics := pkg.GetPerformanceMetrics() // 在 Start 中处理

	// 1. 获取/重置环境
	env := j.jEnvPool.Get()
	defer j.jEnvPool.Put(env) // 确保环境被放回池中

	// 2. 解组 JSON 到 env.Data
	// 注意: json.Unmarshal 会覆盖 env.Data 的内容
	if err := json.Unmarshal(js, &env.Data); err != nil {
		// metrics.IncErrorCount() // 在 Start 中处理
		// metrics.IncMsgErrors("jParser_json_unmarshal") // 在 Start 中处理
		logger.Error("JSON unmarshal failed", zap.Error(err))
		// 返回原始错误，不包装，以便上层判断类型
		return nil, err // 直接返回 unmarshal 错误
	}
	logger.Debug("JSON unmarshalled successfully", zap.Any("data_map", env.Data))

	// 3. 运行编译好的程序，填充 env.Fields
	_, err := expr.Run(j.program, env)
	if err != nil {
		// metrics.IncErrorCount() // 在 Start 中处理
		// metrics.IncMsgErrors("jParser_expr_run") // 在 Start 中处理
		// 可能需要记录 env.Data 以便调试
		logger.Error("Failed running compiled expression", zap.Error(err), zap.Any("json_data", env.Data))
		return nil, fmt.Errorf("运行表达式失败: %w", err) // 包装错误
	}
	logger.Debug("Expression run completed", zap.Any("generated_fields", env.Fields))

	// 4. 检查是否生成了字段
	if len(env.Fields) == 0 {
		logger.Warn("Expression run generated no fields", zap.ByteString("raw_json", js), zap.Any("json_data", env.Data))
		// 这种情况不一定是错误，可能只是当前 JSON 没有匹配的字段
		return []*pkg.Point{}, nil // 返回空切片，表示没有点生成
	}

	// 5. 创建单个 Point
	point := &pkg.Point{
		Device: j.jParserConfig.Device, // 使用配置中的设备名
		Field:  maps.Clone(env.Fields), // 必须克隆，因为 env 会被回收
	}

	pointList := []*pkg.Point{point} // 结果是一个包含单个点的切片

	logger.Debug("Point generated successfully", zap.String("device", point.Device), zap.Any("fields", point.Field))
	// metrics.IncMsgProcessed("jParser") // 在 Start 中处理

	return pointList, nil
}
