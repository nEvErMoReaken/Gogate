package tcpServer

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gw22-train-sam/util"
	"strings"
)

type FixedChunkConfig struct {
	Type     string          `mapstructure:"type"`
	Length   interface{}     `mapstructure:"length"`
	Sections []SectionConfig `mapstructure:"sections"`
}

// SectionConfig 定义
type SectionConfig struct {
	From     FromConfig `mapstructure:"from"` // 偏移量
	Decoding Decoding   `mapstructure:"decoding"`
	For      ForConfig  `mapstructure:"for"`  // 赋值变量
	To       ToConfig   `mapstructure:"to"`   // 字段转换配置
	Desc     string     `mapstructure:"desc"` // 字段说明
}

type ForConfig struct {
	Type    string   `mapstructure:"type"`
	VarName []string `mapstructure:"varName"`
}
type FromConfig struct {
	Byte   int         `mapstructure:"byte"`
	Repeat interface{} `mapstructure:"repeat"`
}

type Decoding struct {
	Method string `mapstructure:"method"`
}

type ToConfig struct {
	Cached     bool     `mapstructure:"cached"`
	Stable     bool     `mapstructure:"stable"`
	DeviceName string   `mapstructure:"device"`
	DeviceType string   `mapstructure:"type"`
	Fields     []string `mapstructure:"fields"`
}

// InitChunks 从配置文件初始化 Chunk
func InitChunks(v *viper.Viper) (ChunkSequence, error) {

	var chunkSequence = ChunkSequence{
		make([]Chunk, 0),
		make(FrameContext),
	}
	chunks := v.Sub("TcpProto").Get("chunks").([]interface{})
	for _, chunk := range chunks {
		// 动态处理不同的 chunkType，生成chunklist
		tmpChunk, err := createChunk(chunk.(map[string]interface{}), &chunkSequence.VarPointer)
		if err != nil {
			return chunkSequence, err
		}

		chunkSequence.Chunks = append(chunkSequence.Chunks, tmpChunk)
	}
	return chunkSequence, nil
}

// createChunk 根据 chunkType 动态创建对应的 Chunk
func createChunk(chunkMap map[string]interface{}, context *FrameContext) (Chunk, error) {
	switch chunkType := chunkMap["type"].(string); chunkType {
	case "FixedLengthChunk":
		var fixedChunkConfig FixedChunkConfig
		err := mapstructure.Decode(chunkMap, &fixedChunkConfig) // 将配置解码为 FixedLengthChunk 结构体
		if err != nil {
			return nil, fmt.Errorf("[createChunk]failed to mapstructure FixedLengthChunk: %v", err)
		}
		//fmt.Printf("FixedChunkConfig: %+v\n", fixedChunkConfig)
		//fmt.Println()
		// 设置默认值: 若 Repeat 未设置，则设置为 1
		for i, section := range fixedChunkConfig.Sections {
			if section.From.Repeat == nil { // 检查是否为空
				fixedChunkConfig.Sections[i].From.Repeat = 1 // 设置默认值
			}
		}

		var length, err1 = parseIntVariable(context, fixedChunkConfig.Length)
		if err1 != nil {
			return nil, fmt.Errorf("[createChunk]failed to parse length: %v", err1)
		}
		// 初始化 FixedLengthChunk(不包含Section)
		var fixedChunk = FixedLengthChunk{
			Length:     length,
			Sections:   make([]Section, 0),
			VarPointer: context,
		}
		// 初始化Section
		for _, section := range fixedChunkConfig.Sections {
			var tmpRepeat, err = parseIntVariable(context, section.From.Repeat)
			if err != nil {
				return nil, fmt.Errorf("[createChunk]failed to parse repeat: %v", err)
			}
			var tmpDecode util.ScriptFunc
			if section.Decoding.Method != "" {
				tmpDecode = util.GetScriptFunc(section.Decoding.Method)
			}

			var tmpSec = Section{
				Repeat:       tmpRepeat,
				Length:       section.From.Byte,
				Decoding:     tmpDecode,
				ToDeviceName: section.To.DeviceName,
				ToDeviceType: section.To.DeviceType,
				PointTarget:  make([]*interface{}, 0),
			}
			// 初始化For指针变量
			for _, varName := range section.For.VarName {
				switch section.For.Type {
				case "string":
					varFor := new(string)
					// 将 *string 转为 interface{}
					varForInterface := interface{}(varFor)
					tmpSec.PointTarget = append(tmpSec.PointTarget, &varForInterface)
					(*context)[varName] = &varForInterface
				case "int":
					varFor := new(int)
					// 将 *int 转为 interface{}
					varForInterface := interface{}(varFor)
					tmpSec.PointTarget = append(tmpSec.PointTarget, &varForInterface)
					(*context)[varName] = &varForInterface
				}

			}
			fixedChunk.Sections = append(fixedChunk.Sections, tmpSec)
		}

		// 将 FrameContext 指针赋值给 FixedLengthChunk
		fixedChunk.VarPointer = context
		return &fixedChunk, nil

	case "ConditionalChunk":
		var conditionalChunk ConditionalChunk
		// TODO: 解析 ConditionalChunk
		return &conditionalChunk, nil

	default:
		return nil, fmt.Errorf("unknown chunk type: %s", chunkType)
	}
}

// parseIntVariable 从字符串中提取变量名, 若无变量则返回原始值
func parseIntVariable(context *FrameContext, value interface{}) (*int, error) {
	// 假设占位符格式为 ${var_name}，我们去掉 "${" 和 "}" 并返回中间的部分
	switch value.(type) {
	case int:
		result := new(int)
		*result = value.(int)
		return result, nil
	case string:
		var sValue = value.(string)
		if strings.HasPrefix(sValue, "${") && strings.HasSuffix(sValue, "}") {
			var varName = sValue[2 : len(sValue)-1]
			// 从 context 中查找对应的变量
			if contextVar, ok := (*context)[varName]; ok {
				// 检查 contextVar 是否非空并且是 *int
				if contextVar != nil {
					if lengthPtr, ok := (*contextVar).(*int); ok {
						return lengthPtr, nil
					} else {
						return nil, fmt.Errorf("context variable '%s' is not of type *int", varName)
					}
				} else {
					return nil, fmt.Errorf("context variable '%s' is nil", varName)
				}
			} else {
				return nil, fmt.Errorf("variable '%s' not found in context", varName)
			}
		}
	}
	return nil, fmt.Errorf("failed to parse int variable: %v", value)
}
