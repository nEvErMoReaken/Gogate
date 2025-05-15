# 协议解析器配置文档

## 1. 配置文件概述

协议解析器配置使用YAML格式定义数据帧的解析结构和处理逻辑。通过配置，您可以定义字节流如何被分段处理、变量如何提取、设备数据点如何生成，以及处理流程如何根据条件路由。

## 2. 基本结构

配置文件结构为一个顶层键（协议名）包含一个列表，列表中每项定义一个处理节点（Section或Skip）：

协议名:
```yaml
协议名:
  ┌──────────┐
  │ section1 │
  └──────────┘
  ┌──────────┐
  │ section2 │
  └──────────┘
  ┌──────────┐
  │ section3 │
  └──────────┘
    # 更多字段...
```
例如：

```yaml
gw05-ats-mq:
  - desc: "第一个处理节点"
    size: 4
    # 更多字段...
  - skip: 3  # Skip节点
  - desc: "第三个处理节点"
    size: 2
    # 更多字段...
```

## 3. Section节点配置

Section是基本处理单元，处理固定大小的字节段：

| 字段 | 类型 | 说明 | 是否可空 |
|------|------|------|------|
| desc | 字符串 | 节点描述，用于日志和调试 |  ✔️ |
| size | 整数 | 处理的字节数量 | ❌ |
| Points | 列表 | 数据点定义列表 | ✔️ |
| Vars | 映射 | 变量定义 | ✔️ |
| Label | 字符串 | 标签，用于路由跳转目标 | ✔️|
| Next | 列表 | 路由规则 | ✔️ |


还有种特殊的Section - Skip节点，它只包含一个Size属性
| 字段 | 类型 | 说明 | 是否可空 |
|------|------|------|------|
| size | 整数 | 处理的字节数量 | ❌ |


### 3.1 Points字段

定义数据点列表，每个点包含Tag（标识）和Field（字段）：

```yaml
Points:
  - Tag:
      id: "'dev1'"        # 注意：字符串字面量需要使用单引号
    Field:
      字段名1: "Bytes[0]"  # 如 "Bytes[0]"
      字段名2: "Bytes[1]"  # 如 "Bytes[1] + 10"
  - Tag:
      id: "'dev2'"
    Field:
      字段名a: "Bytes[2]"
```

### 3.2 Vars字段

定义数据处理中的变量：

```yaml
Vars:
  变量名1: "Bytes[2]"  # 如 "Bytes[2]"
  变量名2: "Vars.变量名1 * 2"  # 如 "Vars.变量名1 * 2"
```

### 3.3 表达式语法

- `Bytes[i]`：引用当前节点数据的第i个字节
- `Vars.变量名`：引用已定义的变量
- 支持算术运算：+, -, *, /, %
- 支持比较运算：==, !=, >, <, >=, <=
- 支持逻辑运算：&&, ||, !
- 字符串操作：`"'字符串字面量'"` (需要用单引号括起)，`string(变量)` (转换为字符串)，`'字符串1' + string(变量)`（字符串连接）

### 3.4 Label和Next字段

Label定义节点标签，Next定义基于条件的路由规则：

```yaml
Label: "标签名"
Next:
  - condition: "条件表达式"  # 如 "Vars.type == 0x01"
    target: "目标标签"
  - condition: "另一个条件"
    target: "另一个目标"
```

目标标签可以是：
- 其他节点的Label值
- "END"：结束处理
- "DEFAULT"：默认下一个节点

## 4. Skip节点配置

Skip节点用于跳过指定数量的字节：

```yaml
skip: 3  # 跳过3个字节
```

## 5. 动态设备名

支持使用表达式动态生成设备标识：

```yaml
Points:
  - Tag:
      id: "'device' + string(Vars.index)"  # 使用string()函数将变量转换为字符串
    Field:
      data: "Bytes[0]"
```

注意：
- 在表达式中的字符串字面量必须用单引号括起，如 `'device'`
- 使用 `string()` 函数将变量转换为字符串
- 字符串连接使用 `+` 运算符

## 6. 循环处理

可以通过变量和条件路由实现循环处理：

1. 初始化循环变量
2. 设置循环开始节点的标签
3. 设置循环条件和目标
4. 处理循环体
5. 在循环结束节点使用条件路由

例如：

