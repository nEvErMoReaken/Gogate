package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gateway/internal/admin/model"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 获取版本集合 (返回 *mongo.Collection)
func versionCollection() *mongo.Collection {
	return getCollection("protocol_versions")
}

// GetVersionsByProtocolID 获取特定协议的所有版本
func GetVersionsByProtocolID(protocolID string) ([]model.ProtocolVersion, error) {
	collection := versionCollection()
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

	var versions []model.ProtocolVersion
	if err = cursor.All(ctx, &versions); err != nil {
		return nil, err
	}
	// 返回空切片而不是 nil
	if versions == nil {
		versions = []model.ProtocolVersion{}
	}
	return versions, nil
}

// CreateVersion 创建新版本
func CreateVersion(version *model.ProtocolVersion) (*model.ProtocolVersion, error) {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 确保 ProtocolID 是有效的 ObjectID
	if version.ProtocolID.IsZero() {
		return nil, errors.New("创建版本时缺少有效的 ProtocolID")
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	version.CreatedAt = now
	version.UpdatedAt = now
	version.ID = primitive.NilObjectID // 让 MongoDB 生成 ID

	result, err := collection.InsertOne(ctx, version)
	if err != nil {
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		version.ID = oid
		return version, nil
	}
	return nil, errors.New("无法获取插入的版本 ID")
}

// GetVersionByID 根据 ID 获取版本
func GetVersionByID(id string) (*model.ProtocolVersion, error) {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的版本 ID 格式")
	}

	var version model.ProtocolVersion
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&version)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Or specific not found error
		}
		return nil, err
	}
	return &version, nil
}

// UpdateVersion 更新版本信息 (version, description)
func UpdateVersion(id string, updateData *model.ProtocolVersion) (*model.ProtocolVersion, error) {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("无效的版本 ID 格式")
	}

	update := bson.M{
		"$set": bson.M{
			"version":     updateData.Version,
			"description": updateData.Description,
			"updatedAt":   primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updatedVersion model.ProtocolVersion
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": objID}, update, opts).Decode(&updatedVersion)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Or specific not found error
		}
		return nil, err
	}
	return &updatedVersion, nil
}

// DeleteVersion 删除版本 (可选实现)
func DeleteVersion(id string) error {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("无效的版本 ID 格式")
	}

	result, err := collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return errors.New("未找到要删除的版本") // Or mongo.ErrNoDocuments
	}

	return nil
}

// GetVersionDefinition 获取指定版本的协议定义
func GetVersionDefinition(id string) (model.ProtocolDefinition, error) {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return model.ProtocolDefinition{}, errors.New("无效的版本 ID 格式")
	}

	var result struct { // 只选择 definition 字段
		Definition model.ProtocolDefinition `bson:"definition"`
	}
	opts := options.FindOne().SetProjection(bson.M{"definition": 1, "_id": 0}) // 投影操作

	err = collection.FindOne(ctx, bson.M{"_id": objID}, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("[DB DEBUG] GetVersionDefinition: 未找到版本 %s 的定义, 返回空定义\n", id)
			return model.ProtocolDefinition{}, nil // 返回空定义而不是错误
		}
		return model.ProtocolDefinition{}, fmt.Errorf("获取协议定义失败: %w", err)
	}
	definitionJSON, errJSON := json.MarshalIndent(result.Definition, "", "  ")
	if errJSON != nil {
		fmt.Printf("[DB WARN] GetVersionDefinition: 序列化从DB读取的 definition 为 JSON 失败: %v\n", errJSON)
	} else {
		fmt.Printf("[DB DEBUG] GetVersionDefinition: 从版本 %s 读取的定义如下:\n%s\n", id, string(definitionJSON))
	}
	return result.Definition, nil
}

// UpdateVersionDefinition 更新指定版本的协议定义
func UpdateVersionDefinition(id string, definition model.ProtocolDefinition) error {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("无效的版本 ID 格式")
	}

	fmt.Println("[DB DEBUG] UpdateVersionDefinition: 即将写入数据库的定义内容:")
	definitionToDbJSON, errDbJSON := json.MarshalIndent(definition, "", "  ")
	if errDbJSON != nil {
		fmt.Printf("[DB WARN] UpdateVersionDefinition: 序列化准备写入DB的 definition 为 JSON 失败: %v\n", errDbJSON)
	} else {
		fmt.Printf("[DB DEBUG] UpdateVersionDefinition: 准备写入版本 %s 的定义如下:\n%s\n", id, string(definitionToDbJSON))
	}
	fmt.Println("[DB DEBUG] UpdateVersionDefinition: 检查准备写入DB的Next规则:")
	for key, steps := range definition {
		fmt.Printf("[DB DEBUG]   Protocol Key: %s\n", key)
		for i, step := range steps {
			fmt.Printf("[DB DEBUG]     Step Index: %d, Desc: %s, Skip: %v\n", i, step.Desc, step.Skip)
			if step.Next != nil {
				for j, rule := range step.Next {
					fmt.Printf("[DB DEBUG]       Rule Index: %d, Condition: '%s', Target: '%s'\n", j, rule.Condition, rule.Target)
				}
			} else {
				fmt.Printf("[DB DEBUG]       Step %d has no Next rules\n", i)
			}
		}
	}

	update := bson.M{
		"$set": bson.M{
			"definition": definition,
			"updatedAt":  primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	result, err := collection.UpdateOne(ctx, bson.M{"_id": objID}, update)
	if err != nil {
		return fmt.Errorf("更新协议定义失败: %w", err)
	}
	if result.MatchedCount == 0 {
		return errors.New("未找到要更新定义的版本")
	}
	fmt.Printf("[DB INFO] UpdateVersionDefinition: 版本 %s 的定义已成功更新到数据库, Matched: %d, Modified: %d\n", id, result.MatchedCount, result.ModifiedCount)
	return nil
}

// CheckVersionExists 检查指定协议下是否存在特定版本号
func CheckVersionExists(protocolID string, version string) (bool, error) {
	collection := versionCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objPID, err := primitive.ObjectIDFromHex(protocolID)
	if err != nil {
		// 直接返回错误，而不是布尔值，让调用者知道 ID 格式有问题
		return false, errors.New("无效的协议 ID 格式")
	}

	filter := bson.M{
		"protocolId": objPID,
		"version":    version,
	}

	// 使用 CountDocuments 来检查是否存在匹配的文档
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		// 处理可能的数据库查询错误
		return false, fmt.Errorf("检查版本存在性时出错: %w", err)
	}

	// 如果 count 大于 0，表示版本已存在
	return count > 0, nil
}
