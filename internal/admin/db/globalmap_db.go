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

// 获取全局映射集合 (返回 *mongo.Collection)
func globalMapCollection() *mongo.Collection {
	return getCollection("global_maps")
}

// GetGlobalMapsByProtocolID 获取特定协议的所有全局映射
func GetGlobalMapsByProtocolID(protocolID string) ([]model.GlobalMap, error) {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objPID, err := primitive.ObjectIDFromHex(protocolID)
	if err != nil {
		return nil, errors.New("无效的协议 ID 格式")
	}

	filter := bson.M{"protocolId": objPID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var globalmaps []model.GlobalMap
	if err = cursor.All(ctx, &globalmaps); err != nil {
		return nil, err
	}
	// 返回空切片而不是 nil
	if globalmaps == nil {
		globalmaps = []model.GlobalMap{}
	}
	return globalmaps, nil
}

// CreateGlobalMap 创建新的全局映射
func CreateGlobalMap(globalmap *model.GlobalMap) (*model.GlobalMap, error) {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 确保 ProtocolID 是有效的 ObjectID
	if globalmap.ProtocolID.IsZero() {
		return nil, errors.New("创建全局映射时缺少有效的 ProtocolID")
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	globalmap.CreatedAt = now
	globalmap.UpdatedAt = now
	globalmap.ID = primitive.NilObjectID // 让 MongoDB 生成 ID

	result, err := collection.InsertOne(ctx, globalmap)
	if err != nil {
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		globalmap.ID = oid
		return globalmap, nil
	}
	return nil, errors.New("无法获取插入的全局映射 ID")
}

// GetGlobalMapByID 根据 ID 获取全局映射
func GetGlobalMapByID(id string) (*model.GlobalMap, error) {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的全局映射 ID 格式")
	}

	var globalmap model.GlobalMap
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&globalmap)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 或者返回特定的 not found 错误
		}
		return nil, err
	}
	return &globalmap, nil
}

// UpdateGlobalMap 更新全局映射 (name, description, content)
func UpdateGlobalMap(id string, updateData *model.GlobalMap) (*model.GlobalMap, error) {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的全局映射 ID 格式")
	}

	update := bson.M{
		"$set": bson.M{
			"name":        updateData.Name,
			"description": updateData.Description,
			"content":     updateData.Content,
			"updatedAt":   primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updatedGlobalMap model.GlobalMap
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": objID}, update, opts).Decode(&updatedGlobalMap)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 或者返回特定的 not found 错误
		}
		return nil, err
	}
	return &updatedGlobalMap, nil
}

// DeleteGlobalMap 删除全局映射
func DeleteGlobalMap(id string) error {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("无效的全局映射 ID 格式")
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("未找到要删除的全局映射") // 或者返回 mongo.ErrNoDocuments
	}

	return nil
}

// DeleteGlobalMapsByProtocolID 删除特定协议的所有全局映射
func DeleteGlobalMapsByProtocolID(protocolID primitive.ObjectID) error {
	collection := globalMapCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.DeleteMany(ctx, bson.M{"protocolId": protocolID})
	if err != nil {
		return fmt.Errorf("删除协议相关的全局映射失败: %w", err)
	}

	return nil
}
