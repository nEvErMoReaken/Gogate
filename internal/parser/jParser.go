package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type jParser struct {
	ctx           context.Context
	jParserConfig jParserConfig
}

type jParserConfig struct {
	Method string `mapstructure:"method"`
}

func init() {
	Register("json", NewJsonParser)
}

// NewJsonParser 创建一个 JSON 解析器
func NewJsonParser(ctx context.Context) (Template, error) {
	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var jC jParserConfig
	err := mapstructure.Decode(v.Parser.Para, &jC)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	return &jParser{
		ctx:           ctx,
		jParserConfig: jC, // 确保赋值给 jParser 的 jParserConfig 字段
	}, nil
}

func (j *jParser) GetType() string {
	return "message"
}

// Start 启动 JSON 解析器
func (j *jParser) Start(rawChan chan []byte, sink chan *pkg.Frame2Point) {
	logger := pkg.LoggerFromContext(j.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("=== jParser 开始处理数据 ===")

	// 1. 获取数据源
	count := 0
	for {
		select {
		case <-j.ctx.Done():
			return
		case data := <-rawChan:
			// 创建读取计时器
			readTimer := metrics.NewTimer("jParser_read")
			readTimer.Stop()

			// 记录接收到的消息
			metrics.IncMsgReceived("jParser")

			// 2. 解析 JSON 数据
			j.ctx = context.WithValue(j.ctx, "ts", time.Now())

			// 创建处理计时器
			processTimer := metrics.NewTimer("jParser_process")
			pointList, err := j.process(data)
			processTimer.Stop()

			count += 1
			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("jParser_process")
				logger.Error("解析 JSON 数据失败", zap.Error(err))
				continue
			}
			// 3. 聚合数据点
			aggregatedPoints, err := pkg.AggregatePoints(pointList)
			if err != nil {
				metrics.IncErrorCount()
				metrics.IncMsgErrors("jParser_aggregate_points")
				logger.Error("聚合数据点失败", zap.Error(err))
				continue
			}
			frameId := fmt.Sprintf("%06X", count)
			// 4. 将解析后的数据发送到策略
			sink <- &pkg.Frame2Point{
				FrameId: frameId,
				Point:   aggregatedPoints,
			}
			// 5. 记录成功处理的消息
			metrics.IncMsgProcessed("jParser")

			// 6. 打印原始数据
			logger.Info("接收到数据", zap.Any("data", data), zap.String("frameId", frameId))
		}
	}
}

func (j *jParser) process(js []byte) (point []*pkg.Point, err error) {
	logger := pkg.LoggerFromContext(j.ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	// 1. 拿到解析函数
	convertFunc, exist := JsonScriptFuncCache[j.jParserConfig.Method]
	if !exist {
		metrics.IncErrorCount()
		metrics.IncMsgErrors("jParser_function_not_found")
		return nil, fmt.Errorf("未找到解析函数: %s", j.jParserConfig.Method)
	}

	// 2. 将 JSON 字符串解析为 map
	var result map[string]interface{}

	// 创建JSON解析计时器
	jsonUnmarshalTimer := metrics.NewTimer("jParser_json_unmarshal")
	err = json.Unmarshal([]byte(js), &result)
	jsonUnmarshalTimer.Stop()

	if err != nil {
		metrics.IncErrorCount()
		metrics.IncMsgErrors("jParser_json_unmarshal")
		return nil, fmt.Errorf("unmarshal JSON 失败: %v", err)
	}

	// 3. 调用解析函数, 组装resPoint
	var resultPoint []*pkg.Point

	// 创建处理函数计时器
	funcTimer := metrics.NewTimer("jParser_convert_func")
	listPoint, err := convertFunc(result)
	funcTimer.Stop()

	if err != nil {
		metrics.IncErrorCount()
		metrics.IncMsgErrors("jParser_convert_func")
		return nil, fmt.Errorf("解析 JSON 失败: %v, 请检查脚本是否正确", err)
	}

	// 记录转换数量
	pointCount := 0
	for _, p := range listPoint {
		resultPoint = append(resultPoint, &pkg.Point{
			Device: p["device"].(string),
			Field:  p["fields"].(map[string]interface{}),
			Ts:     j.ctx.Value("ts").(time.Time),
		})
		pointCount++
	}

	logger.Debug("JSON解析完成",
		zap.Int("pointCount", pointCount),
		zap.String("method", j.jParserConfig.Method))

	return resultPoint, nil
}
