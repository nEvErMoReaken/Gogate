import type { RouteProps } from "react-router";

// --- Go Config 同步类型定义 ---

interface GoDataFilter {
    dev_filter: string;  // 注意: Go 中是下划线，TS 中保持一致或转换为驼峰取决于序列化/反序列化行为
    tele_filter: string;
}

interface GoLogConfig {
    log_path: string;
    max_size: number;
    max_backups: number;
    max_age: number;
    compress: boolean;
    level: string;
    buffer_size: number;
    flush_interval_secs: number;
}

interface GoStrategyConfig {
    type: string;
    enable: boolean;
    filter: GoDataFilter[];
    config: Record<string, any>; // map[string]interface{}
}

interface GoParserConfig {
    type: string;
    config: Record<string, any>; // map[string]interface{}
}

interface GoConnectorConfig {
    type: string;
    config: Record<string, any>; // map[string]interface{}
}

interface GoDispatcherConfig {
    repeat_data_filter: GoDataFilter[];
}

// 主配置结构 (对应 Go Config, 排除 Others)
export interface GatewayConfig {
    parser: GoParserConfig;
    connector: GoConnectorConfig;
    dispatcher: GoDispatcherConfig;
    strategy: GoStrategyConfig[];
    version: string;
    log: GoLogConfig;
}

// --- 现有类型定义 ---

// 协议类型定义 - 添加 config 字段
export interface Protocol {
    id: string;
    name: string;
    description?: string;
    config?: GatewayConfig; // 将配置移到这里
    createdAt?: string;
    updatedAt?: string;
}

// 协议版本类型定义 - 移除 config 字段
export interface ProtocolVersion {
    id: string;
    protocolId: string;
    version: string;
    // config: GatewayConfig; // 移除 config
    description?: string;
    createdAt?: string;
    updatedAt?: string;
}

// GlobalMap类型定义 - 存储全局配置的JSON数据
export interface GlobalMap {
    id: string;
    protocolId: string;
    name: string;
    description?: string;
    content: Record<string, any>; // JSON数据
    createdAt?: string;
    updatedAt?: string;
}

// 路由相关类型扩展
export namespace Route {
    export interface MetaArgs {
        data?: any;
        params?: Record<string, string>;
        location?: any;
    }

    export interface MetaFunction {
        (args: MetaArgs): Array<Record<string, string>>;
    }

    export interface LoaderFunction {
        (args: any): Promise<any>;
    }

    export interface ClientLoaderFunction {
        (args: any): Promise<any>;
    }

    export interface ActionFunction {
        (args: any): Promise<any>;
    }
}
