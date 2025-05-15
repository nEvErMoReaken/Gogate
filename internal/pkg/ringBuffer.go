package pkg

import (
	"errors"
	"io"
	"time"
)

// RingBuffer 是一个为单协程消费和懒汉式数据源读取优化的单线程环形缓冲区。
// 它从一个 io.Reader 中读取数据，并允许消费者按需读取。

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

// ErrSizeNotPowerOf2 表示在创建 RingBuffer 时，提供的大小不是2的幂。

// NewRingBuffer 创建并返回一个具有指定大小的新 RingBuffer 实例。
//
// 输入:
//   - src: io.Reader，数据源
//   - size: uint32，缓冲区大小（必须为2的幂）
//
// 输出:
//   - *RingBuffer: 新的环形缓冲区实例
//   - error: 错误信息（如 size 非2的幂则返回 ErrSizeNotPowerOf2）
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

// ErrSrcNotSet 表示 RingBuffer 的数据源 (io.Reader) 未被设置或为 nil。

// isEmpty 判断缓冲区是否为空
func (r *RingBuffer) isEmpty() bool {
	return r.readPos == r.writePos
}

// isFull 判断缓冲区是否已满
func (r *RingBuffer) isFull() bool {
	return (r.writePos+1)&(r.ringSize-1) == r.readPos
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

// Read 从 RingBuffer 中读取最多 len(p) 字节的数据到 p。
//
// 输入:
//   - p: []byte，目标缓冲区
//
// 输出:
//   - int: 实际读取的字节数
//   - error: 错误信息（如 io.EOF、ErrSrcNotSet 等）
//
// 如果缓冲区为空，它会尝试从底层数据源填充数据。
// 如果在填充后仍然没有数据可读，它可能会返回 io.EOF 或在填充期间发生的其他错误。
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

// ReadFull 从 RingBuffer 中读取确切 len(p) 字节的数据到 p。
//
// 输入:
//   - p: []byte，目标缓冲区
//
// 输出:
//   - error: 错误信息（如 io.ErrUnexpectedEOF、io.ErrNoProgress 等）
//
// 它会持续尝试读取，直到 p 被填满，或遇到错误（非 io.EOF），或发生超时。
// 如果在填满 p 之前数据源结束 (io.EOF)，它将返回 io.ErrUnexpectedEOF。
// 如果在超时内没有读取到任何数据，它会返回 io.ErrNoProgress。
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

// ReadPos 返回当前 RingBuffer 的内部读取位置（逻辑指针）。
//
// 输入: 无
// 输出:
//   - uint32: 当前读取指针
func (r *RingBuffer) ReadPos() uint32 {
	return r.readPos
}

// WritePos 返回当前 RingBuffer 的内部写入位置（逻辑指针）。
//
// 输入: 无
// 输出:
//   - uint32: 当前写入指针
func (r *RingBuffer) WritePos() uint32 {
	return r.writePos
}

// Snapshot 返回 RingBuffer 中从逻辑 start 位置到逻辑 end 位置的数据的拷贝。
// 此方法会正确处理环形边界，即使 end < start (表示数据环绕)。
// 注意：此方法创建数据的拷贝，可能会有性能开销。
//
// 输入:
//   - start: uint32，起始逻辑位置
//   - end: uint32，结束逻辑位置
//
// 输出:
//   - []byte: 拷贝的数据切片
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

// Len 返回 RingBuffer 中当前可供读取的数据字节数。
//
// 输入: 无
// 输出:
//   - uint32: 可读取的数据字节数
func (r *RingBuffer) Len() uint32 {
	if r.writePos >= r.readPos {
		return r.writePos - r.readPos
	}
	return r.ringSize - (r.readPos - r.writePos)
}

// Cap 返回 RingBuffer 的总容量。
// 注意，实际可用容量会比 ringSize 小1，以区分空和满状态。
//
// 输入: 无
// 输出:
//   - uint32: 缓冲区容量
func (r *RingBuffer) Cap() uint32 {
	return r.ringSize - 1 // 注意：预留1字节避免满=空
}

// Reset 重置 RingBuffer，将其读写指针归零，使其看起来为空。
// 它不清空底层字节切片，只是重置状态。
//
// 输入: 无
// 输出: 无
func (r *RingBuffer) Reset() {
	r.readPos = 0
	r.writePos = 0
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
