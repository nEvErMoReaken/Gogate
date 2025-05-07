package parser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

/* ---------- ExprResult 定义 ---------- */

// BEnv 是表达式执行的环境。
// 它包含了当前处理的字节数据、运行时变量、输出字段以及全局变量。
type BEnv struct {
	// Bytes 是当前 Section 处理的原始字节切片
	Bytes []byte
	// Vars 存储由 V() 函数设置的运行时变量
	Vars map[string]interface{}
	// Fields 存储由 F() 函数设置的输出字段
	Fields map[string]interface{}
	// GlobalMap 存储全局配置变量
	GlobalMap map[string]interface{}
}

// Reset 清空 BEnv 的 Vars、Fields 和 Bytes，以便复用。
//
// 输入: 无
// 输出: 无
func (e *BEnv) Reset() {

	for k := range e.Vars {
		delete(e.Vars, k)
	}
	for k := range e.Fields {
		delete(e.Fields, k)
	}
	e.Bytes = nil
}

// ResetFields 清空 BEnv 的 Fields，以便复用。
//
// 输入: 无
// 输出: 无
func (e *BEnv) ResetFields() {
	for k := range e.Fields {
		delete(e.Fields, k)
	}
}

// V 在 Vars 映射中设置一个键值对，并返回 nil。
// 这是 expr 表达式中用于设置运行时变量的函数。
//
// 输入:
//   - key: 变量名
//   - val: 变量值
//
// 输出:
//   - any: 总是返回 nil
func (e *BEnv) V(key string, val interface{}) any {
	e.Vars[key] = val
	return nil // Return nil
}

// F 在 Fields 映射中设置一个键值对，并返回 nil。
// 这是 expr 表达式中用于设置输出字段的函数。
//
// 输入:
//   - key: 字段名
//   - val: 字段值
//
// 输出:
//   - any: 总是返回 nil
func (e *BEnv) F(key string, val interface{}) any {
	e.Fields[key] = val
	return nil // Return nil
}

// CompileNextRoute 编译 nextRoute 表达式, 并将其编译后的程序存储到 Rule 中。
// 输入:
//   - nextRoute: 标签到表达式的映射
//
// 输出:
//   - error: 编译过程中遇到的错误
func CompileNextRoute(nextRoute []Rule) error {

	for i := range nextRoute {
		program, err := expr.Compile(nextRoute[i].Condition, BuildSectionExprOptions()...)
		if err != nil {
			return fmt.Errorf("编译 nextRoute 失败 (condition: %s): %w", nextRoute[i].Condition, err)
		}
		nextRoute[i].Program = program
	}
	return nil
}

// CompileSectionProgram 编译 Section 程序。
//
// 输入:
//   - fields: 字段名到表达式的映射
//   - vars: 变量名到表达式的映射
//
// 输出:
//   - *vm.Program: 编译后的程序
//   - error: 编译过程中遇到的错误
//
// Deprecated: Use CompileVarsProgram or CompileDevPrograms instead.
func CompileSectionProgram(dev map[string]map[string]any, vars map[string]any) (map[string]*vm.Program, error) {
	programs := make(map[string]*vm.Program)

	for devName, fields := range dev {
		source := BuildSectionProgramSource(fields, vars)

		// Handle empty sections (no program needed)
		if source == "" {
			return nil, fmt.Errorf("将 fields 和 vars 转换为 F/V 调用语句失败")
		}

		// Key does not exist, compile and store
		program, err := expr.Compile(source, BuildSectionExprOptions()...)
		if err != nil {
			return nil, fmt.Errorf("编译 Section 程序失败 (source: %s): %w", source, err)
		}
		programs[devName] = program
	}
	return programs, nil
}

// CompileVarsProgram 编译用于设置变量的 Section 程序。
//
// 输入:
//   - vars: 变量名到表达式的映射
//
// 输出:
//   - *vm.Program: 编译后的 V 调用程序，如果 vars 为空则返回 nil。
//   - error: 编译过程中遇到的错误
func CompileVarsProgram(vars map[string]any) (*vm.Program, error) {
	if len(vars) == 0 {
		return nil, nil // 没有变量定义，无需编译
	}
	source := BuildVarsProgramSource(vars)
	if source == "" {
		// BuildVarsProgramSource 理论上在 vars 不为空时不会返回空字符串
		return nil, fmt.Errorf("内部错误：BuildVarsProgramSource 返回了空字符串")
	}

	program, err := expr.Compile(source, BuildSectionExprOptions()...)
	if err != nil {
		return nil, fmt.Errorf("编译 Vars 程序失败 (source: %s): %w", source, err)
	}
	return program, nil
}

