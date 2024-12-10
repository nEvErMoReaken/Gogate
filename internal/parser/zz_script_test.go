package parser

import (
	"context"
	"gateway/internal/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/traefik/yaegi/interp"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAllScripts_Success(t *testing.T) {
	ctx := pkg.WithLogger(context.Background(), logger)
	// 创建临时目录和测试脚本
	tempDir := t.TempDir()
	validScriptPath := filepath.Join(tempDir, "valid_script.go")
	validScriptContent := `
package script
import (
	"fmt"
	"strconv"
)

func ConvertOldGatewayTelemetry(jsonMap map[string]interface{}) ([]map[string]interface{}, error) {
	devName := jsonMap["deviceName"].(string)
	devType := jsonMap["deviceType"].(string)
	fields := jsonMap["fields"].(map[string]interface{})
	return []map[string]interface{}{{"device": devName + devType, "fields": fields}}, nil
}


// DecodeByteToLittleEndianBits 将字节数组解析为小端序位数组
func DecodeByteToLittleEndianBits(data []byte) ([]interface{}, error) {
	// 错误检查，确保输入字节数组非空
	if len(data) == 0 {
		return nil, fmt.Errorf("[Decode8BToLittleEndianBits] 输入字节数组为空")
	}

	// 结果数组，长度是输入字节数组的8倍（每个字节包含8个位）
	result := make([]interface{}, 0, len(data)*8)

	// 遍历每个字节，从低位到高位（小端序），依次提取每个位
	for _, b := range data {
		for i := 0; i < 8; i++ { // 小端序：从低位到高位依次提取位
			// 提取第 i 位
			bit := (b >> i) & 0x01
			// 将提取到的位放入结果数组
			result = append(result, int(bit))
		}
	}
	return result, nil
}
`

	err := os.WriteFile(validScriptPath, []byte(validScriptContent), 0644)
	if err != nil {
		t.Fatalf("创建测试脚本失败: %v", err)
	}

	// 加载脚本并缓存
	err = LoadAllScripts(ctx, tempDir)
	if err != nil {
		t.Fatalf("加载脚本失败: %v", err)
	}

	// 验证缓存1
	if _, exists := ByteScriptFuncCache["DecodeByteToLittleEndianBits"]; !exists {
		t.Error("期望缓存 DecodeByteToLittleEndianBits 失败")
	}

	// 验证缓存2
	if _, exists := JsonScriptFuncCache["ConvertOldGatewayTelemetry"]; !exists {
		t.Error("期望缓存 ConvertOldGatewayTelemetry 失败")
	}
}
func TestGetScriptFunc_DefaultFunc(t *testing.T) {
	// 清空 ByteScriptFuncCache，确保测试环境干净
	ByteScriptFuncCache = make(map[string]ByteScriptFunc)

	// 定义一个不存在的函数名
	funcName := "NonExistentFunc"

	// 调用 GetScriptFunc
	scriptFunc, exists := GetScriptFunc(funcName)

	// 验证：应该返回 false 表示函数不存在
	assert.False(t, exists, "函数 %s 不应该存在于缓存中", funcName)

	// 验证：scriptFunc 应该是 defaultFunc
	result, err := scriptFunc([]byte{}) // 调用默认函数

	// 验证默认函数的返回值
	assert.Nil(t, result, "默认函数应返回 nil")
	assert.Nil(t, err, "默认函数应返回 nil error")
}
func TestLoadAllScripts_InvalidScript(t *testing.T) {
	ctx := pkg.WithLogger(context.Background(), logger)
	// 创建临时目录和无效的测试脚本
	tempDir := t.TempDir()
	invalidScriptPath := filepath.Join(tempDir, "invalid_script.go")
	invalidScriptContent := `
package script
func TestInvalidFunc {
    // missing function body
}
`
	err := os.WriteFile(invalidScriptPath, []byte(invalidScriptContent), 0644)
	if err != nil {
		t.Fatalf("创建无效脚本失败: %v", err)
	}

	// 尝试加载无效脚本，应该返回错误
	err = LoadAllScripts(ctx, tempDir)
	if err == nil {
		t.Fatal("期望加载无效脚本失败，但未得到错误")
	}
	expectedErr := "解释脚本失败"

	if err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("期望错误信息为 '%s'，但得到的是 '%s'", expectedErr, err.Error())
	}
}

func TestGetScriptFunc(t *testing.T) {
	// 模拟一个缓存的脚本函数
	ByteScriptFuncCache["TestFunc"] = func(input []byte) ([]interface{}, error) {
		return []interface{}{"test"}, nil
	}

	// 测试获取存在的函数
	fn, exists := GetScriptFunc("TestFunc")
	if !exists {
		t.Error("期望找到缓存的 TestFunc")
	}

	result, err := fn([]byte("input"))
	if err != nil {
		t.Errorf("期望无错误，但得到: %v", err)
	}
	if len(result) != 1 || result[0] != "test" {
		t.Errorf("期望返回 'test'，但得到: %v", result)
	}

	// 测试获取不存在的函数
	_, exists = GetScriptFunc("NonExistentFunc")
	if exists {
		t.Error("期望找不到 NonExistentFunc，但找到")
	}
}

func TestLoadAllScripts_ReadFileError(t *testing.T) {
	ctx := context.Background()
	scriptDir := "./nonexistent_directory" // 不存在的目录

	err := LoadAllScripts(ctx, scriptDir)
	if err == nil || err.Error() != "不存在的目录 ./nonexistent_directory: CreateFile ./nonexistent_directory: The system cannot find the file specified." {
		t.Errorf("expected file read error, got %v", err)
	}
}

func TestLoadAllScripts_InterInitError(t *testing.T) {

	// 制造一个故意导致初始化失败的情况， 错误的初始化环境
	i := interp.New(interp.Options{})
	err := i.Use(map[string]map[string]reflect.Value{"fmt": {"Println": reflect.ValueOf("invalid")}})
	if err == nil {
		t.Fatalf("expected interpreter initialization error, got nil")
	}
}

func TestExtractAndCacheFunctions_ParseError(t *testing.T) {
	ctx := context.Background()

	// 创建一个包含语法错误的临时 Go 文件
	scriptContent := []byte(`package main; func main() { invalid code }`)
	path := "./invalid.go"

	err := extractAndCacheFunctions(ctx, interp.New(interp.Options{}), path, scriptContent)
	if err == nil || err.Error() != "无法解析脚本文件 : ./invalid.go:1:37: expected ';', found code (and 2 more errors)" {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestExtractAndCacheFunctions_NotFoundInScript(t *testing.T) {
	ctx := context.Background()

	scriptContent := []byte(`package main; func AnotherFunc() {}`) // 不包含目标函数
	path := "./test.go"

	// 初始化解释器并解释内容
	i := interp.New(interp.Options{})
	_, err := i.Eval(string(scriptContent))
	if err != nil {
		t.Fatalf("failed to evaluate script: %v", err)
	}

	// 调用 extractAndCacheFunctions，函数不存在应该触发 "未找到函数" 错误
	err = extractAndCacheFunctions(ctx, i, path, scriptContent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
