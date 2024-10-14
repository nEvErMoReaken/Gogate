package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"gateway/internal/pkg"
	"gateway/util"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type jParser struct {
	dataSource         pkg.DataSource
	SnapshotCollection SnapshotCollection // 快照集合
	mapChan            map[string]chan pkg.Point
	ctx                context.Context
	jParserConfig      jParserConfig
}

type jParserConfig struct {
	method string `mapstructure:"method"`
}

func init() {
	Register("json", NewJsonParser)
}

// NewJsonParser 创建一个 JSON 解析器
func NewJsonParser(dataSource pkg.DataSource, mapChan map[string]chan pkg.Point, ctx context.Context) (Parser, error) {
	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var jC jParserConfig
	err := mapstructure.Decode(v.Parser.Para, &jC)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}

	return &jParser{
		dataSource:         dataSource,
		mapChan:            mapChan,
		ctx:                ctx,
		SnapshotCollection: make(SnapshotCollection),
	}, nil
}

// Start 启动 JSON 解析器
func (j *jParser) Start() {
	// 1. 获取数据源
	dataChan := j.dataSource.Source.(chan string)
	count := 0
	for {
		select {
		case <-j.ctx.Done():
			return
		// 2. 从数据源中读取数据
		case data := <-dataChan:
			// 3. 解析 JSON 数据
			j.ConversionToSnapshot(data)
			// 4. 将解析后的数据发送到策略
			j.SnapshotCollection.LaunchALL(j.ctx, j.mapChan)
			// 5. 打印原始数据
			count += 1
			pkg.LoggerFromContext(j.ctx).Info("接收到数据", zap.Any("data", data), zap.Int("count", count))
		}
	}
}

func (j *jParser) ConversionToSnapshot(js string) {
	// 1. 拿到解析函数
	convertFunc := util.JsonScriptFuncCache[j.jParserConfig.method]
	// 2. 将 JSON 字符串解析为 map
	var result map[string]interface{}
	var err error
	err = json.Unmarshal([]byte(js), &result)
	if err != nil {
		util.ErrChanFromContext(j.ctx) <- fmt.Errorf("unmarshal JSON 失败: %v", err)
	}
	// 3. 调用解析函数
	devName, devType, fields, err := convertFunc(result)
	if err != nil {
		util.ErrChanFromContext(j.ctx) <- fmt.Errorf("解析 JSON 失败: %v, 请检查脚本是否正确", err)
	}
	// 3. 更新 DeviceSnapshot
	snapshot, err := j.SnapshotCollection.GetDeviceSnapshot(devName, devType)
	util.ErrChanFromContext(j.ctx) <- fmt.Errorf("获取快照失败: %v", err)
	for key, value := range fields {
		err := snapshot.SetField(j.ctx, key, value)
		if err != nil {
			util.ErrChanFromContext(j.ctx) <- fmt.Errorf("设置字段失败: %v", err)
		}
	}
}
