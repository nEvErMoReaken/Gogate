package internal

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"go.uber.org/zap"
)

// Pipeline 为函数的主逻辑
type Pipeline struct {
	connector connector.Template
	parser    parser.Template
	strategy  strategy.TemplateCollection
}

func (p *Pipeline) Start(ctx context.Context) {
	pkg.LoggerFromContext(ctx).Info("=== Starting Pipeline ===")
	sourceChan := make(chan pkg.DataSource, 20)
	err := p.connector.Start(sourceChan)
	if err != nil {
		return
	}
	source02 := pkg.PointDataSource{PointChan: make(map[string]chan pkg.Point)}
	for key := range p.strategy {
		source02.PointChan[key] = make(chan pkg.Point, 200)
	}
	// 可接受的资源泄露
	go func() {
		for {
			select {
			case source01 := <-sourceChan:
				p.parser.Start(&source01, &source02)
			}
		}
	}()
	p.strategy.Start(&source02)
	pkg.LoggerFromContext(ctx).Info("=== Pipeline Start Success ===")

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
	pkg.LoggerFromContext(ctx).Info(" Pipeline Info ", zap.Any("connector", pkg.ConfigFromContext(ctx).Connector.Type), zap.Any("parser", pkg.ConfigFromContext(ctx).Parser.Type), zap.Any("strategy", showList))
	return &Pipeline{
		connector: c,
		parser:    p,
		strategy:  s,
	}, nil
}
