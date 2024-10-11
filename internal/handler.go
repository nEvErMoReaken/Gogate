package internal

import (
	"context"
	"fmt"
	"gateway/internal/connector"
	"gateway/internal/pkg"
	"gateway/internal/strategy"
	"gateway/util"
	"go.uber.org/zap"
)

type Handler struct {
	Ctx context.Context
}

func (h *Handler) Start() {
	err := strategy.RunStrategy(pkg.WithLogger(h.Ctx, pkg.LoggerFromContext(h.Ctx).With(zap.String("module", "strategy"))))
	if err != nil {
		util.ErrChanFromContext(h.Ctx) <- fmt.Errorf("failed to run strategy: %w", err)
	}
	// 6. 启动连接器
	err = connector.RunConnector(ctx)
	if err != nil {
		util.ErrChanFromContext(h.Ctx) <- fmt.Errorf("failed to start connector: %w", err)
	}
}
