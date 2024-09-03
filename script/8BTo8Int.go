package script

// Decode8BToInt DecodeFrame takes a byte slice as input and returns the decoded value
func Decode8BToInt(data []byte) interface{} {
	// 结果数组，长度是输入字节数组长度的 8 倍
	result := make([]int, 0, len(data)*8)

	for _, b := range data {
		// 遍历每一个字节，从低位到高位依次提取每一位
		for i := 0; i < 8; i++ {
			// 使用位运算提取第 i 位，并将其转换为 int（0 或 1）
			bit := (b >> i) & 1
			result = append(result, int(bit))
		}
	}
	return result
}