```yaml
# 初始化循环变量
- desc: "初始化循环"
  size: 1
  Vars:
    loop_count: "Bytes[0]"  # 循环次数
    loop_index: "0"         # 初始索引

# 循环开始
- desc: "循环开始"
  size: 1
  Label: "loop_start"
  Points:
    - Tag:
        id: "'device_' + string(Vars.loop_index)"  # 动态设备ID
      Field:
        data: "Bytes[0]"
  Vars:
    loop_count: "Vars.loop_count - 1"  # 递减计数器
    loop_index: "Vars.loop_index + 1"  # 递增索引
  Next:
    - condition: "Vars.loop_count > 0"  # 循环条件
      target: "loop_start"              # 回到循环开始
    - condition: "true"                 # 默认条件
      target: "DEFAULT"                 # 继续下一节点
```

## 7. 配置示例

请参考 `examples/protocol/protocol_example.yml` 文件查看详细示例。

## 8. 最佳实践

1. **合理分段**：根据协议划分合理的处理段
2. **有意义的标签**：使用描述性强的标签名
3. **维护变量**：清晰定义和更新变量
4. **详细注释**：在desc字段提供充分描述
5. **正确的缩进**：保持YAML格式一致性，避免混用制表符和空格
6. **测试验证**：编写测试验证配置正确性，确保所有路径都被测试到
7. **正确的字符串格式**：字符串字面量使用单引号，如 `'deviceName'`
8. **动态字符串拼接**：使用 `string()` 函数将变量转换为字符串，如 `'prefix_' + string(Vars.index)`

## 9. 常见问题排查

1. YAML解析错误：检查缩进、确保未混用制表符与空格
2. 路由不生效：检查条件表达式、验证标签定义
3. 循环问题：确认循环变量正确递增/递减、路由条件准确
4. 动态名称失败：验证变量已定义、使用 `string()` 函数进行类型转换
5. 字符串问题：确保字符串字面量使用了单引号，如 `'deviceName'` 而不是 `deviceName`

通过合理配置，您可以实现复杂的字节流协议解析，包括分支逻辑、循环结构和动态处理流程。

## 10. 详细配置示例解析与流程说明

本章节将详细解析 `examples/protocol/protocol_example.yml` 文件中提供的各种配置示例，帮助您理解不同配置项如何协同工作以实现特定的解析逻辑。

### 10.1 基础解析示例 (`test_proto`)
本示例展示了一个简化的两阶段解析流程，说明了 `Bytes` 引用、`Vars` 的使用以及 `Points` 如何生成最终的数据。

**输入字节流:**
`[ 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08 ]`

**YAML 配置 (`test_proto`):**
```yaml
test_proto:
  - desc: "解析头部信息 (Section 1)"
    size: 4
    Points:
      - Tag:
          id: "'dev1'"
        Field:
          msg_type: "Bytes[0]"      # --> 01 (来自输入字节流)
          payload_len: "Bytes[2]"   # --> 03 (来自输入字节流)
      - Tag:
          id: "'dev2'"
        Field:
          msg_type: "Bytes[1]"      # --> 02 (来自输入字节流)
    Vars:
      test_var: "Bytes[2]"        # Vars.test_var 初始化为 03

  - desc: "解析数据体 (Section 2)"
    size: 2                       # 处理字节流中接下来的 [05 | 06]
    Points:
      - Tag:
          id: "'dev3'"
        Field:
          data1: "Bytes[0]"         # --> 05 (来自Section 2的Bytes)
      - Tag:
          id: "'dev4'"
        Field:
          data_from_var: "Vars.test_var" # --> 03 (来自Vars)
          data2: "Bytes[1]"         # --> 06 (来自Section 2的Bytes)
```

**处理流程与数据状态演变:**

1.  **处理 Section 1 ("解析头部信息")**:
    *   消耗输入字节: `[ 01 | 02 | 03 | 04 ]`
    *   **`Vars` 状态**:
        ```text
        Vars: {
          test_var: 03  // 来自 Bytes[2] (03)
        }
        ```
    *   **生成数据点**:
        ```text
        Point { Tag: { id: "dev1" }, Field: { msg_type: 01, payload_len: 03 } }
        Point { Tag: { id: "dev2" }, Field: { msg_type: 02 } }
        ```
    *   剩余字节流: `[ 05 | 06 | 07 | 08 ]`

