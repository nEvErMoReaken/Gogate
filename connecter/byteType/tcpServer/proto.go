package tcpServer

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"io"
)

type ChunkSequence struct {
	Chunks     []Chunk `mapstructure:"chunks"`
	VarPointer map[string]*interface{}
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

// Chunk 处理器接口
type Chunk interface {
	Process(reader io.Reader, context map[string]*interface{}) error
}

// FixedLengthChunk 实现
type FixedLengthChunk struct {
	Length   int
	Sections []Section
}

func (f *FixedLengthChunk) Process(reader io.Reader, context map[string]interface{}) error {
	fmt.Println("Processing FixedLengthChunk")
	// 读取固定长度数据的逻辑...
	return nil
}

// ConditionalChunk 实现
type ConditionalChunk struct {
	ConditionField string           `mapstructure:"condition_field"`
	Choices        map[string]Chunk `mapstructure:"choices"`
}

func (c *ConditionalChunk) Process(reader io.Reader, context map[string]interface{}) error {
	fmt.Println("Processing ConditionalChunk")
	// 动态选择下一个 Chunk 解析逻辑
	return nil
}

// Section 定义
type Section struct {
	From     FromConfig `mapstructure:"from"` // 偏移量
	Decoding Decoding   `mapstructure:"decoding"`
	For      string     `mapstructure:"for"`  // 赋值变量
	To       ToConfig   `mapstructure:"to"`   // 字段转换配置
	Desc     string     `mapstructure:"desc"` // 字段说明
}

type FromConfig struct {
	Byte   int `mapstructure:"byte"`
	Repeat int `mapstructure:"repeat"`
}

type Decoding struct {
	Method string `mapstructure:"method"`
}

type ToConfig struct {
	Cached bool `mapstructure:"cached"`
	Stable bool `mapstructure:"stable"`
}

// InitChunks 从配置文件初始化 Chunk
func InitChunks(v *viper.Viper) ([]Chunk, error) {
	var chunkProcessors []Chunk

	chunks := v.Get("Proto.chunks").([]interface{}) // 获取 chunks 列表

	for _, chunk := range chunks {
		chunkMap := chunk.(map[string]interface{})
		chunkType := chunkMap["type"].(string)

		var processor Chunk

		switch chunkType {
		case "FixedLengthChunk":
			var fixedChunk FixedLengthChunk
			err := mapstructure.Decode(chunkMap, &fixedChunk) // 解码为 FixedLengthChunk
			if err != nil {
				return nil, err
			}
			processor = &fixedChunk

		case "ConditionalChunk":
			var conditionalChunk ConditionalChunk
			err := mapstructure.Decode(chunkMap, &conditionalChunk) // 解码为 ConditionalChunk
			if err != nil {
				return nil, err
			}
			processor = &conditionalChunk

		default:
			return nil, fmt.Errorf("unknown chunk type: %s", chunkType)
		}

		chunkProcessors = append(chunkProcessors, processor)
	}

	return chunkProcessors, nil
}
