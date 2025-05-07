import type { GatewayConfig, Protocol, ProtocolVersion } from "./types";

// API基础URL
const API_BASE_URL = "/api/v1";

// 处理API响应
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.message || `请求失败: ${response.status}`);
  }
  return response.json();
}

// 网关配置API
export const GatewayAPI = {
  // 获取所有网关配置
  async getAll(): Promise<GatewayConfig[]> {
    const response = await fetch(`${API_BASE_URL}/gateways`);
    return handleResponse<GatewayConfig[]>(response);
  },

  // 获取单个网关配置
  async getById(id: string): Promise<GatewayConfig> {
    const response = await fetch(`${API_BASE_URL}/gateways/${id}`);
    return handleResponse<GatewayConfig>(response);
  },

  // 创建网关配置
  async create(gateway: Omit<GatewayConfig, "id" | "createdAt" | "updatedAt">): Promise<GatewayConfig> {
    const response = await fetch(`${API_BASE_URL}/gateways`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(gateway),
    });
    return handleResponse<GatewayConfig>(response);
  },

  // 更新网关配置
  async update(id: string, gateway: Partial<GatewayConfig>): Promise<GatewayConfig> {
    const response = await fetch(`${API_BASE_URL}/gateways/${id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(gateway),
    });
    return handleResponse<GatewayConfig>(response);
  },

  // 删除网关配置
  async delete(id: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/gateways/${id}`, {
      method: "DELETE",
    });
    return handleResponse<void>(response);
  },
};

// 协议API
export const ProtocolAPI = {
  // 获取所有协议
  async getAll(): Promise<Protocol[]> {
    const response = await fetch(`${API_BASE_URL}/protocols`);
    return handleResponse<Protocol[]>(response);
  },

  // 获取单个协议
  async getById(id: string): Promise<Protocol> {
    const response = await fetch(`${API_BASE_URL}/protocols/${id}`);
    return handleResponse<Protocol>(response);
  },

  // 创建协议
  async create(protocol: Omit<Protocol, "id" | "createdAt" | "updatedAt" | "versions">): Promise<Protocol> {
    const response = await fetch(`${API_BASE_URL}/protocols`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(protocol),
    });
    return handleResponse<Protocol>(response);
  },

  // 更新协议
  async update(id: string, protocol: Partial<Protocol>): Promise<Protocol> {
    const response = await fetch(`${API_BASE_URL}/protocols/${id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(protocol),
    });
    return handleResponse<Protocol>(response);
  },

  // 删除协议
  async delete(id: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/protocols/${id}`, {
      method: "DELETE",
    });
    return handleResponse<void>(response);
  },

  // 获取协议版本列表
  async getVersions(protocolId: string): Promise<ProtocolVersion[]> {
    const response = await fetch(`${API_BASE_URL}/protocols/${protocolId}/versions`);
    return handleResponse<ProtocolVersion[]>(response);
  },

  // 创建协议版本
  async createVersion(
    protocolId: string,
    version: Omit<ProtocolVersion, "id" | "protocolId" | "createdAt" | "updatedAt">
  ): Promise<ProtocolVersion> {
    const response = await fetch(`${API_BASE_URL}/protocols/${protocolId}/versions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(version),
    });
    return handleResponse<ProtocolVersion>(response);
  },
};

// 版本API
export const VersionAPI = {
  // 获取版本详情
  async getById(versionId: string): Promise<ProtocolVersion> {
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}`);
    return handleResponse<ProtocolVersion>(response);
  },

  // 更新版本
  async update(versionId: string, version: Partial<ProtocolVersion>): Promise<ProtocolVersion> {
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(version),
    });
    return handleResponse<ProtocolVersion>(response);
  },

  // 删除版本
  async delete(versionId: string): Promise<void> {
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}`, {
      method: "DELETE",
    });
    return handleResponse<void>(response);
  },

  // 导出YAML
  async exportYaml(versionId: string): Promise<string> {
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}/yaml`);
    return handleResponse<string>(response);
  },

  // 获取版本定义
  async getDefinition(versionId: string): Promise<{ data: any; error: string | null }> {
    console.log("API.ts - getDefinition被调用，versionId:", versionId);
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}/definition`);
    const result = await handleResponse<any>(response);
    console.log("API.ts - getDefinition服务器响应类型:", typeof result.data);
    if (result.data && typeof result.data === 'object') {
      console.log("API.ts - getDefinition响应数据对象结构:",
        Object.keys(result.data).length > 0 ?
          `顶层键数量: ${Object.keys(result.data).length}, 首键: ${Object.keys(result.data)[0]}` :
          "空对象");
    }
    return result;
  },

  // 更新版本定义
  async updateDefinition(versionId: string, definition: any): Promise<{ data: any; error: string | null }> {
    console.log("API.ts - updateDefinition被调用，versionId:", versionId);
    console.log("API.ts - 传入的definition类型:", typeof definition);
    console.log("API.ts - 传入的definition结构:",
      typeof definition === 'object' && definition !== null ?
        `顶层键数量: ${Object.keys(definition).length}, 首键: ${Object.keys(definition)[0]}` :
        "非对象类型");
    console.log("API.ts - 传入的definition数据:", JSON.stringify(definition).substring(0, 500) + "...");

    // 不再需要封装在definition对象内，直接发送解析后的对象
    const response = await fetch(`${API_BASE_URL}/versions/${versionId}/definition`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(definition),
    });

    const result = await handleResponse<any>(response);
    console.log("API.ts - updateDefinition服务器响应:", result);
    return result;
  },
};
