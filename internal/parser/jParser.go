package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/pkg"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// jParserConfig 定义 JParser 的配置结构
type jParserConfig struct {
	// Points 定义了输出点。
	Points []PointExpression `mapstructure:"points"`
	// GlobalMap (可选) 存储全局变量，可在表达式中访问。
	GlobalMap map[string]interface{} `mapstructure:"globalMap"`
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
	if len(jC.Points) == 0 {
		err := errors.New("jParser config requires a non-empty 'points' list")
		logger.Error(err.Error())
		return nil, err
	}

	logger.Info("jParser config validated", zap.String("Points", fmt.Sprintf("%v", jC.Points)), zap.Int("point_count", len(jC.Points)))

	// 3. 构建组合的表达式源码
	var calls []string
	for i, point := range jC.Points {
		for field, expr := range point.Field {
			calls = append(calls, fmt.Sprintf("F%d(%q, %s)", i+1, field, expr))
		}
		for tag, expr := range point.Tag {
			calls = append(calls, fmt.Sprintf("T%d(%q, %s)", i+1, tag, expr))
		}
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
	logger.Info("JParser initialized successfully", zap.String("points", fmt.Sprintf("%v", parser.jParserConfig.Points)))
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
	logger.Debug("Expression run completed", zap.Any("generated_fields", env.Points))

	// 4. 检查是否生成了字段
	if len(env.Points) == 0 {
		logger.Warn("Expression run generated no fields", zap.ByteString("raw_json", js), zap.Any("json_data", env.Data))
		// 这种情况不一定是错误，可能只是当前 JSON 没有匹配的字段
		return []*pkg.Point{}, nil // 返回空切片，表示没有点生成
	}

	// 5. 克隆 Point
	pointList := []*pkg.Point{}
	for _, point := range env.Points {
		// 只添加有内容的Point（Tag或Field不为空）
		if len(point.Tag) > 0 || len(point.Field) > 0 {
			pointList = append(pointList, &pkg.Point{
				Tag:   point.Tag,
				Field: point.Field,
			})
		}
	}

	// metrics.IncMsgProcessed("jParser") // 在 Start 中处理
	logger.Debug("Point generated successfully", zap.Any("fields", pointList))
	// metrics.IncMsgProcessed("jParser") // 在 Start 中处理

	return pointList, nil
}