2.  **处理 Section 2 ("解析数据体")**:
    *   消耗输入字节: `[ 05 | 06 ]` (从剩余字节流中取)
    *   **`Vars` 状态** (保持不变，因为此 Section 未修改 `Vars`):
        ```text
        Vars: {
          test_var: 03
        }
        ```
    *   **生成数据点**:
        ```text
        Point { Tag: { id: "dev3" }, Field: { data1: 05 } }
        Point { Tag: { id: "dev4" }, Field: { data_from_var: 03, data2: 06 } }
        ```
    *   剩余字节流: `[ 07 | 08 ]`

**最终结构:**
```go
type Point struct {
    Tag   map[string]any // 标识信息 (如 id: "dev1")
    Field map[string]any // 字段数据 (如 msg_type: 01)
}
```


### 10.2 路由示例 (`route_proto`)
本示例演示了如何根据从输入数据中提取的变量值，将处理流程导向不同的分支。

**YAML 配置 (`route_proto`):**
```yaml
route_proto:
  - desc: "路由头部" # Section 1
    size: 4
    Points:
      - Tag:
          id: "'dev_head'"
        Field:
          msg_type: "Bytes[0]"
          orig_val: "Bytes[2]"
    Vars:
      road: "Bytes[2]" # 将输入字节流的第3个字节 (索引2) 存入 Vars.road

  - desc: "路由决策" # Section 2
    size: 2
    Points:
      - Tag:
          id: "'dev_route_data'"
        Field:
          data1: "Bytes[0]"
          data2: "Bytes[1]"
    Next:
      - condition: "Vars.road == 0xFF" # 条件1
        target: "type1_handler"
      - condition: "Vars.road == 0x55" # 条件2
        target: "type2_handler"

  - desc: "类型1处理" # Section 3
    size: 1
    Label: "type1_handler"
    Points:
      - Tag:
          id: "'dev_type1'"
        Field:
          handler_data: "Bytes[0]"

  - desc: "类型2处理" # Section 4
    size: 3
    Label: "type2_handler"
    Points:
      - Tag:
          id: "'dev_type2'"
        Field:
          handler_data1: "Bytes[0]"
          handler_data2: "Bytes[1]"
          handler_data3: "Bytes[2]"

  - desc: "聚合处理" # Section 5 (可选)
    size: 1
    Label: "agg"
    Points:
      - Tag:
          id: "'dev_agg'"
        Field:
          agg_data: "Bytes[0]"
```

**流程说明与示例:**

**场景 1: 路由到 `type1_handler`**

*   **输入字节流:** `[ 0xAA | 0xBB | 0xFF | 0xCC | 0x11 | 0x22 | 0x77 | ... ]`
*   **1. 处理 "路由头部" (size: 4)**:
    *   消耗字节: `[ AA | BB | FF | CC ]`
    *   `Vars` 状态: `{ road: 0xFF }` (来自 `Bytes[2]`)
    *   生成数据点: `{ Tag: { id: "dev_head" }, Field: { msg_type: 0xAA, orig_val: 0xFF } }`
    *   剩余字节流: `[ 11 | 22 | 77 | ... ]`
*   **2. 处理 "路由决策" (size: 2)**:
    *   消耗字节: `[ 11 | 22 ]`
    *   生成数据点: `{ Tag: { id: "dev_route_data" }, Field: { data1: 0x11, data2: 0x22 } }`
    *   计算 `Next` 条件: `Vars.road == 0xFF` 为 `true`。
    *   跳转到: `target: "type1_handler"`。
    *   剩余字节流: `[ 77 | ... ]`
*   **3. 处理 "类型1处理" (Label: `type1_handler`, size: 1)**:
    *   消耗字节: `[ 77 ]`
    *   生成数据点: `{ Tag: { id: "dev_type1" }, Field: { handler_data: 0x77 } }`
    *   剩余字节流: `[ ... ]`
    *   *(流程继续到下一个物理节点或根据此节点的 Next 规则)*



**场景 2: 路由到 `type2_handler`**

*   **输入字节流:** `[ 0xDD | 0xEE | 0x55 | 0xFF | 0x33 | 0x44 | 0x88 | 0x99 | 0xAA | ... ]`
*   **1. 处理 "路由头部" (size: 4)**:
    *   消耗字节: `[ DD | EE | 55 | FF ]`
    *   `Vars` 状态: `{ road: 0x55 }` (来自 `Bytes[2]`)
    *   生成数据点: `{ Tag: { id: "dev_head" }, Field: { msg_type: 0xDD, orig_val: 0x55 } }`
    *   剩余字节流: `[ 33 | 44 | 88 | 99 | AA | ... ]`
