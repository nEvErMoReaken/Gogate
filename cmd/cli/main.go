package main

import (
	"bufio"
	"fmt"
	"gateway/cmd/cli/command"
	"os"
	"strings"
)

// 打印欢迎信息和启动logo
func printWelcomeMessage() {
	PrintStartupLogo()
	fmt.Println("Welcome to the GoGate CLI REPL! Type 'exit' to quit.")
	fmt.Println("Type 'help' to see the list of available commands.")
}

// 打印帮助信息
func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  shootone <frame>  Send a single frame to the gateway and get a response.")
	fmt.Println("  help              Show this help message.")
	fmt.Println("  exit              Exit the REPL.")
}
func PrintStartupLogo() {
	logo := `
		 ________  ________  ________  ________  _________  _______
		|\   ____\|\   __  \|\   ____\|\   __  \|\___   ___\\  ___ \
		\ \  \___|\ \  \|\  \ \  \___|\ \  \|\  \|___ \  \_\ \   __/|
		 \ \  \  __\ \  \\\  \ \  \  __\ \   __  \   \ \  \ \ \  \_|/__
		  \ \  \|\  \ \  \\\  \ \  \|\  \ \  \ \  \   \ \  \ \ \  \_|\ \
		   \ \_______\ \_______\ \_______\ \__\ \__\   \ \__\ \ \_______\
			\|_______|\|_______|\|_______|\|__|\|__|    \|__|  \|_______|

`
	fmt.Print(logo)
}

func main() {
	// 创建根命令
	rootCmd := command.NewRootCommand()

	// 创建输入读取器
	scanner := bufio.NewScanner(os.Stdin)

	// 打印欢迎信息
	printWelcomeMessage()

	// 进入 REPL 循环
	for {
		// 打印提示符
		fmt.Print("> ")

		// 读取用户输入
		if !scanner.Scan() {
			break
		}

		// 获取用户输入的命令
		input := scanner.Text()

		// 如果输入是 exit，退出 GoGate
		if strings.ToLower(input) == "exit" {
			fmt.Println("Exiting GoGate...")
			break
		}

		// 如果输入 help，打印帮助信息
		if strings.ToLower(input) == "help" {
			printHelp()
			continue
		}

		// 将用户输入拆分为命令和参数
		args := strings.Fields(input)

		// 如果没有输入命令，跳过
		if len(args) == 0 {
			continue
		}

		// 获取子命令名称
		commandName := args[0]

		// 仅当子命令有效时，才设置参数并执行
		switch commandName {
		case "shootone":
			// 将参数传递给 shootone 子命令
			rootCmd.SetArgs(args) // 设置命令行参数
			if err := rootCmd.Execute(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "generate":
			// 将参数传递给 generate 子命令
			rootCmd.SetArgs(args) // 设置命令行参数
			if err := rootCmd.Execute(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		default:
			// 如果是无效命令，提示用户
			fmt.Printf("Unknown command: %s\n", commandName)
			fmt.Println("Type 'help' to see the list of available commands.")
		}
	}

	// 如果程序执行到这里，说明 REPL 结束
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}
