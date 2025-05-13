package parser

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"

	"gateway/internal/pkg"

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
	Vars map[string]any
	// Points 是本帧所有映射后的点的集合，Program在此基础上增添
	Points []*pkg.Point

	// PointsIndex 是 Points 的索引，用于快速查找
	PointsIndex map[uint64]int
	// GlobalMap 存储全局配置变量
	GlobalMap map[string]any
}

// Reset 清空 BEnv 的 Vars、Fields 和 Bytes，以便复用。
//
// 输入: 无
// 输出: 无
func (e *BEnv) Reset() {

	for k := range e.Vars {
		delete(e.Vars, k)
	}
	e.ResetPoints()
	e.Bytes = nil
}

// ResetPoints 清空 BEnv 的 Points，以便复用。
//
// 输入: 无
// 输出: 无
func (e *BEnv) ResetPoints() {

	// 这里不再释放，交给dispatcher释放，减少拷贝
	// for _, k := range e.Points {
	// 	pkg.PointPoolInstance.Put(k)
	// }
	e.Points = e.Points[:0]
}

// S 在 Points 映射中设置一个键值对，并返回 nil。
// 这是 expr 表达式中用于设置输出点的函数。
//
// 输入:
//   - key: 点名
//   - val: 点值

func (e *BEnv) S(tag map[string]any, field map[string]any) any {
	hash := makeFNVKey(tag)
	if _, ok := e.PointsIndex[hash]; ok {
		// 如果点已经存在，则追加其 Field
		for k, v := range field {
			e.Points[e.PointsIndex[hash]].Field[k] = v
		}
		return nil
	}
	// 如果点不存在，则创建新点
	point := pkg.PointPoolInstance.Get()
	point.Tag = tag
	point.Field = field
	e.Points = append(e.Points, point)
	e.PointsIndex[hash] = len(e.Points) - 1
	return nil
}

func makeFNVKey(tag map[string]any) uint64 {
	keys := make([]string, 0, len(tag))
	for k := range tag {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := fnv.New64a()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(fmt.Sprint(tag[k])))
		h.Write([]byte(";"))
	}
	return h.Sum64()
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
func (e *BEnv) V(key string, val any) any {
	e.Vars[key] = val
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
// 输入:
//   - points: 点名到表达式的映射
//   - vars: 变量名到表达式的映射
//
// 输出:
//   - *vm.Program: 编译后的程序
//   - error: 编译过程中遇到的错误
func CompileSectionProgram(points []PointExpression, vars map[string]any) (*vm.Program, error) {

	injectVars := BuildVarsProgramSource(vars)
	injectPoints := BuildPointsProgramSource(points)
	SectionSource := injectVars + injectPoints + "nil;" // 确保表达式有返回值

	program, err := expr.Compile(SectionSource, BuildSectionExprOptions()...)
	if err != nil {
		return nil, fmt.Errorf("编译 Section 程序失败 (source: %s): %w", SectionSource, err)
	}
	return program, nil
}

// BuildVarsProgramSource 将 vars 映射转换为 V 调用语句字符串。
// 输入:
//   - vars: 变量名到表达式的映射
//
// 输出:
//   - string: 生成的字符串格式为 "V(...); V(...); ...; nil"。
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
	return strings.Join(calls, "; ") + ";"
}