*   **2. 处理 "路由决策" (size: 2)**:
    *   消耗字节: `[ 33 | 44 ]`
    *   生成数据点: `{ Tag: { id: "dev_route_data" }, Field: { data1: 0x33, data2: 0x44 } }`
    *   计算 `Next` 条件: `Vars.road == 0xFF` 为 `false`。 `Vars.road == 0x55` 为 `true`。
    *   跳转到: `target: "type2_handler"`。
    *   剩余字节流: `[ 88 | 99 | AA | ... ]`
*   **3. 处理 "类型2处理" (Label: `type2_handler`, size: 3)**:
    *   消耗字节: `[ 88 | 99 | AA ]`
    *   生成数据点: `{ Tag: { id: "dev_type2" }, Field: { handler_data1: 0x88, handler_data2: 0x99, handler_data3: 0xAA } }`
    *   剩余字节流: `[ ... ]`
    *   *(流程继续...)*

---

### 10.3 循环示例 (`loop_proto`)
此示例展示了如何利用 `Vars` 来控制循环次数，通过 `Label` 和 `Next` 实现循环跳转，并在循环内部使用动态设备名。

**YAML 配置 (`loop_proto`):**
```yaml
loop_proto:
  - desc: "循环指示块" # Section 1
    size: 4
    Points:
      - Tag:
          id: "'dev_head'"
        Field:
          msg_type: "Bytes[0]"
          head_val: "Bytes[2]"
    Vars:
      loop_count: "Bytes[3]"   # 假设读入的是循环次数 N
      loop_index: "0"          # 初始化索引

  - desc: "中间块" # Section 2
    size: 2
    Points:
      - Tag:
          id: "'dev_mid'"
        Field:
          mid_data1: "Bytes[0]"
          mid_data2: "Bytes[1]"

  - desc: "循环开始块" # Section 3
    size: 1
    Label: "loop_start"
    Points:
      - Tag:
          id: "'dev_' + string(Vars.loop_index)" # 动态设备名
        Field:
          start_marker: "Bytes[0]"
    Vars:
      loop_count: "Vars.loop_count - 1" # 次数减1
      loop_index: "Vars.loop_index + 1" # 索引加1

  - desc: "循环体" # Section 4
    size: 1
    Points:
      - Tag:
          id: "'dev_other_' + string(Vars.loop_index)" # 动态设备名
        Field:
          body_data: "Bytes[0]"

  - desc: "循环结束与判断" # Section 5
    size: 1
    Points:
      - Tag:
          id: "'dev_end_' + string(Vars.loop_index)" # 动态设备名
        Field:
          end_marker: "Bytes[0]"
    Next:
      - condition: "Vars.loop_count >= 0" # 循环条件
        target: "loop_start"
      - condition: "true"                # 退出条件
        target: "DEFAULT"

  - desc: "循环后块" # Section 6
    size: 2
    Points:
      - Tag:
          id: "'dev_after_loop'"
        Field:
          final_data1: "Bytes[0]"
          final_data2: "Bytes[1]"
```

**流程说明与示例 (假设循环次数为 2):**

*   **输入字节流:** `[ 0x01 | 0x02 | 0x03 | 0x02 | 0xAA | 0xBB | 0xC0 | 0xD0 | 0xE0 | 0xC1 | 0xD1 | 0xE1 | 0xFF | 0xEE | ... ]`
*   **1. 处理 "循环指示块" (size: 4)**:
    *   消耗字节: `[ 01 | 02 | 03 | 02 ]`
    *   `Vars` 状态: `{ loop_count: 2, loop_index: "0" }` (来自 `Bytes[3]`)
    *   生成数据点: `{ Tag: { id: "dev_head" }, Field: { msg_type: 0x01, head_val: 0x03 } }`
    *   剩余字节流: `[ AA | BB | C0 | D0 | E0 | C1 | D1 | E1 | FF | EE | ... ]`
*   **2. 处理 "中间块" (size: 2)**:
    *   消耗字节: `[ AA | BB ]`
    *   生成数据点: `{ Tag: { id: "dev_mid" }, Field: { mid_data1: 0xAA, mid_data2: 0xBB } }`
    *   剩余字节流: `[ C0 | D0 | E0 | C1 | D1 | E1 | FF | EE | ... ]`
