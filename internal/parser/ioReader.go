package parser

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// IoReader 用于解析二进制数据流
type IoReader struct {
	Chunks []Chunk `mapstructure:"chunks"`
	ctx    context.Context
}
type ConditionalChunkConfig struct {
	Type     string                            `mapstructure:"type"`
	Length   int                               `mapstructure:"length"`
	Sections []SectionConfig                   `mapstructure:"sections"`
	Choices  map[string]map[string]interface{} `mapstructure:"choices"`
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
	Tag      string     `mapstructure:"tag"`
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
	Device string   `mapstructure:"device"`
	Fields []string `mapstructure:"fields"`
}

type Section struct {
	Repeat   interface{}
	Bit      int
	Length   int
	Decoding ByteScriptFunc
	Tag      []string
	ToDevice string   // 这里的设备名称是带模板的，需要解析。例如 ecc_{vobc_id}
	ToVars   []string // 解码后变量的最终去向
	ToFields []string // 解码后字段的最终去向
}

// String Section 的 String 方法
func (s *Section) String() string {
	// 打印 Repeat 指针的值
	repeatVal := fmt.Sprintf("%d", s.Repeat)

	// 打印 Decoding 的地址
	decodingAddr := "nil"
	if s.Decoding != nil {
		decodingAddr = fmt.Sprintf("%p", s.Decoding)
	}

	// 打印 ToVarNames 列表
	varNameStr := "["
	for i, pt := range s.ToVars {
		varNameStr += pt
		if i < len(s.ToVars)-1 {
			varNameStr += ", "
		}
	}
	varNameStr += "]"

	// 打印 ToFieldNames 列表
	fieldTargetStr := "["
	for i, field := range s.ToFields {
		fieldTargetStr += field
		if i < len(s.ToFields)-1 {
			fieldTargetStr += ", "
		}
	}
	fieldTargetStr += "]"

	// 拼接 Section 的详细信息
	return fmt.Sprintf(
		"Section: Repeat=%s, Length=%d, Decoding Addr=%s, Device=%s, VarName=%s, Tag=%s, FieldTarget=%s",
		repeatVal, s.Length, decodingAddr, s.ToDevice, varNameStr, s.Tag, fieldTargetStr)
}

// Chunk 处理器接口
type Chunk interface {
	Process(ctx context.Context, dataSource *pkg.StreamDataSource, frame *[]byte, sink *pkg.AggregatorDataSource) (changedCtx context.Context, err error)
	String() string // 添加 String 方法
}

// step.1 注册
func init() {
	Register("ioReader", NewIoReader)
}

func NewIoReader(ctx context.Context) (Template, error) {

	// 1. 初始化杂项配置文件
	v := pkg.ConfigFromContext(ctx)
	var c ioReaderConfig
	err := mapstructure.Decode(v.Parser.Para, &c)
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("配置文件解析失败", zap.Error(err))
		return nil, fmt.Errorf("配置文件解析失败: %s", err)
	}
	// 2. 初始化协议配置文件
	chunksConfig, exist := pkg.ConfigFromContext(ctx).Others[c.ProtoFile]
	if !exist {
		pkg.LoggerFromContext(ctx).Error("未找到协议文件", zap.String("ProtoFile", c.ProtoFile))
		return nil, fmt.Errorf("未找到协议文件:%s", c.ProtoFile)
	}
	pkg.LoggerFromContext(ctx).Debug("协议文件", zap.Any("chunks", chunksConfig))
	chunks, ok := chunksConfig.(map[string]interface{})
	if !ok {
		pkg.LoggerFromContext(ctx).Error("协议文件格式错误", zap.Any("chunks", chunksConfig))
		return nil, fmt.Errorf("协议文件格式错误")
	}
	// 初始化 IoReader
	ioReader, err := createIoParser(ctx, c, chunks["chunks"].([]interface{}))
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("未找到协议文件", zap.Error(err))
		return nil, fmt.Errorf("初始化IoReader失败: %s", err)
	}
	return &ioReader, nil
}

