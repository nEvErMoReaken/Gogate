package parser

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

// Section 定义了一个数据处理的基本单元，可处理固定大小的字节段。
// 支持表达式解析，可提取字段(Fields)和变量(Vars)。
// 新增: 支持 multi_dev，允许对同一字节块应用多个解析规则。
type Section struct {
	// --- 外部映射字段 ---
	// Desc 描述该 Section 的功能，用于调试和日志记录
	Desc string `mapstructure:"desc"`
	// Size 指定该 Section 处理的字节数
	Size int `mapstructure:"size"`
	// Points 表达式
	PointsExpression []PointExpression `mapstructure:"Points"`
	// Vars 定义变量表达式映射，用于 V() 调用设置变量
	Var map[string]any `mapstructure:"Vars"`
	// Label 指定该 Section 的标签，用于跳转
	Label string `mapstructure:"Label"`
	// NextRules 指定该 Section 的下一个 Section，用于标识和分类,改名避免与方法重复
	NextRules []Rule `mapstructure:"Next"`
	// --- 内部字段 ---
	index   int         // 当前 Section 的索引
	Program *vm.Program // 存储本节点编译后的表达式
}

// PointExpression 定义点表达式
// 格式如下：
//
//	"Points": {
//	  "Tag": {
//	    "tag1": "Bytes[0]",
//	    "tag2": "Bytes[1]"
//	  },
//	  "Field": {
//	    "field1": "Bytes[2]",
//	    "field2": "Bytes[3]"
//	  }
//	}
type PointExpression struct {
	Tag   map[string]string `mapstructure:"Tag"`
	Field map[string]string `mapstructure:"Field"`
}

type Rule struct {
	Condition string `mapstructure:"condition"`
	Target    string `mapstructure:"target"`
	Program   *vm.Program
}

type Skip struct {
	Skip  int `mapstructure:"skip"`
	index int // 当前 Section 的索引
}

func (s *Skip) ProcessWithBytes(ctx context.Context, state *ByteState) (BProcessor, error) {
	// I. 检查数据是否足够
	end := state.Cursor + s.Skip
	if end > len(state.Data) {
		// 返回原始 out，因为没有修改
		return nil, fmt.Errorf("数据不足，需要 %d 字节 (cursor: %d, end: %d, total: %d)",
			s.Skip, state.Cursor, end, len(state.Data))
	}
	// II. 移动光标
	state.Cursor = end
	next, err := s.Route(ctx, state.Nodes)
	// 返回原始 out 和 next 节点/错误
	return next, err
}

func (s *Skip) ProcessWithRing(ctx context.Context, state *StreamState) (BProcessor, error) {

	rawPlace := pkg.ByteCache.Get(uint32(s.Skip))
	err := state.ring.ReadFull(rawPlace)
	if err != nil {
		return nil, fmt.Errorf("从 ring buffer 读取 %d 字节失败: %w", s.Skip, err)
	}
	next, err := s.Route(ctx, state.Nodes)
	return next, err
}

func (s *Skip) Route(ctx context.Context, nodes []BProcessor) (BProcessor, error) {
	if s.index+1 >= len(nodes) {
		return nil, nil
	}
	return nodes[s.index+1], nil

}

func (s *Skip) String() string {
	return fmt.Sprintf("Skip: Skip: %d", s.Skip)
}

