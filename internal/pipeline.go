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

type Pipeline struct {
	Connector  connector.Connector
	Parser     parser.Parser
	Strategies strategy.MapSendStrategy
}

type PipeLines []Pipeline

func (pls PipeLines) StartAll() {
	for _, pipeline := range pls {
		pipeline.Start()
	}
}

func NewPipelines(ctx context.Context) (PipeLines, error) {
	var pipelineGroup = make([]Pipeline, 0)
	// 1. 初始化连接器
	list, err := connector.New(pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Connector"))))
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	for _, c := range list {
		dataSource, err := c.GetDataSource()
		if err != nil {
			return nil, fmt.Errorf("failed to get data source: %w", err)
		}

		// 2. 初始化解析器
		p, err := parser.New(dataSource, pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Parser"))))
		if err != nil {
			return nil, fmt.Errorf("failed to create parser: %w", err)
		}

		// 3. 初始化策略
		s, err := strategy.New(pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Strategy"))))
		if err != nil {
			return nil, fmt.Errorf("failed to create strategy: %w", err)
		}
		pipelineGroup = append(pipelineGroup, Pipeline{
			Connector:  c,
			Parser:     p,
			Strategies: s,
		})
	}

	return pipelineGroup, nil

}

func (pl *Pipeline) Start() {

	// 1. 启动连接器
	go pl.Connector.Start()

	ch := make(chan pkg.Point, 200)
	// 2. 启动解析器
	go pl.Parser.Start(ch)

	// 3. 启动所有策略
	for _, st := range pl.Strategies {
		go func() {
			st.Start()
		}()
	}
}
