# JParser 使用说明

## 简介

JParser 是 GoGate 网关中用于解析 JSON 格式数据的核心组件。它允许用户通过灵活的表达式从输入的 JSON 数据中提取信息，并将其转换为网关内部标准化的数据点 (`pkg.Point`) 格式。

当您的数据源以 JSON 格式提供数据时（例如，来自 HTTP API、MQTT 消息、Kafka 主题或文件），JParser 就是理想的选择。

## 核心概念

-   **JSON 到数据点 (JSON to Points)**: JParser 的主要功能是将一个输入的 JSON 对象或数组，根据配置转换成一个或多个结构化的数据点 (`pkg.Point`)。每个数据点都包含一组标签 (tags) 和一组字段 (fields)。
-   **表达式驱动 (Expression-Driven)**: 数据的提取、转换以及最终数据点中标签和字段的构造逻辑，完全由用户定义的表达式来控制。
-   **`expr-lang` 引擎**: JParser 底层使用强大的 `expr-lang` ([https://github.com/expr-lang/expr](https://github.com/expr-lang/expr)) 作为其表达式求值引擎。建议用户查阅 `expr-lang` 的官方文档以了解其完整的语法和高级功能。

## 配置

JParser 的配置在 GoGate 网关的主配置文件 (通常是 `config.yaml`) 的 `parser` 部分进行。要启用 JParser，您需要将 `parser.type` 设置为 `"jparser"` (或实际注册 JParser 时使用的类型名)，并在 `parser.para` 中提供其特定配置。

```yaml
# config.yaml 示例片段
parser:
  para:
    # JParser 的特定配置在此处定义
    points:
      - tag:
          source_type: "'json_api'" # 表达式，注意字符串字面量需要引号
          # ...更多标签定义...
        field:
          # ...字段定义...
    globalMap:
      default_location: "Building A"
      # ...更多全局变量定义...
```

### `para` 配置详解

`parser.para` 对象包含以下关键字段：

-   **`points`** (`array`, 必需):
    这是一个数组，其中每个元素都是一个对象，定义了一组规则，用于从当前输入的 JSON 数据生成一个 `pkg.Point`。您可以定义多个这样的元素，以便从单个 JSON 输入中生成多个不同的 `pkg.Point`。

    每个 `point` 对象的结构如下:
    -   **`tag`** (`map[string]string`, 可选):
        -   描述：一个键值对映射，用于定义此数据点的标签。
        -   键 (string): 标签的名称。
        -   值 (string): 一个 `expr-lang` 表达式字符串。该表达式的计算结果将作为此标签的值。
    -   **`field`** (`map[string]string`, 可选):
        -   描述：一个键值对映射，用于定义此数据点的字段。
        -   键 (string): 字段的名称。
        -   值 (string): 一个 `expr-lang` 表达式字符串。该表达式的计算结果将作为此字段的值。

    *注意：对于每个 `point` 定义，至少应配置一个 `tag` 或一个 `field`。*

-   **`globalMap`** (`map[string]interface{}`, 可选):
    -   描述：一个键值对映射，用于定义全局变量。这些全局变量可以在 `points` 定义中所有标签和字段的表达式内通过 `GlobalMap['yourKey']` 的形式访问。
    -   键 (string): 全局变量的名称。
    -   值 (any): 全局变量的值 (可以是字符串、数字、布尔等JSON兼容类型)。

## 表达式编写指南

### 基本语法
所有在 `tag` 和 `field` 中定义的表达式都遵循 `expr-lang` 的语法。

### 可用数据与变量

在您的表达式中，可以使用以下预定义变量：

-   **`Data`**:
    -   描述：此变量代表当前正在处理的、已经过反序列化（unmarshalled）的 JSON 输入数据。
    -   访问方式：
        -   如果输入 JSON 是一个对象 (map): `Data['propertyName']`
        -   如果输入 JSON 是一个数组: `Data[index]` (例如 `Data[0]`)
        -   访问嵌套结构: `Data['parentObject']['childProperty']`, `Data['propWithArray'][0]['itemProperty']`

-   **`GlobalMap`**:
    -   描述：此变量代表在 `parser.para.globalMap` 中定义的全局变量映射。
    -   访问方式: `GlobalMap['yourGlobalVariableName']`

### 常用操作与函数示例 (基于 `expr-lang`)

以下是一些常用的 `expr-lang` 操作和内置函数，可用于构建您的表达式：

-   **算术运算**: `Data['value'] * 100 + GlobalMap['offset']`
-   **比较运算**: `Data['status'] == 'active'`, `Data['count'] > 10`
-   **逻辑运算**: `Data['isValid'] && (Data['type'] == 'A' || Data['type'] == 'B')`
-   **条件表达式 (三元运算符)**: `Data['temperature'] > 30 ? 'high' : 'normal'`
-   **成员测试 (检查键是否存在于对象中)**: `'optional_field' in Data` (返回 `true` 或 `false`)
-   **Nil 合并操作符**: `Data['reading'] ?? 0` (如果 `Data['reading']` 是 `nil` 或者 `Data` 中不存在 `reading` 键，则表达式的结果为 `0`)
-   **字符串操作**:
    -   连接: `Data['firstName'] + ' ' + Data['lastName']`
    -   函数: `upper(Data['id'])`, `lower(Data['name'])`, `Data['message'] contains 'error'`, `startsWith(Data['code'], 'ERR_')`
-   **长度获取**: `len(Data['items'])` (用于数组或字符串)
-   **数组/切片操作**: `Data['values'][0]`, `Data['values'][1:3]`, `filter(Data['items'], {.price > 100})`, `map(Data['users'], {.name})`

*提示：请查阅 `expr-lang` 的[官方文档](https://github.com/expr-lang/expr/blob/master/docs/Language-Definition.md)以获取所有内置函数、操作符的完整列表和详细说明。*

### 返回值与类型
-   表达式的计算结果将直接作为其定义的标签或字段的值。
-   `expr-lang` 通常会自动处理数据类型。但在某些复杂情况下，您可能需要使用类型转换函数，例如 `int(Data['count_str'])` 或 `string(Data['numeric_id'])`。
-   **重要**: 在表达式中表示静态字符串值时，必须用单引号或双引号将其括起来。例如：`tag_name: "'static_string'"` 或 `field_name: ""another_string""`。

## 示例

### 示例 1: 基本字段和标签映射

**输入 JSON**:
```json
{
  "deviceId": "sensor-001",
  "timestamp": 1678886400,
  "payload": {
    "temperature": 25.5,
    "humidity": 60.1
  },
  "location": "room_A",
  "active": true
}
```

**`config.yaml` 中 `parser.para` 配置**:
```yaml
# parser.type: "jparser"
# para:
points:
  - tag:
      id: "Data['deviceId']"
      loc: "Data['location']"
      status: "Data['active'] ? 'ON' : 'OFF'" # 条件表达式
    field:
      temp: "Data['payload']['temperature']"
      hum: "Data['payload']['humidity']"
      ts: "Data['timestamp']" # 假设时间戳是数值类型
      source: "'json_sensor_feed'" # 静态字符串标签值
```

**预期输出 (`pkg.Point` 的简化表示)**:
```
Point {
  Tag: {"id": "sensor-001", "loc": "room_A", "status": "ON"},
  Field: {"temp": 25.5, "hum": 60.1, "ts": 1678886400, "source": "json_sensor_feed"}
}
```

### 示例 2: 使用 `GlobalMap` 和处理可能不存在的字段

**输入 JSON**:
```json
{
  "value": 42,
  "status_code": 0
}
```
(假设此JSON有时可能没有 `optional_value` 字段)

**`config.yaml` 配置**:
```yaml
# parser.type: "jparser"
# para:
points:
  - tag:
      region: "GlobalMap['default_region']"
      processed_by: "GlobalMap['processor_version']"
    field:
      original_value: "Data['value']"
      adjusted_value: "Data['value'] * GlobalMap['multiplier']"
      status_text: "Data['status_code'] == 0 ? 'success' : 'error'"
      optional_data: "('optional_value' in Data ? Data['optional_value'] : 'default_if_missing')"
      # 或者使用 nil coalescing (如果 'optional_value' 不存在，Data['optional_value'] 会是 nil)
      # optional_data_coalesced: "Data['optional_value'] ?? 'default_if_missing_or_null'"
globalMap:
  default_region: "EMEA"
  multiplier: 1.1
  processor_version: "v2.1"
```

**预期输出**:
```
Point {
  Tag: {"region": "EMEA", "processed_by": "v2.1"},
  Field: {
    "original_value": 42,
    "adjusted_value": 46.2,
    "status_text": "success",
    "optional_data": "default_if_missing"
    # "optional_data_coalesced": "default_if_missing_or_null"
  }
}
```

### 示例 3: 处理数组和提取特定元素

**输入 JSON**:
```json
{
  "sensor_name": "env_sensor_alpha",
  "readings": [
    {"type": "temperature", "value": 22.3, "unit": "C"},
    {"type": "humidity", "value": 58.0, "unit": "%"},
    {"type": "pressure"}
  ],
  "battery_level": null
}
```

**`config.yaml` 配置**:
```yaml
# parser.type: "jparser"
# para:
points:
  - tag:
      sensor: "Data['sensor_name']"
    field:
      # 安全地访问数组元素及其属性
      temp_value: "len(Data['readings']) > 0 && 'value' in Data['readings'][0] ? Data['readings'][0]['value'] : nil"
      temp_unit: "len(Data['readings']) > 0 && 'unit' in Data['readings'][0] ? Data['readings'][0]['unit'] : 'N/A'"

      # 使用 filter 查找特定类型的读数 (更健壮的方式)
      humidity_reading: "len(filter(Data['readings'], {.type == 'humidity'})) > 0 ? filter(Data['readings'], {.type == 'humidity'})[0]['value'] : nil"

      # 处理 null 和不存在的字段
      pressure_value: "len(filter(Data['readings'], {.type == 'pressure'})) > 0 ? (filter(Data['readings'], {.type == 'pressure'})[0]['value'] ?? 'N/A') : 'No pressure reading'"
      battery: "Data['battery_level'] ?? 100" # 如果battery_level是null或不存在，则为100
      reading_count: "len(Data['readings'])"
```

**预期输出**:
```
Point {
  Tag: {"sensor": "env_sensor_alpha"},
  Field: {
    "temp_value": 22.3,
    "temp_unit": "C",
    "humidity_reading": 58.0,
    "pressure_value": "N/A",
    "battery": 100,
    "reading_count": 3
  }
}
```

### 示例 4: 从单个JSON输入生成多个数据点

**输入 JSON**:
```json
{
  "system_id": "SYS01",
  "metrics": {
    "cpu_load_avg": 0.75,
    "cpu_temp": 65.2,
    "memory_usage_mb": 1024,
    "memory_total_mb": 4096
  },
  "disk_partitions": [
    {"name": "/dev/sda1", "used_gb": 50, "total_gb": 100},
    {"name": "/dev/sdb1", "used_gb": 120, "total_gb": 500}
  ]
}
```

**`config.yaml` 配置**:
```yaml
# parser.type: "jparser"
# para:
points:
  - # 第一个数据点: CPU 相关指标
    tag:
      metric_group: "'cpu'"
      sys_id: "Data['system_id']"
    field:
      load_avg: "Data['metrics']['cpu_load_avg']"
      temperature: "Data['metrics']['cpu_temp']"
  - # 第二个数据点: 内存相关指标
    tag:
      metric_group: "'memory'"
      sys_id: "Data['system_id']"
    field:
      usage_mb: "Data['metrics']['memory_usage_mb']"
      total_mb: "Data['metrics']['memory_total_mb']"
      usage_percent: "(Data['metrics']['memory_usage_mb'] / Data['metrics']['memory_total_mb']) * 100"
  # 可以为每个 disk_partitions 生成一个点 (高级用法，可能需要结合 map 或 forEach (如果 expr-lang 支持))
  # JParser 当前配置结构 (每个 point object 生成一个点) 更适合直接映射。
  # 若要为数组中每个元素生成点，通常需要外部配置多次调用或更复杂的解析逻辑。
  # 此处简化为只取第一个磁盘分区作为示例：
  - # 第三个数据点: 第一个磁盘分区信息
    tag:
      metric_group: "'disk'"
      sys_id: "Data['system_id']"
      partition_name: "len(Data['disk_partitions']) > 0 ? Data['disk_partitions'][0]['name'] : 'N/A'"
    field:
      used_gb: "len(Data['disk_partitions']) > 0 ? Data['disk_partitions'][0]['used_gb'] : 0"
      total_gb: "len(Data['disk_partitions']) > 0 ? Data['disk_partitions'][0]['total_gb'] : 0"

```

**预期输出 (三个 `pkg.Point` 对象)**:
```
// Point 1 (CPU)
Point {
  Tag: {"metric_group": "cpu", "sys_id": "SYS01"},
  Field: {"load_avg": 0.75, "temperature": 65.2}
}

// Point 2 (Memory)
Point {
  Tag: {"metric_group": "memory", "sys_id": "SYS01"},
  Field: {"usage_mb": 1024, "total_mb": 4096, "usage_percent": 25.0}
}

// Point 3 (Disk - first partition)
Point {
  Tag: {"metric_group": "disk", "sys_id": "SYS01", "partition_name": "/dev/sda1"},
  Field: {"used_gb": 50, "total_gb": 100}
}
```

## 输出格式

JParser 最终会生成一个 `pkg.Point` 对象的列表 (`[]*pkg.Point`)。列表中的每个 `pkg.Point` 对象都包含以下两个主要部分：

-   **`Tag (map[string]interface{})`**: 一个字符串到任意类型值的映射，代表该数据点的标签集合。
-   **`Field (map[string]interface{})`**: 一个字符串到任意类型值的映射，代表该数据点的字段（或指标）集合。

这些 `Point` 对象随后会被统一封装在 `pkg.PointPackage` 结构中，由网关的数据处理流水线的后续阶段（例如 `Sink` 模块）进一步处理和分发。

## 注意事项与最佳实践

-   **表达式健壮性**: 当处理可能缺失的JSON字段或复杂的嵌套结构时，强烈建议使用成员测试操作符 (`'key' in Data`) 来检查字段是否存在，或者使用 Nil 合并操作符 (`??`) 来为可能为 `nil` 或不存在的字段提供安全的默认值。这有助于避免表达式在求值过程中因空指针或未定义变量而导致的运行时错误。
-   **性能考虑**: 虽然 `expr-lang` 引擎本身性能优异，但非常复杂或数量庞大的表达式仍然可能对网关的整体吞吐量产生影响。应尽量保持表达式的简洁和高效。如果数据转换逻辑异常复杂，可以考虑将其部分逻辑在数据源端或网关的后续专用处理插件中实现。
-   **错误诊断与日志**: 当JSON数据本身格式错误，或者您编写的表达式存在语法问题或在求值时发生错误（例如，类型不匹配、访问不存在的字段未做保护），JParser 会在网关的日志中记录相关的错误信息。仔细检查这些日志是诊断和解决配置问题的关键。
-   **查阅 `expr-lang` 官方文档**: 对于 `expr-lang` 引擎提供的所有内置函数、操作符、语法特性以及更高级的用法（如闭包、自定义函数等），请务必参考其[官方文档](https://github.com/expr-lang/expr/blob/master/docs/Language-Definition.md)。这将帮助您充分利用其强大功能。
-   **字符串字面量**: 再次强调，在表达式中直接使用的静态字符串值（即非来自`Data`或`GlobalMap`的字符串），必须用单引号 (`'`) 或双引号 (`"`) 包围。例如：`field_name: "'some_static_string'"`。否则，`expr-lang` 会尝试将其解析为变量名或函数名，从而导致错误。
```