// CompileDevPrograms 编译用于为每个设备设置字段的 Section 程序。
//
// 输入:
//   - dev: 设备名到 {字段名到表达式} 的映射
//
// 输出:
//   - map[string]*vm.Program: 设备名到编译后 F 调用程序的映射。
//   - error: 编译过程中遇到的错误
func CompileDevPrograms(dev map[string]map[string]any) (map[string]*vm.Program, error) {
	programs := make(map[string]*vm.Program)

	for devName, fields := range dev {
		source := BuildFieldsProgramSource(fields) // 只使用 Fields 生成 F 调用

		// 如果一个设备没有定义字段，跳过编译
		if source == "" {
			continue
		}

		program, err := expr.Compile(source, BuildSectionExprOptions()...)
		if err != nil {
			return nil, fmt.Errorf("编译 Dev 程序失败 (dev: %s, source: %s): %w", devName, source, err)
		}
		programs[devName] = program
	}
	return programs, nil
}

// BuildSectionProgramSource 将 fields 和 vars 映射转换为 F/V 调用语句字符串。
// 生成的字符串格式为 "F(...); V(...); ...; nil"。
//
// 输入:
//   - fields: 字段名到表达式的映射
//   - vars: 变量名到表达式的映射
//
// 输出:
//   - string: 组合后的 F/V 调用语句字符串，如果 fields 和 vars 都为空，则返回空字符串
//
// Deprecated: Use BuildVarsProgramSource or BuildFieldsProgramSource instead.
func BuildSectionProgramSource(fields map[string]any, vars map[string]any) string {
	var calls []string

	// Add V calls for vars, 先处理变量, fields 就可以使用变量了
	for k, v := range vars {
		calls = append(calls, fmt.Sprintf("V(%q, %v)", k, v))
	}

	// Add F calls for fields (order doesn't strictly matter but process fields first)
	for k, v := range fields {
		calls = append(calls, fmt.Sprintf("F(%q, %v)", k, v))
	}

	if len(calls) == 0 {
		return "" // Return empty if no calls generated
	}

	// Join with semicolon and add final nil return value
	return strings.Join(calls, "; ") + "; nil"
}

// BuildVarsProgramSource 将 vars 映射转换为 V 调用语句字符串。
// 生成的字符串格式为 "V(...); V(...); ...; nil"。
func BuildVarsProgramSource(vars map[string]any) string {
	if len(vars) == 0 {
		return "" // 如果没有变量定义，返回空字符串
	}
	var calls []string
	for k, v := range vars {
		// %v 会将表达式原样放入，例如 Bytes[0]
		calls = append(calls, fmt.Sprintf("V(%q, %v)", k, v))
	}
	// 添加最终的 nil 返回值，确保表达式有返回值
	return strings.Join(calls, "; ") + "; nil"
}

// BuildFieldsProgramSource 将 fields 映射转换为 F 调用语句字符串。
// 生成的字符串格式为 "F(...); F(...); ...; nil"。
// 注意：此函数现在假设 F 函数的签名为 F(fieldName, value)
func BuildFieldsProgramSource(fields map[string]any) string {
	if len(fields) == 0 {
		return "" // 如果没有字段定义，返回空字符串
	}
	var calls []string
	for k, v := range fields {
		// %v 会将表达式原样放入
		calls = append(calls, fmt.Sprintf("F(%q, %v)", k, v))
	}
	// 添加最终的 nil 返回值
	return strings.Join(calls, "; ") + "; nil"
}

// BuildSectionExprOptions 返回用于编译 Section 程序 (F/V 调用) 的 expr 选项。
// 环境设置为 *BEnv，并注册了全局辅助函数。
//
// 输入: 无
// 输出:
//   - []expr.Option: 编译选项切片
func BuildSectionExprOptions() []expr.Option {
	options := []expr.Option{
		// 环境直接是 BEnv 结构体
		expr.Env(&BEnv{}),
	}
	// 同样需要注册全局辅助函数
	return append(options, helpers...)
}

/* ---------- expr helper 注册 ---------- */

// helpers 包含注册给 expr 编译器的全局辅助函数选项。
// 例如 BytesToInt 函数。
var helpers = []expr.Option{
	expr.Function(
		"BytesToInt",
		func(params ...any) (any, error) {
			if len(params) != 2 {
				return nil, errors.New("BytesToInt 需要两个参数")
			}
			// Type assertion with check
			data, ok := params[0].([]byte)
			if !ok {
				return nil, fmt.Errorf("BytesToInt 第一个参数需要 []byte, 得到 %T", params[0])
			}
			endian, ok := params[1].(string)
			if !ok {
				return nil, fmt.Errorf("BytesToInt 第二个参数需要 string, 得到 %T", params[1])
			}

			if len(data) < 4 { // Basic check for data length
				return nil, fmt.Errorf("BytesToInt 需要至少 4 字节数据, 得到 %d", len(data))
			}

			if endian == "little" {
				return int(binary.LittleEndian.Uint32(data)), nil
			}
			return int(binary.BigEndian.Uint32(data)), nil
		},
		// Provide the correct type signature for expr
		new(func([]byte, string) int),
	),
}
