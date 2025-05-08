# GoGate

> 一个零代码、完全依赖配置驱动的数据网关。


# Quick Start

---
```shell
docker run -d -v /path/to/config.yaml:/config.yaml -v /path/to/script:/script -v /path/to/log:/log -p 8080:8080 gogate:latest
```

# How to Use


## Step 1. 指定一份网关连接配置（数据从哪来到哪去）

```yaml
# 连接器相关配置
connector:
#  tcpServer配置.ex：
  type: tcpserver
  config:
    check_crc: true
    timeout: 5m
    url: :8080
    whiteList: false # 白名单模式
# 解析器相关配置
parser:
  type: ioReader # ioReader|json
  config:
    dir: ./script
    protoFile: proto-train2sam-v0.0.1 # 启用哪一份协议
#    method: "ConvertOldGatewayTelemetry"
# 后处理策略相关配置 可以有多个
sink:
  - type: influxdb
    enable: true
    filter: # 格式<设备类型>:<设备名称>:<遥测名称>
       - ".*"
#       - "vobc\\.info:vobc.*:RIOM_sta_3"
    config:
  #    以下是自定义配置项
      url: http://10.17.191.107:8086
      token: mK_0NkLVPW8THIYkn52eqr7enL6IinGp8d5xbXizO1mVxAEk_EuOFxZ9OKWYcwVgi2XmogD6iPcO9KQ8ToVvtQ==
      org: "byd"
      bucket: "test"
      batch_size: 2000
      tags:
      - "data_source"

```


## Step 2. 编排一份网关解析配置 (数据转换为数据点)

### 2.1 如果网关的解析是对一段字节进行解析 - Bparser


详见[字节解析说明](./BParser.md)

### 2.2 如果是对一段json进行解析 - Jparser

详见[Json解析说明](./examples/README.md)




# 程序入口


包含三个部分
- CLi工具
- Admin管理面板
- 网关主进程
