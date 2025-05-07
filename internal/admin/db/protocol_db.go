package db

import (
	"context"
	"errors"
	"fmt"
	"gateway/internal/admin/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 获取协议集合 (返回 *mongo.Collection)
func protocolCollection() *mongo.Collection {
	return getCollection("protocols")
}

// GetAllProtocols 获取所有协议
func GetAllProtocols() ([]model.Protocol, error) {
	collection := protocolCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var protocols []model.Protocol
	if err = cursor.All(ctx, &protocols); err != nil {
		return nil, err
	}
	// 如果没有找到文档，返回空切片而不是错误
	if protocols == nil {
		protocols = []model.Protocol{}
	}
	return protocols, nil
}

// GetProtocolByID 根据 ID 获取协议
func GetProtocolByID(id string) (*model.Protocol, error) {
	collection := protocolCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的协议 ID 格式")
	}

	var protocol model.Protocol
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&protocol)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 或者返回一个特定的 not found 错误
		}
		return nil, err
	}
	return &protocol, nil
}

// CreateProtocol 创建新协议
func CreateProtocol(protocol *model.Protocol) (*model.Protocol, error) {
	collection := protocolCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := primitive.NewDateTimeFromTime(time.Now())
	protocol.CreatedAt = now
	protocol.UpdatedAt = now
	// 确保 ID 为空，以便 MongoDB 生成
	protocol.ID = primitive.NilObjectID

	result, err := collection.InsertOne(ctx, protocol)
	if err != nil {
		return nil, err
	}

	// 获取插入的 ID 并更新对象
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		protocol.ID = oid
		return protocol, nil
	}
	return nil, errors.New("无法获取插入的协议 ID")
}

// UpdateProtocol 更新协议基本信息 (name, description)
func UpdateProtocol(id string, updateData *model.Protocol) (*model.Protocol, error) {
	collection := protocolCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的协议 ID 格式")
	}

	update := bson.M{
		"$set": bson.M{
			"name":        updateData.Name,
			"description": updateData.Description,
			"updatedAt":   primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updatedProtocol model.Protocol
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": objID}, update, opts).Decode(&updatedProtocol)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 或者返回 not found 错误
		}
		return nil, err
	}
	return &updatedProtocol, nil
}

// UpdateProtocolConfig 更新协议配置
func UpdateProtocolConfig(id string, config *model.GatewayConfig) (*model.Protocol, error) {
	collection := protocolCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的协议 ID 格式")
	}

	update := bson.M{
		"$set": bson.M{
			"config":    config,
			"updatedAt": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updatedProtocol model.Protocol
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": objID}, update, opts).Decode(&updatedProtocol)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 或者返回 not found 错误
		}
		return nil, err
	}
	return &updatedProtocol, nil
}

// DeleteProtocol 删除协议并删除其关联的所有版本和全局映射
func DeleteProtocol(id string) error {
	protocolColl := protocolCollection()
	versionColl := versionCollection()                                       // 获取版本集合
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // 增加超时时间以容纳删除操作
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("无效的协议 ID 格式")
	}

	// 1. 删除协议本身
	result, err := protocolColl.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return fmt.Errorf("删除协议失败: %w", err)
	}

	if result.DeletedCount == 0 {
		return errors.New("未找到要删除的协议") // 或者返回 mongo.ErrNoDocuments
	}

	// 2. 删除关联的版本
	// 使用 DeleteMany 删除所有 protocolId 匹配的文档
	_, err = versionColl.DeleteMany(ctx, bson.M{"protocolId": objID})
	if err != nil {
		// 注意：此时协议已删除，但版本删除失败，可能需要日志记录或更复杂的错误处理
		return fmt.Errorf("协议已删除，但删除关联版本失败: %w", err)
	}

	// 3. 删除关联的全局映射
	err = DeleteGlobalMapsByProtocolID(objID)
	if err != nil {
		// 协议和版本已删除，但全局映射删除失败
		return fmt.Errorf("协议和版本已删除，但删除关联全局映射失败: %w", err)
	}

	return nil
}
