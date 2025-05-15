package main

import (
	"context"
	"fmt"
	"gateway/internal/admin/db" // 导入数据库包
	"gateway/internal/admin/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper" // Added viper import
)

// 配置常量 - These will be managed by Viper
// const (
// 	MongoDBConnectionString = "mongodb://10.17.191.106:27017"
// 	// MongoDBConnectionString = "mongodb+srv://crissangelers:bVHh6MDNlExC9hBS@cluster0.cbuxbsn.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0"
// 	DatabaseName = "gateway_admin_v2"
// 	ServerPort   = "8080"
// )

func main() {
	// Viper Configuration
	vip := viper.New() // Use a new Viper instance for better isolation
	vip.SetEnvPrefix("ADMIN")
	vip.AutomaticEnv()

	vip.SetDefault("MONGO_CONNECTION_STRING", "mongodb://10.17.191.106:27017")
	vip.SetDefault("DATABASE_NAME", "gateway_admin_v2")
	vip.SetDefault("SERVER_PORT", "8080")

	mongoConnectionString := vip.GetString("MONGO_CONNECTION_STRING")
	databaseName := vip.GetString("DATABASE_NAME")
	serverPort := vip.GetString("SERVER_PORT")

	// 初始化 MongoDB 连接
	if err := db.InitMongoDB(mongoConnectionString, databaseName); err != nil {
		log.Fatalf("无法初始化 MongoDB: %v", err)
	}
	// 程序退出时关闭数据库连接
	defer db.CloseMongoDB()

	// 初始化 Gin 引擎
	r := router.SetupRouter()

	// 定义服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", serverPort),
		Handler: r,
	}

	// 优雅地启动和关闭服务器
	go func() {
		fmt.Printf("管理后台服务启动于 http://localhost:%s\n", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器 ...")

	// 关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("服务器关闭失败:", err)
	}

	log.Println("服务器已退出")
}
