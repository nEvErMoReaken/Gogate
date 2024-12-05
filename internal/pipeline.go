package internal

import (
	"context"
	"encoding/hex"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/parser"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"go.uber.org/zap"
	"io"
	"time"
)

// Pipeline 为函数的主逻辑
type Pipeline struct {
	connector connector.Template
	parser    parser.Template
	strategy  strategy.TemplateCollection
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
		reader, writer := io.Pipe()
		source := pkg.StreamDataSource{
			Reader: reader,
			Writer: writer,
		}
		var ds pkg.DataSource = &source
		ps := pkg.PointDataSource{PointChan: make(map[string]chan pkg.Point)}
		for _, StrategyConfig := range pkg.ConfigFromContext(ctx).Strategy {
			ps.PointChan[StrategyConfig.Type] = make(chan pkg.Point, 10)
		}
		go tempParser.Start(&ds, &ps)
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
		for key, value := range ps.PointChan {
		loop:
			for {
				select {
				case v := <-value:
					res += fmt.Sprintf("%s: %s\n", key, v.String())
				default:
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
	// 4. 简单校验
	if c.GetType() != p.GetType() {
		return nil, fmt.Errorf("connector and parser type mismatch, %s != %s", c.GetType(), p.GetType())
	}
	pkg.LoggerFromContext(ctx).Info(" Pipeline Info ", zap.Any("connector", pkg.ConfigFromContext(ctx).Connector.Type), zap.Any("parser", pkg.ConfigFromContext(ctx).Parser.Type), zap.Any("strategy", showList))
	return &Pipeline{
		connector: c,
		parser:    p,
		strategy:  s,
	}, nil
}
