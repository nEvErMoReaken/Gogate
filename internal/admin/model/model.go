package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- GatewayConfig 及其子结构 (带 BSON 标签) ---

type GoDataFilter struct {
	DevFilter  string `bson:"dev_filter" json:"dev_filter"`
	TeleFilter string `bson:"tele_filter" json:"tele_filter"`
}

type GoLogConfig struct {
	LogPath           string `bson:"log_path" json:"log_path"`
	MaxSize           int    `bson:"max_size" json:"max_size"`
	MaxBackups        int    `bson:"max_backups" json:"max_backups"`
	MaxAge            int    `bson:"max_age" json:"max_age"`
	Compress          bool   `bson:"compress" json:"compress"`
	Level             string `bson:"level" json:"level"`
	BufferSize        int    `bson:"buffer_size" json:"buffer_size"`
	FlushIntervalSecs int    `bson:"flush_interval_secs" json:"flush_interval_secs"`
}

type GoStrategyConfig struct {
	Type   string                 `bson:"type" json:"type"`
	Enable bool                   `bson:"enable" json:"enable"`
	Filter []GoDataFilter         `bson:"filter" json:"filter"`
	Config map[string]interface{} `bson:"config" json:"config"` // 使用 map[string]interface{} 对应 Go 中的同类型
}

type GoParserConfig struct {
	Type   string                 `bson:"type" json:"type"`
	Config map[string]interface{} `bson:"config" json:"config"`
}

type GoConnectorConfig struct {
	Type   string                 `bson:"type" json:"type"`
	Config map[string]interface{} `bson:"config" json:"config"`
}

type GoDispatcherConfig struct {
	RepeatDataFilter []GoDataFilter `bson:"repeat_data_filter" json:"repeat_data_filter"`
}

// 主配置结构 (GatewayConfig)
// 注意：json 标签是为了与前端/API 规范一致，bson 标签是为了 MongoDB
type GatewayConfig struct {
	Parser     GoParserConfig     `bson:"parser" json:"parser"`
	Connector  GoConnectorConfig  `bson:"connector" json:"connector"`
	Dispatcher GoDispatcherConfig `bson:"dispatcher" json:"dispatcher"`
	Strategy   []GoStrategyConfig `bson:"strategy" json:"strategy"`
	Version    string             `bson:"version" json:"version"`
	Log        GoLogConfig        `bson:"log" json:"log"`
}

// --- Protocol 模型 ---

type Protocol struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Config      *GatewayConfig     `bson:"config,omitempty" json:"config,omitempty"`
	CreatedAt   primitive.DateTime `bson:"createdAt" json:"createdAt"`
	UpdatedAt   primitive.DateTime `bson:"updatedAt" json:"updatedAt"`
}

// NextRule 定义了 Section 中的路由规则
type NextRule struct {
	Condition string `bson:"condition" json:"condition" yaml:"condition"`
	Target    string `bson:"target" json:"target" yaml:"target"`
}

// SectionDefinition 代表 YAML 中的一个 Section 处理段的结构 (用于解析/验证)
// 注意：这个结构体不直接用作 ProtocolVersion.Definition 的类型
type SectionDefinition struct {
	Desc  string                            `bson:"desc" json:"desc" yaml:"desc"`
	Size  int                               `bson:"size" json:"size" yaml:"size"`
	Label string                            `bson:"Label,omitempty" json:"Label,omitempty" yaml:"Label,omitempty"`
	Dev   map[string]map[string]interface{} `bson:"Dev,omitempty" json:"Dev,omitempty" yaml:"Dev,omitempty"`
	Vars  map[string]interface{}            `bson:"Vars,omitempty" json:"Vars,omitempty" yaml:"Vars,omitempty"`
	Next  []NextRule                        `bson:"Next,omitempty" json:"Next,omitempty" yaml:"Next,omitempty"`
}

// SkipDefinition 代表 YAML 中的一个 skip 指令 (用于解析/验证)
type SkipDefinition struct {
	Skip int `bson:"skip" json:"skip" yaml:"skip"`
}

// ProtocolDefinitionStep 代表协议定义列表中的一步，使用 map 存储以支持 Section 或 Skip
// type ProtocolDefinitionStep map[string]interface{}

// 使用一个结构体来明确表示步骤，包含所有可能的字段
// 在反序列化时，根据字段的存在情况判断是 Section 还是 Skip
type ProtocolDefinitionStep struct {
	// Common fields or Section fields
	Desc  string                            `bson:"desc,omitempty" json:"desc,omitempty" yaml:"desc,omitempty"` // omitempty for skip
	Size  int                               `bson:"size,omitempty" json:"size,omitempty" yaml:"size,omitempty"` // omitempty for skip
	Label string                            `bson:"Label,omitempty" json:"Label,omitempty" yaml:"Label,omitempty"`
	Dev   map[string]map[string]interface{} `bson:"Dev,omitempty" json:"Dev,omitempty" yaml:"Dev,omitempty"`
	Vars  map[string]interface{}            `bson:"Vars,omitempty" json:"Vars,omitempty" yaml:"Vars,omitempty"`
	Next  []NextRule                        `bson:"Next,omitempty" json:"Next,omitempty" yaml:"Next,omitempty"`

	// Skip field
	Skip *int `bson:"skip,omitempty" json:"skip,omitempty" yaml:"skip,omitempty"` // Use pointer to distinguish between 0 and not present
}

// ProtocolDefinition 代表完整的协议定义，映射 YAML 顶层结构
type ProtocolDefinition map[string][]ProtocolDefinitionStep

// --- ProtocolVersion 模型 ---

type ProtocolVersion struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ProtocolID  primitive.ObjectID `bson:"protocolId" json:"protocolId"`
	Version     string             `bson:"version" json:"version"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	// 使用正确的 ProtocolDefinition 类型
	Definition ProtocolDefinition `bson:"definition,omitempty" json:"definition,omitempty"`
	CreatedAt  primitive.DateTime `bson:"createdAt" json:"createdAt"`
	UpdatedAt  primitive.DateTime `bson:"updatedAt" json:"updatedAt"`
}

// ProtocolListItem for listing protocols without versions.
type ProtocolListItem struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
	// Maybe add latest/active version info here later
}

// ProtocolVersionListItem for listing versions of a protocol.
type ProtocolVersionListItem struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Version     string             `bson:"version" json:"version"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	IsActive    bool               `bson:"is_active" json:"isActive"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

// Note: Consider adding validation tags (binding:"required") to fields
// like ProtocolConfig if a version is considered incomplete without it.
// For now, protocolConfig is not marked required to allow saving incomplete versions during editing.

// GlobalMap 表示全局映射对象，存储通用的JSON数据
type GlobalMap struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id,omitempty"`
	ProtocolID  primitive.ObjectID     `bson:"protocolId" json:"protocolId"`
	Name        string                 `bson:"name" json:"name"`
	Description string                 `bson:"description,omitempty" json:"description,omitempty"`
	Content     map[string]interface{} `bson:"content" json:"content"`
	CreatedAt   primitive.DateTime     `bson:"createdAt" json:"createdAt"`
	UpdatedAt   primitive.DateTime     `bson:"updatedAt" json:"updatedAt"`
}