// BuildSequence 根据配置创建 Section，并编译其表达式。
// 输入:
//   - configList: 配置列表，每个元素可以是 Skip 或 Section 配置
//
// 输出:
//   - BProcessor: 创建的 BProcessor 链表头节点
//   - error: 创建过程中遇到的错误
//
// BuildSequence 根据配置创建 BProcessor 链表，处理标签和跳转。
func BuildSequence(configList []map[string]any) ([]BProcessor, map[string]int, error) {
	if len(configList) == 0 {
		return nil, nil, nil // 空配置列表，返回 nil 头节点
	}

	nodes := make([]BProcessor, 0, len(configList)) // 存储创建的节点
	labelMap := make(map[string]int)                // 存储标签到索引的映射

	// ---  创建节点并建立标签映射 ---
	for index, config := range configList {
		var newNode BProcessor

		// 检查是否为 Skip 节点
		if skipValAny, ok := config["skip"]; ok {
			var skipIntVal int
			// var err error // err is declared but not used if only type switch is used for assignment

			switch v := skipValAny.(type) {
			case float64:
				// 检查 float64 是否为整数
				if v != float64(int(v)) {
					return nil, nil, fmt.Errorf("skip 值 %.2f 不是一个有效的整数", v)
				}
				skipIntVal = int(v)
			case int:
				skipIntVal = v
			case string: // 保留对字符串形式数字的支持，以防万一，但优先处理数字类型
				parsedVal, parseErr := strconv.Atoi(v)
				if parseErr != nil {
					return nil, nil, fmt.Errorf("skip 值 '%s' 无法转换为整数: %w", v, parseErr)
				}
				skipIntVal = parsedVal
			default:
				return nil, nil, fmt.Errorf("skip 值类型无效: %T, 期望 int, float64 或 string 类型的数字", skipValAny)
			}

			// +++ 添加对 skip 值的校验 +++
			if skipIntVal <= 0 {
				// 考虑 skip 是否允许为0或负数，如果 skip 代表要跳过的字节数，通常应为正数。
				// 根据实际需求调整此校验。假设 skip 必须大于 0。
				return nil, nil, fmt.Errorf("skip 值必须大于 0, 当前值: %d", skipIntVal)
			}
			// +++++++++++++++++++++++

			skipNode := &Skip{
				Skip:  skipIntVal,
				index: index,
				// next 在第二阶段设置
			}
			newNode = skipNode

		} else { // 否则认为是 Section 节点
			var tmpSec Section
			// 使用自定义解码配置来处理标签大小写不敏感或特定映射
			decoderConfig := &mapstructure.DecoderConfig{
				Metadata: nil,
				Result:   &tmpSec,
				TagName:  "mapstructure",
				DecodeHook: mapstructure.ComposeDecodeHookFunc(
					mapstructure.StringToTimeDurationHookFunc(),
					mapstructure.StringToSliceHookFunc(","),
					// 可以添加其他 hook
				),
				// WeaklyTypedInput: true, // 如果需要更宽松的类型转换
			}
			decoder, err := mapstructure.NewDecoder(decoderConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("创建解码器失败: %w", err)
			}

			err = decoder.Decode(config)
			if err != nil {
				desc, _ := config["desc"].(string)
				return nil, nil, fmt.Errorf("解码 Section %d (Desc: %s) 失败: %w", index, desc, err)
			}

			// +++ 添加对 Size 的校验 +++
			if tmpSec.Size <= 0 {
				desc, _ := config["desc"].(string)
				return nil, nil, fmt.Errorf("Section %d (Desc: %s) 配置错误: 'size' 必须大于 0, 实际为 %d", index, desc, tmpSec.Size)
			}
			// +++++++++++++++++++++++

			// --- 修改：调用新的编译函数 ---
			tmpSec.Program, err = CompileSectionProgram(tmpSec.PointsExpression, tmpSec.Var)
			if err != nil {
				return nil, nil, fmt.Errorf("编译 Section %d (Desc: %s) 的 Vars 失败: %w", index, tmpSec.Desc, err)
			}

			err = CompileNextRoute(tmpSec.NextRules)
			if err != nil {
				return nil, nil, fmt.Errorf("编译 Section %d (Desc: %s) 的 Next 失败: %w", index, tmpSec.Desc, err)
			}

			sectionNode := &tmpSec // 创建指针
			sectionNode.index = index
			newNode = sectionNode

			// 如果有标签，添加到 labelMap
			if tmpSec.Label != "" {
				if _, exists := labelMap[tmpSec.Label]; exists {
					return nil, nil, fmt.Errorf("标签 '%s' 在 Section %d (Desc: %s) 处重复定义", tmpSec.Label, index, tmpSec.Desc)
				}
				labelMap[tmpSec.Label] = index
			}
		}

		nodes = append(nodes, newNode)
	}

	return nodes, labelMap, nil // 如果 configList 为空，nodes 也为空
}

// BProcessor 接口定义了基本的处理单元
type BProcessor interface {
	// ProcessWithBytes 使用离散字节数组处理数据
	// 返回修改后的输出切片、下一个要处理的 BProcessor 和错误
	ProcessWithBytes(ctx context.Context, state *ByteState) (BProcessor, error)
	// ProcessWithRing 使用 RingBuffer 流式处理数据
	// 返回修改后的输出切片、下一个要处理的 BProcessor 和错误
	ProcessWithRing(ctx context.Context, state *StreamState) (BProcessor, error)
	// String 返回处理器的字符串表示
	String() string
}

/* ---------- Chunk 的定义 ---------- */

func (s *Section) String() string {
	return fmt.Sprintf("Section: Desc: %s, Size: %d", s.Desc, s.Size)
}