*   **--- 循环 1 开始 ---**
*   **3. 处理 "循环开始块" (Label: `loop_start`, size: 1)**:
    *   消耗字节: `[ C0 ]`
    *   `Vars.loop_index` 当前为 `"0"` -> 生成数据点: `{ Tag: { id: "dev_0" }, Field: { start_marker: 0xC0 } }`
    *   `Vars` 更新: `{ loop_count: 1, loop_index: "1" }`
    *   剩余字节流: `[ D0 | E0 | C1 | D1 | E1 | FF | EE | ... ]`
*   **4. 处理 "循环体" (size: 1)**:
    *   消耗字节: `[ D0 ]`
    *   `Vars.loop_index` 当前为 `"1"` -> 生成数据点: `{ Tag: { id: "dev_other_1" }, Field: { body_data: 0xD0 } }`
    *   剩余字节流: `[ E0 | C1 | D1 | E1 | FF | EE | ... ]`
*   **5. 处理 "循环结束与判断" (size: 1)**:
    *   消耗字节: `[ E0 ]`
    *   `Vars.loop_index` 当前为 `"1"` -> 生成数据点: `{ Tag: { id: "dev_end_1" }, Field: { end_marker: 0xE0 } }`
    *   计算 `Next` 条件: `Vars.loop_count` (值为 `1`) `>= 0` 为 `true`。
    *   跳转到: `target: "loop_start"`。
    *   剩余字节流: `[ C1 | D1 | E1 | FF | EE | ... ]`
*   **--- 循环 2 开始 ---**
*   **6. 处理 "循环开始块" (Label: `loop_start`, size: 1)**:
    *   消耗字节: `[ C1 ]`
    *   `Vars.loop_index` 当前为 `"1"` -> 生成数据点: `{ Tag: { id: "dev_1" }, Field: { start_marker: 0xC1 } }`
    *   `Vars` 更新: `{ loop_count: 0, loop_index: "2" }`
    *   剩余字节流: `[ D1 | E1 | FF | EE | ... ]`
*   **7. 处理 "循环体" (size: 1)**:
    *   消耗字节: `[ D1 ]`
    *   `Vars.loop_index` 当前为 `"2"` -> 生成数据点: `{ Tag: { id: "dev_other_2" }, Field: { body_data: 0xD1 } }`
    *   剩余字节流: `[ E1 | FF | EE | ... ]`
*   **8. 处理 "循环结束与判断" (size: 1)**:
    *   消耗字节: `[ E1 ]`
    *   `Vars.loop_index` 当前为 `"2"` -> 生成数据点: `{ Tag: { id: "dev_end_2" }, Field: { end_marker: 0xE1 } }`
    *   计算 `Next` 条件: `Vars.loop_count` (值为 `0`) `>= 0` 为 `true`。
    *   跳转到: `target: "loop_start"`。
    *   剩余字节流: `[ FF | EE | ... ]`
*   **--- 循环 3 开始 ---**
*   **9. 处理 "循环开始块" (Label: `loop_start`, size: 1)**:
    *   消耗字节: `[ FF ]`
    *   `Vars.loop_index` 当前为 `"2"` -> 生成数据点: `{ Tag: { id: "dev_2" }, Field: { start_marker: 0xFF } }`
    *   `Vars` 更新: `{ loop_count: -1, loop_index: "3" }`
    *   剩余字节流: `[ EE | ... ]`
*   **10. 处理 "循环体" (size: 1)**:
    *   消耗字节: `[ EE ]`
    *   `Vars.loop_index` 当前为 `"3"` -> 生成数据点: `{ Tag: { id: "dev_other_3" }, Field: { body_data: 0xEE } }`
    *   剩余字节流: `[ ... ]`
*   **11. 处理 "循环结束与判断" (size: 1)**:
    *   (假设还有更多字节) 消耗字节: `[ xx ]`
    *   `Vars.loop_index` 当前为 `"3"` -> 生成数据点: `{ Tag: { id: "dev_end_3" }, Field: { end_marker: 0xx } }`
    *   计算 `Next` 条件: `Vars.loop_count` (值为 `-1`) `>= 0` 为 `false`。
    *   计算下一个条件: `condition: "true"` 为 `true`。
    *   跳转到: `target: "DEFAULT"`（进入循环后块）
    *   剩余字节流: `[ ... ]`
