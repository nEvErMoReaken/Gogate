package util

import (
	"fmt"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"go/ast"
	"go/parser"
	"go/token"
	"gw22-train-sam/common"
	"os"
	"path/filepath"
)

type ScriptFunc func([]byte) ([]interface{}, error)

// ScriptFuncCache 脚本函数缓存
var ScriptFuncCache = make(map[string]ScriptFunc)

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
				common.Log.Errorf("[Warning]: 在已读取脚本中未找到 %s 方法 %v\n", funcName, err)
				continue
			}

			// 确保函数签名匹配
			if fnFunc, ok := v.Interface().(func([]byte) ([]interface{}, error)); ok {
				ScriptFuncCache[funcName] = fnFunc // 缓存函数
			} else {
				common.Log.Errorf("[Warning]: %s 方法的签名与预期的 ScriptFunc 类型不匹配\n", funcName)
			}
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
func GetScriptFunc(funcName string) (ScriptFunc, bool) {
	if decodeFunc, exists := ScriptFuncCache[funcName]; exists {
		return decodeFunc, true
	}
	// 返回一个默认的空实现函数
	defaultFunc := func([]byte) ([]interface{}, error) {
		return nil, nil
	}
	return defaultFunc, false
}
