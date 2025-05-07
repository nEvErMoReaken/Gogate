package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var ( // 使用包变量来持有数据库连接
	MongoClient *mongo.Client
	ConfigDB    *mongo.Database
)

// InitMongoDB 初始化 MongoDB 连接
func InitMongoDB(connectionString, dbName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(connectionString)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("连接 MongoDB 失败: %v\n", err)
		return fmt.Errorf("连接 MongoDB 失败: %w", err)
	}

	// 检查连接
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		log.Printf("Ping MongoDB 失败: %v\n", err)
		return fmt.Errorf("ping MongoDB 失败: %w", err)
	}

	MongoClient = client
	ConfigDB = client.Database(dbName)
	fmt.Println("成功连接到 MongoDB!")
	return nil
}

// CloseMongoDB 关闭 MongoDB 连接
func CloseMongoDB() {
	if MongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := MongoClient.Disconnect(ctx); err != nil {
			log.Printf("关闭 MongoDB 连接失败: %v\n", err)
		} else {
			fmt.Println("MongoDB 连接已关闭.")
		}
	}
}

// GetConfigCollection 获取用于存储配置的 MongoDB 集合
func GetConfigCollection() *mongo.Collection {
	if ConfigDB == nil {
		// 在实际应用中，这里应该返回错误或 panic，
		// 但为了简化，我们假设 InitMongoDB 总是在之前被调用
		log.Fatal("MongoDB 数据库未初始化！")
	}
	// 集合名称可以配置，这里硬编码为 "configs"
	return ConfigDB.Collection("configs")
}

// GetProtocolCollection 获取用于存储协议元数据的 MongoDB 集合
func GetProtocolCollection() *mongo.Collection {
	if ConfigDB == nil {
		log.Fatal("MongoDB 数据库未初始化！")
	}
	return ConfigDB.Collection("protocols") // 使用 "protocols" 集合
}

// GetProtocolVersionCollection 获取用于存储协议版本配置的 MongoDB 集合
func GetProtocolVersionCollection() *mongo.Collection {
	if ConfigDB == nil {
		log.Fatal("MongoDB 数据库未初始化！")
	}
	return ConfigDB.Collection("protocol_versions") // 使用 "protocol_versions" 集合
}

// GetGlobalMapCollection 获取用于存储全局映射的 MongoDB 集合
func GetGlobalMapCollection() *mongo.Collection {
	if ConfigDB == nil {
		log.Fatal("MongoDB 数据库未初始化！")
	}
	return ConfigDB.Collection("global_maps") // 使用 "global_maps" 集合
}

// getCollection 获取指定名称的集合
// （注意：实际应用中可能需要更复杂的错误处理）
func getCollection(collectionName string) *mongo.Collection {
	if MongoClient == nil {
		// 在实际应用中，这里应该返回错误或尝试重新连接
		log.Fatal("MongoDB client尚未初始化")
		return nil // 或者 panic
	}
	return MongoClient.Database(ConfigDB.Name()).Collection(collectionName)
}
