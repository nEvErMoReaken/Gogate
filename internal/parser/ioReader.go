package parser

import (
	"bufio"
	"context"
	"fmt"
	"gateway/internal/pkg"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type IoReader struct {
	Reader             io.Reader
	Chunks             []Chunk            `mapstructure:"chunks"`
	SnapshotCollection SnapshotCollection // 快照集合
	mapChan            map[string]chan pkg.Point
	ctx                context.Context
}

type FixedChunkConfig struct {
	Type     string          `mapstructure:"type"`
	Length   interface{}     `mapstructure:"length"`
	Sections []SectionConfig `mapstructure:"sections"`
}

type ioReaderConfig struct {
	Dir       string `mapstructure:"dir"`
	ProtoFile string `mapstructure:"protoFile"`
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
	DeviceName string   `mapstructure:"device"`
	DeviceType string   `mapstructure:"type"`
	Fields     []string `mapstructure:"fields"`
}

// Chunk 处理器接口
type Chunk interface {
	Process(reader io.Reader, frame *[]byte, handler *SnapshotCollection, ctx context.Context) (changedCtx context.Context, err error)
	String() string // 添加 String 方法
}

// step.1 注册
func init() {
	Register("ioReader", NewIoReader)
}

func NewIoReader(dataSource pkg.DataSource, mapChan map[string]chan pkg.Point, ctx context.Context) (Parser, error) {
	reader, ok := dataSource.Source.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("数据源类型错误")
	}
	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var c ioReaderConfig
	err := mapstructure.Decode(v.Parser.Para, &c)
	if err != nil {
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	// 2. 初始化协议配置文件
	//chunks := v.Sub(protoFile).Get("chunks").([]interface{})
	chunksConfig, exist := pkg.ConfigFromContext(ctx).Others[c.ProtoFile]
	if !exist {
		return nil, fmt.Errorf("未找到协议文件: %s", c.ProtoFile)
	}
	pkg.LoggerFromContext(ctx).Debug("协议文件", zap.Any("chunks", chunksConfig))
	chunks, ok := chunksConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("协议文件格式错误")
	}
	// 初始化 IoReader
	ioReader, err := createIoParser(c, ctx, chunks["chunks"].([]interface{}), mapChan, reader, dataSource.MetaData)
	if err != nil {
		return nil, fmt.Errorf("初始化IoReader失败: %s", err)
	}
	return &ioReader, nil
}

// Start 方法用于启动 IoReader
func (r *IoReader) Start() {
	pkg.LoggerFromContext(r.ctx).Info("===IoReader 开始处理数据===")
	for {
		count := 0
		select {
		case <-r.ctx.Done():
			return
		default:
			// 4.1 Frame 数组，用于存储一帧原始报文
			frame := make([]byte, 0)
			ctx := r.ctx
			// 绑定默认时间, 协议中有可能覆盖
			ctx = context.WithValue(ctx, "ts", time.Now())
			// ** 此处是完整的一帧的开始 **
			for index, chunk := range r.Chunks {
				var err error
				ctx, err = chunk.Process(r.Reader, &frame, &r.SnapshotCollection, ctx)
				if err != nil {
					if err == io.EOF {
						pkg.LoggerFromContext(r.ctx).Info("读取到 EOF，等待更多数据")
						return // 读取到 EOF 后可以忽略
					}
					pkg.ErrChanFromContext(r.ctx) <- fmt.Errorf("解析第 %d 个 Chunk 失败: %s", index+1, err) // 其他错误，终止连接
					return                                                                             // 解析失败时终止处理
				}
			}
			pkg.LoggerFromContext(r.ctx).Debug("SnapshotCollection status", zap.Any("SnapshotCollection", r.SnapshotCollection))
			// ** 此处是完整的一帧的结束 **
			count += 1
			//fmt.Println("Frame", frame)
			//fmt.Println(r.SnapshotCollection.GetDeviceSnapshot("device1", "type1"))
			// 4.3 发射所有的快照
			r.SnapshotCollection.LaunchALL(ctx, r.mapChan)
			// 4.4 打印原始报文
			hexString := ""
			for _, b := range frame {
				hexString += fmt.Sprintf("%02X", b)
			}
			pkg.LoggerFromContext(r.ctx).Info("Frame",
				zap.String("count", fmt.Sprintf("%06X", count)), // 使用 6 位 16 进制数格式化 count
				zap.String("frame", hexString))                  // frame 转为16进制字符串
		}
	}

}

// IoReader 的 String 方法
func (r *IoReader) String() string {
	result := "IoReader:\n"
	for i, chunk := range r.Chunks {
		result += fmt.Sprintf("  Chunk %d: %s\n", i+1, chunk.String()) // 调用每个 Chunk 的 String 方法
	}
	return result
}

