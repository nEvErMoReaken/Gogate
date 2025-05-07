import type { Protocol, ProtocolVersion, GatewayConfig, GlobalMap } from './+types/protocols';

// 定义通用的 API 响应类型
interface ApiResponse<T> {
    data?: T;
    error?: string;
    status: number;
}

// 基础 API 请求函数，包含错误处理
async function apiRequest<T>(
    url: string,
    options: RequestInit = {}
): Promise<ApiResponse<T>> {
    try {
        const response = await fetch(url, options);
        const status = response.status;

        // 尝试解析响应
        try {
            const data = await response.json();
            if (!response.ok) {
                // 处理错误响应
                const errorMessage = data.error || data.message || `请求失败 (${status})`;
                console.error(`API Error (${status}):`, errorMessage, 'Raw data:', data);
                return { error: errorMessage, status };
            }
            // 成功响应
            return { data: data as T, status };
        } catch (parseError) {
            // 解析 JSON 失败
            if (!response.ok) {
                console.error(`API Error (${status}), JSON parse failed:`, parseError);
                // 尝试从文本读取错误（如果后端可能返回非 JSON 错误）
                try {
                    const textError = await response.text();
                    return { error: textError || `请求失败 (${status})`, status };
                } catch (textErrorErr) {
                    return { error: `请求失败 (${status})`, status };
                }
            }
            // 空响应但状态码正常
            return { data: {} as T, status };
        }
    } catch (networkError) {
        // 网络错误
        console.error('Network Error:', networkError);
        return { error: '网络错误，无法连接到服务器', status: 0 };
    }
}

// API 客户端
export const API = {
    protocols: {
        // 获取所有协议
        getAll: async (): Promise<ApiResponse<Protocol[]>> => {
            return apiRequest<Protocol[]>('/api/v1/protocols');
        },

        // 获取特定协议
        getById: async (id: string): Promise<ApiResponse<{ protocol: Protocol }>> => {
            return apiRequest<{ protocol: Protocol }>(`/api/v1/protocols/${id}`);
        },

        // 创建新协议
        create: async (data: { name: string; description?: string }): Promise<ApiResponse<Protocol>> => {
            return apiRequest<Protocol>('/api/v1/protocols', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        },

        // 更新协议
        update: async (id: string, data: { name: string; description?: string }): Promise<ApiResponse<Protocol>> => {
            return apiRequest<Protocol>(`/api/v1/protocols/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        },

        // 添加更新协议配置的接口
        updateConfig: async (id: string, config: GatewayConfig): Promise<ApiResponse<Protocol>> => {
            return apiRequest<Protocol>(`/api/v1/protocols/${id}/config`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });
        },

        // 删除协议
        delete: async (id: string): Promise<ApiResponse<void>> => {
            return apiRequest<void>(`/api/v1/protocols/${id}`, {
                method: 'DELETE'
            });
        },

        // 与版本相关的嵌套 API
        versions: {
            // 获取协议的所有版本
            getAll: async (protocolId: string): Promise<ApiResponse<ProtocolVersion[]>> => {
                return apiRequest<ProtocolVersion[]>(`/api/v1/protocols/${protocolId}/versions`);
            },

            // 创建新版本
            create: async (
                protocolId: string,
                data: { version: string; description?: string }
            ): Promise<ApiResponse<ProtocolVersion>> => {
                return apiRequest<ProtocolVersion>(`/api/v1/protocols/${protocolId}/versions`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
            }
        },

        // 与GlobalMap相关的嵌套 API
        globalmaps: {
            // 获取协议的所有GlobalMap
            getAll: async (protocolId: string): Promise<ApiResponse<GlobalMap[]>> => {
                return apiRequest<GlobalMap[]>(`/api/v1/protocols/${protocolId}/globalmaps`);
            },

            // 创建新GlobalMap
            create: async (
                protocolId: string,
                data: { name: string; description?: string; content?: Record<string, any> }
            ): Promise<ApiResponse<GlobalMap>> => {
                return apiRequest<GlobalMap>(`/api/v1/protocols/${protocolId}/globalmaps`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
            }
        }
    },

    versions: {
        // 获取特定版本
        getById: async (id: string): Promise<ApiResponse<ProtocolVersion>> => {
            return apiRequest<ProtocolVersion>(`/api/v1/versions/${id}`);
        },

        // 更新版本
        update: async (
            id: string,
            data: { version?: string; description?: string }
        ): Promise<ApiResponse<ProtocolVersion>> => {
            return apiRequest<ProtocolVersion>(`/api/v1/versions/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        },

        // 删除版本
        delete: async (id: string): Promise<ApiResponse<void>> => {
            return apiRequest<void>(`/api/v1/versions/${id}`, {
                method: 'DELETE'
            });
        },

        // [修改] 获取版本的协议定义 (现在返回 JSON 对象)
        getDefinition: async (id: string): Promise<ApiResponse<any>> => {
            // apiRequest 默认处理 JSON
            return apiRequest<any>(`/api/v1/versions/${id}/definition`);
        },

        // [修改] 更新版本的协议定义 (现在发送 JSON 对象)
        updateDefinition: async (id: string, definitionData: any): Promise<ApiResponse<ProtocolVersion>> => {
            // 确保发送的是 JSON
            return apiRequest<ProtocolVersion>(`/api/v1/versions/${id}/definition`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(definitionData)
            });
        }
    },

    // GlobalMap相关的独立API
    globalmaps: {
        // 获取特定GlobalMap
        getById: async (id: string): Promise<ApiResponse<GlobalMap>> => {
            return apiRequest<GlobalMap>(`/api/v1/globalmaps/${id}`);
        },

        // 更新GlobalMap
        update: async (
            id: string,
            data: { name?: string; description?: string; content?: Record<string, any> }
        ): Promise<ApiResponse<GlobalMap>> => {
            return apiRequest<GlobalMap>(`/api/v1/globalmaps/${id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        },

        // 删除GlobalMap
        delete: async (id: string): Promise<ApiResponse<void>> => {
            return apiRequest<void>(`/api/v1/globalmaps/${id}`, {
                method: 'DELETE'
            });
        }
    },

    // 测试相关 API
    test: {
        section: async (data: any): Promise<ApiResponse<any>> => {
            return apiRequest<any>('/api/v1/test/section', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        }
    }
};

// 导出常用的 hooks 来使用 API
import { useState, useEffect } from 'react';

// 用于获取数据的 hook
export function useApiGet<T>(
    apiCall: () => Promise<ApiResponse<T>>,
    dependencies: any[] = []
) {
    const [data, setData] = useState<T | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let isMounted = true;

        const fetchData = async () => {
            setIsLoading(true);
            setError(null);

            const response = await apiCall();

            if (isMounted) {
                if (response.error) {
                    setError(response.error);
                } else if (response.data) {
                    setData(response.data);
                }
                setIsLoading(false);
            }
        };

        fetchData();

        return () => {
            isMounted = false;
        };
    }, dependencies);

    return { data, isLoading, error };
}