// BuildPointsProgramSource 将 points 映射转换为 F/T 调用语句字符串。
// 输入:
//   - points: 点名到表达式的映射
//
// 输出:
//   - string: 生成的字符串格式为 "F1(...); T1(...); F2(...); T2(...); F3(...); T3(...); nil"。
func BuildPointsProgramSource(points []PointExpression) string {
	if len(points) == 0 {
		return "" // 如果没有字段定义，返回空字符串
	}
	var calls []string
	for _, point := range points {
		if len(point.Field) > 3 {
			break
		}

		// 构建正确的expr map语法
		tagMap := "{"
		for k, v := range point.Tag {
			tagMap += fmt.Sprintf("%q: %s, ", k, v)
		}
		if len(point.Tag) > 0 {
			tagMap = tagMap[:len(tagMap)-2] // 移除末尾的逗号和空格
		}
		tagMap += "}"

		fieldMap := "{"
		for k, v := range point.Field {
			fieldMap += fmt.Sprintf("%q: %s, ", k, v)
		}
		if len(point.Field) > 0 {
			fieldMap = fieldMap[:len(fieldMap)-2] // 移除末尾的逗号和空格
		}
		fieldMap += "}"

		calls = append(calls, fmt.Sprintf("S(%s, %s)", tagMap, fieldMap))
	}
	// 添加最终的 nil 返回值
	return strings.Join(calls, "; ") + ";"
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
	// 添加 sprintf 函数
	expr.Function(
		"sprintf",
		func(params ...any) (any, error) {
			if len(params) < 1 {
				return nil, errors.New("sprintf 需要至少一个参数")
			}
			format, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("sprintf 第一个参数需要 string, 得到 %T", params[0])
			}
			return fmt.Sprintf(format, params[1:]...), nil
		},
		new(func(string, ...interface{}) string),
	),
	// 添加 string 函数，将任意类型转换为字符串
	expr.Function(
		"string",
		func(params ...any) (any, error) {
			if len(params) != 1 {
				return nil, errors.New("string 需要一个参数")
			}
			return fmt.Sprintf("%v", params[0]), nil
		},
		new(func(interface{}) string),
	),
}

// JEnv 是 JSON 处理的表达式执行环境。
type JEnv struct {
	// Data 存储解组后的 JSON 数据 (map[string]interface{})。
	Data map[string]interface{}
	// Points 存储由 F() 函数设置的输出点。
	Points []pkg.Point
	// GlobalMap 存储全局配置变量。
	GlobalMap map[string]interface{}
}

// Reset 清空 JEnv 的 Data 和 Points，以便复用。
func (e *JEnv) Reset() {
	// 清空 map 最高效的方式是重新创建
	e.Data = make(map[string]interface{})
	e.Points = make([]pkg.Point, 0)
	// GlobalMap 不需要重置，它是共享的
}

// F1 用于设置第一个点的 Field
func (e *JEnv) F1(key string, val any) any {
	e.Points[0].Field[key] = val
	return nil // Return nil
}

// F2 用于设置第二个点的 Field
func (e *JEnv) F2(key string, val any) any {
	e.Points[1].Field[key] = val
	return nil // Return nil
}

// F3 用于设置第三个点的 Field
func (e *JEnv) F3(key string, val any) any {
	e.Points[2].Field[key] = val
	return nil // Return nil
}

// T1 用于设置第一个点的 Tag
func (e *JEnv) T1(key string, val any) any {
	e.Points[0].Tag[key] = val
	return nil // Return nil
}

// T2 用于设置第二个点的 Tag
func (e *JEnv) T2(key string, val any) any {
	e.Points[1].Tag[key] = val
	return nil // Return nil
}

// T3 用于设置第三个点的 Tag
func (e *JEnv) T3(key string, val any) any {
	e.Points[2].Tag[key] = val
	return nil // Return nil
}

// JEnvPool 是 JEnv 对象的 sync.Pool，用于复用。
type JEnvPool struct {
	sync.Pool
}

// NewJEnvPool 创建一个新的 JEnvPool。
func NewJEnvPool(globalMap map[string]interface{}) *JEnvPool {
	return &JEnvPool{
		Pool: sync.Pool{
			New: func() any {
				// 初始化时创建空的 map 和三个预初始化的 Point
				return &JEnv{
					Data: make(map[string]interface{}),
					Points: []pkg.Point{
						{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
						{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
						{Tag: make(map[string]interface{}), Field: make(map[string]interface{})},
					},
					GlobalMap: globalMap, // 共享全局 map
				}
			},
		},
	}
}

// Get 从池中获取一个 JEnv 实例。
func (p *JEnvPool) Get() *JEnv {
	return p.Pool.Get().(*JEnv)
}

// Put 将一个 JEnv 实例放回池中。
func (p *JEnvPool) Put(e *JEnv) {
	e.Reset() // 重置状态
	p.Pool.Put(e)
}
