package test

import (
	"context"
	"fmt"
	"gateway/internal/pkg"
	"go.uber.org/zap"
)

type helper struct {
	config  *pkg.Config
	log     *zap.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	errChan chan error
}

func chooseConfig(subject string) (*helper, error) {
	// 1. 初始化common yaml
	var h helper
	var err error
	// 打印当前路径

	h.config, err = pkg.InitCommon("yaml/" + subject)
	if err != nil {
		fmt.Printf("加载配置失败: %s", err)
		return nil, err
	}
	// 2. 初始化log
	h.log = pkg.NewLogger(&h.config.Log)

	h.log.Info("测试程序启动", zap.String("version", h.config.Version))
	h.log.Info("配置信息", zap.Any("common", h.config))
	h.log.Info("*** 初始化流程开始 ***")
	// 3. 创建上下文
	var c context.Context
	c, h.cancel = context.WithCancel(context.Background())
	h.errChan = make(chan error, 10) // 创建一个只写的全局错误通道, 缓存大小为10
	c = pkg.WithErrChan(c, h.errChan)
	// 将config挂载到ctx上
	c = pkg.WithConfig(c, h.config)
	// 将logger挂载到ctx上
	h.ctx = pkg.WithLogger(c, h.log)
	return &h, nil
}