func (r *IoReader) GetType() string {
	return "stream"
}

// Start 方法用于启动 IoReader
func (r *IoReader) Start(source *pkg.DataSource, sink *pkg.AggregatorDataSource) {
	pkg.LoggerFromContext(r.ctx).Info("===IoReader 开始处理数据===")
	dataSource := (*source).(*pkg.StreamDataSource)

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
				ctx, err = chunk.Process(ctx, dataSource, &frame, sink)
				if err != nil {
					if err == io.EOF {
						pkg.LoggerFromContext(r.ctx).Info("读取到 EOF，等待更多数据")
						return // 读取到 EOF 后可以忽略
					}
					pkg.ErrChanFromContext(r.ctx) <- fmt.Errorf("解析第 %d 个 Chunk 失败: %s", index+1, err) // 其他错误，终止连接
					return                                                                             // 解析失败时终止处理
				}

			}
			// ** 此处是完整的一帧的结束 **
			// 通知聚合器可以发送了
			sink.EndChan <- struct{}{}
			count += 1

			// 4.3 打印原始报文
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
	result := "IoReader:"
	for i, chunk := range r.Chunks {
		result += fmt.Sprintf("  Chunk %d: %s", i+1, chunk.String()) // 调用每个 Chunk 的 String 方法
	}
	return result
}

// FixedLengthChunk 实现
type FixedLengthChunk struct {
	Length   interface{} // 为长度或者是变量名
	Sections []Section
}

// 为 FixedLengthChunk 实现 String 方法，打印指针指向的值和指针的地址
func (f *FixedLengthChunk) String() string {
	// 打印 Length 指针的值（解引用）

	lengthVal := fmt.Sprintf("%d", f.Length)

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
		for j, pt := range sec.ToVars {
			varNameStr += pt
			if j < len(sec.ToVars)-1 {
				varNameStr += ", "
			}
		}
		varNameStr += "]"

		// 打印 FieldTarget 列表
		fieldTargetStr := "["
		for j, field := range sec.ToFields {
			fieldTargetStr += field
			if j < len(sec.ToFields)-1 {
				fieldTargetStr += ", "
			}
		}
		fieldTargetStr += "]"

		// 拼接 Section 的详细信息
		sectionsStr += fmt.Sprintf(
			"  Section %d: Repeat=%s, Length=%d, Decoding Addr=%s, Device%s, VarName=%s, FieldTarget=%s",
			i+1, repeatVal, sec.Length, decodingAddr, sec.ToDevice, varNameStr, fieldTargetStr)
	}

	// 打印整个结构体信息
	return fmt.Sprintf("FixedLengthChunk:  Length=%s  Sections:%s", lengthVal, sectionsStr)
}

func (f *FixedLengthChunk) Process(ctx context.Context, dataSource *pkg.StreamDataSource, frame *[]byte, sink *pkg.AggregatorDataSource) (changedCtx context.Context, err error) {
	log := pkg.LoggerFromContext(ctx)
	// ～～～ 定长块的处理逻辑 ～～～
	chunkLen, err := getIntVar(ctx, f.Length)
	if err != nil {
		return ctx, fmt.Errorf("获取FixedLengthChunk长度错误: %v", err)
	}
	// 1. 读取固定长度数据
	data := make([]byte, chunkLen)
	n, err := dataSource.ReadFully(data)
	if err != nil {
		if err == io.EOF {
			// 如果已读取部分数据，可以继续处理
			data = data[:n]
		} else {
			// 其他错误直接返回
			return ctx, fmt.Errorf("读取错误: %v", err)
		}
	}

	// 定长Chunk可以直接追加到frame中
	*frame = append(*frame, data...)
	// 2. 解析数据
	log.Debug("===Processing FixedLengthChunk===")
	log.Debug("FixedLengthChunk", zap.Any("fix", f))
	byteCursor := 0
	for _, sec := range f.Sections {
		var rawPoint *pkg.Point
		// FixLengthChunk 的处理逻辑, 不需要返回 handoff
		ctx, byteCursor, _, rawPoint, err = processSection(ctx, data, sec, byteCursor)
		if err != nil {
			return ctx, err
		}
		if rawPoint != nil {
			sink.PointChan <- *rawPoint
		}

	}
	return ctx, nil
}

