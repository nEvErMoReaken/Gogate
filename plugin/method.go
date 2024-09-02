package plugin

func decode(byteArray []byte) interface{} {
	b := byteArray[0]
	// 返回长度为8的int数组
	var bits [8]int
	for i := 0; i < 8; i++ {
		bits[i] = int(b >> uint(7-i) & 1)
	}
	return bits
}
