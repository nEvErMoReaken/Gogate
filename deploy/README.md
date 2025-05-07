# Gateway 容器部署指南

本指南帮助您使用Docker容器构建和部署Gateway项目的前端和后端服务。

## 目录结构

```
deploy/
├── Dockerfile.backend    - 后端服务的Dockerfile
├── Dockerfile.frontend   - 前端服务的Dockerfile
├── docker-compose.yml    - 协调各个服务的配置文件
├── build.sh              - Linux/Mac构建脚本
├── build.ps1             - Windows构建脚本
├── .env                  - 环境变量配置(将自动创建)
└── README.md             - 本文档
```

## 前置条件

1. 安装 [Docker](https://www.docker.com/get-started)
2. 安装 [Docker Compose](https://docs.docker.com/compose/install/)

## 快速开始

### Linux/Mac环境

1. 打开终端并进入deploy目录
2. 赋予构建脚本执行权限：`chmod +x build.sh`
3. 运行构建脚本：`./build.sh`
4. 根据提示选择是否清理缓存和立即部署

### Windows环境

1. 打开PowerShell并进入deploy目录
2. 确保PowerShell执行策略允许执行脚本：`Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass`
3. 运行构建脚本：`.\build.ps1`
4. 根据提示选择是否清理缓存和立即部署

## 服务说明

构建并启动容器后，以下服务将会运行：

- **前端服务**：访问 http://localhost:3000
- **后端API**：访问 http://localhost:8080

## 技术栈说明

- 后端：Go 1.24，使用Alpine Linux镜像
- 前端：Node.js，React框架，使用Alpine Linux镜像

## 配置参数

`.env`文件包含以下配置参数：

```
# 日志级别
LOG_LEVEL=info            # 日志级别
```

## 常用命令

### 启动服务
```bash
docker-compose -f deploy/docker-compose.yml up -d
```

### 停止服务
```bash
docker-compose -f deploy/docker-compose.yml down
```

### 查看日志
```bash
# 查看所有容器的日志
docker-compose -f deploy/docker-compose.yml logs

# 查看特定服务的日志（如后端）
docker-compose -f deploy/docker-compose.yml logs backend
```

### 重新构建服务
```bash
docker-compose -f deploy/docker-compose.yml build
```

## 故障排除

1. **构建失败**：检查Dockerfile中的路径是否正确，确保源代码目录结构与Dockerfile期望的一致
2. **服务无法启动**：检查端口是否被占用，可通过修改docker-compose.yml中的端口映射解决

## 生产环境部署

对于生产环境部署，建议：

1. 配置HTTPS证书
2. 调整日志级别
3. 配置适当的日志轮转策略
4. 使用Docker Swarm或Kubernetes进行容器编排

## 注意事项

- 容器使用桥接网络互相通信
- 配置文件以卷的形式挂载到容器中
- 前后端容器均使用非root用户运行，增强安全性