// ConditionalChunk 用于处理带条件的 Chunk
type ConditionalChunk struct {
	Length   interface{}
	Handoff  string
	Choices  map[string]Chunk
	Sections []Section
}

func (c *ConditionalChunk) Process(ctx context.Context, dataSource *pkg.StreamDataSource, frame *[]byte, sink *pkg.AggregatorDataSource) (changedCtx context.Context, err error) {
	log := pkg.LoggerFromContext(ctx)
	// ～～～ 定长块的处理逻辑 ～～～
	chunkLen, err := getIntVar(ctx, c.Length)
	if err != nil {
		return ctx, fmt.Errorf("获取ConditionalChunk长度错误: %v", err)
	}
	// 1. 读取固定长度数据
	data := make([]byte, chunkLen)
	n, err := dataSource.ReadFully(data)
	if err != nil {
		if err == io.EOF {
			// 如果已读取部分数据，可以继续处理
			data = data[:n]
		} else {
			// 其他错误直接返回
			return ctx, fmt.Errorf("读取错误: %v", err)
		}
	}

	// 定长Chunk可以直接追加到frame中
	*frame = append(*frame, data...)
	// 2. 解析数据
	log.Debug("===Processing ConditionalChunk===")
	//log.Debug(fmt.Sprintf("frame: %v", frame))
	log.Debug("ConditionalChunk", zap.Any("Condition", c))
	byteCursor := 0
	for _, sec := range c.Sections {
		var rawPoint *pkg.Point
		log.Debug(fmt.Sprintf("Section: %v", sec.String()))
		ctx, byteCursor, c.Handoff, rawPoint, err = processSection(ctx, data, sec, byteCursor)
		if err != nil {
			return ctx, err
		}
		if rawPoint != nil {
			sink.PointChan <- *rawPoint
		}

	}
	// 3. 处理 Handoff
	choices, exist := c.Choices[c.Handoff]
	log.Debug("Going to the Choice", zap.String("Choice", c.Handoff))
	if !exist {
		return ctx, fmt.Errorf("未找到对应的 Choice: %s", c.Handoff)
	}
	ctx, err = choices.Process(ctx, dataSource, frame, sink)
	if err != nil {
		return ctx, err
	}

	return ctx, nil
}

