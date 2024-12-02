package script

import (
	"fmt"
	"strconv"
)

/*
JsonType的脚本
遵循格式: type JsonScriptFunc func(map[string]interface{}) (string, string, map[string]interface{}, error)
*/

// ConvertOldGatewayTelemetry 将旧网关遥测数据转换为新格式
func ConvertOldGatewayTelemetry(jsonMap map[string]interface{}) (string, string, map[string]interface{}, error) {
	devName := jsonMap["deviceName"].(string)
	devType := jsonMap["deviceType"].(string)
	fields := jsonMap["fields"].(map[string]interface{})
	return devName, devType, fields, nil
}

/*  ByteType的脚本
遵循格式: type ByteScriptFunc func([]byte) ([]interface{}, error)
*/

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

// DecodeByteToBigEndianBits DecodeFrame takes a byte slice as input and returns the decoded value
func DecodeByteToBigEndianBits(data []byte) ([]interface{}, error) {
	// 错误检查
	if len(data) == 0 {
		return nil, fmt.Errorf("[DecodeByteToBigEndianBits]输入字节数组为空")
	}
	// 结果数组，长度是输入字节数组长度的 8 倍
	result := make([]interface{}, 0, len(data)*8)

	for _, b := range data {
		// 遍历每一个字节，从高位到低位依次提取每一位
		for i := 7; i >= 0; i-- {
			// 提取第 i 位
			bit := (b >> i) & 0x01
			// 将提取到的位放入结果数组
			result = append(result, int(bit))
		}
	}
	return result, nil
}

// BytesToBigEndianInt 将字节数组解析 结果放入一个长度为1的数组
func BytesToBigEndianInt(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}
	// 检查字节数组是否超过 int 的字节数限制（假设 int 为 64 位）
	if len(data) > 8 {
		return nil, fmt.Errorf("input data exceeds size of int")
	}

	// 处理所有字节，将其转换为 int
	result := 0
	for _, b := range data {
		result = result<<8 + int(b)
	}
	// 将结果放入一个长度为 1 的 interface{} 数组
	return []interface{}{result}, nil
}

// Decode8BitTo1Id 将 8 位的数据转换为 1 位的数据, 例如 0x02 -> 2 结果放入一个长度为1的数组
func Decode8BitTo1Id(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}
	// 只处理第一个字节的数据，转换为 int
	result := strconv.Itoa(int(data[0]))

	// 将结果放入一个长度为 1 的 interface{} 数组
	return []interface{}{result}, nil
}
