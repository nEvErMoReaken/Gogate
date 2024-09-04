package script

// Decode8BToInt DecodeFrame takes a byte slice as input and returns the decoded value
func Decode8BToInt(data []byte) interface{} {
	// 结果数组，长度是输入字节数组长度的 8 倍
	result := make([]int, 0, len(data)*8)

	for _, b := range data {
		// 遍历每一个字节，从高位到低位依次提取每一位
		for i := 7; i >= 0; i-- {
			// 提取第 i 位
			bit := (b >> i) & 0x01
			// 将提取到的位放入结果数组
			result = append(result, int(bit))
		}
	}
	return result
}