// processSection 处理 Section , 这里的rawPoint是可能返回空的
func processSection(
	ctx context.Context,
	data []byte,
	sec Section,
	byteCursor int,
) (updatedCtx context.Context, updatedCursor int, handoff string, rawPoint *pkg.Point, err error) {
	// 1. 获取 Repeat 值
	repeat, err := getIntVar(ctx, sec.Repeat)
	if err != nil {
		return ctx, byteCursor, handoff, nil, err
	}
	for i := 0; i < repeat; i++ {
		// 2.1. 根据 Sec 的 length 解码
		if sec.Decoding == nil {
			// 如果没有解码函数，直接跳过
			byteCursor += sec.Length
			continue
		}
		// 检查游标是否超出数据长度
		if byteCursor+sec.Length > len(data) {
			return ctx, byteCursor, handoff, nil, fmt.Errorf("游标超出数据长度")
		}
		// 解码
		var decoded []interface{}
		// TODO 在此加入Recover机制, 动态脚本的内容是不可控的
		if sec.Length == 0 {
			log := pkg.LoggerFromContext(ctx)
			log.Debug("解码数据(Length=0)",
				zap.Int("游标位置", byteCursor),
				zap.Binary("数据", data[byteCursor:byteCursor+1]))
			decoded, err = sec.Decoding(data[byteCursor : byteCursor+1])
		} else {
			if byteCursor+sec.Length > len(data) {
				log := pkg.LoggerFromContext(ctx)
				log.Error("游标将超出数据范围",
					zap.Int("游标位置", byteCursor),
					zap.Int("尝试读取长度", sec.Length),
					zap.Int("数据总长度", len(data)))
			}
			log := pkg.LoggerFromContext(ctx)
			log.Debug("解码数据",
				zap.Int("游标位置", byteCursor),
				zap.Int("读取长度", sec.Length),
				zap.Binary("数据", data[byteCursor:byteCursor+sec.Length]))
			decoded, err = sec.Decoding(data[byteCursor : byteCursor+sec.Length])
		}
		if err != nil {
			log := pkg.LoggerFromContext(ctx)
			log.Error("数据解码失败",
				zap.Error(err),
				zap.Int("游标位置", byteCursor),
				zap.Int("读取长度", sec.Length),
				zap.String("Section信息", sec.String()))
			return ctx, byteCursor, handoff, nil, err
		}

		log := pkg.LoggerFromContext(ctx)
		log.Debug("数据解码成功",
			zap.Int("解码后数据长度", len(decoded)),
			zap.Any("解码后数据", decoded))
		// 2.2 移动游标
		byteCursor += sec.Length

		// 2.3 保存解码后的数据到对应的 VarName 下标内
		if len(sec.ToVars) != 0 && len(sec.ToVars) != len(decoded) {
			log := pkg.LoggerFromContext(ctx)
			log.Error("解码数据与变量不匹配",
				zap.Int("解码数据长度", len(decoded)),
				zap.Int("变量名称长度", len(sec.ToVars)),
				zap.Any("解码后数据", decoded),
				zap.Strings("变量名称列表", sec.ToVars),
				zap.Int("当前游标位置", byteCursor),
				zap.String("Section信息", sec.String()))

			return ctx, byteCursor, handoff, nil, fmt.Errorf(
				"解码后的数据长度与 VarNames 长度不匹配, %d != %d, 解码数据: %v, 变量列表: %v",
				len(decoded), len(sec.ToVars), decoded, sec.ToVars,
			)
		}
		// 2.4 处理动态变量
		for ii, pt := range sec.ToVars {
			ctx = context.WithValue(ctx, pt, decoded[ii])
		}
		if len(sec.ToFields) != 0 && len(sec.ToFields) != len(decoded) {
			log := pkg.LoggerFromContext(ctx)
			log.Error("解码数据与字段不匹配",
				zap.Int("解码数据长度", len(decoded)),
				zap.Int("字段名称长度", len(sec.ToFields)),
				zap.Any("解码后数据", decoded),
				zap.Strings("字段名称列表", sec.ToFields),
				zap.Int("当前游标位置", byteCursor),
				zap.String("Section信息", sec.String()))

			return ctx, byteCursor, handoff, nil, fmt.Errorf(
				"解码后的数据长度与 FieldNames 长度不匹配, %d != %d, 解码数据: %v, 字段列表: %v",
				len(decoded), len(sec.ToFields), decoded, sec.ToFields,
			)
		}
		var deviceName string
		// 2.5 更新Choice
		if ContainsTag(sec.Tag, "handoff") {
			handoff = decoded[0].(string)
		}

		if sec.ToDevice == "" {
			continue
		}
		// 2.6 展开变量至 DeviceName
		deviceName, err = sec.parseToDevice(ctx)

		if err != nil {
			return ctx, byteCursor, handoff, nil, err
		}
		// 2.7 组装数据点
		rawPoint = &pkg.Point{
			Device: deviceName,
			Field:  make(map[string]interface{}),
			Ts:     ctx.Value("ts").(time.Time),
		}
		// 2.8 设置数据点的字段
		for ii, de := range decoded {
			rawPoint.Field[sec.ToFields[ii]] = de
		}
	}

	return ctx, byteCursor, handoff, rawPoint, nil
}

