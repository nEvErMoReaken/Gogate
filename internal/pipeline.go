package internal

import (
	"context"
	"encoding/hex"
	"fmt"
	"gateway/internal/adapter"
	"gateway/internal/aggregator"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"io"
	"time"

	"go.uber.org/zap"
)

// Pipeline 为函数的主逻辑
type Pipeline struct {
	connector  adapter.Template
	parser     parser.Template
	aggregator *aggregator.Aggregator
	strategy   strategy.TemplateCollection
	source01   *pkg.ConnectorDataSource
	source02   *pkg.Parser2DispatcherChan
	source03   *pkg.Dispatch2DataSourceChan
}

// ShootOne 发射-回执一条数据，用于cli和测试
func ShootOne(ctx context.Context, oriFrame string) (res string, err error) {
	pkg.LoggerFromContext(ctx).Info("=== Shoot One ===")
	tempParser, err := parser.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Parser"))
	if err != nil {
		return "", fmt.Errorf("failed to create Parser, %s ", err)
	}

	// 1. 判断当前环境类型
	if tempParser.GetType() == "stream" {
		// 1. 初始化数据源
		reader, writer := io.Pipe()
		source := pkg.StreamDataSource{
			Reader: reader,
			Writer: writer,
		}
		// ShootInput -> source01 -> parser -> source02 -> aggregator -> source03 -> ShowOutput
		var source01 pkg.DataSource = &source
		source02 := pkg.Parser2DispatcherChan{PointChan: make(chan pkg.Point), EndChan: make(chan struct{})}
		source03 := pkg.Dispatch2DataSourceChan{PointChan: make(map[string]chan pkg.Point)}

		// 2. 初始化parser和aggregator
		var a *aggregator.Aggregator
		a = aggregator.New(ctx)
		for _, StrategyConfig := range pkg.ConfigFromContext(ctx).Strategy {
			source03.PointChan[StrategyConfig.Type] = make(chan pkg.Point, 10)
		}
		go a.Start(&source02, &source03)
		go tempParser.Start(&source01, &source02)
		var data []byte
		data, err = hex.DecodeString(oriFrame)
		if err != nil {
			return "", err
		}
		_, err = writer.Write(data)
		if err != nil {
			return "", err
		}
		time.Sleep(400 * time.Millisecond)
		// 2. 监听、汇总所有管道信息
		for key, value := range source03.PointChan {
		loop:
			for {
				select {
				case v := <-value:
					res += fmt.Sprintf("%s: %s\n", key, v.String())
				default:
					res += "no data"
					break loop
				}
			}
		}

	} else if tempParser.GetType() == "message" {
		return "", fmt.Errorf("only stream parser is supported")
	}

	return res, err
}

// Start 启动管道
func (p *Pipeline) Start(ctx context.Context) {
	logger := pkg.LoggerFromContext(ctx)
	metrics := pkg.GetPerformanceMetrics() // 获取性能指标实例

	logger.Info("=== Starting Pipeline ===")

	// Step.1 启动连接器

	err := p.connector.Start(p.source01)

	if err != nil {
		logger.Error("=== Connector Start Failed ===", zap.Error(err))
		return
	}

	// Step.2 启动Parser
	// 可接受的资源泄露
	go func() {
		for {
			select {
			case source01 := <-p.source01Chan:
				parserTimer := metrics.NewTimer("Parser_Process")
				metrics.IncRequestCount() // 增加请求计数
				p.parser.Start(&source01, p.source02)
				parserTimer.StopAndLog(logger)
			}
		}
	}()

	// Step.3 启动Aggregator
	go func() {
		aggregatorTimer := metrics.NewTimer("Aggregator_Start")
		p.aggregator.Start(p.source02, p.source03)
		aggregatorTimer.StopAndLog(logger)
	}()

	// Step.4 启动Strategy
	strategyTimer := metrics.NewTimer("Strategy_Start")
	p.strategy.Start(p.source03)
	strategyTimer.StopAndLog(logger)

	logger.Info("=== Pipeline Start Success ===")
}

func NewPipeline(ctx context.Context) (*Pipeline, error) {
	pkg.LoggerFromContext(ctx).Info("=== Building Pipeline ===")
	// 非阻塞方法
	// 0. 校验流程合法性
	var err error

	// 1. 初始化Connector
	var c adapter.Template
	c, err = adapter.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Connector"))
	if err != nil {
		return nil, fmt.Errorf("failed to create connector, %s ", err)
	}
	var p parser.Template
	// 2. 初始化Parser, 此处仅校验用
	p, err = parser.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Parser"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Parser, %s ", err)
	}
	// 3. 初始化Strategy
	var s strategy.TemplateCollection
	s, err = strategy.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Strategy"))
	if err != nil {
		return nil, fmt.Errorf("failed to create Startegy, %s ", err)
	}
	pkg.LoggerFromContext(ctx).Info("=== Pipeline Build Success ===")
	var showList []string
	for _, config := range pkg.ConfigFromContext(ctx).Strategy {
		if config.Enable {
			showList = append(showList, config.Type)
		}
	}

	// 4. 初始化Aggregator
	var a *aggregator.Aggregator
	a = aggregator.New(ctx)

	// 5. 初始化数据源
	source01 := make(pkg.ConnectorDataSource)
	source02 := pkg.Parser2DispatcherChan{PointChan: make(chan pkg.Point, 200), EndChan: make(chan struct{}, 20)}
	source03 := make(pkg.Dispatch2DataSourceChan)
	for key := range s {
		source03[key] = make(chan pkg.Point, 200)
	}

	// 6. 简单校验
	if c.GetType() != p.GetType() {
		return nil, fmt.Errorf("connector and parser type mismatch, %s != %s", c.GetType(), p.GetType())
	}

	pkg.LoggerFromContext(ctx).Info(" Pipeline Info ", zap.Any("connector", pkg.ConfigFromContext(ctx).Connector.Type), zap.Any("parser", pkg.ConfigFromContext(ctx).Parser.Type), zap.Any("strategy", showList))
	return &Pipeline{
		connector:  c,
		parser:     p,
		strategy:   s,
		aggregator: a,
		source01:   &source01,
		source02:   &source02,
		source03:   &source03,
	}, nil
}
