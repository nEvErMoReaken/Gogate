package main

import (
	"fmt"
	"gateway/cmd/cli/command"
	"os"
)

func main() {

	// 创建根命令
	rootCmd := command.NewRootCommand()

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