func (c *ConditionalChunk) String() string {
	// 打印 Length 的值
	lengthVal := fmt.Sprintf("%v", c.Length)

	// 打印 Handoff 的信息
	handoffStr := ""
	handoffStr += fmt.Sprintf(c.Handoff)

	// 打印 Choices 的信息
	choicesStr := ""
	for key, chunk := range c.Choices {
		choicesStr += fmt.Sprintf("  Choice %s: %v", key, chunk)
	}

	// 打印 Sections 的信息
	sectionsStr := ""
	for i, sec := range c.Sections {
		// 打印 Repeat 指针的值
		repeatVal := fmt.Sprintf("%d", sec.Repeat)

		// 打印 Decoding 的地址
		decodingAddr := "nil"
		if sec.Decoding != nil {
			decodingAddr = fmt.Sprintf("%p", sec.Decoding)
		}

		// 打印 ToVarNames 列表
		varNameStr := "["
		for j, pt := range sec.ToVars {
			varNameStr += pt
			if j < len(sec.ToVars)-1 {
				varNameStr += ", "
			}
		}
		varNameStr += "]"

		// 打印 ToFieldNames 列表
		fieldTargetStr := "["
		for j, field := range sec.ToFields {
			fieldTargetStr += field
			if j < len(sec.ToFields)-1 {
				fieldTargetStr += ", "
			}
		}
		fieldTargetStr += "]"

		// 拼接 Section 的详细信息
		sectionsStr += fmt.Sprintf(
			"  Section %d: Repeat=%s, Length=%d, Decoding Addr=%s, Device%s, VarName=%s, FieldTarget=%s",
			i+1, repeatVal, sec.Length, decodingAddr, sec.ToDevice, varNameStr, fieldTargetStr)
	}

	// 拼接整个 ConditionalChunk 的字符串信息
	return fmt.Sprintf(
		"ConditionalChunk:  Length=%s  Handoff: %s  Choices: %s  Sections: %s",
		lengthVal, handoffStr, choicesStr, sectionsStr)
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
func (s *Section) parseToDevice(context context.Context) (string, error) {
	if s.ToDevice == "" {
		return "", fmt.Errorf("ToDevice 为空")
	}
	// 如果不包含模板变量，直接返回
	if !strings.Contains(s.ToDevice, "${") {
		return s.ToDevice, nil
	}
	// 使用正则表达式提取模板中的变量
	re := regexp.MustCompile(`\${(.*?)}`)
	matches := re.FindAllStringSubmatch(s.ToDevice, -1)

	// 将模板变量替换为 context 中对应的值
	result := s.ToDevice
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
func createIoParser(ctx context.Context, c ioReaderConfig, chunks []interface{}) (IoReader, error) {
	log := pkg.LoggerFromContext(ctx)
	log.Info("当前启用的协议文件", zap.String("protocol", c.ProtoFile))
	var chunkSequence = IoReader{
		make([]Chunk, 0),
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
	log.Debug("IoReader 初始化成功")
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
		// 设置默认值: 若 Repeat 未设置，则设置为 1
		for i, section := range fixedChunkConfig.Sections {
			if section.From.Repeat == nil { // 检查是否为空
				fixedChunkConfig.Sections[i].From.Repeat = 1 // 设置默认值
			}
		}

		// 初始化 FixedLengthChunk(不包含Section)
		var fixedChunk = FixedLengthChunk{
			Length:   fixedChunkConfig.Length,
			Sections: make([]Section, 0),
		}

		// 初始化Section
		for _, section := range fixedChunkConfig.Sections {
			var tmpSec = Section{
				Repeat:   section.From.Repeat,
				Length:   section.From.Byte,
				Decoding: nil,
				ToDevice: section.To.Device,
				ToVars:   section.For.VarName,
				ToFields: make([]string, 0),
				Tag:      strings.Split(section.Tag, ":"),
			}
			for i := 0; i < len(section.To.Fields); i++ {
				fieldValue := section.To.Fields[i]

				// 如果包含模板语法如 "field_{1..8}"
				if strings.Contains(fieldValue, "{") && strings.Contains(fieldValue, "..") && strings.Contains(fieldValue, "}") {
					// 解析模板
					fields, err := expandFieldTemplate(fieldValue)
					if err != nil {
						return nil, fmt.Errorf("解析模板失败: %v", err)
					}
					// 展开结果追加到字段列表
					tmpSec.ToFields = append(tmpSec.ToFields, fields...)
				} else {
					// 普通字段直接追加
					tmpSec.ToFields = append(tmpSec.ToFields, fieldValue)
				}
			}
			if section.Decoding.Method != "" {
				tmpDecoding, exist := GetScriptFunc(section.Decoding.Method)
				if exist {
					tmpSec.Decoding = tmpDecoding
				} else {
					return nil, fmt.Errorf("未找到解码函数: %s", section.Decoding.Method)
				}
			}

			fixedChunk.Sections = append(fixedChunk.Sections, tmpSec)
		}

		// 将 model.FrameContext 指针赋值给 FixedLengthChunk
		return &fixedChunk, nil

	case "ConditionalChunk":
		var conditionalChunkConfig ConditionalChunkConfig
		err := mapstructure.Decode(chunkMap, &conditionalChunkConfig) // 将配置解码为 FixedLengthChunk 结构体
		if err != nil {
			return nil, fmt.Errorf("[createChunk]failed to mapstructure conditionalChunk: %v", err)
		}
		// 设置默认值: 若 Repeat 未设置，则设置为 1
		for i, section := range conditionalChunkConfig.Sections {
			if section.From.Repeat == nil { // 检查是否为空
				conditionalChunkConfig.Sections[i].From.Repeat = 1 // 设置默认值
			}
		}

		// 初始化 ConditionalChunkConfig
		conditionalChunk := ConditionalChunk{
			Length:   conditionalChunkConfig.Length,
			Sections: make([]Section, 0),
			Choices:  make(map[string]Chunk),
		}
		// 将Choice中涉及到的Chunk递归初始化
		for key, value := range conditionalChunkConfig.Choices {
			chunk, err := createChunk(value)
			if err != nil {
				return nil, fmt.Errorf("创建 ConditionalChunk 失败: %v", err)
			}
			conditionalChunk.Choices[key] = chunk
		}
		// 初始化Section
		for _, section := range conditionalChunkConfig.Sections {
			var tmpSec = Section{
				Repeat:   section.From.Repeat,
				Length:   section.From.Byte,
				Decoding: nil,
				ToDevice: section.To.Device,
				ToVars:   section.For.VarName,
				ToFields: make([]string, 0),
				Tag:      strings.Split(section.Tag, ":"),
			}
			for i := 0; i < len(section.To.Fields); i++ {
				fieldValue := section.To.Fields[i]

				// 如果包含模板语法如 "field_{1..8}"
				if strings.Contains(fieldValue, "{") && strings.Contains(fieldValue, "..") && strings.Contains(fieldValue, "}") {
					// 解析模板
					fields, err := expandFieldTemplate(fieldValue)
					if err != nil {
						return nil, fmt.Errorf("解析模板失败: %v", err)
					}
					// 展开结果追加到字段列表
					tmpSec.ToFields = append(tmpSec.ToFields, fields...)
				} else {
					// 普通字段直接追加
					tmpSec.ToFields = append(tmpSec.ToFields, fieldValue)
				}
			}
			if section.Decoding.Method != "" {
				tmpDecoding, exist := GetScriptFunc(section.Decoding.Method)
				if exist {
					tmpSec.Decoding = tmpDecoding
				} else {
					return nil, fmt.Errorf("未找到解码函数: %s", section.Decoding.Method)
				}
			}
			conditionalChunk.Sections = append(conditionalChunk.Sections, tmpSec)
		}
		return &conditionalChunk, nil

	default:
		return nil, fmt.Errorf("unknown chunk type: %s", chunkType)
	}
}

// ContainsTag checks if a slice contains a specific tag.
func ContainsTag(slice []string, tag string) bool {
	for _, v := range slice {
		if v == tag {
			return true
		}
	}
	return false
}
