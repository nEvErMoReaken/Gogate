package byteType

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gw22-train-sam/common"
	"gw22-train-sam/model"
	"gw22-train-sam/util"
	"io"
	"strings"
	"time"
)

// Chunk 处理器接口
type Chunk interface {
	Process(reader io.Reader, frame *[]byte, collection *model.SnapshotCollection) error
	String() string // 添加 String 方法
}

type ChunkSequence struct {
	Chunks             []Chunk `mapstructure:"chunks"`
	VarPointer         model.FrameContext
	snapShotCollection *model.SnapshotCollection // 快照集合
}

// ProcessAll 方法：处理整个 ChunkSequence
func (c *ChunkSequence) ProcessAll(deviceId string, reader io.Reader, frame *[]byte) error {
	// 初始化一些变量
	// deviceId
	result := new(interface{})
	*result = deviceId
	c.VarPointer["deviceId"] = result
	// ts
	timeNow := new(interface{})
	*timeNow = time.Now()
	c.VarPointer["ts"] = timeNow
	// 处理每一个 Chunk
	for index, chunk := range c.Chunks {
		err := chunk.Process(reader, frame, c.snapShotCollection)
		if err != nil {
			if err == io.EOF {
				return err
			}
			return fmt.Errorf("[HandleConnection] 解析第 %d 个 Chunk 失败: %s", index+1, err) // 其他错误，终止连接
		}
	}
	// 到此位置时 所有快照更新完毕
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
	Length     *interface{}
	Sections   []Section
	VarPointer *model.FrameContext
}

// 为 FixedLengthChunk 实现 String 方法，打印指针指向的值和指针的地址
func (f *FixedLengthChunk) String() string {
	// 打印 Length 指针的值（解引用）
	lengthVal := "nil"
	if f.Length != nil {
		lengthVal = fmt.Sprintf("%d", *f.Length)
	}

	// 打印 Sections 指针中的值
	sectionsStr := ""
	for i, sec := range f.Sections {
		// 打印 Repeat 指针的值和地址
		repeatVal := "nil"
		repeatAddr := "nil"
		if sec.Repeat != nil {
			repeatVal = fmt.Sprintf("%d", *sec.Repeat)
			repeatAddr = fmt.Sprintf("%p", sec.Repeat) // 打印地址
		}

		// 打印 Decoding 指针的地址
		decodingAddr := "nil"
		if sec.Decoding != nil {
			decodingAddr = fmt.Sprintf("%p", sec.Decoding) // 打印地址
		}

		// 打印 PointTarget 列表和指针地址
		pointTargetStr := "["
		for j, pt := range sec.PointTarget {
			if pt == nil {
				pointTargetStr += "nil"
			} else {
				pointTargetStr += fmt.Sprintf("%p", pt) // 打印指针地址
			}
			if j < len(sec.PointTarget)-1 {
				pointTargetStr += ", "
			}
		}
		pointTargetStr += "]"

		// 打印 FieldTarget 列表
		fieldTargetStr := "["
		for j, field := range sec.FieldTarget {
			fieldTargetStr += field
			if j < len(sec.FieldTarget)-1 {
				fieldTargetStr += ", "
			}
		}
		fieldTargetStr += "]"

		// 拼接 Section 的详细信息
		sectionsStr += fmt.Sprintf(
			"  Section %d: Repeat=%s (Addr: %s), Length=%d, Decoding Addr=%s, DeviceName=%s, DeviceType=%s, PointTarget=%s, FieldTarget=%s\n",
			i+1, repeatVal, repeatAddr, sec.Length, decodingAddr, sec.ToDeviceName, sec.ToDeviceType, pointTargetStr, fieldTargetStr)
	}

	// 打印整个结构体信息
	return fmt.Sprintf("FixedLengthChunk:\n  Length=%s\n  Sections:\n%s", lengthVal, sectionsStr)
}

func (f *FixedLengthChunk) Process(reader io.Reader, frame *[]byte, collection *model.SnapshotCollection) error {
	// ～～～ 定长块的处理逻辑 ～～～
	// 1. 读取固定长度数据
	data := make([]byte, (*f.Length).(int))
	n, err := io.ReadFull(reader, data)
	if err != nil {
		// 处理 EOF 错误
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("读取错误: %v", err)
	}
	// 处理部分读取
	if n < (*f.Length).(int) {
		common.Log.Warnf("只读取了 %d 字节，而不是期望的 %d 字节", n, *f.Length)
	}
	// 定长Chunk可以直接追加到frame中
	*frame = append(*frame, data...)
	// 2. 解析数据
	common.Log.Debugf("Processing FixedLengthChunk")
	cursor := 0
	for index, sec := range f.Sections {
		for i := 0; i < (*sec.Repeat).(int); i++ {
			// 2.1. 根据Sec的length解码
			if sec.Decoding == nil {
				common.Log.Debugf("Step.%+v: Loop.%+v: Jump For %+v Byte", index+1, i+1, sec.Length)
				// 如果没有解码函数，直接跳过
				cursor += sec.Length
				continue
			}
			if cursor+sec.Length > len(data) {
				return fmt.Errorf("游标超出数据长度")
			}
			decoded, err := sec.Decoding(data[cursor : cursor+sec.Length])
			common.Log.Debugf("Step.%+v: Loop.%+v: Decoded: %v Byte to %+v", index+1, i+1, sec.Length, decoded)
			if err != nil {
				return err
			}
			// 2.2 移动游标
			cursor += sec.Length

			// 2.3 保存解码后的数据到对应的 PointTarget下标内
			// sec.PointTarget[i] = decoded[i]
			// 假设解码后有三个速度值： v1,v2,v3 分别对应变量名为 vobc_speed1, vobc_speed2, vobc_speed3
			// v1 -> vobc_speed1,
			// v2 -> vobc_speed2,
			// v3 -> vobc_speed3
			// 变量后续用于For(如果有的话）, 供后续Section使用
			if len(sec.PointTarget) != 0 && len(sec.PointTarget) != len(decoded) {
				return fmt.Errorf("解码后的数据长度与PointTarget长度不匹配, %d != %d", len(decoded), len(sec.PointTarget))
			}
			for i, pt := range sec.PointTarget {

				*pt = decoded[i]
			}
			// 2.4 设备快照更新逻辑
			// 注，这里的ToDeviceName是可能包含${}的，需要解析
			if sec.ToDeviceType == "" || sec.ToDeviceName == "" {
				continue
			}
			tarSnapshot := collection.GetDeviceSnapshot(sec.ToDeviceName, sec.ToDeviceType)
			tarSnapshot.SetDeviceName(f.VarPointer)
			if len(decoded) != len(sec.FieldTarget) {
				return fmt.Errorf("解码后的数据长度与FieldTarget长度不匹配, %d != %d", len(decoded), len(sec.FieldTarget))
			}
			for index, field := range sec.FieldTarget {
				tarSnapshot.SetField(field, decoded[index])
			}
			tarSnapshot.Ts = (*(*f.VarPointer)["ts"]).(time.Time)
		}
	}
	if cursor != len(data) {
		common.Log.Warnf("游标未到达数据末尾，有漏数据的风险。游标位置：%d，数据长度：%d", cursor, len(data))
	}
	return nil
}

