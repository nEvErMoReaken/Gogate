package internal

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"go.uber.org/zap"
	"time"
)

// StartParser 启动解析器和策略处理数据
func StartParser(ctx context.Context, dataSource pkg.DataSource) (strategy.MapSendStrategy, error) {
	// 1. 初始化策略
	mapChan := make(map[string]chan pkg.Point)
	s, err := strategy.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Strategy"))
	if err != nil {
		return nil, fmt.Errorf("failed to create strategy: %w", err)
	}
	for key, value := range s {
		mapChan[key] = value.GetChan()
	}

	// 2. 初始化解析器
	p, err := parser.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Parser"), dataSource, mapChan)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	// 3.启动解析器
	go p.Start()

	return s, nil
}

// handleLazyConnector 处理懒创建连接器的逻辑
func handleLazyConnector(ctx context.Context, readyChan <-chan pkg.DataSource) {
	for {
		select {
		case <-ctx.Done():
			time.Sleep(1 * time.Second) // 等待 1 秒，确保所有数据源都已经关闭
			return                      // 退出上下文时停止 Pipeline
		case dataSource := <-readyChan:
			s, err := StartParser(ctx, dataSource)
			if err != nil {
				pkg.LoggerFromContext(ctx).Error("failed to start parser", zap.Error(err))
				pkg.ErrChanFromContext(ctx) <- fmt.Errorf("failed to start parser: %w", err)
				return
			}
			for _, sl := range s {
				sl.Start()
			}
		}
	}
}

// handleEagerConnector 处理主动启动型连接器的逻辑
func handleEagerConnector(ctx context.Context, c connector.Connector) {
	dataSource, err := c.GetDataSource()
	if err != nil {
		pkg.ErrChanFromContext(ctx) <- fmt.Errorf("failed to get data source: %w", err)
		return
	}
	s, err := StartParser(ctx, dataSource)
	if err != nil {
		pkg.ErrChanFromContext(ctx) <- fmt.Errorf("failed to start parser: %w", err)
		return
	}
	for _, sl := range s {
		sl.Start()
	}
}

func StartPipeline(ctx context.Context) {
	// 0. 初始化连接器
	c, err := connector.New(pkg.WithLoggerAndModule(ctx, pkg.LoggerFromContext(ctx), "Connector"))
	if err != nil {
		pkg.LoggerFromContext(ctx).Error("failed to create connector", zap.Error(err))
		pkg.ErrChanFromContext(ctx) <- fmt.Errorf("failed to create connector: %w", err)
		return
	}

	// 1. 启动连接器，启动 Connector 后，数据源由 Parser 处理
	go c.Start()

	readyChan := c.Ready()

	// 2. 处理懒连接器（有 readyChan）或主动连接器（无 readyChan）
	if readyChan != nil {
		pkg.LoggerFromContext(ctx).Debug("===正在启动懒连接器===")
		// 懒创建型连接器
		go handleLazyConnector(ctx, readyChan)
	} else {
		pkg.LoggerFromContext(ctx).Debug("===正在启动主动连接器===")
		// 主动启动型连接器
		handleEagerConnector(ctx, c)
	}
}
