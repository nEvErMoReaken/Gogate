package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io"
	"time"
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
func (j *jParser) Start(source *pkg.DataSource, sink *pkg.AggregatorDataSource) {
	dataSource := (*source).(*pkg.MessageDataSource)

	// 1. 获取数据源
	count := 0
	for {
		data, err := dataSource.ReadOne()
		// 2. 处理从数据源读取的数据
		if err != nil {
			if err == io.EOF {
				// 如果读取到 EOF，认为是正常结束，退出循环
				//pkg.LoggerFromContext(j.ctx).Info("数据源读取完成，EOF")
				continue
			}
			// 如果读取发生错误，输出错误日志并退出
			pkg.LoggerFromContext(j.ctx).Error("数据源读取失败", zap.Error(err))
			continue
		}
		var pointList []*pkg.Point
		// 3. 解析 JSON 数据
		j.ctx = context.WithValue(j.ctx, "ts", time.Now())
		pointList, err = j.process(string(data))
		if err != nil {
			pkg.LoggerFromContext(j.ctx).Error("解析 JSON 数据失败", zap.Error(err))
			continue
		}

		// 4. 将解析后的数据发送到策略
		for _, point := range pointList {
			sink.PointChan <- *point
		}
		// 5. 通知策略数据源已经结束
		sink.EndChan <- struct{}{}
		// 6. 打印原始数据
		count += 1
		pkg.LoggerFromContext(j.ctx).Info("接收到数据", zap.Any("data", data), zap.Int("count", count))
	}
}

func (j *jParser) process(js string) (point []*pkg.Point, err error) {
	// 1. 拿到解析函数
	convertFunc, exist := JsonScriptFuncCache[j.jParserConfig.Method]
	if !exist {
		return nil, fmt.Errorf("未找到解析函数: %s", j.jParserConfig.Method)
	}
	// 2. 将 JSON 字符串解析为 map
	var result map[string]interface{}

	err = json.Unmarshal([]byte(js), &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshal JSON 失败: %v", err)
	}
	// 3. 调用解析函数, 组装resPoint
	var resultPoint []*pkg.Point
	listPoint, err := convertFunc(result)
	if err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %v, 请检查脚本是否正确", err)
	}
	for _, p := range listPoint {
		resultPoint = append(resultPoint, &pkg.Point{
			Device: p["device"].(string),
			Field:  p["fields"].(map[string]interface{}),
			Ts:     j.ctx.Value("ts").(time.Time),
		})
	}

	return resultPoint, nil
}
