package byteType

type SectionModel struct {
	ByteOffset   func() int                // 偏移量计算器，可能是固定值或基于某个变量
	DecodeMethod *func([]byte) interface{} // 解码方法
	VarPointer   *map[string]interface{}   // 指向存储变量的公共指针字典
	Repeat       int                       // 重复次数
}
