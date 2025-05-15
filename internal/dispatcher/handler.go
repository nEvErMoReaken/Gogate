package dispatcher

import (
	"fmt"
	"gateway/internal/pkg"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// TEnv 是表达式执行的环境。
// 它包含了当前处理的字节数据、运行时变量、输出字段以及全局变量。
type TEnv struct {
	// Bytes 是当前 Section 处理的原始字节切片
	Tag map[string]any
}

type Handler struct {
	LatestTs           time.Time
	LatestFrameId      string
	pointList          []*pkg.PointPackage
	Strategy           []pkg.StrategyConfig
	StrategyFilterList map[string]*vm.Program
}

// BuildTagExprOptions 返回用于编译 Tag 表达式的 expr 选项。
// 环境设置为 *TEnv，并注册了全局辅助函数。
//
// 输入: 无
// 输出:
//   - []expr.Option: 编译选项切片
func BuildTagExprOptions() []expr.Option {
	options := []expr.Option{
		// 环境直接是 BEnv 结构体
		expr.Env(&TEnv{}),
	}
	// 同样需要注册全局辅助函数
	return options
}

// NewHandler 创建一个新的处理器
//
// 输入:
//   - strategyConfigs: 策略配置列表
//
// 输出:
//   - *handler: 新的处理器
//   - error: 错误
func NewHandler(strategyConfigs []pkg.StrategyConfig) (*Handler, error) {
	handler := &Handler{
		LatestTs:           time.Time{},
		LatestFrameId:      "",
		pointList:          []*pkg.PointPackage{},
		Strategy:           strategyConfigs,
		StrategyFilterList: make(map[string]*vm.Program),
	}

	// 编译策略过滤表达式
	for _, strategy := range strategyConfigs {
		filter := strings.Join(strategy.Filter, " && ")
		program, err := expr.Compile(filter, BuildTagExprOptions()...)
		if err != nil {
			return nil, fmt.Errorf("编译策略过滤表达式失败: %w", err)
		}
		handler.StrategyFilterList[strategy.Type] = program
	}
	return handler, nil
}

// Dispatch 分发点
//
// 输入:
//   - pointList: 要分发的点列表
//
// 输出:
//   - map[string]*pkg.PointPackage: 分发后的点包
func (h *Handler) Dispatch(pointList *pkg.PointPackage) (map[string]*pkg.PointPackage, error) {
	defer h.Clean()
	h.LatestFrameId = pointList.FrameId
	h.LatestTs = pointList.Ts
	readyPointPackage := make(map[string]*pkg.PointPackage)
	for _, point := range pointList.Points {
		err := h.AddPoint(point, readyPointPackage)
		if err != nil {
			return nil, err
		}
	}
	return readyPointPackage, nil
}

// AddPoint 添加点
//
// 输入:
//   - point: 要添加的点
//   - readyPointPackage: 已准备好的点包
//
// 输出:
//   - error: 错误
func (h *Handler) AddPoint(point *pkg.Point, readyPointPackage map[string]*pkg.PointPackage) error {
	var clonedPoint *pkg.Point

	// 只遍历一次策略列表
	for _, strategy := range h.Strategy {
		program := h.StrategyFilterList[strategy.Type]
		result, err := expr.Run(program, TEnv{Tag: point.Tag})
		if err != nil {
			return fmt.Errorf("执行策略过滤表达式失败: %w", err)
		}

		// 如果匹配该策略
		if result.(bool) {
			// 延迟创建克隆：只在第一次匹配策略时创建
			if clonedPoint == nil {
				// 创建point的深拷贝
				clonedPoint = pkg.PointPoolInstance.Get()
				// 复制Tag
				for k, v := range point.Tag {
					clonedPoint.Tag[k] = v
				}
				// 复制Field
				for k, v := range point.Field {
					clonedPoint.Field[k] = v
				}
			}

			// 确保策略对应的PointPackage已创建
			if _, ok := readyPointPackage[strategy.Type]; !ok {
				readyPointPackage[strategy.Type] = &pkg.PointPackage{
					FrameId: h.LatestFrameId,
					Ts:      h.LatestTs,
					Points:  []*pkg.Point{},
				}
			}

			// 将克隆添加到当前匹配的策略中
			readyPointPackage[strategy.Type].Points = append(readyPointPackage[strategy.Type].Points, clonedPoint)
		}
	}

	return nil
}

// Clean 清理处理器
//
// 输入: 无
//
// 输出: 无
func (h *Handler) Clean() {
	h.LatestTs = time.Time{}
	h.LatestFrameId = ""
	for _, pointPackage := range h.pointList {
		for _, point := range pointPackage.Points {
			pkg.PointPoolInstance.Put(point)
		}
	}
	h.pointList = []*pkg.PointPackage{}
}