// FixedLengthChunk 实现
type FixedLengthChunk struct {
	length   interface{} // 为长度或者是变量名
	Sections []Section
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

func (f *FixedLengthChunk) Process(reader io.Reader, frame *[]byte, handler *SnapshotCollection, ctx context.Context) (changedCtx context.Context, err error) {
	log := pkg.LoggerFromContext(ctx)
	// ～～～ 定长块的处理逻辑 ～～～
	chunkLen, err := getIntVar(ctx, f.length)
	if err != nil {
		return ctx, fmt.Errorf("获取FixedLengthChunk长度错误: %v", err)
	}
	// 1. 读取固定长度数据
	// 包装 reader 为 bufio.Reader，缓冲读取
	bufReader := bufio.NewReader(reader)
	data := make([]byte, chunkLen)
	n, err := io.ReadFull(bufReader, data) // 使用 bufio.Reader 缓冲读取数据
	if err != nil {
		// 处理 EOF 错误
		if err == io.EOF {
			return ctx, err
		}
		return ctx, fmt.Errorf("读取错误: %v", err)
	}
	// 处理部分读取
	if n < chunkLen {
		return ctx, fmt.Errorf("读取到的数据长度不足: %d < %d", n, chunkLen)
	}
	// 定长Chunk可以直接追加到frame中
	*frame = append(*frame, data...)
	// 2. 解析数据
	log.Debug("===Processing FixedLengthChunk===")
	log.Debug("FixedLengthChunk", zap.Any("fix", f))
	byteCursor := 0
	for _, sec := range f.Sections {
		var parsedToDeviceName string
		var tarSnapshot *DeviceSnapshot
		repeat, err := getIntVar(ctx, sec.Repeat)
		if err != nil {
			return ctx, err
		}
		for i := 0; i < repeat; i++ {

			// 2.1. 根据Sec的length解码
			if sec.Decoding == nil {
				// 如果没有解码函数，直接跳过
				byteCursor += sec.Length
				continue
			}

			if byteCursor+sec.Length > len(data) {
				return ctx, fmt.Errorf("游标超出数据长度")
			}
			var decoded []interface{}
			var err1 error
			// 这样设计的的目的是解决1个字节中有多个设备数据的情况， 即长度为0时不移动游标，而继续解析当前字节
			if sec.Length == 0 {
				decoded, err1 = sec.Decoding(data[byteCursor : byteCursor+1])
			} else {
				decoded, err1 = sec.Decoding(data[byteCursor : byteCursor+sec.Length])
			}

			if err1 != nil {
				return ctx, err1
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
				return ctx, fmt.Errorf("解码后的数据长度与VarNames长度不匹配, %d != %d", len(decoded), len(sec.ToVarNames))
			}
			for i, pt := range sec.ToVarNames {
				// 解码后放入到变量池中
				ctx = context.WithValue(ctx, pt, decoded[i])
			}
			// 2.4 设备快照更新逻辑
			// 注，这里的ToDeviceName是可能包含${}的，需要解析
			if sec.ToDeviceType == "" || sec.ToDeviceName == "" {
				continue
			}
			// 避免重复解析
			if parsedToDeviceName == "" {
				parsedToDeviceName, err = sec.parseToDeviceName(ctx)
				if err != nil {
					return ctx, err
				}
			}

			tarSnapshot, err = handler.GetDeviceSnapshot(parsedToDeviceName, sec.ToDeviceType)
			if err != nil {
				return ctx, err
			}

			if len(sec.ToFieldNames) != 0 && len(sec.ToFieldNames) != len(decoded) {
				return ctx, fmt.Errorf("解码后的数据长度与FieldNames长度不匹配, %d != %d", len(decoded), len(sec.ToFieldNames))
			}
			for ii, de := range decoded {
				//fmt.Println("sec.ToFieldNames[ii]:", sec.ToFieldNames[ii], "de:", de)
				err := tarSnapshot.SetField(ctx, sec.ToFieldNames[ii], de)
				if err != nil {
					return ctx, err
				}
			}

			// data_source对于客户端应该是常驻变量， TODO 后续考虑是否用配置文件配置
			err := tarSnapshot.SetField(ctx, "data_source", ctx.Value("data_source"))
			if err != nil {
				return ctx, err
			}
		}
	}
	//if cursor != len(data) {
	//	common.Log.Warnf("游标未到达数据末尾，有漏数据的风险。游标位置：%d，数据长度：%d", cursor, len(data))
	//}
	return ctx, nil
}

// ConditionalChunk 实现
type ConditionalChunk struct {
	ConditionField string           `mapstructure:"condition_field"`
	Choices        map[string]Chunk `mapstructure:"choices"`
	Sections       []Section
}

func (c *ConditionalChunk) Process(reader io.Reader, frame *[]byte, handler *SnapshotCollection, ctx context.Context) (changedCtx context.Context, err error) {
	fmt.Println("Processing ConditionalChunk")
	// 打印一下所有字段 避免sonar检测
	fmt.Println(reader, frame, handler, ctx)
	// 动态选择下一个 Chunk 解析逻辑
	return ctx, nil
}

func (c *ConditionalChunk) String() string {
	return fmt.Sprintf("ConditionalChunk (ConditionField: %s, Choices: %d)", c.ConditionField, len(c.Choices))
}

type Section struct {
	Repeat       interface{}
	Bit          int
	Length       int
	Decoding     ByteScriptFunc
	ToDeviceName string // 这里的设备名称是带模板的，需要解析。例如 ecc_{vobc_id}
	ToDeviceType string
	ToVarNames   []string // 解码后变量的最终去向
	ToFieldNames []string // 解码后字段的最终去向
}

func getIntVar(ctx context.Context, key interface{}) (int, error) {
	switch key.(type) {
	case int:
		return key.(int), nil
	case string:
		res := ctx.Value(key.(string))
		if res == nil {
			return 0, fmt.Errorf("未找到变量 %+v", key)
		}
		t, ok := res.(int)
		if !ok {
			return 0, fmt.Errorf("变量 %+v 类型错误", key)
		}
		return t, nil
	}
	return 0, fmt.Errorf("未知类型")
}

// 解析 ToDeviceName 中的模板变量
func (s *Section) parseToDeviceName(context context.Context) (string, error) {
	// 如果不包含模板变量，直接返回
	if !strings.Contains(s.ToDeviceName, "${") {
		return s.ToDeviceName, nil
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
			contextVar := context.Value(templateVar)
			if contextVar != nil {
				// 替换模板中的变量
				result = strings.Replace(result, "${"+templateVar+"}", contextVar.(string), -1)
			} else {
				// 如果没有找到变量，可以考虑报错或使用默认值
				return "", fmt.Errorf("未找到模板变量: %s", templateVar)
			}
		}
	}

	return result, nil
}

