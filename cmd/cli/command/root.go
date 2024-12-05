package command

import (
	"context"
	"fmt"
	"gateway/internal"
	"gateway/internal/pkg"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewRootCommand 创建根命令
func NewRootCommand() *cobra.Command {
	// 创建根命令
	rootCmd := &cobra.Command{
		Use:   "gateway-cli",
		Short: "Gateway CLI for testing and managing frames",
		Long:  `Gateway CLI is used for testing and managing frames with various commands.`,
	}

	// 添加子命令
	rootCmd.AddCommand(NewShootOneCommand()) // 确保子命令正确添加

	return rootCmd
}

// NewShootOneCommand 创建 shootOne 子命令
func NewShootOneCommand() *cobra.Command {
	var oriFrame string // 定义输入参数

	cmd := &cobra.Command{
		Use:   "shootone",
		Short: "Send a single frame and get a response",
		Long:  `Send a single frame to the gateway and receive a response for testing purposes.`,
		Args:  cobra.ExactArgs(1), // 规定必须接受一个参数
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取位置参数 frame
			oriFrame = args[0]

			// 初始化上下文和日志
			// 1. 初始化common yaml
			config, err := pkg.InitCommon("yaml")
			if err != nil {
				fmt.Printf("[main] 加载配置失败: %s", err)
			}

			// 2. 初始化log
			log := zap.NewNop()

			// 3. 创建上下文
			ctx, _ := context.WithCancel(context.Background())
			errChan := make(chan error, 10) // 创建一个只写的全局错误通道, 缓存大小为10
			ctx = pkg.WithErrChan(ctx, errChan)
			// 将config挂载到ctx上
			ctxWithConfig := pkg.WithConfig(ctx, config)
			// 将logger挂载到ctx上
			ctxWithConfigAndLogger := pkg.WithLogger(ctxWithConfig, log)

			// 调用 ShootOne 函数
			result, err := internal.ShootOne(ctxWithConfigAndLogger, oriFrame)
			if err != nil {
				return fmt.Errorf("failed to process frame: %w", err)
			}

			fmt.Println("Result:", result)
			return nil
		},
	}

	return cmd
}