func (s *Section) ProcessWithRing(ctx context.Context, state *StreamState) (BProcessor, error) {

	// ----校验，环境准备----
	if s.Size <= 0 {
		return nil, fmt.Errorf("Section 大小必须大于0, 当前大小: %d", s.Size)
	}
	// I. 使用 ByteCache 获取预热或新建的 buffer
	// Get 方法保证返回的 slice 长度等于 s.Size，无需额外检查
	rawData := pkg.ByteCache.Get(uint32(s.Size))

	_, err := state.ring.Read(rawData)
	if err != nil {
		// 无需放回 ByteCache
		return nil, fmt.Errorf("从 ring buffer 读取 %d 字节失败: %w", s.Size, err)
	}

	state.Env.Bytes = rawData

	_, err = expr.Run(s.Program, state.Env)
	if err != nil {
		return nil, fmt.Errorf("执行表达式失败: %w", err)
	}

	// ----- 路由 ----
	next, err := s.Route(ctx, state.Env, state.LabelMap, state.Nodes)
	return next, err
}

// 定义 NextType 类型
const (
	DEFAULT int = -99 // 改为一个明显的负值，避免与任何可能的节点索引冲突
	END     int = -1
	WRONG   int = -2
)

// Route 根据环境变量和标签映射表，确定下一个要执行的节点, 并设置 s.next
// 输入:
//   - env: 环境变量
//   - labelMap: 标签到索引的映射表
//   - nodes: 节点列表
//
// 输出:
//   - error: 路由过程中遇到的错误，成功时为 nil
func (s *Section) Route(ctx context.Context, env *BEnv, labelMap map[string]int, nodes []BProcessor) (BProcessor, error) {
	log := pkg.LoggerFromContext(ctx)
	var nextIndex = WRONG  // 使用 nextIndex 避免与 BProcessor next 混淆
	var targetLabel string // 存储匹配规则的目标标签

	log.Debug("节点开始路由评估", zap.Int("index", s.index), zap.String("desc", s.Desc))
	// 打印关键变量值
	if loopCount, ok := env.Vars["loop_count"]; ok {
		log.Debug("loop_count", zap.Any("value", loopCount), zap.Any("type", reflect.TypeOf(loopCount)))
	}
	if loopIndex, ok := env.Vars["loop_index"]; ok {
		log.Debug("loop_index", zap.Any("value", loopIndex), zap.Any("type", reflect.TypeOf(loopIndex)))
	}

	if len(s.NextRules) == 0 {
		nextIndex = DEFAULT // 默认下一个节点
		log.Debug("节点没有 NextRules, 使用默认路由 (下一节点)")
	}

	for i, rule := range s.NextRules {
		log.Debug("评估规则", zap.Int("index", i), zap.String("condition", rule.Condition), zap.String("target", rule.Target))

		if rule.Program == nil {
			// 在 Route 阶段增加对未编译规则的检查可能更健壮
			log.Error("规则条件未编译", zap.String("condition", rule.Condition), zap.String("target", rule.Target))
			return nil, fmt.Errorf("路由规则条件未编译 (Condition: %s, Target: %s)", rule.Condition, rule.Target)
		}

		isMatch, err := expr.Run(rule.Program, env)
		if err != nil {
			log.Error("执行条件表达式失败", zap.String("condition", rule.Condition), zap.String("target", rule.Target), zap.Error(err))
			return nil, fmt.Errorf("执行路由条件表达式失败 (Condition: %s): %w", rule.Condition, err)
		}

		log.Debug("条件评估结果", zap.String("condition", rule.Condition), zap.Any("result", isMatch), zap.Any("type", reflect.TypeOf(isMatch)))

		if matched, ok := isMatch.(bool); ok && matched {
			log.Debug("条件匹配成功")
			if rule.Target == "END" {
				nextIndex = END
				log.Debug("目标为END, 将结束处理")
			} else if rule.Target == "DEFAULT" {
				// 如果目标标签为 DEFAULT，则设置为默认下一个节点
				nextIndex = DEFAULT
				log.Debug("目标为DEFAULT, 将使用默认路由 (下一节点)")
			} else {
				// 查找目标标签对应的索引
				targetIdx, exists := labelMap[rule.Target]
				if !exists {
					log.Error("目标标签未找到", zap.String("target", rule.Target))
					return nil, fmt.Errorf("路由目标标签 '%s' 在标签映射中未找到", rule.Target)
				}
				nextIndex = targetIdx
				targetLabel = rule.Target // 记录找到的标签
				log.Debug("目标标签映射到节点索引", zap.String("target", rule.Target), zap.Int("index", targetIdx))

			}

			// 修改：找到第一个匹配的规则后立即跳出循环，不再评估后续规则
			log.Debug("找到匹配规则，不再评估后续规则")
			break
		} else if !ok {
			// 如果表达式结果不是布尔值，也视为错误
			log.Error("条件表达式结果非布尔值", zap.Any("result", isMatch), zap.Any("type", reflect.TypeOf(isMatch)))
			return nil, fmt.Errorf("路由条件表达式结果非布尔值 (Condition: %s, ResultType: %T)", rule.Condition, isMatch)
		} else {
			log.Debug("条件不匹配")
		}
	}

	switch nextIndex {
	case DEFAULT:
		// Default 默认下一个节点
		nextDefaultIndex := s.index + 1
		log.Debug("计算默认路由目标索引", zap.Int("current_index", s.index), zap.Int("next_index", nextDefaultIndex))

		// 处理边界情况 - 当DEFAULT意味着到达节点末尾时
		if nextDefaultIndex >= len(nodes) {
			log.Debug("已到达节点末尾，处理结束")
			return nil, nil // 到达末尾
		}

		log.Debug("使用默认路由到下一节点", zap.Int("index", nextDefaultIndex))
		return nodes[nextDefaultIndex], nil
	case END:
		log.Debug("明确结束处理")
		return nil, nil // 显式结束
	case WRONG:
		// 存在规则但无一匹配
		// --- 修改：添加 env.Vars 和 s.NextRules 的格式化输出 ---
		// Format Vars for better readability
		var varsStrBuilder strings.Builder
		varsStrBuilder.WriteString("{")
		varsCount := 0
		for k, v := range env.Vars {
			if varsCount > 0 {
				varsStrBuilder.WriteString(", ")
			}
			varsStrBuilder.WriteString(fmt.Sprintf("%s=%v (%T)", k, v, v))
			varsCount++
		}
		varsStrBuilder.WriteString("}")
		varsStr := varsStrBuilder.String()

		// Format Rules for better readability (showing only Condition and Target)
		var rulesStrBuilder strings.Builder
		rulesStrBuilder.WriteString("[")
		for i, rule := range s.NextRules {
			if i > 0 {
				rulesStrBuilder.WriteString(", ")
			}
			rulesStrBuilder.WriteString(fmt.Sprintf("{Condition: %q, Target: %q}", rule.Condition, rule.Target))
		}
		rulesStrBuilder.WriteString("]")
		rulesStr := rulesStrBuilder.String()

		log.Error("所有路由规则都不匹配", zap.String("formatted_rules", rulesStr), zap.String("current_vars", varsStr))
		return nil, fmt.Errorf("所有路由规则都不匹配, 当前变量状态: %s, 请检查配置或运行时变量, 尝试的规则: %s", varsStr, rulesStr)
		// --- 修改结束 ---
	default:
		// 跳转到指定标签对应的索引
		if nextIndex < 0 || nextIndex >= len(nodes) {
			// 检查索引是否越界
			log.Error("路由目标索引超出范围", zap.Int("index", nextIndex))
			return nil, fmt.Errorf("路由目标索引 %d (来自标签 '%s') 超出节点范围 [0-%d]", nextIndex, targetLabel, len(nodes)-1)
		}

		log.Debug("路由到标签", zap.String("target", targetLabel), zap.Int("index", nextIndex))
		return nodes[nextIndex], nil
	}
}