// createIoParser 从配置文件初始化 Chunk
func createIoParser(c ioReaderConfig, ctx context.Context, chunks []interface{}, mapChan map[string]chan pkg.Point, reader io.Reader, metadata map[string]interface{}) (IoReader, error) {
	log := pkg.LoggerFromContext(ctx)
	log.Info("当前启用的协议文件: %s", zap.String("protocol", c.ProtoFile))
	// 初始化 SnapshotCollection
	ctx = context.WithValue(ctx, "deviceId", metadata["deviceId"])
	ctx = context.WithValue(ctx, "ts", metadata["ts"])
	var chunkSequence = IoReader{
		reader,
		make([]Chunk, 0),
		make(SnapshotCollection),
		mapChan,
		ctx,
	}

	for _, chunk := range chunks {
		// 动态处理不同的 chunkType，生成chunkSequence
		tmpChunk, err := createChunk(chunk.(map[string]interface{}))
		if err != nil {
			return chunkSequence, err
		}

		chunkSequence.Chunks = append(chunkSequence.Chunks, tmpChunk)
	}
	log.Info("IoReader 初始化成功:\n %+v", zap.Any("IoReader", chunkSequence))
	return chunkSequence, nil
}

// 解析类似 "efef_{1..8}" 范围并展开
func expandFieldTemplate(template string) ([]string, error) {
	// 使用正则表达式匹配 "{a..b}" 的范围
	re := regexp.MustCompile(`\{(\d+)\.\.(\d+)}`) // 匹配 ${a..b} 的范围
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

	// 去掉 ${} 部分，提取前缀部分 (例如 "field")
	prefix := template[:strings.Index(template, "{")]

	// 生成字段名称数组
	result := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, fmt.Sprintf("%s%d", prefix, i)) // 拼接前缀和数字
	}

	return result, nil
}

// createChunk 根据 chunkType 动态创建对应的 Chunk
func createChunk(chunkMap map[string]interface{}) (Chunk, error) {
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

			tmpDecoding, exist := GetScriptFunc(section.Decoding.Method)
			if exist {
				tmpSec.Decoding = tmpDecoding
			} else {
				return nil, fmt.Errorf("未找到解码函数: %s", section.Decoding.Method)
			}

			fixedChunk.Sections = append(fixedChunk.Sections, tmpSec)
		}

		// 将 model.FrameContext 指针赋值给 FixedLengthChunk
		return &fixedChunk, nil

	case "ConditionalChunk":
		var conditionalChunk ConditionalChunk
		// TODO: 解析 ConditionalChunk
		return &conditionalChunk, nil

	default:
		return nil, fmt.Errorf("unknown chunk type: %s", chunkType)
	}
}

// @Desperate parseVariable 从字符串中提取变量名, 若无变量则返回原始值
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
