package tcpServer

import (
	"fmt"
	"gw22-train-sam/util"
	"io"
)

// FrameContext 每一帧中, 也就是多Chunks共享的上下文
type FrameContext map[string]*interface{}

// Chunk 处理器接口
type Chunk interface {
	Process(reader io.Reader, context map[string]*interface{}) error
	String() string // 添加 String 方法
}

type ChunkSequence struct {
	Chunks     []Chunk `mapstructure:"chunks"`
	VarPointer FrameContext
}

// Process 方法：处理整个 ChunkSequence
func (c *ChunkSequence) Process(reader io.Reader) error {
	context := make(map[string]*interface{}) // 共享上下文，用于传递变量

	// 处理每一个 Chunk
	for _, chunk := range c.Chunks {
		err := chunk.Process(reader, context) // 传递共享的上下文
		if err != nil {
			return err
		}
	}

	return nil
}

// ChunkSequence 的 String 方法
func (c *ChunkSequence) String() string {
	result := "ChunkSequence:\n"
	for i, chunk := range c.Chunks {
		result += fmt.Sprintf("  Chunk %d: %s\n", i+1, chunk.String()) // 调用每个 Chunk 的 String 方法
	}
	return result
}

// FixedLengthChunk 实现
type FixedLengthChunk struct {
	Length     *int
	Sections   []Section
	VarPointer *FrameContext
}

func (f *FixedLengthChunk) Process(reader io.Reader, context map[string]*interface{}) error {
	fmt.Println("Processing FixedLengthChunk")
	// 读取固定长度数据的逻辑...
	return nil
}

func (f *FixedLengthChunk) String() string {
	return fmt.Sprintf("FixedLengthChunk (Length: %d, Sections: %d)", f.Length, len(f.Sections))
}

// ConditionalChunk 实现
type ConditionalChunk struct {
	ConditionField string           `mapstructure:"condition_field"`
	Choices        map[string]Chunk `mapstructure:"choices"`
	VarPointer     *FrameContext
	Sections       []Section
}

func (c *ConditionalChunk) Process(reader io.Reader, context map[string]*interface{}) error {
	fmt.Println("Processing ConditionalChunk")
	// 动态选择下一个 Chunk 解析逻辑
	return nil
}

func (c *ConditionalChunk) String() string {
	return fmt.Sprintf("ConditionalChunk (ConditionField: %s, Choices: %d)", c.ConditionField, len(c.Choices))
}

type Section struct {
	Repeat       *int
	Length       int
	Decoding     util.ScriptFunc
	ToDeviceName string
	ToDeviceType string
}
