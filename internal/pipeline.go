package internal

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/dispatcher"
	"gateway/internal/pkg"
	"gateway/internal/sink"

	"go.uber.org/zap"
)

// Pipeline 为函数的主逻辑
type Pipeline struct {
	connector  connector.Template
	dispatcher *dispatcher.Dispatcher
	strategy   sink.TemplateCollection
}

// Start 启动管道
func (p *Pipeline) Start(ctx context.Context) {
	logger := pkg.LoggerFromContext(ctx)

	logger.Info("=== Starting Pipeline ===")

	// Step.1 启动连接器
	parser2dispatcher := make(pkg.Parser2DispatcherChan, 200)
	dispatcher2sink := make(pkg.Dispatch2SinkChan, 200)
	err := p.connector.Start(&parser2dispatcher)

	if err != nil {
		logger.Error("=== Connector Start Failed ===", zap.Error(err))
		return
	}

	// Step.2 启动Dispatcher
	p.dispatcher.Start(&parser2dispatcher, &dispatcher2sink)

	// Step.3 启动Strategy

	p.strategy.Start(&dispatcher2sink)

	logger.Info("=== Pipeline Start Success ===")
}

func NewPipeline(ctx context.Context) (*Pipeline, error) {
	pkg.LoggerFromContext(ctx).Info("=== Building Pipeline ===")
	// 非阻塞方法
	// 0. 校验流程合法性
	var err error

	// 1. 初始化Connector
	var c connector.Template
	c, err = connector.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Connector"))
	if err != nil {
		return nil, fmt.Errorf("failed to create connector, %s ", err)
	}

	// 3. 初始化Strategy
	var s sink.TemplateCollection
	s, err = sink.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Strategy"))
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
	var a *dispatcher.Dispatcher
	a = dispatcher.New(ctx, &pkg.ConfigFromContext(ctx).Dispatcher)

	pkg.LoggerFromContext(ctx).Info(" Pipeline Info ", zap.Any("connector", pkg.ConfigFromContext(ctx).Connector.Type), zap.Any("parser", pkg.ConfigFromContext(ctx).Parser.Type), zap.Any("strategy", showList))
	return &Pipeline{
		connector:  c,
		dispatcher: a,
		strategy:   s,
	}, nil
}
