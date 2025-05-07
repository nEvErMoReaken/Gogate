package pkg

const maxCachedTimes = 10

var count = 0

type ByteStore struct {
	cache map[int][]byte
}

var ByteCache = NewByteStore()

func NewByteStore() *ByteStore {
	return &ByteStore{
		cache: make(map[int][]byte),
	}
}

// Get 返回一个指定长度的 byte slice，如果可复用则取出复用，否则新建
func (b *ByteStore) Get(size uint32) []byte {
	sizeInt := int(size)
	if list, ok := b.cache[sizeInt]; ok && len(list) > 0 {
		return list[:]
	} else if count <= maxCachedTimes {
		count++
		b.cache[sizeInt] = make([]byte, sizeInt)
		return b.cache[sizeInt]
	} else {
		// 熔断机制
		return make([]byte, size)
	}
}