func (s *Section) ProcessWithBytes(ctx context.Context, state *ByteState) (BProcessor, error) {
	log := pkg.LoggerFromContext(ctx)
	// I. 检查数据是否足够
	end := state.Cursor + s.Size
	log.Debug("节点开始处理", zap.Int("index", s.index), zap.String("desc", s.Desc), zap.Int("size", s.Size))

	if end > len(state.Data) {
		// 返回原始 out，因为没有修改
		log.Error("数据不足", zap.Int("size", s.Size), zap.Int("cursor", state.Cursor), zap.Int("data_len", len(state.Data)))
		return nil, fmt.Errorf("数据不足，需要 %d 字节 (cursor: %d, end: %d, total: %d)",
			s.Size, state.Cursor, end, len(state.Data))
	}

	// II. 获取数据切片 (零拷贝)
	rawData := state.Data[state.Cursor:end]
	log.Debug("获取数据片段", zap.Int("cursor", state.Cursor), zap.Int("end", end), zap.Int("length", len(rawData)), zap.Any("data", rawData))

	// III. 执行表达式
	state.Env.Bytes = rawData
	log.Debug("处理开始前变量状态", zap.Any("vars", state.Env.Vars))

	_, err := expr.Run(s.Program, state.Env)
	if err != nil {
		return nil, fmt.Errorf("执行表达式失败: %w", err)
	}

	// 在所有 Dev 处理完成后移动光标
	state.Cursor = end
	log.Debug("处理完成，光标移至位置", zap.Int("cursor", state.Cursor))
	log.Debug("处理完成后最终变量状态", zap.Any("vars", state.Env.Vars)) // 确认最终 Vars 状态

	log.Debug("准备执行路由...")

	// V. 执行路由
	next, err := s.Route(ctx, state.Env, state.LabelMap, state.Nodes)
	if err != nil {
		log.Error("路由失败", zap.Error(err))
	} else if next != nil {
		// 添加类型检查，避免类型断言错误
		if section, ok := next.(*Section); ok {
			log.Debug("路由到下一节点", zap.Int("index", section.index), zap.String("desc", section.Desc))
		} else {
			log.Debug("路由到下一节点", zap.Any("next", next), zap.String("type", reflect.TypeOf(next).String()))
		}
	} else {
		log.Debug("路由结果为 nil，处理将结束")
	}

	// 返回最终修改后的 next 节点和错误状态
	return next, err
}

