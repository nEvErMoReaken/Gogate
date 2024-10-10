package util

import (
	"fmt"
	"gateway/logger"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

// ByteScriptFunc 定义脚本函数签名
type ByteScriptFunc func([]byte) ([]interface{}, error)

// JsonScriptFunc 定义了一种脚本函数的签名，用于处理解析后的 JSON 数据。
//
// 参数:
//
//	data: 从 JSON 解析后的数据，类型为 map[string]interface{}。
//
// 返回:
//
//	devName: 设备名称，字符串。
//	devType: 设备类型，字符串。
//	fields: 设备的字段数据，类型为 map[string]interface{}。
//	err: 如果出现错误，返回错误信息，否则为 nil。
type JsonScriptFunc func(map[string]interface{}) (string, string, map[string]interface{}, error)

// ByteScriptFuncCache 脚本函数缓存
var ByteScriptFuncCache = make(map[string]ByteScriptFunc)

// JsonScriptFuncCache 脚本函数缓存
var JsonScriptFuncCache = make(map[string]JsonScriptFunc)

// extractAndCacheFunctions 解析Go文件并缓存函数名
func extractAndCacheFunctions(i *interp.Interpreter, path string, scriptContent []byte) error {
	// 使用 go/parser 和 go/ast 解析 Go 源码文件
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, scriptContent, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("无法解析脚本文件 %s: %w", path, err)
	}

	// 遍历 AST，查找函数定义
	for _, decl := range node.Decls {
		// 仅处理函数声明
		if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
			funcName := fn.Name.Name
			// 从解释器中获取该函数
			v, err := i.Eval("script." + funcName)
			if err != nil {
				logger.Log.Errorf("[Warning]: 在已读取脚本中未找到 %s 方法 %v\n", funcName, err)
				continue
			}

			// 尝试将函数解析为 ByteScriptFunc
			if fnFunc, ok := v.Interface().(ByteScriptFunc); ok {
				ByteScriptFuncCache[funcName] = fnFunc // 缓存 ByteScriptFunc 类型函数
				logger.Log.Infof("[Info]: %s 方法已缓存为 ByteScriptFunc", funcName)
				continue
			}

			// 尝试将函数解析为 JsonScriptFunc
			if fnFunc, ok := v.Interface().(JsonScriptFunc); ok {
				JsonScriptFuncCache[funcName] = fnFunc // 缓存 JsonScriptFunc 类型函数
				logger.Log.Infof("[Info]: %s 方法已缓存为 JsonScriptFunc", funcName)
				continue
			}

			// 如果不匹配任何已知函数签名，输出警告
			logger.Log.Errorf("[Warning]: %s 方法的签名与预期的 ByteScriptFunc 或 JsonScriptFunc 类型不匹配\n", funcName)
		}
	}

	return nil
}

// LoadAllScripts 加载/script目录下的所有脚本并缓存
func LoadAllScripts(scriptDir string) error {
	// 初始化yaegi解释器
	i := interp.New(interp.Options{})
	err := i.Use(stdlib.Symbols)
	if err != nil {
		return err
	}

	// 遍历/script目录下的所有Go脚本文件
	err = filepath.Walk(scriptDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 仅处理 .go 文件
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			// 读取脚本文件内容
			scriptContent, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("读取脚本失败 %s: %w", path, err)
			}

			// 解释脚本内容
			_, err = i.Eval(string(scriptContent))
			if err != nil {
				return fmt.Errorf("解释脚本失败 %s: %w", path, err)
			}

			// 解析文件并提取函数名
			err = extractAndCacheFunctions(i, path, scriptContent)
			if err != nil {
				return fmt.Errorf("提取函数失败 %s: %w", path, err)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// GetScriptFunc 获取缓存的脚本函数, 如果不存在则返回一个默认的函数(多用于jump场景)
func GetScriptFunc(funcName string) (ByteScriptFunc, bool) {
	if decodeFunc, exists := ByteScriptFuncCache[funcName]; exists {
		return decodeFunc, true
	}
	// 返回一个默认的空实现函数
	defaultFunc := func([]byte) ([]interface{}, error) {
		return nil, nil
	}
	return defaultFunc, false
}
