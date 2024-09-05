package util

import (
	"gw22-train-sam/logger"
	"os"
	"path/filepath"
)

import (
	"fmt"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// ScriptFuncCache 脚本函数缓存
var ScriptFuncCache = make(map[string]func([]byte) interface{})

// LoadAllScripts 函数：加载/script目录下的所有脚本并缓存
func LoadAllScripts(scriptDir string, methods []string) error {
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
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 脚本已经全部加载并解释，现在可以缓存其中的函数
	for _, funcName := range methods {
		v, err := i.Eval("script." + funcName)
		if err != nil {
			logger.Log.Errorf("[Warning]: 在已读取脚本中未找到 %s 方法 %v\n", funcName, err)
			continue
		}
		ScriptFuncCache[funcName] = v.Interface().(func([]byte) interface{})
	}

	return nil
}

// GetScriptFunc 获取缓存的脚本函数
func GetScriptFunc(funcName string) (func([]byte) interface{}, error) {
	if decodeFunc, exists := ScriptFuncCache[funcName]; exists {
		return decodeFunc, nil
	}
	return nil, fmt.Errorf("方法名 %s 未注册", funcName)
}
