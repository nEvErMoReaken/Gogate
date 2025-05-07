package parser

import (
	"fmt"
)

/* ---------- VarStore 定义 ---------- */

// VarStore 使用 map[string]interface{} 存储变量，简化结构但牺牲编译时类型安全。
// 注意：此实现不是线程安全的。如果需要并发访问，应添加锁。
type VarStore map[string]interface{}

// NewVarStore 创建并初始化一个新的 VarStore。
func NewVarStore() VarStore {
	return make(VarStore, 64) // 返回 map 类型本身
}

/* ---------- 写入 ---------- */

// Set 向 VarStore 中设置键值对。
// 它会覆盖具有相同键的任何现有值。
func (v VarStore) Set(key string, val any) {
	// map[string]interface{} 可以存储任何类型，无需类型转换
	if val == nil {
		delete(v, key) // 如果值为 nil，则删除键
		return
	}
	v[key] = val
}

/* ---------- 读取（通用 + 类型安全版） ---------- */

// Get 从 VarStore 中获取值。
// 返回值和表示键是否存在的布尔值。
func (v VarStore) Get(key string) (any, bool) {
	val, ok := v[key]
	return val, ok
}

// GetInt 尝试获取一个整数值。
// 如果键存在且值为整数类型（int, int64 等）或可以无损转换为 int64 的 float64，则返回 int64 和 true。
func (v VarStore) GetInt(key string) (int64, bool) {
	val, ok := v[key]
	if !ok {
		return 0, false
	}
	switch x := val.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), true // 注意潜在溢出，但符合原 VarStore 行为
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		if x > (1<<63 - 1) {
			return 0, false
		} // 溢出检查
		return int64(x), true
	case float32:
		f64 := float64(x)
		if f64 == float64(int64(f64)) { // 检查是否能无损转换为 int64
			return int64(f64), true
		}
	case float64:
		if x == float64(int64(x)) { // 检查是否能无损转换为 int64
			return int64(x), true
		}
	}
	return 0, false // 类型不匹配或无法转换
}

// GetFloat 尝试获取一个浮点数值。
// 如果键存在且值为浮点类型（float32, float64）或整数类型，则返回 float64 和 true。
func (v VarStore) GetFloat(key string) (float64, bool) {
	val, ok := v[key]
	if !ok {
		return 0, false
	}
	switch x := val.(type) {
	case float32:
		return float64(x), true
	case float64:
		return x, true
	// 允许从整数转换
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	}
	return 0, false // 类型不匹配
}

// GetStr 尝试获取一个字符串值。
// 如果键存在且值为字符串类型，则返回 string 和 true。
// 如果值为其他类型，则尝试将其转换为字符串表示形式。
func (v VarStore) GetStr(key string) (string, bool) {
	val, ok := v[key]
	if !ok {
		return "", false
	}
	switch x := val.(type) {
	case string:
		return x, true
	default:
		// 对于非字符串类型，返回其默认字符串表示形式
		return fmt.Sprintf("%v", x), true
	}
}

/* ---------- 删除 / 清空 ---------- */

// Del 从 VarStore 中删除一个键。
func (v VarStore) Del(key string) {
	delete(v, key)
}

// Reset 清空 VarStore 中的所有键值对。
func (v VarStore) Reset() {
	for k := range v {
		delete(v, k)
	}
}

// GetAll 在 VarStore 是 map 类型时不再需要，因为可以直接迭代 map。
// 如果确实需要在其他地方获取副本，可以添加一个 Clone 方法。

// Clone 创建并返回 VarStore 的一个浅拷贝。
func (v VarStore) Clone() VarStore {
	newMap := make(VarStore, len(v))
	for key, value := range v {
		newMap[key] = value // 注意：这仍然是浅拷贝
	}
	return newMap
}
