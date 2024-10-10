package ioReader

import (
	"fmt"
	"gateway/internal/pkg"
	"gateway/logger"
	"gateway/util"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Chunk 处理器接口
type Chunk interface {
	Process(reader io.Reader, frame *[]byte, collection *pkg.SnapshotCollection, config *pkg.Config) error
	String() string // 添加 String 方法
}

// FrameContext 每一帧中, 也就是多Chunks共享的上下文
type FrameContext map[string]interface{}

type ChunkSequence struct {
	Chunks             []Chunk `mapstructure:"chunks"`
	VarPointer         FrameContext
	SnapShotCollection *pkg.SnapshotCollection // 快照集合
}

// ProcessAll 方法：处理整个 ChunkSequence
func (c *ChunkSequence) ProcessAll(deviceId string, reader io.Reader, frame *[]byte, config *pkg.Config) error {
	// 初始化一些变量
	// deviceId
	c.VarPointer["deviceId"] = deviceId

	// ts
	c.VarPointer["ts"] = time.Now()
	// 处理每一个 Chunk
	for index, chunk := range c.Chunks {
		err := chunk.Process(reader, frame, c.SnapShotCollection, config)
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
	length   interface{} // 为长度或者是变量名
	Sections []Section
	VarPool  *FrameContext
}

// CalLength 方法用于加载长度
func (f *FixedLengthChunk) CalLength() int {
	switch f.length.(type) {
	case int:
		return f.length.(int)
	case string:
		return (*f.VarPool)[f.length.(string)].(int)
	}
	return 0 // 默认为0
}

// 为 FixedLengthChunk 实现 String 方法，打印指针指向的值和指针的地址
func (f *FixedLengthChunk) String() string {
	// 打印 Length 指针的值（解引用）

	lengthVal := fmt.Sprintf("%d", f.length)

	// 打印 Sections 指针中的值
	sectionsStr := ""
	for i, sec := range f.Sections {
		// 打印 Repeat 指针的值和地址

		repeatVal := fmt.Sprintf("%d", sec.Repeat)

		// 打印 Decoding 指针的地址
		decodingAddr := "nil"
		if sec.Decoding != nil {
			decodingAddr = fmt.Sprintf("%p", sec.Decoding) // 打印地址
		}

		// 打印 ForFields 列表
		varNameStr := "["
		for j, pt := range sec.ToVarNames {
			varNameStr += pt
			if j < len(sec.ToVarNames)-1 {
				varNameStr += ", "
			}
		}
		varNameStr += "]"

		// 打印 FieldTarget 列表
		fieldTargetStr := "["
		for j, field := range sec.ToFieldNames {
			fieldTargetStr += field
			if j < len(sec.ToFieldNames)-1 {
				fieldTargetStr += ", "
			}
		}
		fieldTargetStr += "]"

		// 拼接 Section 的详细信息
		sectionsStr += fmt.Sprintf(
			"  Section %d: Repeat=%s, Length=%d, Decoding Addr=%s, DeviceName=%s, DeviceType=%s, VarName=%s, FieldTarget=%s\n",
			i+1, repeatVal, sec.Length, decodingAddr, sec.ToDeviceName, sec.ToDeviceType, varNameStr, fieldTargetStr)
	}

	// 打印整个结构体信息
	return fmt.Sprintf("FixedLengthChunk:\n  Length=%s\n  Sections:\n%s", lengthVal, sectionsStr)
}

func (f *FixedLengthChunk) Process(reader io.Reader, frame *[]byte, collection *pkg.SnapshotCollection, config *pkg.Config) error {
	// ～～～ 定长块的处理逻辑 ～～～
	chunkLen := f.CalLength()
	// 1. 读取固定长度数据
	data := make([]byte, chunkLen)
	n, err := io.ReadFull(reader, data)
	if err != nil {
		// 处理 EOF 错误
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("读取错误: %v", err)
	}
	// 处理部分读取
	if n < f.CalLength() {
		logger.Log.Warnf("只读取了 %d 字节，而不是期望的 %d 字节", n, chunkLen)
	}
	// 定长Chunk可以直接追加到frame中
	*frame = append(*frame, data...)
	// 2. 解析数据
	logger.Log.Debugf("Processing FixedLengthChunk")
	byteCursor := 0
	for index, sec := range f.Sections {
		var parsedToDeviceName string
		var tarSnapshot *pkg.DeviceSnapshot

		for i := 0; i < sec.CalRepeat(f.VarPool); i++ {
			// 2.1. 根据Sec的length解码
			if sec.Decoding == nil {
				logger.Log.Debugf("Step.%+v: Loop.%+v: Jump For %+v Byte", index+1, i+1, sec.Length)
				// 如果没有解码函数，直接跳过
				byteCursor += sec.Length
				continue
			}
			if byteCursor+sec.Length > len(data) {
				return fmt.Errorf("游标超出数据长度")
			}
			var decoded []interface{}
			var err1 error
			// 这样设计的的目的是解决1个字节中有多个设备数据的情况， 即长度为0时不移动游标，而继续解析当前字节
			if sec.Length == 0 {
				decoded, err1 = sec.Decoding(data[byteCursor : byteCursor+1])
			} else {
				decoded, err1 = sec.Decoding(data[byteCursor : byteCursor+sec.Length])
			}

			logger.Log.Debugf("Step.%+v: Loop.%+v: Decoded: %v Byte to %+v", index+1, i+1, sec.Length, decoded)
			if err1 != nil {
				return err1
			}
			// 2.2 移动游标
			byteCursor += sec.Length

			// 2.3 保存解码后的数据到对应的 VarName下标内
			// sec.VarName[i] = decoded[i]
			// 假设解码后有三个速度值： v1,v2,v3 分别对应变量名为 vobc_speed1, vobc_speed2, vobc_speed3
			// v1 -> vobc_speed1,
			// v2 -> vobc_speed2,
			// v3 -> vobc_speed3
			// 变量后续用于For(如果有的话）, 供后续Section使用
			if len(sec.ToVarNames) != 0 && len(sec.ToVarNames) != len(decoded) {
				return fmt.Errorf("解码后的数据长度与VarNames长度不匹配, %d != %d", len(decoded), len(sec.ToVarNames))
			}
			for i, pt := range sec.ToVarNames {
				// 解码后放入到变量池中
				(*f.VarPool)[pt] = decoded[i]
			}
			// 2.4 设备快照更新逻辑
			// 注，这里的ToDeviceName是可能包含${}的，需要解析
			if sec.ToDeviceType == "" || sec.ToDeviceName == "" {
				continue
			}
			// 避免重复解析
			if parsedToDeviceName == "" {
				parsedToDeviceName = sec.parseToDeviceName(f.VarPool)
			}
			if tarSnapshot == nil {
				tarSnapshot = collection.GetDeviceSnapshot(parsedToDeviceName, sec.ToDeviceType)
			}

			if len(sec.ToFieldNames) != 0 && len(sec.ToFieldNames) != len(decoded) {
				return fmt.Errorf("解码后的数据长度与FieldNames长度不匹配, %d != %d", len(decoded), len(sec.ToFieldNames))
			}
			for ii, de := range decoded {
				tarSnapshot.SetField(sec.ToFieldNames[ii], de, config)
			}
			tarSnapshot.Ts = (*f.VarPool)["ts"].(time.Time)
			// data_source对于客户端应该是常驻变量， TODO 后续考虑是否用配置文件配置
			tarSnapshot.SetField("data_source", (*f.VarPool)["data_source"], config)
		}
	}
	//if cursor != len(data) {
	//	common.Log.Warnf("游标未到达数据末尾，有漏数据的风险。游标位置：%d，数据长度：%d", cursor, len(data))
	//}
	return nil
}

// ConditionalChunk 实现
type ConditionalChunk struct {
	ConditionField string           `mapstructure:"condition_field"`
	Choices        map[string]Chunk `mapstructure:"choices"`
	VarPointer     *FrameContext
	Sections       []Section
}

func (c *ConditionalChunk) Process(reader io.Reader, frame *[]byte, collection *pkg.SnapshotCollection, config *pkg.Config) error {
	fmt.Println("Processing ConditionalChunk")
	// 动态选择下一个 Chunk 解析逻辑
	return nil
}

func (c *ConditionalChunk) String() string {
	return fmt.Sprintf("ConditionalChunk (ConditionField: %s, Choices: %d)", c.ConditionField, len(c.Choices))
}

type Section struct {
	Repeat       interface{}
	Bit          int
	Length       int
	Decoding     util.ByteScriptFunc
	ToDeviceName string // 这里的设备名称是带模板的，需要解析。例如 ecc_{vobc_id}
	ToDeviceType string
	ToVarNames   []string // 解码后变量的最终去向
	ToFieldNames []string // 解码后字段的最终去向
}

// CalRepeat 方法用于加载重复次数
func (s *Section) CalRepeat(ctx *FrameContext) int {
	switch s.Repeat.(type) {
	case int:
		return s.Repeat.(int)
	case string:
		return (*ctx)[s.Repeat.(string)].(int)
	}
	return 0 // 默认为0
}

// 解析 ToDeviceName 中的模板变量
func (s *Section) parseToDeviceName(context *FrameContext) string {
	// 如果不包含模板变量，直接返回
	if !strings.Contains(s.ToDeviceName, "${") {
		return s.ToDeviceName
	}
	// 使用正则表达式提取模板中的变量
	re := regexp.MustCompile(`\${(.*?)}`)
	matches := re.FindAllStringSubmatch(s.ToDeviceName, -1)

	// 将模板变量替换为 context 中对应的值
	result := s.ToDeviceName
	for _, match := range matches {
		if len(match) > 1 {
			// match[1] 是模板中的变量名
			templateVar := match[1]
			// 从 context 中查找变量的值
			contextVar := (*context)[templateVar]
			if contextVar != nil {
				// 替换模板中的变量
				result = strings.Replace(result, "${"+templateVar+"}", contextVar.(string), -1)
			} else {
				// 如果没有找到变量，可以考虑报错或使用默认值
				logger.Log.Errorf("模板变量 %s 未找到", templateVar)
			}
		}
	}

	return result
}

// InitChunks 从配置文件初始化 Chunk
func InitChunks(v *viper.Viper, protoFile string) (ChunkSequence, error) {
	logger.Log.Infof("当前启用的协议文件: %s", protoFile)
	// 初始化 SnapshotCollection
	snapshotCollection := make(pkg.SnapshotCollection)
	var chunkSequence = ChunkSequence{
		make([]Chunk, 0),
		make(FrameContext),
		&snapshotCollection,
	}
	chunks := v.Sub(protoFile).Get("chunks").([]interface{})
	for _, chunk := range chunks {
		// 动态处理不同的 chunkType，生成chunkSequence
		tmpChunk, err := createChunk(chunk.(map[string]interface{}), &chunkSequence.VarPointer)
		if err != nil {
			return chunkSequence, err
		}

		chunkSequence.Chunks = append(chunkSequence.Chunks, tmpChunk)
	}
	logger.Log.Infof("ChunkSequence 初始化成功:\n %+v", chunkSequence)
	return chunkSequence, nil
}

// 解析类似 "efef_{1..8}" 范围并展开
func expandFieldTemplate(template string) ([]string, error) {
	// 使用正则表达式匹配 "{a..b}" 的范围
	re := regexp.MustCompile(`\{(\d+)\.\.(\d+)}`) // 匹配大括号里的范围
	matches := re.FindStringSubmatch(template)
	if len(matches) != 3 {
		return nil, fmt.Errorf("无法解析模板: %s", template)
	}

	// 提取起始和结束范围
	start, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("解析起始数字错误: %v", err)
	}
	end, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("解析结束数字错误: %v", err)
	}

	// 检查范围有效性
	if start > end {
		return nil, fmt.Errorf("起始数字不能大于结束数字: %d..%d", start, end)
	}

	// 提取前缀部分 (例如 "efef_")
	prefix := template[:strings.Index(template, "{")]

	// 生成字段名称数组
	result := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, fmt.Sprintf("%s%d", prefix, i)) // 拼接前缀和数字
	}

	return result, nil
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
		//fmt.Println()
		// 设置默认值: 若 Repeat 未设置，则设置为 1
		for i, section := range fixedChunkConfig.Sections {
			if section.From.Repeat == nil { // 检查是否为空
				fixedChunkConfig.Sections[i].From.Repeat = 1 // 设置默认值
			}
		}

		// 初始化 FixedLengthChunk(不包含Section)
		var fixedChunk = FixedLengthChunk{
			length:   fixedChunkConfig.Length,
			Sections: make([]Section, 0),
			VarPool:  context,
		}
		// 初始化Section
		for _, section := range fixedChunkConfig.Sections {
			var tmpSec = Section{
				Repeat:       section.From.Repeat,
				Length:       section.From.Byte,
				Decoding:     nil,
				ToDeviceName: section.To.DeviceName,
				ToDeviceType: section.To.DeviceType,
				ToVarNames:   section.For.VarName,
				ToFieldNames: make([]string, 0),
			}
			for i := 0; i < len(section.To.Fields); i++ {
				// 如果 section.To.Fields 中是 "field${1..8}" 形式
				if len(section.To.Fields) != 0 && strings.Contains(section.To.Fields[0], "{") && strings.Contains(section.To.Fields[0], "..") && strings.Contains(section.To.Fields[0], "}") {
					// 解析模板 "field${1..8}"
					fields, err := expandFieldTemplate(section.To.Fields[0])
					if err != nil {
						return nil, fmt.Errorf("解析模板失败: %v\n", err)
					}
					// 展开结果追加 FieldTarget
					tmpSec.ToFieldNames = append(tmpSec.ToFieldNames, fields...)
				} else {
					// 否则直接追加
					tmpSec.ToFieldNames = append(tmpSec.ToFieldNames, section.To.Fields[i])
				}
			}

			tmpDecoding, exist := util.GetScriptFunc(section.Decoding.Method)
			if exist {
				tmpSec.Decoding = tmpDecoding
			}

			fixedChunk.Sections = append(fixedChunk.Sections, tmpSec)
		}

		// 将 model.FrameContext 指针赋值给 FixedLengthChunk
		fixedChunk.VarPool = context
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
func parseVariable(value interface{}) (isVariable bool, Variable string, err error) {
	// 假设占位符格式为 ${var_name}，我们去掉 "${" 和 "}" 并返回中间的部分
	switch value.(type) {
	case int:
		return false, "", nil
	case string:
		var sValue = value.(string)
		if strings.HasPrefix(sValue, "${") && strings.HasSuffix(sValue, "}") {
			return true, sValue[2 : len(sValue)-1], nil
		} else {
			return true, "", fmt.Errorf("failed to parse int variable: %v", value)
		}
	}
	return true, "", fmt.Errorf("UnKnown Type: %v", value)
}
