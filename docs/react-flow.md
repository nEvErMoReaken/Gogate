# 协议编排 - 需求清单

本文档概述了使用 React Flow 构建协议编排界面的需求。

## I. 核心功能

1.  **可视化流程编辑器:** 实现一个 React Flow 画布，用于可视化地表示 YAML 中定义的协议解析逻辑。
2.  **YAML <-> Flow 转换:**
    *   **YAML 到 Flow:** 能够解析协议版本的 YAML 配置，并将其准确地渲染为 React Flow 图表（节点和边）。
    *   **Flow 到 YAML:** 能够将 React Flow 图表的当前状态（节点、边、数据）转换回等效且有效的 YAML 配置字符串。
    * 切换位于最上方，是一个tab形式
3.  **双向切换:** 提供机制（例如按钮）让用户在 React Flow 可视化编辑器和原始 YAML 文本编辑器视图之间轻松切换。在一个视图中所做的更改应能被转换并反映在另一个视图中（从 YAML 切换回 Flow 时，可能需要显式的“应用”或“同步”操作来处理潜在的解析错误）。
4.  **保存:** 主要需要保存的是生成的 YAML 配置。 "保存" 操作应触发 Flow -> YAML 转换（如果在 Flow 视图中），或使用 YAML 编辑器的内容（如果在 YAML 视图中，经过验证后），并通过 API 调用更新协议版本的配置。

## II. 可视化元素 (React Flow)

1.  **自定义节点:**
    *   **`SectionNode`:** 视觉上独特的节点，显示关键信息（例如 `desc`, `size`）。必须存储与 YAML `Section` 相关的所有数据 (`desc`, `size`, `Label`, `Dev`, `Vars`)。
    *   **`SkipNode`:** 简单的节点，显示要跳过的字节数。存储 `skip` 值。
    *   **`LoopNode` (可视化分组):** 一种机制（例如 React Flow 的分组功能），用于可视化地组织 YAML 中检测到的构成循环结构的节点。此节点应直观地表示循环结构。
2.  **边:**
    *   **默认边:** 根据 YAML 列表顺序自动连接节点，除非被 `Next` 规则覆盖。应具有视觉区分（例如虚线或灰色）。
    *   **条件边:** 表示 `Next` 规则。源自 `SectionNode`。应将 `condition` 显示为可编辑的标签。连接到与 `target` `Label` 对应的节点。目标为 "END" 的边应做适当处理（例如指向一个可视化的“结束”元素或采用不同样式）。

## III. 交互与编辑

1.  **节点选择与编辑:**
    *   单击任何节点 (`Section`, `Skip`, 可能包括 `LoopNode`) 应打开一个侧边面板（详见下文）。
    *   侧边面板将显示一个针对所选节点类型的表单，允许编辑其关联数据 (`desc`, `size`, `skip`, `Label`, `Dev`, `Vars`, 循环条件等)。
    *   在侧边面板表单中编辑字段应更新 React Flow 状态中相应节点的数据。
    *   **`Dev` / `Vars` 编辑:** 在侧边面板表单中提供用户友好的方式来添加、编辑和删除 `Dev` 和 `Vars` 的键值对（例如动态输入对列表）。需要在 UI 中考虑对动态设备名称 (`"dev_${var}"`) 的支持。
    *   **表达式:** 表达式 (`Dev`/`Vars`/`condition`) 的输入字段暂时使用简单的文本输入框。
2.  **边编辑:**
    *   单击*条件*边应打开侧边面板。
    *   侧边面板应允许编辑与该边关联的 `condition` 字符串。
3.  **添加节点:** 提供添加新的 `Section` 或 `Skip` 节点的机制（例如工具栏、从选项板拖放）。新节点应可插入到现有节点之间。
4.  **删除节点/边:** 允许用户删除节点和条件边。删除节点应能适当地处理重连或移除关联边。
5.  **连接节点 (条件边):** 提供直观的方式在 `SectionNode` 之间绘制*新的*条件边。此操作应在侧边面板中提示输入 `condition`。
6.  **循环创建/编辑 (高级):**
    *   **检测:** YAML -> Flow 转换必须能够可靠地检测循环模式。
    *   **可视化:** 使用 `LoopNode` 分组。
    *   **编辑:** 允许修改循环参数（可能通过在侧边面板中编辑相关节点上的底层 `Vars` 和 `Next` 规则）。需要处理将节点可视化地添加*到*或移*出*循环组的操作。

## IV. 布局与 UI

1.  **主面板:** React Flow 画布应占据主要内容区域。
2.  **侧边面板:**
    *   当选中节点或条件边时，一个面板（例如抽屉或固定的侧边栏）应从右侧滑入/出现。
    *   面板样式应保持一致（例如圆角，使用 Shadcn UI 组件）。
    *   它应包含用于编辑所选元素属性的表单。
    *   当选择更改或清除时，它应关闭或更新。
3.  **工具栏 (可选但推荐):** 可以提供用于添加节点、触发 YAML 转换/查看、保存等的按钮。
4.  **自动布局:** 加载 YAML 或进行重大结构更改时，应应用自动布局算法（例如 Dagre）来合理地定位节点。用户仍应能够手动调整位置。





node 转 yaml

流程
1. 从开始节点开始 遍历（开始不需要转为yaml项，仅在node中表示哪个是主流程—）
    - 遍历到每个节点，追加到yaml列表的后方，如果有上面分配给自己的label，置为自己的label
    - 如果有多个子节点
        - 本节点按照优先级创建next rules列表序，有几个节点就有几条rules
        - 构建next rule时，给每一个子节点分配一个label，这个label可以是 当前下标-n的形式，例如，当前是第3个节点，有3个子节点，分别按优先级为 L3-1, L3-2, L3-3
    - 如果只有一个子节点
        - 如果condition不是ture，那么还是要编写next rules，分配一个label与上面同规则。列表长度为1
        - 如果condition是ture
            - 如果下一个节点被遍历过了
                - 如果下一个节点已经有label 则 将本节点target 至为 该label
                - 如果下一个节点没有label，则编写默认为ture的label，分配label，同上述（说明此线路合并入了一条线路
    - 如果没有边，或者下一个节点为end，则终止
- 采用DFS方式遍历，遍历到每个节点时，优先走入优先级最高的节点分支
-