// ConditionalChunk 实现
type ConditionalChunk struct {
	ConditionField string           `mapstructure:"condition_field"`
	Choices        map[string]Chunk `mapstructure:"choices"`
	VarPointer     *model.FrameContext
	Sections       []Section
}

func (c *ConditionalChunk) Process(reader io.Reader, frame *[]byte, collection *model.SnapshotCollection) error {
	fmt.Println("Processing ConditionalChunk")
	// 动态选择下一个 Chunk 解析逻辑
	return nil
}

func (c *ConditionalChunk) String() string {
	return fmt.Sprintf("ConditionalChunk (ConditionField: %s, Choices: %d)", c.ConditionField, len(c.Choices))
}

type Section struct {
	Repeat       *interface{}
	Length       int
	Decoding     util.ScriptFunc
	ToDeviceName string
	ToDeviceType string
	PointTarget  []*interface{} // 解码后变量的最终去向
	FieldTarget  []string
}

// InitChunks 从配置文件初始化 Chunk
func InitChunks(v *viper.Viper, protoFile string) (ChunkSequence, error) {
	var chunkSequence = ChunkSequence{
		make([]Chunk, 0),
		make(model.FrameContext),
		nil,
	}
	fmt.Println(protoFile)
	chunks := v.Sub(protoFile).Get("chunks").([]interface{})
	for _, chunk := range chunks {
		// 动态处理不同的 chunkType，生成chunkSequence
		tmpChunk, err := createChunk(chunk.(map[string]interface{}), &chunkSequence.VarPointer)
		if err != nil {
			return chunkSequence, err
		}

		chunkSequence.Chunks = append(chunkSequence.Chunks, tmpChunk)
	}
	common.Log.Infof("ChunkSequence 初始化成功 %+v", chunkSequence)
	return chunkSequence, nil
}

// createChunk 根据 chunkType 动态创建对应的 Chunk
func createChunk(chunkMap map[string]interface{}, context *model.FrameContext) (Chunk, error) {
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

		var length, err1 = parseVariable(context, fixedChunkConfig.Length)
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
			var tmpRepeat, err = parseVariable(context, section.From.Repeat)
			if err != nil {
				return nil, fmt.Errorf("[createChunk]failed to parse repeat: %v", err)
			}

			var tmpSec = Section{
				Repeat:       tmpRepeat,
				Length:       section.From.Byte,
				Decoding:     nil,
				ToDeviceName: section.To.DeviceName,
				ToDeviceType: section.To.DeviceType,
				PointTarget:  make([]*interface{}, 0),
				FieldTarget:  section.To.Fields,
			}
			tmpDecoding, exist := util.GetScriptFunc(section.Decoding.Method)
			if exist {
				tmpSec.Decoding = tmpDecoding
			}
			// 初始化For指针变量
			for _, varName := range section.For.VarName {
				varFor := new(interface{})
				// 双向绑定逻辑：将变量指针存入Section中
				tmpSec.PointTarget = append(tmpSec.PointTarget, varFor)
				// 将变量指针存入context中
				(*context)[varName] = varFor
			}
			fixedChunk.Sections = append(fixedChunk.Sections, tmpSec)
		}

		// 将 model.FrameContext 指针赋值给 FixedLengthChunk
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

// parseVariable 从字符串中提取变量名, 若无变量则返回原始值
func parseVariable(context *model.FrameContext, value interface{}) (*interface{}, error) {
	// 假设占位符格式为 ${var_name}，我们去掉 "${" 和 "}" 并返回中间的部分
	switch value.(type) {
	case int:
		result := new(interface{})
		*result = value
		return result, nil
	case string:
		var sValue = value.(string)
		if strings.HasPrefix(sValue, "${") && strings.HasSuffix(sValue, "}") {
			var varName = sValue[2 : len(sValue)-1]
			//fmt.Println(varName)
			// 从 context 中查找对应的变量
			if contextVar, ok := (*context)[varName]; ok {
				// 检查 contextVar 是否非空并且是 *int
				return contextVar, nil
			} else {
				return nil, fmt.Errorf("variable '%s' not found in context", varName)
			}
		}
	}
	return nil, fmt.Errorf("failed to parse int variable: %v", value)
}
