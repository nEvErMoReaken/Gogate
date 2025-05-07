// 网关配置类型
export interface GatewayConfig {
  id: string;
  name: string;
  description: string;
  listenAddress: string;
  port: number;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  protocolId?: string;
  protocol?: Protocol;
}

// 协议类型
export interface Protocol {
  id: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  versions: ProtocolVersion[];
}

// 协议版本类型
export interface ProtocolVersion {
  id: string;
  protocolId: string;
  versionNumber: string;
  config: ProtocolConfig;
  createdAt: string;
  updatedAt: string;
  isActive: boolean;
}

// 协议配置类型
export interface ProtocolConfig {
  nodes: ProtocolNode[];
  edges: ProtocolEdge[];
}

// 协议节点类型
export interface ProtocolNode {
  id: string;
  type: string;
  position: {
    x: number;
    y: number;
  };
  data: {
    label: string;
    [key: string]: any;
  };
}

// 协议边类型
export interface ProtocolEdge {
  id: string;
  source: string;
  target: string;
  type?: string;
  animated?: boolean;
  label?: string;
}

// 协议节点类型枚举
export enum NodeType {
  INPUT = 'input',
  PROCESS = 'process',
  OUTPUT = 'output',
  ROUTER = 'router',
} 