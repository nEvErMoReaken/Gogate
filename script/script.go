package script

import "fmt"

// Decode8BToInt DecodeFrame takes a byte slice as input and returns the decoded value
func Decode8BToInt(data []byte) ([]interface{}, error) {
	// 错误检查
	if len(data) == 0 {
		return nil, fmt.Errorf("[Decode8BToInt]输入字节数组为空")
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

// Decode8BitTo1Int 将 8 位的数据转换为 1 位的数据, 例如 0x02 -> 2 结果放入一个长度为1的数组
func Decode8BitTo1Int(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}
	// 只处理第一个字节的数据，转换为 int
	result := int(data[0])

	// 将结果放入一个长度为 1 的 interface{} 数组
	return []interface{}{result}, nil
}