// getIntVar 从 VarStore 获取整数变量值。
// 支持直接的整数值和存储在 VarStore 中的变量名。
//
// 输入:
//   - varStore: 变量存储器 (map[string]interface{})
//   - key: 要获取的变量键名或直接的整数/浮点数值
//
// 输出:
//   - int: 获取到的整数值
//   - error: 获取过程中遇到的错误，成功时为 nil
//
// Deprecated: 使用 program 解码替代
func getIntVar(varStore VarStore, key interface{}) (int, error) {
	switch typedKey := key.(type) {
	case int:
		return typedKey, nil
	case float64: // 支持配置中直接写数字 (YAML 可能解析为 float64)
		if typedKey == float64(int(typedKey)) {
			return int(typedKey), nil
		}
		return 0, fmt.Errorf("无法将非整数浮点数 %.2f 用作重复次数", typedKey)
	case string: // key 是变量名
		val, ok := varStore[typedKey]
		if !ok {
			return 0, fmt.Errorf("未找到重复次数变量 '%s'", typedKey)
		}
		// 尝试从 VarStore 中获取的值转换成 int
		switch v := val.(type) {
		case int:
			return v, nil
		case int64: // 可能从 expr 返回 int64
			return int(v), nil
		case float64: // 可能从 expr 返回 float64
			if v == float64(int(v)) {
				return int(v), nil
			}
			return 0, fmt.Errorf("变量 '%s' 的值 (%.2f) 不是整数，无法用作重复次数", typedKey, v)
		case string: // 尝试从字符串转换
			iVal, err := strconv.Atoi(v)
			if err != nil {
				return 0, fmt.Errorf("无法将变量 '%s' 的字符串值 '%s' 转换为整数: %w", typedKey, v, err)
			}
			return iVal, nil
		default:
			return 0, fmt.Errorf("变量 '%s' 的类型 (%T) 无法用作重复次数", typedKey, val)
		}
	default:
		return 0, fmt.Errorf("无效的重复次数类型: %T (%v)", key, key)
	}
}

// parseToDevice 解析包含 ${varname} 占位符的设备名模板。
// 它从 VarStore (map) 中获取变量值（可以是任何类型），并将其转换为字符串插入模板中。
var reTemplate = regexp.MustCompile(`\${([^}]+)}`) // Cache the regex for efficiency

// parseToDevice 解析包含 ${varname} 占位符的设备名模板。
// 它从 VarStore (map) 中获取变量值（可以是任何类型），并将其转换为字符串插入模板中。
// Deprecated: 使用 program解码替代
func parseToDevice(vs VarStore, template string) (string, error) { // Takes map value
	if !strings.Contains(template, "${") {
		return template, nil // No placeholders, return as is
	}

	var firstError error
	expanded := reTemplate.ReplaceAllStringFunc(template, func(match string) string {
		// If an error already occurred during replacement, just return the placeholder
		if firstError != nil {
			return match
		}

		// Extract key from ${key}
		key := match[2 : len(match)-1]

		// Get the value as interface{} from the map
		val, ok := vs[key]
		if !ok {
			// Store the first error encountered
			firstError = fmt.Errorf("未找到模板变量 '%s'", key)
			return match // Return the original placeholder on error
		}

		// Convert the retrieved value (any type) to its string representation
		return fmt.Sprintf("%v", val)
	})

	// If any error occurred during the replacements, return it
	if firstError != nil {
		return "", firstError
	}

	return expanded, nil
}
