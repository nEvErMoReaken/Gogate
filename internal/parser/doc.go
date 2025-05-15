// Package parser 负责对 GoGate 数据网关接收到的原始数据进行解析。
//
// 本包的核心功能是将来自 connector 包的原始数据（如字节流、JSON 字符串）
// 转换为网关内部统一的结构化数据点格式 (通常是 pkg.Point 或 pkg.PointPackage)。
// 它支持多种解析逻辑，例如：
//   - BParser: 用于自定义的二进制字节流解析。
//   - JParser: 用于 JSON 格式数据的解析和字段提取。
//
// 解析器通常是高度可配置的，允许用户通过配置文件定义复杂的解析规则。
package parser
