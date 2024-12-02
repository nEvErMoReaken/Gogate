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
	SnapshotCollection SnapshotCollection // 快照集合
	ctx                context.Context
	jParserConfig      jParserConfig
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
		ctx:                ctx,
		SnapshotCollection: make(SnapshotCollection),
		jParserConfig:      jC, // 确保赋值给 jParser 的 jParserConfig 字段
	}, nil
}

func (j *jParser) GetType() string {
	return "message"
}

// Start 启动 JSON 解析器
func (j *jParser) Start(source *pkg.DataSource, sinkMap *pkg.PointDataSource) {
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
				return
			}
			// 如果读取发生错误，输出错误日志并退出
			pkg.LoggerFromContext(j.ctx).Error("数据源读取失败", zap.Error(err))
			return
		}

		// 3. 解析 JSON 数据
		j.ConversionToSnapshot(string(data))
		j.ctx = context.WithValue(j.ctx, "ts", time.Now())

		// 4. 将解析后的数据发送到策略
		j.SnapshotCollection.LaunchALL(j.ctx, sinkMap)

		// 5. 打印原始数据
		count += 1
		pkg.LoggerFromContext(j.ctx).Info("接收到数据", zap.Any("data", data), zap.Int("count", count))
	}
}

func (j *jParser) ConversionToSnapshot(js string) {
	// 1. 拿到解析函数
	convertFunc, exist := JsonScriptFuncCache[j.jParserConfig.Method]
	if !exist {
		pkg.ErrChanFromContext(j.ctx) <- fmt.Errorf("未找到解析函数: %s", j.jParserConfig.Method)
	}
	// 2. 将 JSON 字符串解析为 map
	var result map[string]interface{}
	var err error
	err = json.Unmarshal([]byte(js), &result)
	if err != nil {
		pkg.ErrChanFromContext(j.ctx) <- fmt.Errorf("unmarshal JSON 失败: %v", err)
	}
	// 3. 调用解析函数
	devName, devType, fields, err := convertFunc(result)
	if err != nil {
		pkg.ErrChanFromContext(j.ctx) <- fmt.Errorf("解析 JSON 失败: %v, 请检查脚本是否正确", err)
	}
	// 3. 更新 DeviceSnapshot
	snapshot, err := j.SnapshotCollection.GetDeviceSnapshot(devName, devType)
	if err != nil {
		pkg.ErrChanFromContext(j.ctx) <- fmt.Errorf("获取快照失败: %v", err)
	}
	for key, value := range fields {
		err = snapshot.SetField(j.ctx, key, value)
		if err != nil {
			pkg.ErrChanFromContext(j.ctx) <- fmt.Errorf("设置字段失败: %v", err)
		}
	}
}