*   **--- 循环结束 ---**
*   **12. 处理 "循环后块" (size: 2)**:
    *   (假设至少有2个字节剩余) 消耗字节: `[ yy | zz ]`
    *   生成数据点: `{ Tag: { id: "dev_after_loop" }, Field: { final_data1: 0yy, final_data2: 0zz } }`
    *   剩余字节流: `[ ... ]`

---

### 10.4 `END` 目标路由示例 (`end_target_proto`)
此示例说明了如何在满足特定条件时，使用 `target: "END"` 来提前终止整个协议解析流程。

**YAML 配置 (`end_target_proto`):**
```yaml
end_target_proto:
  - desc: "Section A - 设置条件"
    size: 2
    Points:
      - Tag:
          id: "'dev_a'"
        Field:
          data1: "Bytes[0]"
    Vars:
      stop_flag: "Bytes[1]" # 第二个字节决定是否停止

  - desc: "Section B - 条件路由"
    size: 1
    Points:
      - Tag:
          id: "'dev_b'"
        Field:
          data_b: "Bytes[0]"
    Next:
      - condition: "Vars.stop_flag == 0xEE" # 条件：如果 stop_flag 等于 0xEE
        target: "END"                   # 特殊目标：立即结束处理
      - condition: "true"                # 默认条件
        target: "SectionC"

  - desc: "Section C - 如果未停止则执行"
    size: 1
    Label: "SectionC"
    Points:
      - Tag:
          id: "'dev_c'"
        Field:
          data_c: "Bytes[0]"
```

**流程说明与示例:**

**场景 1: 提前结束 (`stop_flag == 0xEE`)**

*   **输入字节流:** `[ 0xAA | 0xEE | 0xBB | 0xCC | ... ]`
*   **1. 处理 "Section A" (size: 2)**:
    *   消耗字节: `[ AA | EE ]`
    *   `Vars` 状态: `{ stop_flag: 0xEE }` (来自 `Bytes[1]`)
    *   生成数据点: `{ Tag: { id: "dev_a" }, Field: { data1: 0xAA } }`
    *   剩余字节流: `[ BB | CC | ... ]`
*   **2. 处理 "Section B" (size: 1)**:
    *   消耗字节: `[ BB ]`
    *   生成数据点: `{ Tag: { id: "dev_b" }, Field: { data_b: 0xBB } }`
    *   计算 `Next` 条件: `Vars.stop_flag == 0xEE` 为 `true`。
    *   执行 `target: "END"`。
    *   **整个协议处理流程立即停止。** "Section C" 不会被执行。
    *   剩余字节流: `[ CC | ... ]` (虽然还有剩余，但处理已终止)

**场景 2: 正常继续 (`stop_flag != 0xEE`)**

*   **输入字节流:** `[ 0xAA | 0xDD | 0xBB | 0xCC | ... ]` (假设 `stop_flag` 为 `0xDD`)
*   **1. 处理 "Section A" (size: 2)**:
    *   消耗字节: `[ AA | DD ]`
    *   `Vars` 状态: `{ stop_flag: 0xDD }`
    *   生成数据点: `{ Tag: { id: "dev_a" }, Field: { data1: 0xAA } }`
    *   剩余字节流: `[ BB | CC | ... ]`
*   **2. 处理 "Section B" (size: 1)**:
    *   消耗字节: `[ BB ]`
    *   生成数据点: `{ Tag: { id: "dev_b" }, Field: { data_b: 0xBB } }`
    *   计算 `Next` 条件: `Vars.stop_flag == 0xEE` 为 `false`。
    *   计算下一个条件: `condition: "true"` 为 `true`。
    *   跳转到: `target: "SectionC"`。
    *   剩余字节流: `[ CC | ... ]`
*   **3. 处理 "Section C" (Label: `SectionC`, size: 1)**:
    *   消耗字节: `[ CC ]`
    *   生成数据点: `{ Tag: { id: "dev_c" }, Field: { data_c: 0xCC } }`
    *   剩余字节流: `[ ... ]`
    *   *(流程继续...)*

## 11. 设计理念：表达式引擎、处理流程与数据生命周期

本章节旨在阐述驱动协议解析器配置背后的一些核心设计思想，包括其依赖的 `expr` 表达式语言、数据处理流程的"链式"与"路由"模型，以及关键数据的生命周期管理。这有助于更深入地理解配置项如何协同工作。

