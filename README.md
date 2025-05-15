

# GoGate

> 一个零代码、完全依赖配置驱动的数据网关。


---

**GoGate** 是一款灵活、高效、完全由配置驱动的数据网关。主要解决Json和Bytes场景下配置无法统一问题，在灵活性和性能之间找平衡点。

## ✨ 主要特性 (Key Features)

*   **🚀 配置驱动 (Configuration-Driven)**: 核心理念，通过直观的 YAML 文件定义完整的数据流和处理逻辑，无需编码。
*   **🔌 多种连接器 (Versatile Connectors)**: 内置并可扩展多种连接器，支持从 TCP/UDP 服务、HTTP API、消息队列等多种来源接收数据，并将处理后的数据发送到 InfluxDB、控制台、HTTP 端点等多种目标。
*   **📜 灵活的解析器 (Flexible Parsers)**:
    *   **BParser**: 强大的字节流解析引擎，支持复杂二进制协议的声明式定义和解析。
    *   **JParser**: 便捷的 JSON 数据解析与转换能力。
*   **🔧强大的数据处理策略 (Powerful Sink Strategies)**: 通过过滤器精细控制数据的流向，支持基于设备、遥测点、标签等多种条件进行数据筛选和路由。
*   **💻 可视化管理 (Admin Panel)**: (开发中/若已具备) 提供用户友好的 Web 界面，用于协议编排、网关配置管理、运行时状态监控和在线调试。
*   **📦 容器化部署 (Containerized Deployment)**: 提供 Docker 镜像和 `docker-compose` 配置，实现快速、一致的部署和扩展。
*   **高性能 (High Performance)**: 基于 Go 语言构建，为高吞吐量和低延迟场景优化。
*   **可扩展性 (Extensible)**: 模块化设计，方便开发者根据需求定制开发新的连接器、解析器或处理插件。

## 🏗️ 架构概览 (Architecture Overview)

GoGate 的核心处理流程围绕以下几个关键组件构建：

1.  **连接器 (Connectors)**: 作为数据的入口和出口。负责从外部数据源（如 TCP 连接、HTTP 请求）接收数据，或将处理后的数据点发送到外部系统（如数据库、消息队列）。
2.  **解析器 (Parser)**: 接收来自连接器的原始数据（字节流、JSON 文本等），根据预定义的协议规则（如BParser的协议文件、JParser的转换规则）将其解析和转换为结构化的数据点 (Data Points)。
3.  **数据汇/策略 (Sink/Strategies)**: 对解析器生成的数据点进行最终处理和分发。每个 Sink 可以包含多个处理策略，通过过滤器 (Filters) 确定哪些数据点应被处理，然后将数据发送到具体的目标（如 InfluxDB）。
4.  **协议编排 (Protocol Orchestration)**: 这是定义数据如何被解析和处理的核心。对于 BParser，这通常是一个详细描述字节结构和解析逻辑的配置文件；对于 JParser，则可能是一系列 JSON路径表达式和转换规则。
5.  **管理面板 (Admin Panel)**: (若适用) 提供一个 Web UI，用于创建和管理协议编排、配置网关实例、监控运行状态和进行在线测试。
6.  **网关核心 (Gateway Core)**: 驱动整个数据流的主进程，加载配置，管理各组件的生命周期。
7.  **命令行工具 (CLI Tool)**: 提供辅助管理功能，如配置校验、协议管理等。

## 🚀 快速开始 (Getting Started)

体验 GoGate 的强大功能非常简单。我们推荐使用 Docker 和 Docker Compose 进行本地环境的快速部署。

### 先决条件 (Prerequisites)

