package json

import (
	"encoding/json"
	"gateway/internal/pkg"
	"gateway/logger"
	"gateway/model"
	"gateway/util"
)

func ConversionToSnapshot(js string, config *JsonParseConfig, collection *model.SnapshotCollection, comm *pkg.Config) {
	// 1. 拿到解析函数
	convertFunc := util.JsonScriptFuncCache[config.rules.Method]
	// 2. 将 JSON 字符串解析为 map
	var result map[string]interface{}

	err := json.Unmarshal([]byte(js), &result)
	if err != nil {
		logger.Log.Errorf("Unmarshal JSON 失败: %v", err)
	}
	// 3. 调用解析函数
	devName, devType, fields, err1 := convertFunc(result)
	if err1 != nil {
		logger.Log.Fatalf("解析 JSON 失败: %v, 请检查脚本是否正确", err1)
	}
	// 3. 更新 DeviceSnapshot
	snapshot := collection.GetDeviceSnapshot(devName, devType)
	for key, value := range fields {
		snapshot.SetField(key, value, comm)
	}
}