### 11.1 `expr` 表达式引擎简介

在您的YAML配置文件中，诸如 `Points` 字段中的 `Tag`/`Field`、`Vars` 字段以及 `Next` 规则中的 `condition` 字段，其动态行为是由 [Expr 表达式语言](https://expr-lang.org/docs/getting-started) 驱动的。

Expr 是一个为 Go 语言设计的简洁、快速且可扩展的表达式语言。其主要特性包括：
*   **内存安全**：防止常见的内存相关漏洞。
*   **类型安全**：在编译（加载配置）时进行类型检查，确保表达式与可用数据类型兼容。
*   **保证终止**：确保表达式求值不会无限循环。
*   **无副作用**：表达式求值本身不会修改其环境之外的状态，使得行为更可预测。

在本协议解析器中，`expr` 负责：
*   **数据提取与转换**：例如，从原始字节 (`Bytes[i]`) 中提取数据，或基于已有的 `Vars` 计算新值。
*   **条件逻辑判断**：在 `Next` 路由规则中，根据当前 `Vars` 和 `GlobalMap` 的状态决定处理流程的走向。
*   **字符串处理**：通过 `string()` 函数将变量转换为字符串，以及使用 `+` 进行字符串连接。

### 11.2 处理流程：链式思想与条件路由

协议的解析过程可以被视为一个结构化的数据流处理。

*   **"链式"思想 (Linked List Analogy)**：
    YAML配置文件中定义的 `Section` 节点（以及 `Skip` 节点）按其在列表中的顺序，自然形成了一个处理链。数据从第一个节点开始，依次（或根据路由规则跳转）流经后续节点，每个节点对数据进行一部分特定的解析或处理。

*   **条件路由 (Routing)**：
    通过 `Section` 节点中的 `Label` (定义跳转目标) 和 `Next` (定义跳转规则) 字段，实现了强大的动态路由能力。
    *   每个 `Next` 规则包含一个 `condition` (其值为一个 `expr` 表达式字符串) 和一个 `target` (目标 `Label`，或特殊值如 "END", "DEFAULT")。
    *   当一个 `Section` 处理完毕后，系统会依次计算其 `Next` 规则中的 `condition` 表达式。第一个求值为 `true` 的条件的对应 `target` 将决定下一个处理节点。
    *   这使得可以构建复杂的处理逻辑，如：
        *   **条件分支**：根据 `Vars.some_flag == true` 跳转到不同处理路径。
        *   **循环处理**：通过将 `target` 指向当前节点或之前的节点，并配合 `Vars` 中的计数器或状态变量进行条件控制。
        *   **默认路径**：通常设置一个 `condition: "true"` 的规则作为保底，确保流程总有明确的下一跳。

### 11.3 数据生命周期：`GlobalMap`, `Vars` 与 `Point`

在数据处理过程中，主要涉及以下几类数据，它们有不同的生命周期和作用：

*   **`GlobalMap` (全局映射)**
    *   **角色**：通常用于存储在整个协议解析会话期间（例如，从设备连接开始到断开，或处理一个完整文件的过程）相对稳定或不常变的数据。这可能包括设备静态信息、全局配置参数、认证状态等。
    *   **与 `expr` 的交互**：`expr` 表达式（在 `Points.Tag`, `Points.Field`, `Vars` 或 `Next.condition` 中）可以**读取** `GlobalMap` 中的值，以用于计算或条件判断。例如：`"Vars.value > GlobalMap.threshold"`。
    *   **更新**：`expr` 表达式本身不应直接修改 `GlobalMap`。`GlobalMap` 的初始化和更新通常由更高层的 Go 应用逻辑控制。
    *   **生命周期**：在一次完整的解析会话中通常只加载一次或在特定事件下更新，其内容在处理多个数据帧/包之间是共享和持续的。

*   **`Vars` (局部/帧变量)**
    *   **角色**：用于存储在处理单个数据单元（如一个网络数据包、一条消息，可称之为一"帧"）时动态提取、计算和更新的变量。它们是流程中状态传递和中间结果保存的主要载体。
    *   **与 `expr` 的交互**：
        *   **写入/修改**：`Section` 节点中的 `Points` 和 `Vars` 字段内的 `expr` 表达式的主要作用就是计算结果并创建或更新 `Vars` 中的变量。例如：`Vars: { "payloadLength": "Bytes[0] * 256 + Bytes[1]" }`。
        *   **读取**：`Next.condition` 中的 `expr` 表达式，以及后续 `Section` 中 `Points` 和 `Vars` 字段的表达式，都会读取 `Vars` 的当前值。
    *   **生命周期**：
        1.  **初始化**：在开始处理每一"帧"新数据时，`Vars` 通常会被清空或重置为一组初始状态。
        2.  **迭代更新**：随着处理流程经过各个 `Section`，`Vars` 中的内容会根据 `expr` 表达式的定义被逐步填充和修改。
        3.  **最终状态**：当一"帧"数据处理完毕，`Vars` 中就包含了该帧的所有解析结果和相关的中间状态。

*   **`Point` (输出数据点)**
    *   **角色**：代表协议解析流程的最终结构化输出。它通常是一个或一组包含有意义的测量值、状态或事件的数据记录，预备用于存储、上报或进一步分析。
    *   **与 `expr` 的交互**：`expr` 表达式直接通过 `Points` 配置中的 `Tag` 和 `Field` 表达式生成数据点。这些表达式可以引用 `Bytes[]`, `Vars` 和 `GlobalMap` 内容。
    *   **结构**：每个 `Point` 包含：
        *   `Tag`: 包含设备标识信息的映射，如 `{ "id": "device_name" }`。
        *   `Field`: 包含实际数据字段的映射，如 `{ "temperature": 25.5, "status": "ok" }`。
    *   **生命周期**：
        1.  **生成**：在每个 `Section` 处理过程中根据 `Points` 配置生成。
        2.  **聚合**：多个相同 `Tag` 的 `Point` 可能会在后续处理中被合并或聚合。
        3.  **输出**：处理完成后，`Point` 被传递到下游系统进行存储、分析或转发。

### 11.4 概念示例：简易传感器数据包解析

假设有以下数据和流程：

*   **`GlobalMap` (初始化时)**: `{ "sensor_unit": "Celsius", "data_offset": 5 }`
*   **输入数据"帧"**: 一串字节流

**处理流程概念：**

1.  **`Section_ReadType`**:
    *   `Vars`: `{ "packet_type": "Bytes[0]" }` (读取第0个字节作为包类型)
    *   `Next`:
        *   `condition: "Vars.packet_type == 1"`, `target: "Section_ProcessSensorV1"`
        *   `condition: "Vars.packet_type == 2"`, `target: "Section_ProcessSensorV2"`

2.  **`Section_ProcessSensorV1`**:
    *   `Vars`: `{ "raw_value": "Bytes[GlobalMap.data_offset]" }` (从 `GlobalMap.data_offset` 指定的偏移读取传感器原始值)
    *   `Vars`: `{ "scaled_value": "Vars.raw_value / 10.0" }` (对原始值进行缩放)
    *   `Points`:
        ```yaml
        Points:
          - Tag:
              id: "'sensor_v1'"
            Field:
              value: "Vars.scaled_value"
              unit: "GlobalMap.sensor_unit"
              raw: "Vars.raw_value"
        ```
    *   `Next`: `condition: "true"`, `target: "Section_Output"`

3.  **`Section_Output`**: (结束处理)
    *   此时 `Vars` 可能包含: `{ "packet_type": 1, "raw_value": 250, "scaled_value": 25.0, ... }`
    *   生成的 `Point` 可能是:
        ```
        {
          Tag: { "id": "sensor_v1" },
          Field: {
            "value": 25.0,       // 来自 Vars.scaled_value
            "unit": "Celsius",    // 来自 GlobalMap.sensor_unit
            "raw": 250           // 来自 Vars.raw_value
          }
        }
        ```

这个例子展示了：
*   `expr` 表达式用于从字节流 (`Bytes[...]`) 和已有变量 (`Vars...`, `GlobalMap...`) 中提取和计算新值，填充到 `Vars`。
*   `expr` 表达式用于在 `Next` 规则中进行条件判断，实现路由。
*   `GlobalMap` 提供上下文配置，`Vars` 在处理过程中累积数据，`Points` 定义如何生成最终的数据点输出。
*   字符串字面量使用单引号，如 `"'sensor_v1'"`。

通过理解这些核心概念，您可以更有效地设计和调试协议解析配置。
