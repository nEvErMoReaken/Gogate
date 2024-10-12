package internal

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"gateway/util"
	"go.uber.org/zap"
)

// StartParser 启动解析器和策略处理数据
func StartParser(ctx context.Context, dataSource interface{}) (strategy.MapSendStrategy, error) {
	// 1. 初始化策略
	mapChan := make(map[string]chan pkg.Point)
	s, err := strategy.New(pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Strategy"))))
	if err != nil {
		return nil, fmt.Errorf("failed to create strategy: %w", err)
	}
	for key, value := range s {
		mapChan[key] = value.GetChan()
	}

	// 2. 初始化解析器
	p, err := parser.New(dataSource, mapChan, pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Parser"))))
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	// 3.启动解析器
	go p.Start()

	return s, nil
}

// handleLazyConnector 处理懒创建连接器的逻辑
func handleLazyConnector(ctx context.Context, readyChan <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return // 退出上下文时停止 Pipeline
		case dataSource := <-readyChan:
			s, err := StartParser(ctx, dataSource)
			if err != nil {
				util.ErrChanFromContext(ctx) <- fmt.Errorf("failed to start parser: %w", err)
				return
			}
			for _, sl := range s {
				sl.Start()
			}
		}
	}
}

// handleEagerConnector 处理主动启动型连接器的逻辑
func handleEagerConnector(c connector.Connector, ctx context.Context) {
	dataSource, err := c.GetDataSource()
	if err != nil {
		util.ErrChanFromContext(ctx) <- fmt.Errorf("failed to get data source: %w", err)
		return
	}
	s, err := StartParser(ctx, dataSource)
	if err != nil {
		util.ErrChanFromContext(ctx) <- fmt.Errorf("failed to start parser: %w", err)
		return
	}
	for _, sl := range s {
		sl.Start()
	}
}

func Start(ctx context.Context) {
	// 0. 初始化连接器
	c, err := connector.New(pkg.WithLogger(ctx, pkg.LoggerFromContext(ctx).With(zap.String("module", "Connector"))))
	if err != nil {
		util.ErrChanFromContext(ctx) <- fmt.Errorf("failed to create connector: %w", err)
		return
	}

	// 1. 启动连接器，启动 Connector 后，数据源由 Parser 处理
	c.Start()
	readyChan := c.Ready()

	// 2. 处理懒连接器（有 readyChan）或主动连接器（无 readyChan）
	if readyChan != nil {
		// 懒创建型连接器
		go handleLazyConnector(ctx, readyChan)
	} else {
		// 主动启动型连接器
		handleEagerConnector(c, ctx)
	}
}
