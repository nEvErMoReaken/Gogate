# 网关管理平台 API 规范文档

本文档定义了网关管理平台的 API 接口规范，用于前后端开发团队参考。所有 API 端点都以 `/api/v1` 为前缀。

## 协议相关 API

### 获取协议列表

- **请求方法**: `GET`
- **URL**: `/api/v1/protocols`
- **描述**: 获取所有协议的列表
- **参数**: 无
- **响应**:
  ```json
  [
    {
      "id": "string",
      "name": "string",
      "description": "string",
      "createdAt": "string (ISO 日期)",
      "updatedAt": "string (ISO 日期)"
    }
  ]
  ```
- **使用位置**:
  - `web/app/routes/home.tsx` (首页协议列表)
  - `web/app/routes/protocols/index.tsx` (协议列表页)

### 获取协议详情

- **请求方法**: `GET`
- **URL**: `/api/v1/protocols/:protocolId`
- **描述**: 获取特定协议的详细信息
- **参数**:
  - `protocolId`: 协议 ID
- **响应**:
  ```json
  {
    "protocol": {
      "id": "string",
      "name": "string",
      "description": "string",
      "config": "object (GatewayConfig)",
      "createdAt": "string (ISO 日期)",
      "updatedAt": "string (ISO 日期)"
    }
  }
  ```
- **使用位置**:
  - `web/app/routes/protocols/detail.tsx` (协议详情页)
  - `web/app/routes/protocols/edit.tsx` (协议编辑页)
  - `web/app/routes/protocols/versions/new.tsx` (创建新版本页)

### 创建协议

- **请求方法**: `POST`
- **URL**: `/api/v1/protocols`
- **描述**: 创建一个新的协议
- **请求体**:
  ```json
  {
    "name": "string",
    "description": "string"
  }
  ```
- **响应**: 新创建的协议对象
  ```json
  {
    "id": "string",
    "name": "string",
    "description": "string",
    "config": "object (GatewayConfig)",
    "createdAt": "string (ISO 日期)",
    "updatedAt": "string (ISO 日期)"
  }
  ```
- **使用位置**: `web/app/routes/protocols/new.tsx` (创建协议页)

### 更新协议

- **请求方法**: `PUT`
- **URL**: `/api/v1/protocols/:protocolId`
- **描述**: 更新特定协议的信息
- **参数**:
  - `protocolId`: 协议 ID
- **请求体**:
  ```json
  {
    "name": "string",
    "description": "string"
  }
  ```
- **响应**: 更新后的协议对象
- **使用位置**: `web/app/routes/protocols/edit.tsx` (协议编辑页)

### (新增) 更新协议配置

- **请求方法**: `PUT`
- **URL**: `/api/v1/protocols/:protocolId/config`
- **描述**: 更新特定协议的网关配置
- **参数**:
  - `protocolId`: 协议 ID
- **请求体**: `GatewayConfig` 对象
  ```json
  {
    "parser": { ... },
    "connector": { ... },
    // ... 其他 GatewayConfig 字段
  }
  ```
- **响应**: 更新后的协议对象 (包含更新后的 config)
- **使用位置**: (待实现的协议配置编辑页)

## 协议版本相关 API

### 获取协议版本列表

- **请求方法**: `GET`
- **URL**: `/api/v1/protocols/:protocolId/versions`
- **描述**: 获取特定协议的所有版本
- **参数**:
  - `protocolId`: 协议 ID
- **响应**: 版本对象数组
  ```json
  [
    {
      "id": "string",
      "protocolId": "string",
      "version": "string",
      "description": "string",
      "createdAt": "string (ISO 日期)",
      "updatedAt": "string (ISO 日期)"
    }
  ]
  ```
- **使用位置**: `web/app/routes/protocols/detail.tsx` (协议详情页，版本列表部分)

### 创建协议版本

- **请求方法**: `POST`
- **URL**: `/api/v1/protocols/:protocolId/versions`
- **描述**: 为特定协议创建一个新版本
- **参数**:
  - `protocolId`: 协议 ID
- **请求体**:
  ```json
  {
    "version": "string",
    "description": "string"
  }
  ```
- **响应**: 新创建的版本对象
- **使用位置**: `web/app/routes/protocols/versions/new.tsx` (创建版本页)

## 版本相关 API

### 获取版本详情

- **请求方法**: `GET`
- **URL**: `/api/v1/versions/:versionId`
- **描述**: 获取特定版本的详细信息
- **参数**:
  - `versionId`: 版本 ID
- **响应**: 版本对象
  ```json
  {
    "id": "string",
    "protocolId": "string",
    "version": "string",
    "description": "string",
    "createdAt": "string (ISO 日期)",
    "updatedAt": "string (ISO 日期)"
  }
  ```
- **使用位置**:
  - `web/app/routes/versions/detail.tsx` (版本详情页)
  - `web/app/routes/versions/edit.tsx` (版本编辑页)

### 更新版本

- **请求方法**: `PUT`
- **URL**: `/api/v1/versions/:versionId`
- **描述**: 更新特定版本的信息
- **参数**:
  - `versionId`: 版本 ID
- **请求体**:
  ```json
  {
    "version": "string",
    "description": "string"
  }
  ```
- **响应**: 更新后的版本对象
- **使用位置**: `web/app/routes/versions/edit.tsx` (版本编辑页)

## 测试相关 API

### 部分测试 API

- **请求方法**: `POST`
- **URL**: `/api/v1/test/section`
- **描述**: 测试接口，具体功能待定义
- **使用位置**: `web/app/routes/test/section.tsx`

## 前端访问后端 API 的最佳实践

1. **使用一致的错误处理**:
   ```javascript
   try {
     const response = await fetch('/api/v1/protocols');
     if (!response.ok) {
       // 尝试解析错误信息
       let errorMsg = "请求失败";
       try {
         const errData = await response.json();
         errorMsg = errData.message || errData.error || errorMsg;
       } catch (e) { /* 忽略解析错误 */ }
       throw new Error(errorMsg);
     }
     const data = await response.json();
     // 处理成功响应...
   } catch (err) {
     console.error("API 调用出错:", err);
     // 显示错误给用户...
   }
   ```

2. **为所有 API 调用提供加载状态和错误处理**:
   ```javascript
   const [data, setData] = useState(null);
   const [isLoading, setIsLoading] = useState(true);
   const [error, setError] = useState(null);

   useEffect(() => {
     const fetchData = async () => {
       setIsLoading(true);
       setError(null);
       try {
         // API 调用...
         setData(result);
       } catch (err) {
         setError(err.message);
       } finally {
         setIsLoading(false);
       }
     };

     fetchData();
   }, [/* 依赖项 */]);
   ```

3. **统一管理 API URL**:
   可以考虑创建一个单独的 API 客户端文件，统一管理所有 API 调用，例如：

   ```javascript
   // api.ts
   export const API = {
     protocols: {
       getAll: () => fetch('/api/v1/protocols').then(res => res.json()),
       getById: (id) => fetch(`/api/v1/protocols/${id}`).then(res => res.json()),
       create: (data) => fetch('/api/v1/protocols', {
         method: 'POST',
         headers: { 'Content-Type': 'application/json' },
         body: JSON.stringify(data)
       }).then(res => res.json()),
       // ...更多协议相关 API
     },
     versions: {
       // 版本相关 API
     }
   };
   ```

   然后在组件中使用：
   ```javascript
   import { API } from '../api';

   // 在组件内
   const protocols = await API.protocols.getAll();
