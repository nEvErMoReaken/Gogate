package pkg

import (
	"errors"
	"io"
	"time"
)

// RingBuffer 是一个单线程懒汉式环形缓冲区
// 优化场景：单协程消费，懒汉式读取数据源

type RingBuffer struct {
	buf      []byte
	ringSize uint32
	readPos  uint32
	writePos uint32
	src      io.Reader
}

const maxConsecutiveEmptyReads = 100

var (
	ErrSizeNotPowerOf2 = errors.New("大小必须是2的幂")
	errNegativeRead    = errors.New("读取为负值")
	ErrSrcNotSet       = errors.New("数据源未设置")
)

// NewRingBuffer 创建一个环形缓冲区
func NewRingBuffer(src io.Reader, size uint32) (*RingBuffer, error) {
	if size&(size-1) != 0 {
		return nil, ErrSizeNotPowerOf2
	}
	return &RingBuffer{
		buf:      make([]byte, size),
		ringSize: size,
		src:      src,
	}, nil
}

// isEmpty 判断缓冲区是否为空
func (r *RingBuffer) isEmpty() bool {
	return r.readPos == r.writePos
}

// isFull 判断缓冲区是否已满
func (r *RingBuffer) isFull() bool {
	return (r.writePos+1)&(r.ringSize-1) == r.readPos
}

// availableWrite 计算环形缓冲区可写入的空间大小
func (r *RingBuffer) availableWrite() uint32 {
	if r.isFull() {
		return 0
	}

	if r.writePos >= r.readPos {
		// 写在读后面，可用空间 = 缓冲区大小 - (写指针 - 读指针) - 1
		return r.ringSize - (r.writePos - r.readPos) - 1
	}
	// 写在读前面，可用空间 = 读指针 - 写指针 - 1
	return r.readPos - r.writePos - 1
}

// continuousWriteSpace 计算从当前写入位置开始的连续可写入空间
func (r *RingBuffer) continuousWriteSpace() (uint32, uint32) {
	if r.isFull() {
		return 0, 0
	}

	// 计算物理写入位置
	offset := r.writePos & (r.ringSize - 1)

	// 如果读在写前面或与写相等
	if r.readPos <= r.writePos {
		// 可以写到缓冲区末尾，但要保留一个位置
		if r.readPos > 0 {
			// 读不在开头，可以写到缓冲区末尾
			return r.ringSize - offset, offset
		} else {
			// 读在开头，末尾需要留一个位置
			return r.ringSize - offset - 1, offset
		}
	} else {
		// 读在写后面，可以写到读指针前一个位置
		return r.readPos - offset - 1, offset
	}
}

// Read 从环形缓冲区读取数据
func (r *RingBuffer) Read(p []byte) (int, error) {
	if r.src == nil {
		return 0, ErrSrcNotSet
	}
	// 如果请求长度为0，按照规范直接返回0, nil
	if len(p) == 0 {
		return 0, nil
	}

	// 检查是否为空
	if r.isEmpty() {
		// 缓冲区为空，尝试填充
		err := r.fill()

		// 填充后重新检查是否为空
		if r.isEmpty() {
			// 如果仍然为空，返回填充时遇到的错误或EOF
			if err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		// 如果读到了数据，即使有错误也先返回数据
		// 错误会在下次读取时处理
	}
	// 计算可读取数据量
	var available uint32
	rp := r.readPos
	wp := r.writePos

	if wp >= rp {
		available = wp - rp
	} else {
		available = r.ringSize - (rp - wp)
	}

	offset := rp & (r.ringSize - 1)
	first := r.ringSize - offset
	toRead := uint32(len(p))
	if toRead > available {
		toRead = available
	}

	if toRead <= first {
		copy(p, r.buf[offset:offset+toRead])
	} else {
		copy(p, r.buf[offset:])
		copy(p[first:], r.buf[0:toRead-first])
	}

	// 更新读取位置
	newReadPos := rp + toRead
	if newReadPos >= r.ringSize {
		newReadPos -= r.ringSize
	}
	r.readPos = newReadPos

	return int(toRead), nil
}

// fill 从数据源填充环形缓冲区
func (r *RingBuffer) fill() error {
	for i := maxConsecutiveEmptyReads; i > 0; i-- {
		// 如果缓冲区已满，不能继续填充
		if r.isFull() {
			return nil
		}

		// 计算可连续写入的空间和偏移量
		space, offset := r.continuousWriteSpace()
		if space == 0 {
			// 理论上不会发生，因为已经检查了isFull
			continue
		}

		// 从源读取数据到缓冲区
		n, err := r.src.Read(r.buf[offset : offset+space])
		if n < 0 {
			return errNegativeRead
		}

		if n > 0 {
			// 更新写指针，使用模运算处理环绕
			r.writePos = (r.writePos + uint32(n)) % r.ringSize
			return nil
		}

		if err != nil {
			return err
		}
	}

	return io.ErrNoProgress
}

// ReadFull 读取确切的len(p)字节到p
func (r *RingBuffer) ReadFull(p []byte) error {
	startTime := time.Now()
	timeout := 500 * time.Millisecond
	n := 0
	for n < len(p) {
		if time.Since(startTime) > timeout {
			if n > 0 {
				return io.ErrUnexpectedEOF
			}
			return io.ErrNoProgress
		}

		nn, err := r.Read(p[n:])
		n += nn

		if n == len(p) {
			return nil
		}

		if err == io.EOF {
			time.Sleep(10 * time.Microsecond)
			err = nil
			continue
		}
		if err != nil {
			return err
		}

		if nn == 0 {
			time.Sleep(10 * time.Microsecond)
		}
	}

	return nil
}

// ReadPos 返回读指针
func (r *RingBuffer) ReadPos() uint32 {
	return r.readPos
}

// WritePos 返回写指针
func (r *RingBuffer) WritePos() uint32 {
	return r.writePos
}

// Snapshot 返回从 start 到 end 的拷贝（环形处理）
func (r *RingBuffer) Snapshot(start, end uint32) []byte {
	var result []byte
	if end >= start {
		result = make([]byte, end-start)
		copy(result, r.buf[start&(r.ringSize-1):end&(r.ringSize-1)])
	} else {
		// 跨界
		firstPart := r.ringSize - (start & (r.ringSize - 1))
		total := end + r.ringSize - start
		result = make([]byte, total)

		copy(result, r.buf[start&(r.ringSize-1):])
		copy(result[firstPart:], r.buf[:end&(r.ringSize-1)])
	}
	return result
}

// ======= 保留方法 ========

// Len 获取环形缓冲区中可读取的数据量
func (r *RingBuffer) Len() uint32 {
	if r.writePos >= r.readPos {
		return r.writePos - r.readPos
	}
	return r.ringSize - (r.readPos - r.writePos)
}

// Cap 获取环形缓冲区的容量
func (r *RingBuffer) Cap() uint32 {
	return r.ringSize - 1 // 注意：预留1字节避免满=空
}

// Reset 重置环形缓冲区
func (r *RingBuffer) Reset() {
	r.readPos = 0
	r.writePos = 0
}