*   [Docker](https://www.docker.com/get-started)
*   [Docker Compose](https://docs.docker.com/compose/install/) (推荐用于本地设置)
*   [Git](https://git-scm.com/downloads)

### 1. 使用 Docker Compose (推荐)

这是在本地开发和测试环境中运行 GoGate 的推荐方式。

1.  **克隆项目仓库**:
    ```bash
    git clone https://github.com/YOUR_USERNAME/gogate.git
    cd gogate
    ```
    *(请将 `YOUR_USERNAME/gogate.git` 替换为您的实际仓库地址)*

2.  **进入部署目录并准备配置**:
    ```bash
    cd deploy
    ./build.sh # Linux/macOS
    # 或者用 PowerShell (Windows):
    # .\build.ps1
    ```
    此脚本将根据模板创建初始的 `.env` 文件，您可以按需修改其中的配置 (如数据库连接、端口等)。

3.  **启动服务**:
    ```bash
    docker-compose up -d
    ```

4.  **访问服务**:
    *   **管理面板 (Admin Panel)**: `http://localhost:3000` (如果您的 `docker-compose` 包含前端服务)
    *   **网关后端 API (Gateway API)**: `http://localhost:8080` (或您在 `.env` 中配置的端口)

### 2. 使用 `docker run` (单个容器部署)

如果您希望直接运行单个 GoGate 容器实例：

```bash
# 创建必要的本地目录
mkdir -p ./gogate_config ./gogate_scripts ./gogate_logs

# 运行 Docker 容器 (请将 /path/to/ 替换为实际的绝对路径)
docker run -d \
  -v /path/to/your/config.yaml:/config.yaml \
  -v /path/to/your/script_dir:/script \
  -v /path/to/your/log_dir:/log \
  -p 8080:8080 \
  YOUR_DOCKER_IMAGE_NAME:latest # 例如：gogate:latest 或您自定义的镜像名
```
参数说明:
*   `-v /path/to/your/config.yaml:/config.yaml`: 挂载您的主配置文件。
*   `-v /path/to/your/script_dir:/script`: 挂载协议定义文件或脚本所在的目录。
*   `-v /path/to/your/log_dir:/log`: 挂载日志输出目录。
*   `-p 8080:8080`: 将容器的网关端口映射到宿主机。

## ⚙️ 核心配置 (`config.yaml`)

GoGate 的行为完全由一个主配置文件（通常是 `config.yaml`）驱动。以下是主要配置项的说明：

### 连接器配置 (`connector`)

定义网关如何接收数据。

```yaml
# config.yaml
connector:
  # type: tcpserver | udpserver | http | mqtt_client | ... (支持的类型)
  type: tcpserver  # 示例：配置一个TCP服务器作为数据入口
  config:
    # TCP服务器特定配置
    url: ":8080"             # 监听的地址和端口
    timeout: "5m"            # 连接超时时间 (例如: 5m, 10s, 500ms)
    check_crc: true          # 是否进行CRC校验 (特定协议可能需要)
    whiteList: false         # 是否启用白名单模式，仅允许特定IP连接
    # buffer_size: 8192      # 读取缓冲区大小 (字节)
    # max_connections: 100   # 最大并发连接数
```

### 解析器配置 (`parser`)

指定用于解析输入数据的协议和方法。

```yaml
# config.yaml
parser:
  config:
    # protoFile: 指向在 /script 目录中定义的协议文件名 (不含扩展名)
    # 这个文件由协议编排功能生成或手动编写，定义了数据如何被解析
    protoFile: "your-protocol-definition-name"

    # method: (可选) 如果使用Go语言插件进行解析，这里指定插件中导出函数的名称。
    # method: "ConvertOldGatewayTelemetry"
```
`protoFile` 是协议编排的核心，它决定了如何将原始数据（如字节流、JSON）转换为结构化的键值对（数据点）。

### 数据汇/目标策略配置 (`sink`)

定义数据在解析后如何被处理、过滤并发送到最终目的地。可以配置多个 `sink` 实例。

```yaml
# config.yaml
sink:
  - type: "influxdb"  # Sink类型，例如: influxdb, console, http_post, kafka 等
    enable: true       # 是否启用此Sink实例
    filter:            # 过滤器规则：字符串数组，用于筛选哪些数据点会发送到此Sink
                       # 每个字符串是一个表达式，具体语法取决于GoGate的过滤引擎实现
                       # 示例:
      - ".*"           # 匹配所有数据点 (通常用于默认Sink或测试)
      # - "tags.device_type == 'sensor' && fields.temperature > 25.0" # 条件过滤
      # - "name =~ 'vobc.*' && tags.station_id == 'S101'" # 正则匹配和标签过滤
    config:
      # 以下是 InfluxDB Sink 的特定配置示例
      url: "http://10.17.191.107:8086" # InfluxDB实例地址
      token: "YOUR_INFLUXDB_TOKEN_HERE" # InfluxDB 访问令牌
      org: "byd"                       # InfluxDB组织名称
      bucket: "test"                   # InfluxDB存储桶名称
      batch_size: 2000                 # 批量写入大小
      flush_interval: "5s"           # 批量写入的刷新间隔
      tags:                            # 额外附加到所有从此Sink发出的数据点上的静态标签
        - "data_source"
        # - "location:factory_A"

  # - type: "console" # 另一个Sink实例：打印到控制台，用于调试
  #   enable: true
  #   filter:
  #     - ".*" # 打印所有数据
```

### 日志配置 (`log`)
控制网关的日志记录行为。
```yaml
# config.yaml
log:
  log_path: "./gateway.log"    # 日志文件路径 (若为空，则输出到控制台)
  max_size: 512                # 每个日志文件的最大尺寸 (MB)
  max_backups: 1000            # 保留的旧日志文件最大数量
  max_age: 365                 # 日志文件最大保留天数
  compress: true               # 是否压缩旧的日志文件
  level: "info"                # 日志级别 (例如: debug, info, warn, error, fatal)
  buffer_size: 0               # 日志写入缓冲区大小 (0表示禁用缓冲)
  flush_interval_secs: 0       # 日志刷新到磁盘的间隔 (秒, 0表示实时写入或依赖缓冲策略)
```

## 📜 数据解析逻辑定义 (Data Parsing)

GoGate 的核心能力之一是其灵活的数据解析机制。根据数据源的格式，您可以选择不同的解析方式：

*   **字节流解析 (BParser)**: 适用于处理二进制协议、自定义TCP/UDP报文等场景。您需要定义一份详细的协议描述文件。
    *   ➡️ **查阅详细指南**: [字节解析说明](./BParser.md)

*   **JSON解析 (JParser)**: 适用于处理来自HTTP API、消息队列等的JSON格式数据。通过配置可以方便地提取和转换JSON字段。
    *   ➡️ **查阅详细指南与示例**: [Json解析说明与示例](./JParser.md)


## 📦 模块与组件 (Modules & Components)

GoGate 主要包含以下几个可独立或协同工作的模块：

*   **网关核心 (Gateway Core)**: 这是运行数据处理流水线的主服务进程。它加载所有配置，启动连接器，通过解析器处理数据，并根据策略将数据发送到Sinks。
*   **管理面板 (Admin Panel)**: (若适用) 一个基于Web的用户界面，允许用户：
    *   创建、编辑和管理协议编排文件 (例如 BParser 的协议定义)。
    *   配置和管理网关实例的 `config.yaml`。
    *   监控网关的运行状态、数据流量和错误。
    *   可能提供在线的数据测试和调试工具。
*   **命令行工具 (CLI Tool)**: 一个命令行实用程序，可能用于：
    *   校验 `config.yaml` 和协议文件的语法正确性。
    *   离线测试协议解析逻辑。
    *   (未来可能) 管理网关实例的启动、停止和状态。

