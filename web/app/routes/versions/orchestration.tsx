import React from 'react';
import { useLoaderData, useNavigate, useParams } from "react-router";
import type { Route, ProtocolVersion, Protocol, GatewayConfig } from "../../+types/protocols";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { useState, useEffect, useCallback, useRef, memo, forwardRef, useImperativeHandle, useMemo } from "react";
import { API } from "../../api";
import {
    ReactFlow,
    Controls,
    Background,
    useNodesState,
    useEdgesState,
    ReactFlowProvider,
    MiniMap,
    Handle,
    Position,
    applyNodeChanges,
    applyEdgeChanges,
    useReactFlow,
    MarkerType,
    Panel,
    getBezierPath,
    EdgeLabelRenderer,
    getSmoothStepPath, // <-- Import getSmoothStepPath
} from '@xyflow/react';
import type {
    Node,
    Edge,
    NodeTypes,
    EdgeTypes,
    Connection,
    EdgeProps,
    NodeProps
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import yaml from 'js-yaml';
import { cn } from "@/lib/utils";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
    SheetClose,
} from "@/components/ui/sheet";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Popover,
    PopoverContent,
} from "@/components/ui/popover";
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip";
import { toast } from "sonner";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Trash2, PlusCircle, Plus, Settings2, Variable, ChevronDown, Play as PlayIcon, Binary } from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import dagre from '@dagrejs/dagre';
import clsx from 'clsx';
// --- MODIFIED: Uncomment LoopNode import ---
import { LoopNode } from './LoopNode'; // <-- Import LoopNode
// --- END MODIFICATION ---
import { Badge } from "@/components/ui/badge"; // <-- 添加 Badge 导入

// --- Top-Level Helper Function ---
// 辅助函数：比较两个YAML字符串实质是否相同
const isSameYamlContent = (yaml1: string, yaml2: string): boolean => {
    if (yaml1 === yaml2) return true;

    try {
        const obj1 = yaml.load(yaml1);
        const obj2 = yaml.load(yaml2);
        return JSON.stringify(obj1) === JSON.stringify(obj2);
    } catch (error) {
        console.error("YAML比较出错:", error);
        return false;
    }
};

// 定义 Loader 返回类型
interface LoaderData {
    version: ProtocolVersion | null;
    protocol: Protocol | null; // <-- Add protocol field
    error?: string;
    yamlConfig?: string;
}

// 定义节点数据类型
interface SectionNodeData {
    desc: string;
    size: number;
    Label?: string;
    Dev?: Record<string, Record<string, string>>;
    Vars?: Record<string, string>;
    Next?: Array<{ condition: string, target: string }>;
    type: 'section';
    severity?: 'info' | 'warning' | 'error';
    yamlIndex?: number;
    [key: string]: any;
}

// --- NEW: Loop Node Data Interface ---
// --- MODIFIED: Export interface ---
export interface LoopNodeData {
    type: 'loop';
    loopCondition: string;
    parentNode?: string; // Parent loop node ID
    extent?: 'parent'; // Limit dragging to parent
    [key: string]: any;
}

// --- END NEW ---

interface SkipNodeData {
    size: number;
    type: 'skip';
    yamlIndex?: number;
    parentNode?: string; // Added for potential nesting
    extent?: 'parent'; // Added for potential nesting
    [key: string]: any;
}

interface EndNodeData {
    type: 'end';
    parentNode?: string; // Added for potential nesting
    extent?: 'parent'; // Added for potential nesting
    [key: string]: any;
}

// 添加 StartNodeData 接口
interface StartNodeData {
    type: 'start';
    desc?: string; // 可选描述
    isEditing?: boolean; // 添加编辑状态
    displayIndex?: number; // 添加显示索引
    parentNode?: string; // Added for potential nesting (less likely needed for start)
    extent?: 'parent'; // Added for potential nesting (less likely needed for start)
    [key: string]: any;
}

// 边数据类型
interface EdgeData {
    condition?: string;
    isDefault?: boolean;
    priority?: number; // 改为可选字段
    [key: string]: any; // 添加索引签名以满足 Record<string, unknown> 约束
}

// 定义选中节点的状态类型
interface SelectedNodeInfo {
    id: string;
    data: SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData; // <-- Added LoopNodeData
}

// 定义 Vars 表单条目类型
interface VarEntry {
    id: number;
    key: string;
    value: string;
}

// 定义 Dev 字段条目类型
interface DevFieldEntry {
    id: number;
    key: string; // 字段名
    value: string; // 表达式
}

// 定义 Dev 设备条目类型
interface DevEntry {
    id: number;
    deviceName: string;
    fields: DevFieldEntry[];
}

// 类型守卫函数
function isSectionNodeData(data: any): data is SectionNodeData {
    return (
        typeof data === 'object' &&
        data !== null &&
        typeof data.desc === 'string' &&
        typeof data.size === 'number' &&
        data.type === 'section'
        // 可选属性不需要在这里检查，除非它们对于区分是绝对必要的
    );
}

// --- 定义节点大致尺寸 (用于布局计算) ---
const nodeWidth = 180;
const nodeHeight = 75; // 使用一个平均值
// --- 为 LoopNode 内部布局添加内边距 ---
const loopNodePaddingX = 60; // 左右内边距 (增加到 60)
const loopNodePaddingY = 60; // 上下内边距 (增加到 60)

// --- 2. 创建 getLayoutedElements 函数 (可以放在 FlowCanvas 组件外部或内部) ---
// 使用更精确的类型 Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData>
// --- MODIFIED: Add LoopNodeData to union ---
const getLayoutedElements = (nodes: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[], edges: Edge<EdgeData>[], direction = 'TB') => {
    // console.log("[getLayoutedElements] Starting layout with dynamic loop sizes, direction:", direction); // REMOVE
    const isHorizontal = direction === 'LR';

    // --- 简化的预处理：计算 LoopNode 的初始尺寸 ---
    const loopNodeSizes = new Map<string, { width: number, height: number }>();
    const loopNodes = nodes.filter(n => n.type === 'loop');

    loopNodes.forEach(loopNode => {
        const children = nodes.filter(n => n.parentId === loopNode.id);

        // 根据子节点数量估算初始尺寸
        if (children.length === 0) {
            // 空 LoopNode 处理
            const fallbackWidth = isHorizontal ? 200 : nodeWidth + 100;
            const fallbackHeight = isHorizontal ? nodeHeight + 100 : 200;
            loopNodeSizes.set(loopNode.id, {
                width: fallbackWidth,
                height: fallbackHeight
            });
            // console.log(`[getLayoutedElements] Empty LoopNode ${loopNode.id}, using default size: ${fallbackWidth}x${fallbackHeight}`); // REMOVE
        } else {
            // 基于子节点数量的简单估算
            const estimatedWidth = isHorizontal
                ? Math.max(300, nodeWidth + children.length * 100)
                : nodeWidth + 160;

            const estimatedHeight = isHorizontal
                ? nodeHeight + 160
                : Math.max(300, nodeHeight + children.length * 100);

            loopNodeSizes.set(loopNode.id, {
                width: estimatedWidth,
                height: estimatedHeight
            });
            // console.log(`[getLayoutedElements] LoopNode ${loopNode.id} with ${children.length} children, estimated size: ${estimatedWidth}x${estimatedHeight}`); // REMOVE
        }
    });
    // --- 结束简化预处理 ---

    // --- 主布局 ---
    const dagreGraph = new dagre.graphlib.Graph();
    dagreGraph.setDefaultEdgeLabel(() => ({ priority: 0 }));
    dagreGraph.setGraph({
        rankdir: direction,
        nodesep: 120,  // 增加间距
        ranksep: 170,  // 增加间距
        marginx: 30,
        marginy: 30
    });

    // 设置节点
    nodes.forEach((node) => {
        let width = nodeWidth;
        let height = nodeHeight;

        if (node.type === 'loop' && loopNodeSizes.has(node.id)) {
            const dynamicSize = loopNodeSizes.get(node.id)!;
            width = dynamicSize.width;
            height = dynamicSize.height;
            // console.log(`[getLayoutedElements - MainLayout] Using dynamic size for LoopNode ${node.id}: W=${width}, H=${height}`); // REMOVE
        } else if (node.type === 'end') {
            width = 150;
        } else if (node.type === 'skip') {
            height = 60;
        }

        dagreGraph.setNode(node.id, { width, height });
    });

    // 设置边（跳过内部边）
    edges.forEach((edge) => {
        const sourceNode = nodes.find(n => n.id === edge.source);
        const targetNode = nodes.find(n => n.id === edge.target);

        // 内部边不纳入主布局计算
        if (sourceNode?.parentId && sourceNode.parentId === targetNode?.parentId) {
            // console.log(`[getLayoutedElements] Skipping internal edge for main layout: ${edge.id}`); // REMOVE
            return;
        }

        dagreGraph.setEdge(edge.source, edge.target);
    });

    // 应用主布局
    dagre.layout(dagreGraph);

    // --- 第一阶段：设置顶层节点位置 ---
    const finalNodePositions = new Map<string, { x: number, y: number }>();

    nodes.forEach((node) => {
        // 只处理顶层节点
        if (!node.parentId || node.type !== 'section' && node.type !== 'skip' && node.type !== 'end') {
            const nodeWithPosition = dagreGraph.node(node.id);

            if (!nodeWithPosition) {
                console.warn(`[getLayoutedElements] Node ${node.id} not found in main layout graph.`);
                node.position = node.position || { x: Math.random() * 100, y: Math.random() * 100 };
                finalNodePositions.set(node.id, node.position);
                return;
            }

            // 设置连接点位置
            node.targetPosition = isHorizontal ? Position.Left : Position.Top;
            node.sourcePosition = isHorizontal ? Position.Right : Position.Bottom;

            // 计算节点左上角位置
            const layoutWidth = nodeWithPosition.width || nodeWidth;
            const layoutHeight = nodeWithPosition.height || nodeHeight;

            node.position = {
                x: nodeWithPosition.x - layoutWidth / 2,
                y: nodeWithPosition.y - layoutHeight / 2,
            };

            finalNodePositions.set(node.id, node.position);
            // console.log(`[getLayoutedElements] Set position for top-level node ${node.id}: (${node.position.x}, ${node.position.y})`); // REMOVE
        }
    });

    // --- 第二阶段：为每个 LoopNode 单独计算和应用内部布局 ---
    // console.log(`[getLayoutedElements] Applying internal layouts for LoopNodes...`); // REMOVE

    loopNodes.forEach(loopNode => {
        // 获取父 LoopNode 的位置
        const loopPosition = finalNodePositions.get(loopNode.id);
        if (!loopPosition) {
            console.warn(`[getLayoutedElements] Cannot find position for LoopNode ${loopNode.id}, skipping internal layout.`);
            return;
        }

        // 获取子节点
        const children = nodes.filter(n => n.parentId === loopNode.id);
        if (children.length === 0) {
            // console.log(`[getLayoutedElements] LoopNode ${loopNode.id} has no children, skipping internal layout.`); // REMOVE
            return;
        }

        // console.log(`[getLayoutedElements] Creating layout for LoopNode ${loopNode.id} with ${children.length} children at position: (${loopPosition.x}, ${loopPosition.y})`); // REMOVE

        // --- 简单明确的尺寸计算和样式设置 ---

        // 获取当前循环节点的尺寸
        const currentSize = loopNodeSizes.get(loopNode.id) || { width: 300, height: 300 };

        // 根据内部子节点数量和类型，计算所需的空间
        // 计算子节点总高度/宽度
        const CHILD_VERTICAL_SPACING = 100; // 垂直间距
        const CHILD_HORIZONTAL_SPACING = 100; // 水平间距

        let totalChildrenHeight = 0;
        let totalChildrenWidth = 0;

        children.forEach((child, index) => {
            let childWidth = nodeWidth;
            let childHeight = nodeHeight;
            if (child.type === 'skip') childHeight = 60;
            if (child.type === 'end') childWidth = 150;

            totalChildrenHeight += childHeight;
            totalChildrenWidth += childWidth;

            // 除了最后一个子节点，每个子节点后添加间距
            if (index < children.length - 1) {
                totalChildrenHeight += CHILD_VERTICAL_SPACING;
                totalChildrenWidth += CHILD_HORIZONTAL_SPACING;
            }
        });

        // 计算所需的最小尺寸（包含所有子节点加内边距）
        const requiredWidth = isHorizontal
            ? totalChildrenWidth + loopNodePaddingX * 2
            : Math.max(nodeWidth + loopNodePaddingX * 2, currentSize.width);

        const requiredHeight = isHorizontal
            ? Math.max(nodeHeight + loopNodePaddingY * 2, currentSize.height)
            : totalChildrenHeight + loopNodePaddingY * 2;

        // 使用更大的尺寸，确保有足够的空间
        const newWidth = Math.max(currentSize.width, requiredWidth);
        const newHeight = Math.max(currentSize.height, requiredHeight);

        // console.log(`[getLayoutedElements] Calculated size for LoopNode ${loopNode.id}: ${newWidth}x${newHeight} (required: ${requiredWidth}x${requiredHeight})`); // REMOVE

        // 设置循环节点尺寸
        // 1. 更新尺寸缓存
        loopNodeSizes.set(loopNode.id, { width: newWidth, height: newHeight });

        // 2. 直接设置节点样式
        if (!loopNode.style) loopNode.style = {};
        loopNode.style.width = `${newWidth}px`;
        loopNode.style.height = `${newHeight}px`;

        // 3. 在数据中也存储尺寸
        loopNode.data.width = newWidth;
        loopNode.data.height = newHeight;

        // 4. 确保样式对象存在
        if (!loopNode.data.style) loopNode.data.style = {};
        loopNode.data.style.width = `${newWidth}px`;
        loopNode.data.style.height = `${newHeight}px`;

        // --- 手动设置子节点位置（简单垂直或水平排列） ---

        // 计算起始位置（相对于父节点的(0,0)点，并考虑内边距）
        let relativeStartX, relativeStartY;

        if (isHorizontal) {
            // 横向布局：子节点从左到右排列
            relativeStartX = loopNodePaddingX; // 从左内边距开始
            // 垂直居中（相对于父节点高度）
            relativeStartY = (newHeight / 2) - (nodeHeight / 2); // 初始节点的Y轴居中
        } else {
            // 纵向布局：子节点从上到下排列
            // 水平居中（相对于父节点宽度）
            relativeStartX = (newWidth / 2) - (nodeWidth / 2); // 初始节点的X轴居中
            relativeStartY = loopNodePaddingY; // 从上内边距开始
        }

        // console.log(`[getLayoutedElements] Starting relative position for children inside ${loopNode.id}: (${relativeStartX}, ${relativeStartY})`); // REMOVE

        // 为每个子节点计算位置（相对于父节点）
        let currentRelativeX = relativeStartX;
        let currentRelativeY = relativeStartY;

        children.forEach((child, index) => {
            let childWidth = nodeWidth;
            let childHeight = nodeHeight;
            if (child.type === 'skip') childHeight = 60;
            if (child.type === 'end') childWidth = 150;

            // 当前子节点的相对位置 (左上角)
            let finalRelativeX, finalRelativeY;

            if (isHorizontal) {
                // X 坐标递增，Y 坐标保持垂直居中
                finalRelativeX = currentRelativeX;
                // Y 轴对齐（基于子节点高度进行微调，使其中心对齐基线）
                finalRelativeY = relativeStartY + (nodeHeight / 2) - (childHeight / 2);
            } else {
                // Y 坐标递增，X 坐标保持水平居中
                // X 轴对齐（基于子节点宽度进行微调，使其中心对齐基线）
                finalRelativeX = relativeStartX + (nodeWidth / 2) - (childWidth / 2);
                finalRelativeY = currentRelativeY;
            }

            // 设置节点相对于父节点的位置
            child.position = { x: finalRelativeX, y: finalRelativeY };

            // 设置连接点位置 (保持不变)
            child.targetPosition = isHorizontal ? Position.Left : Position.Top;
            child.sourcePosition = isHorizontal ? Position.Right : Position.Bottom;

            // --- 移除保存绝对位置到 finalNodePositions 的逻辑，因为我们现在用相对位置 ---
            // finalNodePositions.set(child.id, child.position);

            // console.log(`[getLayoutedElements] Set relative position for child ${child.id} (${index + 1}/${children.length}) in ${loopNode.id}: (${finalRelativeX}, ${finalRelativeY})`); // REMOVE

            // 更新下一个节点的起始相对位置
            if (isHorizontal) {
                currentRelativeX += childWidth + CHILD_HORIZONTAL_SPACING;
            } else {
                currentRelativeY += childHeight + CHILD_VERTICAL_SPACING;
            }
        });

        // console.log(`[getLayoutedElements] Completed layout for LoopNode ${loopNode.id}`); // REMOVE
    });

    // console.log(`[getLayoutedElements] Layout finished with direction: ${direction}`); // REMOVE
    return { nodes, edges };
};
// --- END MODIFICATION ---

// 自定义节点组件 - Adjust layout and add Dev/Vars icons
const SectionNodeComponent = ({ data, id }: { data: SectionNodeData & { yamlIndex?: number, isHovered?: boolean, onAddConnectedNode?: Function }, id: string }) => {
    // console.log('SectionNode Data:', data); // <-- 移除日志
    const devNames = data.Dev ? Object.keys(data.Dev) : [];
    const varNames = data.Vars ? Object.keys(data.Vars) : [];
    const isEditing = data.isEditing;
    const isHovered = data.isHovered;
    const onAddConnectedNode = data.onAddConnectedNode;

    // --- Component JSX ---
    return (
        // 添加 group 类用于悬停控制
        <div className={cn(
            "group section-node bg-white border-2 border-red-400 rounded-md p-3 shadow-md min-w-[180px] flex flex-col transition-all duration-150 ease-in-out relative", // 添加 relative
            isEditing && "ring-2 ring-blue-300 ring-offset-1 shadow-lg border-blue-500"
        )}>
            {/* 第一行: 节点描述 | 序号 | 大小 */}
            <div className="flex items-center justify-between whitespace-nowrap gap-3 mb-2">
                <span className="font-semibold text-gray-600 text-sm truncate flex items-center" title={`Node #${typeof data.yamlIndex === 'number' ? data.yamlIndex + 1 : '-'}`}>
                    <Binary className="h-4 w-4 mr-1.5 text-gray-500 flex-shrink-0" />
                    Node #{typeof data.yamlIndex === 'number' ? data.yamlIndex + 1 : '-'}
                </span>
                <div className="flex items-center shrink-0 space-x-2 text-xs text-gray-600">
                    <span className="flex items-center" title={`Size: ${data.size} Bytes`}>
                        📏 {data.size} B
                    </span>
                </div>
            </div>

            {/* 第二行: Dev 图标 -> Dev 徽章 */}
            {devNames.length > 0 && (
                <div className="flex items-center text-xs text-teal-700 mb-1.5 space-x-1" title={`Dev Devices: ${devNames.join(', ')}`}>
                    <span className="font-medium shrink-0 mr-1">Dev:</span>
                    <div className="flex items-center flex-wrap gap-1">
                        {devNames.map(name => (
                            <Badge key={`dev-${name}`} variant="outline" className="text-teal-700 border-teal-200 bg-teal-50 px-1.5 py-0 text-xs font-normal">
                                {name}
                            </Badge>
                        ))}
                    </div>
                </div>
            )}

            {/* 第三行: Vars 图标 -> Vars 徽章 */}
            {varNames.length > 0 && (
                <div className="flex items-center text-xs text-purple-700 mb-1.5 space-x-1" title={`Vars: ${varNames.join(', ')}`}>
                    <span className="font-medium shrink-0 mr-1">Vars:</span>
                    <div className="flex items-center flex-wrap gap-1">
                        {varNames.map(name => (
                            <Badge key={`var-${name}`} variant="secondary" className="text-purple-700 border-purple-200 bg-purple-50 px-1.5 py-0 text-xs font-normal">
                                {name}
                            </Badge>
                        ))}
                    </div>
                </div>
            )}

            {/* 第四行: 实际描述文本 */}
            {data.desc && (
                <div className="mt-2 text-xs text-gray-500 break-words">
                    {data.desc}
                </div>
            )}

            {/* Handles (unchanged) */}
            <Handle type="target" position={Position.Top} id="t" className="!top-[-5px]" />
            <Handle type="source" position={Position.Bottom} id="s" className="!bottom-[-5px]" />

            {/* --- 添加悬停显示的 '+' 按钮和菜单 --- */}
            {onAddConnectedNode && (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <button
                            className={cn(
                                "absolute -bottom-2 left-1/2 -translate-x-1/2 z-10 rounded-full bg-background border border-primary p-0.5 text-primary shadow-sm transition-opacity duration-150 flex items-center justify-center",
                                isHovered ? "opacity-100" : "opacity-0"
                            )}
                            onClick={(e) => e.stopPropagation()} // 阻止触发节点点击
                            title="添加后续节点"
                        >
                            <Plus className="h-2.5 w-2.5" />
                        </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent sideOffset={5}>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'section')}>
                            📋 添加 Section 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'skip')}>
                            ⏭️ 添加 Skip 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'end')}>
                            🏁 添加 End 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'loop')}>
                            🔄 添加 Loop 节点
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            )}
            {/* --- 结束添加 '+' 按钮 --- */}
        </div>
    );
};

// 自定义 SectionNode props 比较函数
const sectionNodePropsAreEqual = (prevProps: any, nextProps: any): boolean => {
    // 比较影响视觉渲染的 props
    if (
        prevProps.id !== nextProps.id ||
        prevProps.data.type !== nextProps.data.type ||
        prevProps.data.desc !== nextProps.data.desc ||
        prevProps.data.size !== nextProps.data.size ||
        prevProps.data.Label !== nextProps.data.Label ||
        prevProps.data.yamlIndex !== nextProps.data.yamlIndex ||
        prevProps.data.isEditing !== nextProps.data.isEditing ||
        prevProps.data.isHovered !== nextProps.data.isHovered
    ) {
        return false; // Prop 不同，需要渲染
    }

    // 比较 Dev 和 Vars 的内容 (使用 JSON.stringify)
    try {
        const prevDevString = JSON.stringify(prevProps.data.Dev || {});
        const nextDevString = JSON.stringify(nextProps.data.Dev || {});
        if (prevDevString !== nextDevString) return false;

        const prevVarsString = JSON.stringify(prevProps.data.Vars || {});
        const nextVarsString = JSON.stringify(nextProps.data.Vars || {});
        if (prevVarsString !== nextVarsString) return false;
    } catch (e) {
        console.error("Error comparing node data for SectionNode:", e);
        return false; // 比较出错，最好重新渲染
    }

    // 如果所有相关 props 都相同，则跳过渲染
    // 注意：我们故意忽略了 data.onAddConnectedNode 的比较
    return true;
};

// 使用 memo 和自定义比较函数导出 SectionNode
const SectionNode = memo(SectionNodeComponent, sectionNodePropsAreEqual);

// SkipNode - Add YAML index display
// --- MODIFY: Refactor for memo with custom comparison ---
// const SkipNode = memo(({ data, id }: { data: SkipNodeData & { yamlIndex?: number, isEditing?: boolean, isHovered?: boolean, onAddConnectedNode?: Function }, id: string }) => {
const SkipNodeComponent = ({ data, id }: { data: SkipNodeData & { yamlIndex?: number, isEditing?: boolean, isHovered?: boolean, onAddConnectedNode?: Function }, id: string }) => {
    // console.log('SkipNode Data:', data); // <-- 移除日志
    const isEditing = data.isEditing;
    const isHovered = data.isHovered;
    const onAddConnectedNode = data.onAddConnectedNode;

    return (
        // 添加 group 类用于悬停控制
        <div className={cn(
            "group skip-node bg-white border-2 border-gray-400 rounded-md p-3 shadow-md w-[180px] flex flex-col min-h-[60px] transition-all duration-150 ease-in-out relative", // 添加 relative
            isEditing && "ring-2 ring-blue-300 ring-offset-1 shadow-lg border-blue-500"
        )}>
            {/* --- 使用 yamlIndex 进行统一编号 --- */}
            <div className="flex justify-between items-center mb-1">
                <span className="font-semibold text-gray-600 text-sm">
                    ⏭️ Node #{typeof data.yamlIndex === 'number' ? data.yamlIndex + 1 : '-'}
                </span>
            </div>

            {/* Existing size display */}
            <div className="text-xs text-gray-600 mt-1">跳过: {data.size} 字节</div>

            {/* Handles */}
            <Handle type="target" position={Position.Top} id="t" className="!top-[-5px]" />
            <Handle type="source" position={Position.Bottom} id="s" className="!bottom-[-5px]" />

            {/* --- 添加悬停显示的 '+' 按钮和菜单 --- */}
            {onAddConnectedNode && (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <button
                            className={cn(
                                // --- MODIFY: Smaller Style ---
                                "absolute -bottom-2 left-1/2 -translate-x-1/2 z-10 rounded-full bg-background border border-primary p-0.5 text-primary shadow-sm transition-opacity duration-150 flex items-center justify-center",
                                isHovered ? "opacity-100" : "opacity-0"
                                // --- End MODIFY ---
                            )}
                            onClick={(e) => e.stopPropagation()}
                            title="添加后续节点"
                        >
                            {/* --- MODIFY: Smaller Icon --- */}
                            <Plus className="h-2.5 w-2.5" />
                            {/* --- End MODIFY --- */}
                        </button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent sideOffset={5}>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'section')}>
                            📋 添加 Section 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'skip')}>
                            ⏭️ 添加 Skip 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'end')}>
                            🏁 添加 End 节点
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'loop')}>
                            🔄 添加 Loop 节点
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            )}
            {/* --- 结束添加 '+' 按钮 --- */}
        </div>
    );
};

// 自定义 SkipNode props 比较函数
const skipNodePropsAreEqual = (prevProps: any, nextProps: any): boolean => {
    // 比较影响视觉渲染的 props
    if (
        prevProps.id !== nextProps.id ||
        prevProps.data.type !== nextProps.data.type ||
        prevProps.data.size !== nextProps.data.size ||
        prevProps.data.yamlIndex !== nextProps.data.yamlIndex ||
        prevProps.data.isEditing !== nextProps.data.isEditing ||
        prevProps.data.isHovered !== nextProps.data.isHovered
    ) {
        return false; // Prop 不同，需要渲染
    }

    // 如果所有相关 props 都相同，则跳过渲染
    // 注意：我们故意忽略了 data.onAddConnectedNode 的比较
    return true;
};

// 使用 memo 和自定义比较函数导出 SkipNode
const SkipNode = memo(SkipNodeComponent, skipNodePropsAreEqual);
// --- End MODIFY ---

// 新增 EndNode 组件
const EndNode = memo(({ data }: { data: EndNodeData & { isEditing?: boolean } }) => {
    const isEditing = data.isEditing;

    return (
        <div className={cn(
            "end-node bg-white border-2 border-purple-500 rounded-md p-3 shadow-md w-[150px] flex flex-col min-h-[60px] transition-all duration-150 ease-in-out",
            isEditing && "ring-2 ring-blue-300 ring-offset-1 shadow-lg border-blue-500"
        )}>
            {/* 第一行标题 */}
            <div className="flex items-center justify-between mb-1">
                <span className="font-semibold text-purple-700 text-sm">
                    🏁 结束节点
                </span>
            </div>

            {/* 说明文本 */}
            <div className="text-xs text-gray-600">流程终止</div>

            {/* 只有 Target Handle */}
            <Handle type="target" position={Position.Top} id="t" className="!top-[-5px]" />
        </div>
    );
});

// 创建 StartNode 组件，使用类型断言解决类型兼容性问题
// --- MODIFY: Refactor for memo with custom comparison ---
// const StartNode = memo(({ data, selected, id }: { data: StartNodeData & { isHovered?: boolean, onAddConnectedNode?: Function }, selected?: boolean, id: string }) => {
const StartNodeComponent = ({ data, selected, id }: { data: StartNodeData & { isHovered?: boolean, onAddConnectedNode?: Function }, selected?: boolean, id: string }) => {
    // --- 使用 cn 并与其他节点样式对齐 ---
    const isHovered = data.isHovered;
    const onAddConnectedNode = data.onAddConnectedNode;
    const nodeClasses = cn(
        // 基础样式
        "group bg-white border-2 rounded-md p-3 shadow-md min-w-[180px] flex flex-col transition-all duration-150 ease-in-out relative", // 添加 relative 和 group
        // 特殊颜色标识
        "border-green-500",
        // 选中状态样式
        selected && "ring-2 ring-green-300 ring-offset-1 shadow-lg border-green-600"
    );

    return (
        <>
            {/* 只需要底部连接点 */}
            <Handle
                type="source"
                position={Position.Bottom}
                style={{ background: '#555', width: '8px', height: '8px' }}
                id="source-bottom"
            />

            <div className={nodeClasses}>
                <div className="flex items-center gap-2 font-medium">
                    <PlayIcon className="h-5 w-5 text-green-600" />
                    <span>开始节点</span>
                </div>
                {data.desc && (
                    <div className="mt-2 text-sm text-gray-500">
                        {data.desc}
                    </div>
                )}

                {/* --- 添加悬停显示的 '+' 按钮和菜单 --- */}
                {onAddConnectedNode && (
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <button
                                className={cn(
                                    // --- MODIFY: Smaller Style ---
                                    "absolute -bottom-2 left-1/2 -translate-x-1/2 z-10 rounded-full bg-background border border-primary p-0.5 text-primary shadow-sm transition-opacity duration-150 flex items-center justify-center",
                                    isHovered ? "opacity-100" : "opacity-0"
                                    // --- End MODIFY ---
                                )}
                                onClick={(e) => e.stopPropagation()}
                                title="添加后续节点"
                            >
                                {/* --- MODIFY: Smaller Icon --- */}
                                <Plus className="h-2.5 w-2.5" />
                                {/* --- End MODIFY --- */}
                            </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent sideOffset={5}>
                            <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'section')}>
                                📋 添加 Section 节点
                            </DropdownMenuItem>
                            <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'skip')}>
                                ⏭️ 添加 Skip 节点
                            </DropdownMenuItem>
                            <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'end')}>
                                🏁 添加 End 节点
                            </DropdownMenuItem>
                            <DropdownMenuItem onSelect={() => onAddConnectedNode(id, 'loop')}>
                                🔄 添加 Loop 节点
                            </DropdownMenuItem>
                        </DropdownMenuContent>
                    </DropdownMenu>
                )}
                {/* --- 结束添加 '+' 按钮 --- */}
            </div>
        </>
    );
};

// 自定义 StartNode props 比较函数
const startNodePropsAreEqual = (prevProps: any, nextProps: any): boolean => {
    // 比较影响视觉渲染的 props
    if (
        prevProps.id !== nextProps.id ||
        prevProps.selected !== nextProps.selected || // 检查 selected 状态
        prevProps.data.type !== nextProps.data.type ||
        prevProps.data.desc !== nextProps.data.desc || // 检查 desc
        prevProps.data.isHovered !== nextProps.data.isHovered
    ) {
        return false; // Prop 不同，需要渲染
    }

    // 如果所有相关 props 都相同，则跳过渲染
    // 注意：我们故意忽略了 data.onAddConnectedNode 的比较
    return true;
};

// 使用 memo 和自定义比较函数导出 StartNode
const StartNode = memo(StartNodeComponent, startNodePropsAreEqual);
// --- End MODIFY ---

// 定义自定义边 - 移除 onClick prop
const ConditionEdge = memo(({
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    data, // Contains isDefault, condition, priority
    selected // 添加selected属性
}: any) => {
    // --- MODIFIED: Use getBezierPath ---
    const [edgePath, labelX, labelY] = getBezierPath({
        sourceX,
        sourceY,
        sourcePosition,
        targetX,
        targetY,
        targetPosition,
        curvature: 0.5, // 控制曲线的弯曲程度
    });
    // --- END MODIFICATION ---

    // const edgePath = `M${sourceX},${sourceY} C${sourceX},${sourceY + 50} ${targetX},${targetY - 50} ${targetX},${targetY}`; // <-- Keep old line commented for reference
    const defaultColor = '#9ca3af'; // Tailwind gray-400
    const selectedColor = '#ff5500'; // 保持选中颜色不变
    const strokeColor = selected ? selectedColor : defaultColor;
    const strokeWidth = selected ? 3 : 2;

    // 构建提示文本 (这个不再需要，但在 TooltipContent 中重新构建)
    // const tooltipText = `优先级: ${data?.priority ?? 0}${data?.condition ? `\n条件: ${data.condition}` : ''}`;

    return (
        <TooltipProvider>
            <Tooltip delayDuration={200}>
                <TooltipTrigger asChild>
                    <g className="cursor-pointer">
                        {/* Transparent wider path for easier clicking */}
                        <path
                            id={`${id}-clickarea`}
                            stroke="transparent"
                            strokeWidth={10}
                            d={edgePath}
                            fill="none"
                        />
                        {/* Visible path */}
                        <path
                            id={id}
                            stroke={strokeColor}
                            d={edgePath}
                            fill="none"
                            strokeWidth={strokeWidth}
                        />
                        {/* REMOVED condition text from edge */}
                    </g>
                </TooltipTrigger>
                <TooltipContent
                    side="right" // Force tooltip to the right
                    sideOffset={4} // Reduce offset to bring it closer
                    className="condition-edge-tooltip-content bg-white text-gray-800 border border-gray-200 shadow-md text-xs px-3 py-2 rounded"
                >
                    <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-1.5">
                            <span className="text-gray-500">🔢 优先级:</span>
                            <span className="font-medium">{data?.priority ?? 0}</span>
                        </div>
                        {data?.condition && (
                            <div className="flex items-center gap-1.5">
                                <span className="text-gray-500">❓ 条件:</span>
                                <span className="font-medium">{data.condition}</span>
                            </div>
                        )}
                    </div>
                </TooltipContent>
            </Tooltip>
        </TooltipProvider>
    );
});

// YAML解析与转换功能 - 使用索引标签，并在 Next 规则中转换 ID
// --- MODIFIED: Add LoopNodeData to union ---
const parseYamlToFlowElements = (yamlContent: string): { nodes: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[], edges: Edge<EdgeData>[] } => {
    // --- MODIFICATION: Add detailed logging and handle dynamic root key ---
    console.log("[parseYamlToFlowElements] Starting YAML parsing");
    // Log the raw input YAML content for debugging
    console.log(`[parseYamlToFlowElements] Received YAML content:\\n${yamlContent}`);
    // --- END MODIFICATION ---
    try {
        const nodes: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[] = [];
        let edges: Edge<EdgeData>[] = []; // Use 'let' instead of 'const' since we'll reassign it
        const labelToNodeId = new Map<string, string>();

        // 创建起始节点
        const startNode: Node<StartNodeData> = {
            id: 'start-node',
            type: 'start',
            position: { x: 250, y: 25 },
            data: {
                type: 'start',
            }
        };
        nodes.push(startNode);

        // 解析YAML内容
        const parsed = yaml.load(yamlContent) as any;

        // --- MODIFICATION: Handle dynamic root key ---
        let protocolNodes: any[] = []; // Initialize empty array for protocol nodes

        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            const rootKeys = Object.keys(parsed);
            if (rootKeys.length > 0) {
                const dynamicRootKey = rootKeys[0]; // Assume the first key is the dynamic root
                const potentialNodeList = parsed[dynamicRootKey];

                if (Array.isArray(potentialNodeList)) {
                    protocolNodes = potentialNodeList;
                    console.log(`[parseYamlToFlowElements] Successfully extracted ${protocolNodes.length} nodes under dynamic root key: ${dynamicRootKey}`);
                } else {
                    console.warn(`[parseYamlToFlowElements] Value under root key '${dynamicRootKey}' is not an array. Found type: ${typeof potentialNodeList}`);
                }
            } else {
                console.warn("[parseYamlToFlowElements] Parsed YAML object has no keys.");
            }
        } else {
            console.warn(`[parseYamlToFlowElements] Parsed YAML is not a valid object or is an array. Type: ${typeof parsed}`);
        }

        // Check if protocolNodes array is valid
        if (protocolNodes.length === 0) {
            console.log("[parseYamlToFlowElements] No valid protocol nodes found under root key, returning only start node.");
            // No need to explicitly return here, the rest of the code will handle the empty list
        }
        // --- END MODIFICATION ---

        // --- MODIFICATION: Use the extracted 'protocolNodes' array instead of 'parsed.protocol' ---
        // if (!parsed || !parsed.protocol || !Array.isArray(parsed.protocol)) { // OLD CHECK
        //     console.log("[parseYamlToFlowElements] YAML 为空或无效，只保留起始节点");
        //     return { nodes, edges };
        // }

        // console.log(`[parseYamlToFlowElements] Found ${parsed.protocol.length} protocol items`); // OLD LOG

        // 存储节点之间的连接关系，用于检测循环
        type Connection = { source: string, target: string, condition: string };
        const connections: Connection[] = [];

        // 第一遍：创建所有节点并建立标签映射 (Use 'protocolNodes')
        protocolNodes.forEach((item: any, index: number) => {
            // --- END MODIFICATION ---
            if ('skip' in item) {
                const skipNode: Node<SkipNodeData> = {
                    id: `skip-${index}`,
                    type: 'skip',
                    position: { x: 0, y: 0 },
                    data: {
                        type: 'skip',
                        size: item.skip,
                        yamlIndex: index
                    }
                };
                nodes.push(skipNode);
                if (item.Label) {
                    labelToNodeId.set(item.Label, skipNode.id);
                    console.log(`[Label Mapping] Set label ${item.Label} to node ${skipNode.id}`);
                }
            } else {
                const sectionNode: Node<SectionNodeData> = {
                    id: `section-${index}`,
                    type: 'section',
                    position: { x: 0, y: 0 },
                    data: {
                        type: 'section',
                        desc: item.desc || `Section ${index + 1}`,
                        size: item.size || 1,
                        Label: item.Label,
                        Dev: item.Dev || {},
                        Vars: item.Vars || {},
                        yamlIndex: index
                    }
                };
                nodes.push(sectionNode);
                if (item.Label) {
                    labelToNodeId.set(item.Label, sectionNode.id);
                    console.log(`[Label Mapping] Set label ${item.Label} to node ${sectionNode.id}`);
                }
            }
        });

        // 第二遍：处理连接关系 (包括隐式连接) 并收集所有连接信息 (Use 'protocolNodes')
        // --- MODIFICATION: Use 'protocolNodes' ---
        protocolNodes.forEach((item: any, index: number) => {
            // --- END MODIFICATION ---
            const sourceId = `${item.skip ? 'skip' : 'section'}-${index}`;

            if (item.Next && Array.isArray(item.Next)) {
                console.log(`[parseYamlToFlowElements] Processing Next conditions for node at index ${index}`);
                // 按照YAML中的顺序设置优先级
                item.Next.forEach((next: any, priority: number) => {
                    let targetId: string;
                    let targetLabel = next.target;
                    let isSelfLoop = false;

                    if (next.target === 'END') {
                        // 为END目标创建新的结束节点
                        targetId = `end-${Date.now()}-${Math.random()}`;
                        const endNode: Node<EndNodeData> = {
                            id: targetId,
                            type: 'end',
                            position: { x: 0, y: 0 },
                            data: { type: 'end' }
                        };
                        nodes.push(endNode);
                    } else if (next.target === 'DEFAULT') {
                        // 为DEFAULT目标创建特殊处理
                        // 修改：对于DEFAULT, 指向下一个顺序节点，不再自动创建END节点
                        const currentIndex = index;
                        const nextIndex = currentIndex + 1;

                        // --- MODIFICATION: Check against 'protocolNodes.length' ---
                        if (nextIndex < protocolNodes.length) {
                            // --- END MODIFICATION ---
                            // 有下一个顺序节点
                            // --- MODIFICATION: Get item from 'protocolNodes' ---
                            const nextItem = protocolNodes[nextIndex];
                            // --- END MODIFICATION ---
                            targetId = `${nextItem.skip ? 'skip' : 'section'}-${nextIndex}`;
                        } else {
                            // 没有下一个节点，跳过创建连接（不再创建END节点）
                            console.log(`[parseYamlToFlowElements] No next sequential node for DEFAULT at index ${index}, skipping connection`);
                            return; // 跳过当前迭代，不创建连接
                        }
                    } else if (next.target === targetLabel && item.Label === targetLabel) {
                        // 检测自引用（节点的Next指向自己的标签）
                        isSelfLoop = true;
                        targetId = sourceId;
                        console.log(`Detected self-reference in YAML: Label ${targetLabel} points to itself`);
                    } else {
                        // 查找目标节点ID
                        targetId = labelToNodeId.get(next.target) || '';
                        if (!targetId) {
                            console.warn(`找不到标签 ${next.target} 对应的节点`);
                            // 为找不到的标签创建一个占位符END节点，并添加视觉提示
                            targetId = `missing-${next.target}-${Date.now()}-${Math.random()}`;
                            const placeholderNode: Node<EndNodeData> = {
                                id: targetId,
                                type: 'end',
                                position: { x: 0, y: 0 },
                                data: {
                                    type: 'end',
                                    missingLabel: next.target,
                                    severity: 'error'
                                }
                            };
                            nodes.push(placeholderNode);
                        }
                    }

                    // 记录连接关系
                    connections.push({
                        source: sourceId,
                        target: targetId,
                        condition: next.condition || 'true'
                    });

                    // 创建边，设置优先级
                    const edge: Edge<EdgeData> = {
                        id: `edge-${sourceId}-${targetId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                        source: sourceId,
                        target: targetId,
                        type: 'condition',
                        data: {
                            condition: next.condition,
                            priority: priority,
                            isDefault: priority === 0
                        }
                    };
                    edges.push(edge);
                });
            }
        });

        // 第三遍：检测循环结构并创建Loop节点
        const detectAndCreateLoops = (): Edge<EdgeData>[] => {
            // 创建节点关系图
            const nodeGraph = new Map<string, string[]>();
            nodes.forEach(node => {
                nodeGraph.set(node.id, []);
            });

            connections.forEach(conn => {
                const outgoing = nodeGraph.get(conn.source) || [];
                outgoing.push(conn.target);
                nodeGraph.set(conn.source, outgoing);
            });

            // 跟踪已处理成循环一部分的节点
            const processedInLoop = new Set<string>();

            // 检测自循环（节点直接指向自己）
            const selfLoops = connections.filter(conn => conn.source === conn.target);

            // 处理自循环
            selfLoops.forEach(loop => {
                // 如果节点已经是某个循环的一部分，则跳过
                if (processedInLoop.has(loop.source)) {
                    console.log(`[Loop Detection] Skipping already processed self-loop at node ${loop.source}`);
                    return;
                }

                console.log(`[Loop Detection] Found self-loop at node ${loop.source}`);

                // 创建Loop节点
                const loopNode: Node<LoopNodeData> = {
                    id: `loop-${Date.now()}-${Math.random()}`,
                    type: 'loop',
                    position: { x: 0, y: 0 },
                    data: {
                        type: 'loop',
                        loopCondition: loop.condition
                    }
                };

                // 先从原数组中移除自循环节点
                const selfLoopNode = nodes.find(n => n.id === loop.source);
                let selfLoopIndex = -1;
                if (selfLoopNode) {
                    selfLoopIndex = nodes.findIndex(n => n.id === loop.source);
                    if (selfLoopIndex > -1) {
                        nodes.splice(selfLoopIndex, 1);
                    }

                    // 修改节点为子节点
                    selfLoopNode.parentId = loopNode.id;
                    selfLoopNode.extent = 'parent' as const;
                }

                // 先添加父节点，再添加子节点
                nodes.push(loopNode);
                if (selfLoopNode) {
                    nodes.push(selfLoopNode);
                }

                // 删除自循环边
                const edgeIndex = edges.findIndex(e =>
                    e.source === loop.source && e.target === loop.target);
                if (edgeIndex !== -1) {
                    edges.splice(edgeIndex, 1);
                }

                // --- 添加日志 ---
                console.log(`[Loop Connection - SelfLoop] Before redirecting incoming edges for loop target ${loop.source}. Current Edges:`, JSON.stringify(edges.map(e => ({ id: e.id, s: e.source, t: e.target, targetActual: e.target })), null, 2));
                // --- 结束日志 ---

                // 处理指向自循环节点的边，改为指向Loop节点
                edges.forEach(edge => {
                    if (edge.target === loop.source && edge.source !== loopNode.id) {
                        edge.target = loopNode.id;
                        console.log(`[Loop Connection] Redirected edge from ${edge.source} to loop node ${loopNode.id} (was: ${loop.source})`);
                    }
                });

                // 标记节点已处理
                processedInLoop.add(loop.source);

                // 找出所有从循环子节点出发的边（不包括自循环边，因为已删除）
                const childOutgoingEdges = edges.filter(e =>
                    e.source === loop.source && e.target !== loop.source);

                if (childOutgoingEdges.length > 0) {
                    console.log(`[Loop Connection] Found ${childOutgoingEdges.length} outgoing edges from loop child ${loop.source}`);

                    // 创建从循环节点出发的新边，指向相同的目标
                    childOutgoingEdges.forEach(childEdge => {
                        const newEdge: Edge<EdgeData> = {
                            id: `edge-${loopNode.id}-${childEdge.target}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                            source: loopNode.id,
                            target: childEdge.target,
                            type: childEdge.type,
                            data: {
                                ...childEdge.data,
                                condition: childEdge.data?.condition || 'true'
                            }
                        };
                        edges.push(newEdge);
                        console.log(`[Loop Connection] Created new edge from loop node ${loopNode.id} to ${childEdge.target}`);
                    });

                    // 删除原子节点的所有外出边
                    edges = edges.filter(e => e.source !== loop.source);
                    console.log(`[Loop Connection] Removed all outgoing edges from loop child ${loop.source}`);
                }
            });

            // 检测更复杂的循环（节点间形成环）
            // 使用简化的DFS循环检测
            const visited = new Set<string>();
            const inStack = new Set<string>();
            const cyclesSources = new Set<string>();
            const cyclesTargets = new Set<string>();
            let cycleClosingEdgeCondition: string | undefined = undefined; // 新增：用于存储关闭循环的边的条件

            const detectCycle = (nodeId: string, path: string[] = []) => {
                // 如果节点已经是某个循环的一部分，则跳过
                if (processedInLoop.has(nodeId)) {
                    return false;
                }

                if (inStack.has(nodeId)) {
                    // 找到循环
                    const cycleStart = path.indexOf(nodeId);
                    const cycle = path.slice(cycleStart);

                    // 记录循环的源和目标
                    cycle.forEach(id => cyclesSources.add(id));
                    for (let i = 0; i < cycle.length - 1; i++) {
                        cyclesTargets.add(cycle[i + 1]);
                    }
                    cyclesTargets.add(cycle[0]); // 最后一个节点指向第一个

                    // --- 新增逻辑：尝试找到关闭循环的边的条件 ---
                    // 循环的最后一个节点是 path[path.length - 1]
                    // 循环的起始节点是 nodeId
                    const closingEdgeSource = path[path.length - 1];
                    const closingEdgeTarget = nodeId;
                    const closingEdge = connections.find(conn => conn.source === closingEdgeSource && conn.target === closingEdgeTarget);
                    if (closingEdge && closingEdge.condition && closingEdge.condition.trim() !== '') {
                        cycleClosingEdgeCondition = closingEdge.condition;
                        console.log(`[Loop Detection] Found cycle: ${cycle.join(' -> ')}. Closing edge condition: '${cycleClosingEdgeCondition}'`);
                    } else {
                        console.log(`[Loop Detection] Found cycle: ${cycle.join(' -> ')}. No specific condition found for the closing edge.`);
                    }
                    // --- 结束新增逻辑 ---
                    return true;
                }

                if (visited.has(nodeId)) return false;

                visited.add(nodeId);
                inStack.add(nodeId);
                path.push(nodeId);

                const neighbors = nodeGraph.get(nodeId) || [];
                for (const neighbor of neighbors) {
                    if (detectCycle(neighbor, [...path])) {
                        return true;
                    }
                }

                inStack.delete(nodeId);
                return false;
            };

            // 从每个未访问的节点开始检测循环
            nodes.forEach(node => {
                // 跳过已处理的节点
                if (processedInLoop.has(node.id)) {
                    return;
                }

                if (!visited.has(node.id)) {
                    detectCycle(node.id);
                }
            });

            // 处理检测到的复杂循环
            if (cyclesSources.size > 0) {
                // 检查是否所有节点已被处理
                const allProcessed = Array.from(cyclesSources).every(id => processedInLoop.has(id));
                if (allProcessed) {
                    console.log(`[Loop Detection] Skipping already processed cycle nodes`);
                    return edges; // <-- 修改：返回 edges 而不是 undefined
                }

                // --- 修改：使用 cycleClosingEdgeCondition (如果存在) ---
                let chosenLoopCondition = 'true'; // 默认值
                if (cycleClosingEdgeCondition) {
                    chosenLoopCondition = cycleClosingEdgeCondition;
                    console.log(`[Loop Detection] Complex loop: Using closing edge condition: '${chosenLoopCondition}'`);
                } else {
                    // 如果没有找到特定的闭环条件，再尝试从入口边推断 (之前的逻辑)
                    const entryEdgesToCycle: Edge<EdgeData>[] = [];
                    edges.forEach(edge => {
                        // 确保边存在于原始的、未被修改的 `edges` 列表中
                        // 并且这条边是从循环外部指向循环内部的
                        if (cyclesSources.has(edge.target) && !cyclesSources.has(edge.source)) {
                            // 进一步确认这条边确实存在于 `connections` (代表原始YAML的连接)
                            const originalConnection = connections.find(conn => conn.source === edge.source && conn.target === edge.target);
                            if (originalConnection) {
                                entryEdgesToCycle.push(edge);
                            }
                        }
                    });

                    if (entryEdgesToCycle.length === 1) {
                        const entryEdge = entryEdgesToCycle[0];
                        if (entryEdge.data?.condition && entryEdge.data.condition.trim() !== '') {
                            chosenLoopCondition = entryEdge.data.condition;
                            console.log(`[Loop Detection] Complex loop (no closing edge condition): Using condition from single entry edge ${entryEdge.id} ('${chosenLoopCondition}')`);
                        } else {
                            console.log(`[Loop Detection] Complex loop (no closing edge condition): Single entry edge ${entryEdge.id} has no condition, defaulting to 'true'`);
                        }
                    } else if (entryEdgesToCycle.length > 1) {
                        const defaultPriorityEntryEdges = entryEdgesToCycle.filter(edge => edge.data?.priority === 0);
                        if (defaultPriorityEntryEdges.length === 1) {
                            const entryEdge = defaultPriorityEntryEdges[0];
                            if (entryEdge.data?.condition && entryEdge.data.condition.trim() !== '') {
                                chosenLoopCondition = entryEdge.data.condition;
                                console.log(`[Loop Detection] Complex loop (no closing edge condition): Using condition from default priority entry edge ${entryEdge.id} ('${chosenLoopCondition}')`);
                            } else {
                                console.log(`[Loop Detection] Complex loop (no closing edge condition): Default priority entry edge ${entryEdge.id} has no condition, defaulting to 'true'`);
                            }
                        } else {
                            const firstEntryEdge = entryEdgesToCycle[0];
                            if (firstEntryEdge?.data?.condition && firstEntryEdge.data.condition.trim() !== '') {
                                chosenLoopCondition = firstEntryEdge.data.condition;
                                console.log(`[Loop Detection] Complex loop (no closing edge condition): Multiple entry edges. Using condition from first entry edge ${firstEntryEdge.id} ('${chosenLoopCondition}'). User may need to verify.`);
                            } else {
                                console.log(`[Loop Detection] Complex loop (no closing edge condition): Multiple entry edges. First entry edge ${firstEntryEdge?.id} has no condition, defaulting to 'true'. User may need to verify.`);
                            }
                        }
                    } else {
                        console.log("[Loop Detection] Complex loop (no closing edge condition): No distinct entry edges found from outside the cycle. Defaulting loopCondition to 'true'.");
                    }
                }
                // --- 结束修改 ---

                // 创建一个Loop节点表示循环
                const loopNode: Node<LoopNodeData> = {
                    id: `loop-${Date.now()}-${Math.random()}`,
                    type: 'loop',
                    position: { x: 0, y: 0 },
                    data: {
                        type: 'loop',
                        // loopCondition: 'true' // 默认条件，需要用户调整 // OLD LINE
                        loopCondition: chosenLoopCondition // 使用推断的或默认的条件
                    }
                };

                // 暂存循环中的节点
                const cycleNodes: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[] = [];
                const cycleNodeIndices: number[] = [];

                // 找出所有循环中的节点并从原数组中移除
                cyclesSources.forEach(nodeId => {
                    // 跳过已处理的节点
                    if (processedInLoop.has(nodeId)) {
                        return;
                    }

                    const nodeIndex = nodes.findIndex(n => n.id === nodeId);
                    if (nodeIndex !== -1) {
                        const cycleNode = nodes[nodeIndex];
                        // 修改为Loop的子节点
                        cycleNode.parentId = loopNode.id;
                        cycleNode.extent = 'parent' as const;
                        cycleNodes.push(cycleNode);
                        cycleNodeIndices.push(nodeIndex);
                        // 标记为已处理
                        processedInLoop.add(nodeId);
                    }
                });

                // 按索引从大到小删除，避免删除时索引变化导致错误
                cycleNodeIndices.sort((a, b) => b - a);
                cycleNodeIndices.forEach(index => {
                    nodes.splice(index, 1);
                });

                // 先添加父节点，再添加所有子节点
                nodes.push(loopNode);
                cycleNodes.forEach(node => {
                    nodes.push(node);
                });

                // --- 修改：不再删除循环内部的边 ---
                // edges = edges.filter(edge =>
                //     !(cyclesSources.has(edge.source) && cyclesTargets.has(edge.target)));
                console.log("[Loop Connection - Complex] Preserving internal edges within the cycle.");
                // --- 结束修改 ---

                // --- 添加日志 ---
                console.log(`[Loop Connection - Complex] Before redirecting incoming edges for cycle targets ${Array.from(cyclesSources)}. Current Edges:`, JSON.stringify(edges.map(e => ({ id: e.id, s: e.source, t: e.target, targetActual: e.target })), null, 2));
                // --- 结束日志 ---

                // 处理指向循环节点的边，改为指向Loop节点
                edges.forEach(edge => {
                    if (cyclesSources.has(edge.target) && !cyclesSources.has(edge.source)) {
                        edge.target = loopNode.id;
                        console.log(`[Complex Loop Connection] Redirected edge from ${edge.source} to loop node ${loopNode.id}`);
                    }
                });

                // 处理循环中节点的外部连接
                // 找出所有从循环内节点出发指向循环外节点的边
                const cycleOutgoingEdges = edges.filter(edge =>
                    cyclesSources.has(edge.source) && !cyclesSources.has(edge.target));

                if (cycleOutgoingEdges.length > 0) {
                    console.log(`[Complex Loop Connection] Found ${cycleOutgoingEdges.length} outgoing edges from cycle nodes`);

                    // 创建从循环节点出发的新边，指向相同的目标
                    cycleOutgoingEdges.forEach(cycleEdge => {
                        const newEdge: Edge<EdgeData> = {
                            id: `edge-${loopNode.id}-${cycleEdge.target}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                            source: loopNode.id,
                            target: cycleEdge.target,
                            type: cycleEdge.type,
                            data: cycleEdge.data
                        };
                        edges.push(newEdge);
                        console.log(`[Complex Loop Connection] Created new edge from loop node ${loopNode.id} to ${cycleEdge.target}`);
                    });

                    // 删除循环内节点指向循环外节点的边
                    edges = edges.filter(edge =>
                        !(cyclesSources.has(edge.source) && !cyclesSources.has(edge.target)));
                    console.log(`[Complex Loop Connection] Removed outgoing edges from cycle nodes to external targets`);
                }
            }

            // 返回修改后的edges
            return edges;
        };

        // 执行循环检测和处理
        edges = detectAndCreateLoops();

        // 确保节点顺序正确：父节点必须在子节点之前
        const ensureNodeOrder = () => {
            console.log("[Node Ordering] Ensuring parent nodes appear before children");

            // 创建父子关系映射
            const childToParent = new Map<string, string>();
            nodes.forEach(node => {
                if (node.parentId) {
                    childToParent.set(node.id, node.parentId);
                }
            });

            // 检查并修复顺序
            let reordered = false;
            for (let i = 0; i < nodes.length; i++) {
                const node = nodes[i];
                if (node.parentId) {
                    // 查找父节点在数组中的位置
                    const parentIndex = nodes.findIndex(n => n.id === node.parentId);
                    if (parentIndex > i) {
                        // 父节点出现在子节点之后，需要重新排序
                        console.log(`[Node Ordering] Parent node ${node.parentId} appears after child node ${node.id}, reordering`);

                        // 移除子节点
                        const childNode = nodes.splice(i, 1)[0];

                        // 移除后索引会变化，所以需要重新计算父节点位置
                        const newParentIndex = nodes.findIndex(n => n.id === node.parentId);

                        // 将子节点插入到父节点之后
                        nodes.splice(newParentIndex + 1, 0, childNode);

                        reordered = true;
                        // 从当前位置重新开始检查，因为顺序已经改变
                        i = -1;
                    }
                }
            }

            if (reordered) {
                console.log("[Node Ordering] Node order has been corrected");
            } else {
                console.log("[Node Ordering] Node order is already correct");
            }
        };

        // 确保节点顺序正确
        ensureNodeOrder();

        // 将第一个非开始节点与开始节点连接
        const firstNodeId = nodes.find(n => n.id !== 'start-node')?.id;
        if (firstNodeId) {
            edges.push({
                id: `start-node-edge-${firstNodeId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                source: 'start-node',
                target: firstNodeId,
                type: 'condition',
                data: {
                    isDefault: true,
                    priority: 0
                }
            });
        }

        // 应用自动布局
        const layoutedElements = getLayoutedElements(nodes, edges, 'TB');
        console.log(`[parseYamlToFlowElements] Finished processing with ${nodes.length} nodes and ${edges.length} edges (before layout). Layout applied.`);

        // --- 添加边去重逻辑 ---
        const uniqueEdgesMap = new Map<string, Edge<EdgeData>>();
        layoutedElements.edges.forEach(edge => {
            uniqueEdgesMap.set(edge.id, edge);
        });
        const uniqueEdges = Array.from(uniqueEdgesMap.values());
        if (uniqueEdges.length < layoutedElements.edges.length) {
            console.warn(`[parseYamlToFlowElements] Duplicate edge IDs detected and removed. Original count: ${layoutedElements.edges.length}, Unique count: ${uniqueEdges.length}`);
        }
        // --- 结束去重逻辑 ---

        return {
            nodes: layoutedElements.nodes,
            edges: uniqueEdges // 返回去重后的边
        };
    } catch (error) {
        console.error("[parseYamlToFlowElements] 解析YAML出错:", error);
        return {
            nodes: [{
                id: 'start-node',
                type: 'start',
                position: { x: 250, y: 25 },
                data: {
                    type: 'start',
                }
            }],
            edges: []
        };
    }
};
// --- END MODIFICATION ---

// 将Flow图转换回YAML - 使用DFS确保分支顺序正确
const convertFlowToYaml = (
    nodes: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[],
    edges: Edge<EdgeData>[],
    protocolId: string, // Keep this named protocolId for consistency internally
    version: string
): string => {
    const protocolName = protocolId; // Use the passed name/id here
    console.log(`[convertFlowToYaml] Starting YAML generation for ${protocolName}_${version} using DFS.`); // <-- Update log format
    try {
        // --- 1. Preprocessing: Build maps and identify necessary labels ---
        const fullNodeMap = new Map<string, Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>>(
            nodes.map(n => [n.id, n])
        );

        let startNodeId = 'start-node';

        // 验证开始节点存在
        if (!fullNodeMap.has(startNodeId)) {
            console.error("[convertFlowToYaml] 开始节点未找到!");
            return yaml.dump({ protocol: [] }, { lineWidth: -1, sortKeys: false });
        }

        // 增加标签映射初始化
        const nodeIdToLabelMap = new Map<string, string>();
        const nodeIdToIndexMap = new Map<string, number>();

        // 为需要标签的节点分配标签 (从 L1 开始)
        let labelCounter = 1; // Initialize counter to 1
        nodes.forEach((node, index) => {
            nodeIdToIndexMap.set(node.id, index);
            // Assign labels to both Section and Skip nodes that might be targets
            // Exclude start, end, and loop nodes from receiving labels
            if ((node.data.type === 'section' || node.data.type === 'skip') && node.id !== startNodeId) {
                const label = `L${labelCounter++}`;
                nodeIdToLabelMap.set(node.id, label);
                console.log(`[Label Assign] Assigned ${label} to node ${node.id} (${node.data.type})`);
            } else if (node.id !== startNodeId && node.type !== 'end') {
                console.log(`[Label Assign] Skipped label assignment for node ${node.id} (${node.data.type})`);
            }
        });

        // --- Loop Node Preprocessing: Identify loop connections ---
        // Map to track loop nodes and their entry/exit connections
        const loopInfo = new Map<string, {
            firstChildId?: string; // First node inside loop
            lastChildId?: string;  // Last node inside loop
            childIds: string[];    // All child nodes
            exitCondition?: string; // Condition for exiting loop
            nextNodeId?: string;   // Node to go to after loop exits
            hasChildren: boolean;  // Whether the loop has any children
        }>();

        // Find loop nodes and identify their child relationships
        nodes.forEach((node: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>) => {
            if (node.type === 'loop') { // Check node.type instead of data.type
                // Initialize loop info
                loopInfo.set(node.id, {
                    childIds: [],
                    hasChildren: false
                });

                // Find all children (nodes that have this loop as parent)
                const childNodes = nodes.filter((n: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>) =>
                    n.parentId === node.id);

                if (childNodes.length > 0) {
                    // Mark that this loop has children
                    loopInfo.get(node.id)!.hasChildren = true;

                    // Store all child IDs
                    loopInfo.get(node.id)!.childIds = childNodes.map((n: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>) => n.id);

                    // The first child is important for loop entry
                    loopInfo.get(node.id)!.firstChildId = childNodes[0].id;
                    console.log(`[Loop Preprocess] Found first child ${childNodes[0].id} for loop ${node.id}`);

                    // The last child is important for loop back connection
                    loopInfo.get(node.id)!.lastChildId = childNodes[childNodes.length - 1].id;
                    console.log(`[Loop Preprocess] Found last child ${childNodes[childNodes.length - 1].id} for loop ${node.id}`);
                }

                // Find edges coming out of this loop node
                const outgoingEdges = edges.filter((e: Edge<EdgeData>) => e.source === node.id);
                if (outgoingEdges.length > 0) {
                    // First edge is typically the exit path
                    const exitEdge = outgoingEdges[0];
                    loopInfo.get(node.id)!.exitCondition = exitEdge.data?.condition || `!(${(node.data as LoopNodeData).loopCondition})`;
                    loopInfo.get(node.id)!.nextNodeId = exitEdge.target;
                    console.log(`[Loop Preprocess] Found exit path to ${exitEdge.target} for loop ${node.id}`);
                }
            }
        });

        // --- 2. DFS Traversal for Order and YAML Object Construction ---
        const visited = new Set<string>();
        const yamlNodesOutput: any[] = [];

        const traverse = (nodeId: string) => {
            if (!nodeId || visited.has(nodeId) || nodeId === "virtual-end") {
                return;
            }

            const currentNode = fullNodeMap.get(nodeId);
            if (!currentNode) {
                console.warn(`[traverse] Node ID ${nodeId} not found in fullNodeMap.`);
                return;
            }

            // Handle Start Node separately (no YAML output, just traverse children)
            if (currentNode.type === 'start') {
                console.log(`[traverse] Processing start node ${nodeId}`);
                visited.add(nodeId); // Mark start node as visited

                const outgoingEdges = edges
                    .filter(edge => edge.source === nodeId)
                    .sort((a, b) => (a.data?.priority ?? 0) - (b.data?.priority ?? 0));

                // Prioritize the lowest priority edge (e.g., default path)
                const defaultEdge = outgoingEdges.shift(); // Remove the first edge (lowest priority)
                if (defaultEdge) {
                    console.log(`[traverse Start] Traversing default target first: ${defaultEdge.target}`);
                    traverse(defaultEdge.target);
                }

                // Traverse remaining children (branches)
                console.log(`[traverse Start] Traversing remaining ${outgoingEdges.length} children.`);
                outgoingEdges.forEach(edge => {
                    traverse(edge.target);
                });
                return;
            }

            // --- Processing for non-start nodes (Section/Skip) ---
            console.log(`[traverse] Visiting node: ${nodeId}`);
            visited.add(nodeId); // Mark current node as visited

            const { data } = currentNode;
            let sectionObj: Record<string, any> = {};
            const nodeLabel = nodeIdToLabelMap?.get(nodeId);

            // --- Build the YAML object for the current node ---
            if (data.type === 'skip' && typeof data.size === 'number') {
                sectionObj = { skip: data.size };
                // Label is less common for skip, but add if exists
                if (nodeLabel) sectionObj.Label = nodeLabel;
            } else if (data.type === 'section') {
                sectionObj = { desc: data.desc, size: data.size };
                if (data.Dev && Object.keys(data.Dev).length > 0) sectionObj.Dev = data.Dev;
                if (data.Vars && Object.keys(data.Vars).length > 0) sectionObj.Vars = data.Vars;
                if (nodeLabel) sectionObj.Label = nodeLabel;

                // Check if this is the last node in a loop
                const loopParentId = currentNode.parentId;
                if (loopParentId) {
                    const loopNodeInfo = loopInfo.get(loopParentId);
                    if (loopNodeInfo && loopNodeInfo.lastChildId === nodeId && loopNodeInfo.firstChildId) {
                        // This is the last node in a loop - add a loop back connection
                        const firstChildLabel = nodeIdToLabelMap.get(loopNodeInfo.firstChildId);
                        if (firstChildLabel) {
                            // Get the loop node data to access its condition
                            const loopNode = fullNodeMap.get(loopParentId);
                            if (loopNode && loopNode.type === 'loop') {
                                const loopCondition = (loopNode.data as LoopNodeData).loopCondition || 'true';

                                // Ensure Next array exists
                                sectionObj.Next = sectionObj.Next || [];

                                // Add a loop back condition
                                sectionObj.Next.push({
                                    condition: loopCondition,
                                    target: firstChildLabel
                                });

                                // Add exit condition - either to the next node after loop or DEFAULT
                                if (loopNodeInfo.nextNodeId) {
                                    const nextNodeLabel = nodeIdToLabelMap.get(loopNodeInfo.nextNodeId);
                                    if (nextNodeLabel) {
                                        // Add exit to next node
                                        sectionObj.Next.push({
                                            condition: 'true', // 使用"true"作为默认条件，而不是循环条件
                                            target: nextNodeLabel
                                        });
                                    } else if (loopNodeInfo.nextNodeId.includes('end')) {
                                        // Add exit to END
                                        sectionObj.Next.push({
                                            condition: 'true', // 使用"true"作为默认条件，而不是循环条件
                                            target: "END"
                                        });
                                    }
                                } else {
                                    // 修改：不再自动添加DEFAULT目标，只有当有明确的后续节点时才添加
                                    console.log(`[Loop Handling] No next node defined for loop exit from ${nodeId}. Skip adding exit condition.`);
                                }

                                console.log(`[Loop Handling] Added loop back from ${nodeId} to ${loopNodeInfo.firstChildId} with condition ${loopCondition}`);
                            }
                        }
                    }
                }
            } else if (currentNode.type === 'loop') { // Use currentNode.type instead of data.type
                // Handle loop nodes
                const loopData = data as LoopNodeData;
                const loopNodeInfo = loopInfo.get(nodeId);

                // Skip loop nodes without children
                if (!loopNodeInfo || !loopNodeInfo.hasChildren) {
                    console.log(`[Loop Handling] Skipping loop node ${nodeId} - no children`);

                    // If there's a next node, traverse to it directly
                    if (loopNodeInfo?.nextNodeId) {
                        console.log(`[Loop Handling] Traversing directly to next node ${loopNodeInfo.nextNodeId}`);
                        traverse(loopNodeInfo.nextNodeId);
                    }
                    return; // Skip this node entirely
                }

                // Skip representing empty loop nodes in YAML
                if (loopNodeInfo.childIds.length === 0) {
                    console.log(`[Loop Handling] Skipping empty loop node ${nodeId} in YAML output`);
                    return;
                }

                // For loop nodes with children, we don't create a YAML entry for the loop itself
                // Instead, we traverse its children directly
                console.log(`[Loop Handling] Processing loop node ${nodeId} with ${loopNodeInfo.childIds.length} children`);

                // First traverse the first child
                if (loopNodeInfo.firstChildId) {
                    console.log(`[Loop Handling] Traversing first child ${loopNodeInfo.firstChildId}`);
                    traverse(loopNodeInfo.firstChildId);
                }

                // Then traverse other children (except the first)
                loopNodeInfo.childIds
                    .filter(childId => childId !== loopNodeInfo.firstChildId)
                    .forEach(childId => {
                        console.log(`[Loop Handling] Traversing other child ${childId}`);
                        traverse(childId);
                    });

                // Finally, traverse to the node after the loop (if any)
                if (loopNodeInfo.nextNodeId) {
                    console.log(`[Loop Handling] Traversing to post-loop node ${loopNodeInfo.nextNodeId}`);
                    traverse(loopNodeInfo.nextNodeId);
                }

                // Skip the rest of the processing for loop nodes
                return;
            } else if (data.type === 'end') {
                // End nodes don't appear in the YAML list directly
                // Their existence is implied by `target: END` in a parent's Next rule
                console.log(`[traverse] Reached End node ${nodeId}, stopping this path.`);
                return;
            }

            // --- Get outgoing edges and prepare Next rules (for both Section and Skip) ---
            const outgoingEdges = edges
                .filter(edge => edge.source === nodeId)
                .sort((a, b) => (a.data?.priority ?? 0) - (b.data?.priority ?? 0));

            // Only add Next rules for non-loop nodes (Section and Skip nodes)
            if (outgoingEdges.length > 0 && currentNode.type !== 'loop') { // Check node.type instead of data.type
                const nextRules = outgoingEdges.map((edge: Edge<EdgeData>) => {
                    const targetNode = fullNodeMap.get(edge.target);
                    let targetLabel: string | undefined | null = null; // Initialize targetLabel

                    if (targetNode?.type === 'end') {
                        targetLabel = 'END';
                    } else if (targetNode?.type === 'loop') {
                        // --- NEW: Handle edges pointing to a Loop Node ---
                        const loopNodeInfo = loopInfo.get(edge.target);
                        if (loopNodeInfo?.firstChildId) {
                            targetLabel = nodeIdToLabelMap.get(loopNodeInfo.firstChildId);
                            if (targetLabel) {
                                console.log(`[convertFlowToYaml Traverse] Edge from ${nodeId} points to Loop ${edge.target}. Targeting first child label: ${targetLabel}`);
                            } else {
                                console.warn(`[convertFlowToYaml Traverse] Edge from ${nodeId} points to Loop ${edge.target}, but first child ${loopNodeInfo.firstChildId} has no label. Skipping rule.`);
                                targetLabel = null; // Explicitly mark for skipping
                            }
                        } else {
                            console.warn(`[convertFlowToYaml Traverse] Edge from ${nodeId} points to Loop ${edge.target}, but loop has no first child. Skipping rule.`);
                            targetLabel = null; // Explicitly mark for skipping
                        }
                        // --- END NEW ---
                    } else {
                        // Regular node target
                        targetLabel = nodeIdToLabelMap.get(edge.target);
                    }

                    // Important: Only add rule if target label is valid
                    if (targetLabel) {
                        return { condition: edge.data?.condition || 'true', target: targetLabel };
                    } else {
                        // Log if targetLabel was explicitly set to null (loop handling issues) or simply not found
                        if (targetLabel === null) {
                            // Warning already logged above
                        } else {
                            console.warn(`[convertFlowToYaml Traverse] Could not find label for target node ${edge.target} from source ${nodeId}. Skipping this Next rule.`);
                        }
                        return null; // Filter this out later
                    }
                }).filter(rule => rule !== null); // Remove null entries from skipped rules

                if (nextRules.length > 0) {
                    sectionObj.Next = sectionObj.Next || [];
                    sectionObj.Next.push(...nextRules);
                }
            }
            // --- End Next rules preparation ---

            // --- Add the constructed object to the output list (PRE-ORDER) ---
            yamlNodesOutput.push(sectionObj);
            console.log(`[traverse] Added node ${nodeId} (${data.type}) object to YAML output list.`);

            // For loop nodes, we already handle traversal in the node processing logic
            if (currentNode.type === 'loop') { // Check node.type instead of data.type
                return; // Skip the rest of the traversal for loop nodes
            }

            // --- Traverse children: Prioritize lowest priority path ---
            const remainingEdges = [...outgoingEdges]; // Create a copy to modify
            const defaultEdge = remainingEdges.shift(); // Get and remove the lowest priority edge

            if (defaultEdge) {
                // Check if target is END before traversing
                const defaultTargetNode = fullNodeMap.get(defaultEdge.target);
                if (defaultTargetNode?.type !== 'end') {
                    console.log(`[traverse ${nodeId}] Traversing default target first: ${defaultEdge.target}`);
                    traverse(defaultEdge.target);
                } else {
                    console.log(`[traverse ${nodeId}] Default target ${defaultEdge.target} is END node, not traversing.`);
                }
            }

            // Traverse remaining children (branches)
            console.log(`[traverse ${nodeId}] Traversing remaining ${remainingEdges.length} children.`);
            remainingEdges.forEach(edge => {
                // Check if target is END before traversing
                const targetNode = fullNodeMap.get(edge.target);
                if (targetNode?.type !== 'end') {
                    traverse(edge.target);
                } else {
                    console.log(`[traverse ${nodeId}] Branch target ${edge.target} is END node, not traversing.`);
                }
            });
        }; // End of traverse function

        console.log(`[convertFlowToYaml] Starting DFS traversal from: ${startNodeId}`);
        traverse(startNodeId);

        // --- MODIFICATION: Use dynamic root key ---
        const rootKey = `${protocolName}_${version}`; // Construct the dynamic key with underscore
        const yamlObject = { [rootKey]: yamlNodesOutput }; // Use the dynamic key
        // --- END MODIFICATION ---

        return yaml.dump(yamlObject, { lineWidth: -1, sortKeys: false });
    } catch (error) {
        console.error("生成YAML失败:", error);
        const fallbackYaml = { protocol: [{ desc: "错误恢复节点(异常)", size: 1 }] }; // Keep fallback simple
        return yaml.dump(fallbackYaml, { lineWidth: -1, sortKeys: false });
    }
};

// Loader Function - Ensure yamlConfig is always a string
export const clientLoader = async ({ params }: { params: { versionId: string } }) => {
    const versionId = params.versionId;
    let version: ProtocolVersion | null = null;
    let protocol: Protocol | null = null; // <-- Initialize protocol
    let yamlConfig: string = "";
    let error: string | undefined;
    let definitionError: string | undefined;
    let protocolError: string | undefined;
    let definitionResponse: any;

    console.log("======== 开始加载版本数据 ========");
    console.log("版本ID:", versionId);

    try {
        // 1. Get version info
        console.log("正在获取版本基本信息...");
        const versionResponse = await API.versions.getById(versionId);
        console.log("版本信息响应状态:", versionResponse.error ? "错误" : "成功");

        if (versionResponse.error) {
            error = `获取版本信息失败: ${versionResponse.error}`;
            console.error("获取版本信息失败:", versionResponse.error);
        } else {
            version = versionResponse.data || null;
            console.log("获取到的版本信息:", version ? { id: version.id, protocolId: version.protocolId, version: version.version } : "null");

            if (!version) {
                console.error("未找到版本信息，提前返回");
                return { version: null, protocol: null, error: "未找到版本信息", yamlConfig: "" }; // <-- Return null protocol
            }

            // --- NEW: Fetch Protocol details ---
            try {
                console.log(`正在获取协议详情 (ID: ${version.protocolId})...`);
                const protocolResponse = await API.protocols.getById(version.protocolId);
                console.log("Protocol API Response:", JSON.stringify(protocolResponse).substring(0, 300)); // Log raw response
                if (protocolResponse.error) {
                    protocolError = `获取协议详情失败: ${protocolResponse.error}`;
                    console.error(protocolError);
                } else {
                    // --- FIX Linter Error: Access nested data if necessary ---
                    // Check if data exists and has a 'protocol' property
                    if (protocolResponse.data && typeof protocolResponse.data === 'object' && 'protocol' in protocolResponse.data) {
                        console.log("Accessing nested protocol data...")
                        protocol = (protocolResponse.data as any).protocol || null;
                    } else {
                        // Assume data is the protocol object directly (original logic)
                        console.log("Accessing direct protocol data...")
                        protocol = protocolResponse.data || null;
                    }
                    // --- END FIX ---
                    console.log("获取到的协议详情:", protocol ? { id: protocol.id, name: protocol.name } : "null");
                }
            } catch (protoFetchErr: any) {
                protocolError = `获取协议详情时网络或代码出错: ${protoFetchErr.message || String(protoFetchErr)}`;
                console.error(protocolError);
            }
            // --- END NEW ---
        }

        // 2. Get definition (even if version info had a non-critical error)
        try {
            console.log("正在获取协议定义...");
            definitionResponse = await API.versions.getDefinition(versionId);
            console.log("定义响应状态:", definitionResponse.error ? "错误" : "成功");

            if (definitionResponse.error) {
                definitionError = `获取协议定义失败: ${definitionResponse.error}`;
                console.error("获取协议定义失败:", definitionResponse.error);

                // --- FIX Linter Error: Adjust default YAML creation ---
                // Use the fetched protocol NAME if available, otherwise the ID, or fallback
                const protocolKeyName = protocol?.name || version?.protocolId || 'unknown_protocol';
                // Ensure the default structure matches a simple protocol array
                const defaultStructure = { [protocolKeyName]: [{ desc: "初始节点", size: 1 }] };
                try {
                    yamlConfig = yaml.dump(defaultStructure, { lineWidth: -1, sortKeys: false });
                    console.warn(`无法从API获取YAML定义，使用默认结构: ${protocolKeyName}`);
                } catch (dumpError: any) {
                    console.error("Failed to dump default structure to YAML:", dumpError);
                    yamlConfig = "protocol:\n  - desc: ErrorFallbackNode\n    size: 1"; // Absolute fallback
                }
                // --- END FIX ---
            } else {
                let definitionData = definitionResponse.data;
                console.log("获取到的定义数据类型:", typeof definitionData);

                if (definitionData === null) {
                    console.warn("警告: 定义数据为null");
                } else if (typeof definitionData === 'object') {
                    console.log("定义对象结构:",
                        `顶层键数量: ${Object.keys(definitionData as object).length}, ` +
                        `键列表: ${Object.keys(definitionData as object).join(", ")}`);

                    // 检查第一个键下的内容结构
                    if (Object.keys(definitionData as object).length > 0) {
                        const firstKey = Object.keys(definitionData as object)[0];
                        const firstValue = (definitionData as any)[firstKey];
                        console.log(`首键 "${firstKey}" 的值类型:`, typeof firstValue);

                        if (Array.isArray(firstValue)) {
                            console.log(`首键对应的数组长度: ${firstValue.length}`);
                            if (firstValue.length > 0) {
                                console.log("数组第一项类型:", typeof firstValue[0]);
                                console.log("数组第一项结构:", JSON.stringify(firstValue[0]).substring(0, 200));
                            }
                        } else {
                            console.warn(`警告: 首键 "${firstKey}" 的值不是数组，而是 ${typeof firstValue}`);
                        }
                    }
                }

                // Convert definitionData to YAML string
                if (typeof definitionData === 'object' && definitionData !== null) {
                    console.log("Loader received definition as object, converting to YAML string...");
                    try {
                        yamlConfig = yaml.dump(definitionData, { lineWidth: -1, sortKeys: false });
                        console.log("YAML转换结果 (前100个字符):", yamlConfig.substring(0, 100));
                    } catch (dumpError: any) {
                        console.error("Failed to dump definition object to YAML:", dumpError);
                        definitionError = `转换定义为YAML失败: ${dumpError.message}`;
                        yamlConfig = ""; // Fallback to empty string
                    }
                } else if (typeof definitionData === 'string') {
                    yamlConfig = definitionData.trim() || "";
                    console.log("定义是字符串格式 (前100个字符):", yamlConfig.substring(0, 100));
                } else {
                    yamlConfig = ""; // Fallback
                    console.log("定义是未知格式:", definitionData);
                }

                // Handle empty YAML after successful fetch/conversion
                if (!yamlConfig) {
                    const protocolName = version?.protocolId || 'unknown_protocol';
                    yamlConfig = `${protocolName}:\n  - desc: "初始节点"\n    size: 1\n`;
                    console.warn("API返回或转换后的YAML定义为空，使用默认结构。");
                }
            }
        } catch (defFetchError: any) {
            console.error('获取协议定义时出错:', defFetchError);
            definitionError = `获取协议定义时网络或代码出错: ${defFetchError.message || String(defFetchError)}`;
            // Provide default YAML on fetch error
            const protocolName = version?.protocolId || 'unknown_protocol';
            yamlConfig = `${protocolName}:\n  - desc: "初始节点"\n    size: 1\n`;
        }

        // Combine errors if multiple occurred
        if (protocolError) {
            error = error ? `${error}; ${protocolError}` : protocolError;
        }
        if (definitionError) {
            error = error ? `${error}; ${definitionError}` : definitionError;
        }

        console.log("======== 加载完成 ========");
        console.log("最终YAML结果长度:", yamlConfig.length);
        console.log("最终YAML结果 (前200个字符):", yamlConfig.substring(0, 200) + (yamlConfig.length > 200 ? "..." : ""));
        return { version, protocol, yamlConfig, error }; // <-- Return protocol

    } catch (fetchError: any) {
        // Catch errors from the initial version fetch or other unexpected issues
        console.error('加载版本或定义时整体出错:', fetchError);
        const errorMessage = `加载数据时出错: ${fetchError.message || String(fetchError)}`;

        // --- Simplify Catch Block ---
        // Always return a valid LoaderData structure in case of complete failure
        // Use basic fallback values
        const fallbackYaml = "unknown_protocol:\n  - desc: \"ErrorFallbackNode\"\n    size: 1";

        return {
            version: null,      // Explicitly null
            protocol: null,     // Explicitly null
            yamlConfig: fallbackYaml, // Basic fallback YAML string
            error: errorMessage  // The captured error message
        };
        // --- End Simplify Catch Block ---
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const version = data?.version as Omit<ProtocolVersion, 'config'> | undefined;
    return [
        { title: version ? `协议编排: ${version.version}` : '协议编排 - 网关管理' },
        { name: "description", content: '协议编排界面' },
    ];
};

// --- Define Ref Handle Type ---
export interface FlowCanvasHandle {
    addNode: (type: 'section' | 'skip' | 'end' | 'start' | 'loop') => void; // <-- Added 'loop'
    validateConnectivity: () => { isValid: boolean; unconnectedNodeIds: string[] };
    triggerLayout: (direction: 'TB' | 'LR') => void;
    getYamlString: (protocolId: string, version: string) => string; // ADDED
}

// --- Define Props Interface BEFORE the component ---
interface FlowCanvasProps {
    initialYaml: string;
    onYamlChange: (newYaml: string) => void;
    versionId?: string; // Pass versionId if needed for actions inside FlowCanvas
}

// --- FlowCanvas Component ---
// Wrap with forwardRef
const FlowCanvas = memo(forwardRef<FlowCanvasHandle, FlowCanvasProps>(({ initialYaml, onYamlChange, versionId }: FlowCanvasProps, ref) => {
    // 移除显式泛型，让 TS 推断
    const reactFlowInstance = useReactFlow();
    const reactFlowWrapper = useRef<HTMLDivElement | null>(null);

    // --- 移除 initialFlowElements 的 useMemo 逻辑，改为初始化空值 ---
    // Initialize hooks with empty initial values
    // --- MODIFIED: Add LoopNodeData to union ---
    const [nodes, setNodes, onNodesChange] = useNodesState<Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>>([]);
    // --- END MODIFICATION ---
    const [edges, setEdges, onEdgesChange] = useEdgesState<Edge<EdgeData>>([]);
    // --- End NEW Initialization ---

    // --- 增强 useEffect 逻辑，在组件挂载和 initialYaml 变化时解析 YAML 并设置状态 ---
    useEffect(() => {
        console.log("[FlowCanvas useEffect] Component mounted or initialYaml changed, parsing YAML and setting state.");
        if (!initialYaml) {
            // 如果 initialYaml 为空，设置空状态
            setNodes([]);
            setEdges([]);
            return;
        }

        try {
            // 解析 YAML 并应用布局，这一步已经包含了边去重逻辑
            const parsed = parseYamlToFlowElements(initialYaml);

            // 使用解析和布局后的结果设置状态
            setNodes(parsed.nodes);
            setEdges(parsed.edges);

            // 新增逻辑：在节点和边设置完成后调整视图
            if (reactFlowInstance && (parsed.nodes.length > 0 || parsed.edges.length > 0)) {
                window.requestAnimationFrame(() => {
                    // 在 requestAnimationFrame 回调中再次检查 reactFlowInstance
                    if (reactFlowInstance) {
                        reactFlowInstance.fitView();
                        console.log("[FlowCanvas useEffect] fitView called after initial parse via requestAnimationFrame.");
                    }
                });
            }

            // 可以选择在此处添加日志，协助调试
            console.log(`[FlowCanvas useEffect] Successfully parsed and set ${parsed.nodes.length} nodes and ${parsed.edges.length} edges.`);
        } catch (error) {
            console.error("[FlowCanvas useEffect] Error parsing YAML or setting flow elements:", error);
            // 出错时设置空状态
            setNodes([]);
            setEdges([]);
            // 显示错误提示
            toast.error("解析YAML定义时出错，已清空画布。");
        }
        // 依赖 initialYaml，所以在组件挂载和 initialYaml 变化时都会执行
    }, [initialYaml, setNodes, setEdges, reactFlowInstance]); // <-- Add reactFlowInstance to dependencies
    // --- 结束增强 ---

    // Other states remain the same
    const [isPopoverOpen, setIsPopoverOpen] = useState(false);
    // --- MODIFIED: Add LoopNodeData to union ---
    const [selectedNode, setSelectedNode] = useState<Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData> | null>(null);
    const [editFormData, setEditFormData] = useState<Partial<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>>({});
    // --- END MODIFICATION ---
    const [popoverPosition, setPopoverPosition] = useState<{ top: number; left: number } | null>(null);
    const [varEntries, setVarEntries] = useState<VarEntry[]>([]);
    const [devEntries, setDevEntries] = useState<DevEntry[]>([]);

    // 添加选中的边和边条件编辑状态
    const [selectedEdge, setSelectedEdge] = useState<Edge<EdgeData> | null>(null);
    const [isEdgePopoverOpen, setIsEdgePopoverOpen] = useState(false);
    const [edgeCondition, setEdgeCondition] = useState<string>("");
    const [edgePopoverPosition, setEdgePopoverPosition] = useState<{ top: number; left: number } | null>(null);
    const [contextMenuEdge, setContextMenuEdge] = useState<Edge<EdgeData> | null>(null);
    const [isContextMenuOpen, setIsContextMenuOpen] = useState(false);
    const [contextMenuPosition, setContextMenuPosition] = useState<{ top: number; left: number } | null>(null);

    // --- NEW: State for hovered node ---
    const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);

    // Define node types & edge types
    // --- MODIFIED: Memoize nodeTypes ---
    const nodeTypes: NodeTypes = useMemo(() => ({
        section: SectionNode,
        skip: SkipNode,
        end: EndNode,
        start: StartNode as any, // 使用类型断言避免复杂的类型问题
        loop: LoopNode as any, // <-- Register LoopNode with 'as any'
        // --- Include custom node components in dependency array --- Add LoopNode
    }), [SectionNode, SkipNode, EndNode, StartNode, LoopNode]);
    // --- END MODIFICATION ---

    const edgeTypes: EdgeTypes = useMemo(() => ({ condition: ConditionEdge }), [ConditionEdge]); // Also memoize edgeTypes for consistency

    // 添加节点状态跟踪日志 (Keep for debugging)
    useEffect(() => {
    }, [nodes]);

    // --- Ref for Debounce Timeout --- (移除 previousStructureRef)
    const debounceTimeoutRef = useRef<NodeJS.Timeout | null>(null);
    // --- Debounce Delay (e.g., 300ms) --- (保持不变)
    const DEBOUNCE_DELAY = 300;

    // --- 移除 getStructuralRepresentation 函数定义 (不再需要) ---
    // const getStructuralRepresentation = useCallback((nodes: Node[], edges: Edge[]): string => { ... }, []);

    // --- 修改 useEffect，移除结构比较逻辑 ---
    // useEffect(() => {
    //     if (debounceTimeoutRef.current) {
    //         clearTimeout(debounceTimeoutRef.current);
    //     }
    //     debounceTimeoutRef.current = setTimeout(() => {
    //         console.log("[FlowCanvas Debounced] Nodes or edges changed, recalculating YAML.");
    //         const currentNodes = reactFlowInstance.getNodes();
    //         const currentEdges = reactFlowInstance.getEdges();
    //         if (currentNodes.length > 0) {
    //             const newYaml = convertFlowToYaml(currentNodes, currentEdges);
    //             const isEffectivelySameAsInitial = isSameYamlContent(newYaml, initialYaml);
    //             if (!isEffectivelySameAsInitial) {
    //                 const generatedIsEmpty = newYaml.includes("protocol: []");
    //                 const initialIsNotEmpty = initialYaml && !initialYaml.includes("protocol: []");
    //                 if (generatedIsEmpty && initialIsNotEmpty && currentNodes.length > 0) {
    //                     console.log("[FlowCanvas Debounced] Skipping parent notification: Generated empty YAML from non-empty nodes.");
    //                 } else {
    //                     console.log("[FlowCanvas Debounced] YAML changed, notifying parent.");
    //                     onYamlChange(newYaml);
    //                 }
    //             } else {
    //                 console.log("[FlowCanvas Debounced] Resulting YAML is same as initial. No parent update.");
    //             }
    //         } else {
    //             console.log("[FlowCanvas Debounced] No nodes, skipping YAML generation.");
    //         }
    //     }, DEBOUNCE_DELAY);
    //     return () => {
    //         if (debounceTimeoutRef.current) {
    //             clearTimeout(debounceTimeoutRef.current);
    //         }
    //     };
    // }, [nodes, edges, initialYaml, onYamlChange, DEBOUNCE_DELAY, reactFlowInstance]);

    // --- 修改 generateYaml 函数，添加日志控制 ---
    const generateYaml = useCallback(() => {
        console.log("[FlowCanvas] 手动触发YAML生成");
        // --- MODIFIED: Add LoopNodeData to union ---
        const currentNodes = reactFlowInstance.getNodes() as Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[];
        // --- END MODIFICATION ---
        const currentEdges = reactFlowInstance.getEdges() as Edge<EdgeData>[];

        if (currentNodes.length > 0) {
            const originalConsoleLog = console.log;
            console.log = function () { };

            // --- REVERT: Call convertFlowToYaml without protocolId/version for now ---
            // This internal call's result isn't directly used for the root key anyway
            const newYaml = convertFlowToYaml(currentNodes, currentEdges, 'internal', 'temp'); // Use placeholders

            // 恢复日志功能
            console.log = originalConsoleLog;

            const isEffectivelySameAsInitial = isSameYamlContent(newYaml, initialYaml);
            if (!isEffectivelySameAsInitial) {
                console.log("[FlowCanvas] YAML已更新，通知父组件");
                onYamlChange(newYaml); // Still notify with the content, even if root key is placeholder
            } else {
                console.log("[FlowCanvas] YAML内容未变化，跳过更新");
            }
        }
    }, [reactFlowInstance, initialYaml, onYamlChange]);

    // --- NEW: Node Hover Handlers ---
    const onNodeMouseEnter = useCallback((event: React.MouseEvent, node: Node) => {
        setHoveredNodeId(node.id);
    }, []);

    const onNodeMouseLeave = useCallback(() => {
        setHoveredNodeId(null);
    }, []);
    // --- End NEW Handlers ---

    // --- 修改 useImperativeHandle ---
    useImperativeHandle(ref, () => ({
        addNode,
        validateConnectivity,
        triggerLayout: handleLayout,
        getYamlString: (protocolId: string, version: string) => { // <-- Accept params here
            console.log(`[FlowCanvas] getYamlString called with protocolId: ${protocolId}, version: ${version}`);

            // 直接从 reactFlowInstance 获取最新状态，而不是使用组件的 state
            // 这确保我们能够捕获最新添加的节点，即使它们还没有完全反映在 state 中
            const currentNodes = reactFlowInstance.getNodes() as Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[];
            const currentEdges = reactFlowInstance.getEdges() as Edge<EdgeData>[];

            console.log(`[FlowCanvas getYamlString] Using latest instance state: ${currentNodes.length} nodes, ${currentEdges.length} edges`);

            // --- 添加日志：打印 Loop 节点数据 ---
            const loopNodesData = currentNodes
                .filter(node => node.type === 'loop')
                .map(node => ({ id: node.id, data: node.data }));
            console.log('[FlowCanvas getYamlString] Loop Node Data before conversion:', JSON.stringify(loopNodesData, null, 2));
            // --- 结束日志 ---

            if (currentNodes.length === 0) {
                console.log("[FlowCanvas getYamlString] No nodes, returning empty YAML");
                // --- MODIFICATION: Use underscore separator for fallback ---
                const rootKey = `${protocolId || 'unknown'}_${version || 'unknown'}`; // Use underscore
                return yaml.dump({ [rootKey]: [] }, { lineWidth: -1, sortKeys: false });
            }

            try {
                // Pass protocolId and version here
                const newYaml = convertFlowToYaml(currentNodes, currentEdges, protocolId, version);
                console.log("[FlowCanvas getYamlString] Generated YAML length:", newYaml.length);
                return newYaml;
            } catch (error) {
                console.error("[FlowCanvas getYamlString] Error generating YAML:", error);
                // --- MODIFICATION: Use underscore separator for fallback ---
                const rootKey = `${protocolId || 'unknown'}_${version || 'unknown'}`; // Use underscore
                return yaml.dump({ [rootKey]: [] }, { lineWidth: -1, sortKeys: false }); // Return safe YAML
            }
        }
    }));

    // Handle connection creation - Store targetId in source node temp data
    const onConnect = useCallback((connection: Connection) => {
        const { source, target } = connection;
        if (!source || !target) return;

        // --- MODIFIED: Add LoopNodeData to union when finding nodes ---
        const sourceNode = nodes.find(n => n.id === source);
        const targetNode = nodes.find(n => n.id === target);
        // --- END MODIFICATION ---

        if (!sourceNode || !targetNode) {
            console.error("Source or target node not found during connection");
            return;
        }

        // Check for self-loop connections
        if (source === target) {
            console.log("Detected self-loop connection. Creating loop node instead.");

            // Create a loop node
            const loopNode: Node<LoopNodeData> = {
                id: `loop-${Date.now()}`,
                type: 'loop',
                position: {
                    x: sourceNode.position.x - 50,
                    y: sourceNode.position.y - 50
                },
                data: {
                    type: 'loop',
                    loopCondition: 'true'
                }
            };

            // Update the source node to be a child of the loop node
            const updatedSourceNode = {
                ...sourceNode,
                parentId: loopNode.id,
                extent: 'parent' as const, // Use const assertion to specify exact type
                position: { x: 50, y: 50 } // Position relative to parent
            };

            // Remove the original node and add the loop and child node
            setNodes(nds => {
                const filteredNodes = nds.filter(n => n.id !== source);
                return [...filteredNodes, loopNode, updatedSourceNode];
            });

            // Update edges targeting the original node to target the loop node
            setEdges(eds => {
                return eds.map(edge => {
                    if (edge.target === source && edge.source !== loopNode.id) {
                        return { ...edge, target: loopNode.id };
                    }
                    return edge;
                });
            });

            return;
        }

        // Check for duplicate connections FIRST
        const existingEdgesToNewTarget = edges.filter(e => e.source === source && e.target === target);
        if (existingEdgesToNewTarget.length > 0) {
            toast.info("节点之间已经存在连接。");
            return;
        }

        // Check allowed source types
        if (sourceNode.type !== 'section' && sourceNode.type !== 'start' && sourceNode.type !== 'skip' && sourceNode.type !== 'loop') {
            toast.error("连接错误: 只有 Section, Skip, Loop 和 Start 节点可以有传出连接。");
            return;
        }

        // --- Refactor: Use helper function ---
        // Create the basic new edge (priority/default handled by helper)
        const newEdge: Edge<EdgeData> = {
            id: `edge-${source}-${target}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
            source,
            target,
            type: 'condition',
            data: { condition: "true" } // Start with a basic condition
        };

        // Call helper function to get the final edge list with updated priorities/defaults
        const updatedEdges = updateEdgesForSource(source, [...edges, newEdge]);

        setEdges(updatedEdges);
        // --- End Refactor ---

    }, [nodes, edges, setNodes, setEdges]); // Note: updateEdgesForSource is not a dep as it's defined outside

    // --- Vars Dynamic Form Handlers ---
    const handleVarChange = (id: number, field: 'key' | 'value', value: string) => {
        setVarEntries(currentEntries => {
            let updatedEntries = currentEntries.map(entry =>
                entry.id === id ? { ...entry, [field]: value } : entry
            );

            // Check if the edited entry is now empty
            const editedEntry = updatedEntries.find(entry => entry.id === id);
            const editedEntryIsEmpty = editedEntry && editedEntry.key.trim() === '' && editedEntry.value.trim() === '';

            // Auto-delete: If an entry becomes empty and it's not the only entry, remove it
            if (editedEntryIsEmpty && updatedEntries.length > 1) {
                updatedEntries = updatedEntries.filter(entry => entry.id !== id);
            }
            // Note: We don't automatically delete the *only* entry if it becomes empty.
            // It will be filtered out during the save process (handleApplyEdit).

            return updatedEntries;
        });
    };

    // --- Dev Dynamic Form Handlers ---
    const handleDeviceNameChange = (deviceId: number, value: string) => {
        setDevEntries(currentEntries => {
            let updatedEntries = currentEntries.map(entry =>
                entry.id === deviceId ? { ...entry, deviceName: value } : entry
            );

            // Auto-delete device if name is cleared and it only contains one empty field
            const editedDevice = updatedEntries.find(entry => entry.id === deviceId);
            // Check if the device name is empty *and* all its fields are effectively empty placeholders
            const allFieldsEmpty = editedDevice?.fields.every(f => f.key.trim() === '' && f.value.trim() === '');
            if (editedDevice && editedDevice.deviceName.trim() === '' && allFieldsEmpty) {
                // Only delete if it's not the only device entry
                if (updatedEntries.length > 1) {
                    updatedEntries = updatedEntries.filter(entry => entry.id !== deviceId);
                }
            }

            return updatedEntries;
        });
    };

    const handleDevFieldChange = (deviceId: number, fieldId: number, field: 'key' | 'value', value: string) => {
        setDevEntries(currentEntries => {
            let deviceIndex = -1;
            let fieldIndex = -1;

            // Find indices and update the specific field
            let updatedEntries = currentEntries.map((deviceEntry, dIdx) => {
                if (deviceEntry.id === deviceId) {
                    deviceIndex = dIdx;
                    const updatedFields = deviceEntry.fields.map((fieldEntry, fIdx) => {
                        if (fieldEntry.id === fieldId) {
                            fieldIndex = fIdx;
                            return { ...fieldEntry, [field]: value };
                        }
                        return fieldEntry;
                    });
                    return { ...deviceEntry, fields: updatedFields };
                }
                return deviceEntry;
            });

            // --- Auto-delete field --- Find the possibly updated field
            const targetDevice = updatedEntries[deviceIndex];
            const editedField = targetDevice?.fields[fieldIndex];
            const editedFieldIsEmpty = editedField && editedField.key.trim() === '' && editedField.value.trim() === '';

            if (editedFieldIsEmpty && targetDevice.fields.length > 1) {
                updatedEntries = updatedEntries.map((devEntry, dIdx) => {
                    if (dIdx === deviceIndex) {
                        return {
                            ...devEntry,
                            fields: devEntry.fields.filter(f => f.id !== fieldId)
                        };
                    }
                    return devEntry;
                });
            }

            return updatedEntries;
        });
    };

    // Restore handleAddField function
    const handleAddField = (deviceId: number) => {
        setDevEntries(currentEntries =>
            currentEntries.map(deviceEntry => {
                if (deviceEntry.id === deviceId) {
                    // Add a new empty field to this specific device
                    return {
                        ...deviceEntry,
                        fields: [...deviceEntry.fields, { id: Date.now(), key: '', value: '' }]
                    };
                }
                return deviceEntry;
            })
        );
    };

    // Restore handleAddDevice function
    const handleAddDevice = () => {
        setDevEntries(currentEntries => [
            ...currentEntries,
            // Add a new empty device with one empty field
            { id: Date.now(), deviceName: '', fields: [{ id: Date.now() + 1, key: '', value: '' }] }
        ]);
    };

    // --- Restore handleEditFormChange definition BEFORE handleApplyEdit ---
    const handleEditFormChange = (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const { name, value, type } = event.target;
        setEditFormData(prev => ({
            ...prev,
            [name]: type === 'number' ? parseInt(value, 10) || 0 : value
        }));
    };

    // --- handleApplyEdit ---
    const handleApplyEdit = useCallback(() => {
        if (!selectedNode) return;
        const varsObject: Record<string, string> = varEntries.reduce((acc, entry) => {
            if (entry.key.trim()) {
                acc[entry.key.trim()] = entry.value;
            }
            return acc;
        }, {} as Record<string, string>);
        const devObject: Record<string, Record<string, string>> = devEntries.reduce((acc, deviceEntry) => {
            const deviceName = deviceEntry.deviceName.trim();
            if (!deviceName) return acc;
            const fieldsObject = deviceEntry.fields.reduce((fieldsAcc, fieldEntry) => {
                const fieldKey = fieldEntry.key.trim();
                if (fieldKey || fieldEntry.value.trim()) {
                    fieldsAcc[fieldKey] = fieldEntry.value;
                }
                return fieldsAcc;
            }, {} as Record<string, string>);
            if (Object.keys(fieldsObject).length > 0) {
                acc[deviceName] = fieldsObject;
            }
            return acc;
        }, {} as Record<string, Record<string, string>>);

        setNodes((nds) =>
            nds.map((n): Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData> => {
                if (n.id === selectedNode.id) {
                    let updatedData: SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData;
                    if (n.data.type === 'section' && isSectionNodeData(n.data)) {
                        const currentData: SectionNodeData = n.data;
                        const formData = editFormData as Partial<Omit<SectionNodeData, 'Vars' | 'Dev' | 'Next'>>;
                        updatedData = {
                            ...currentData,
                            desc: formData.desc ?? currentData.desc,
                            size: formData.size ?? currentData.size,
                            Label: formData.Label ?? currentData.Label,
                            Vars: varsObject,
                            Dev: devObject,
                        };
                    } else if (n.data.type === 'skip') {
                        const currentData: SkipNodeData = n.data;
                        const formData = editFormData as Partial<SkipNodeData>;
                        updatedData = {
                            ...currentData,
                            size: formData.size ?? currentData.size,
                            // Keep parentNode/extent if they exist, but don't edit here
                            parentNode: n.data.parentNode,
                            extent: n.data.extent,
                        };
                    } else if (n.data.type === 'end') {
                        const currentData: EndNodeData = n.data;
                        const formData = editFormData as Partial<EndNodeData>;
                        updatedData = {
                            ...currentData,
                            type: formData.type ?? currentData.type,
                            // Keep parentNode/extent if they exist, but don't edit here
                            parentNode: n.data.parentNode,
                            extent: n.data.extent,
                        };
                    } else if (n.data.type === 'start') {
                        const currentData: StartNodeData = n.data;
                        const formData = editFormData as Partial<StartNodeData>;
                        updatedData = {
                            ...currentData,
                            desc: formData.desc ?? currentData.desc,
                            // Keep parentNode/extent if they exist, but don't edit here
                            parentNode: n.data.parentNode,
                            extent: n.data.extent,
                        };
                    } else if (n.data.type === 'loop') {
                        // 处理循环节点
                        const currentData = n.data as LoopNodeData;
                        const formData = editFormData as Partial<LoopNodeData>;
                        updatedData = {
                            ...currentData,
                            loopCondition: formData.loopCondition ?? currentData.loopCondition,
                            // 保留父节点和范围属性
                            parentNode: n.data.parentNode,
                            extent: n.data.extent,
                        };
                    } else {
                        updatedData = n.data;
                    }
                    return { ...n, data: updatedData };
                }
                return n;
            })
        );
        setIsPopoverOpen(false);
        setSelectedNode(null);
        setPopoverPosition(null);
        setVarEntries([]);
        setDevEntries([]);
    }, [
        selectedNode,
        editFormData,
        varEntries,
        devEntries,
        setNodes,
        setIsPopoverOpen,
        setSelectedNode,
        setPopoverPosition,
        setVarEntries,
        setDevEntries,
    ]);

    // 保存边条件 - ALWAYS set isDefault to false on save
    const handleSaveEdgeCondition = useCallback(() => {
        if (!selectedEdge) return;
        console.log('[handleSaveEdgeCondition] Saving condition for edge:', selectedEdge.id, 'New condition:', edgeCondition);

        setEdges(eds => eds.map(e => {
            if (e.id === selectedEdge.id) {
                return {
                    ...e,
                    data: {
                        ...e.data,
                        isDefault: false,
                        condition: edgeCondition || "true",
                        priority: e.data?.priority ?? 0 // 保持现有优先级
                    }
                };
            }
            return e;
        }));

        setIsEdgePopoverOpen(false);
        setSelectedEdge(null);
        console.log('[handleSaveEdgeCondition] Popover closed.');
    }, [selectedEdge, edgeCondition, setEdges, setIsEdgePopoverOpen, setSelectedEdge]);

    // --- MODIFIED: Edge click now opens Context Menu ---
    const handleEdgeClick = useCallback((event: React.MouseEvent, edge: Edge<EdgeData>) => {
        console.log('[handleEdgeClick] Triggered for context menu on edge:', edge.id);
        event.preventDefault();
        event.stopPropagation();

        // Close other popovers first
        if (isPopoverOpen) { console.log('[handleEdgeClick] Closing node popover...'); handleApplyEdit(); }
        if (isEdgePopoverOpen) { console.log('[handleEdgeClick] Closing edge condition popover...'); handleSaveEdgeCondition(); }

        // Calculate position
        const screenX = event.clientX;
        const screenY = event.clientY;

        // Open context menu
        setContextMenuEdge(edge);
        setContextMenuPosition({ top: screenY + 5, left: screenX + 5 });
        setIsContextMenuOpen(true);
        console.log('[handleEdgeClick] Context menu state set for edge:', edge.id);

    }, [
        // --- RESTORED Dependencies ---
        isPopoverOpen, isEdgePopoverOpen,
        handleApplyEdit, handleSaveEdgeCondition,
        setContextMenuEdge, setContextMenuPosition, setIsContextMenuOpen,
        // Include setters potentially used by handleSaveEdgeCondition indirectly or for stability
        reactFlowInstance, setSelectedEdge, setEdgeCondition, setEdgePopoverPosition, setIsEdgePopoverOpen, selectedEdge
    ]);

    // 节点点击处理 - Allow opening popover for Skip nodes too
    // --- MODIFIED: Add LoopNodeData to union ---
    const onNodeClick = useCallback((event: React.MouseEvent, node: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>) => {
        // --- END MODIFICATION ---
        // --- Close context menu if open ---
        if (isContextMenuOpen) {
            setIsContextMenuOpen(false);
            setContextMenuEdge(null);
        }
        // --- End close context menu ---

        // Close edge popover if open
        if (isEdgePopoverOpen) { handleSaveEdgeCondition(); }

        // Prevent re-opening node popover if same node clicked
        if (selectedNode && node.id === selectedNode.id && isPopoverOpen) { return; }

        // Save previous node edit if switching
        if (selectedNode && node.id !== selectedNode.id && isPopoverOpen) { handleApplyEdit(); }

        const flowNode = reactFlowInstance.getNode(node.id);
        if (!flowNode) return;

        // --- Open Popover for BOTH Section and Skip Nodes ---
        // --- MODIFIED: Exclude 'loop' from opening popover for now ---
        if (node.type === 'section' || node.type === 'skip') {
            // --- END MODIFICATION ---
            setSelectedNode(node);
            setEditFormData({ ...node.data });

            // Initialize Vars/Dev ONLY for Section nodes
            if (node.type === 'section' && isSectionNodeData(node.data)) {
                const initialVarsData = node.data.Vars || {};
                setVarEntries(Object.entries(initialVarsData).map(([key, value], index) => ({ id: Date.now() + index, key, value })));
                const initialDevData = node.data.Dev || {};
                setDevEntries(Object.entries(initialDevData).map(([deviceName, fields], deviceIndex) => ({
                    id: Date.now() + deviceIndex * 1000,
                    deviceName,
                    fields: Object.entries(fields).map(([key, value], fieldIndex) => ({ id: Date.now() + deviceIndex * 1000 + fieldIndex + 1, key, value }))
                })));
            } else {
                // Ensure Vars/Dev are empty for Skip nodes
                setVarEntries([]);
                setDevEntries([]);
            }

            // Calculate popover position (same for both)
            const nodeRect = flowNode.measured;
            const nodePosition = flowNode.position ? flowNode.position : { x: 0, y: 0 };
            const nodeWidth = nodeRect?.width || 180;

            // --- NEW: 检查节点是否在循环节点内部，如果是，需考虑父节点位置 ---
            let targetFlowPosition;
            if (flowNode.parentId) {
                // 如果节点有父节点，我们需要考虑相对于父节点的位置
                const parentNode = reactFlowInstance.getNode(flowNode.parentId);
                if (parentNode && parentNode.type === 'loop') {
                    // 获取父节点的位置
                    const parentPosition = parentNode.position || { x: 0, y: 0 };

                    // 计算子节点相对于父节点的实际位置
                    // 子节点的position是相对于父节点的，所以我们需要加上父节点的位置
                    const absoluteNodePosition = {
                        x: parentPosition.x + nodePosition.x,
                        y: parentPosition.y + nodePosition.y
                    };

                    // 设置悬浮窗口位置在节点的右侧
                    targetFlowPosition = {
                        x: absoluteNodePosition.x + nodeWidth + 10,
                        y: absoluteNodePosition.y
                    };

                    console.log(`[onNodeClick] Node ${node.id} is inside loop node ${flowNode.parentId}. Setting popover position based on absolute position.`);
                } else {
                    // 如果父节点不是循环节点，使用默认位置计算
                    targetFlowPosition = { x: nodePosition.x + nodeWidth + 10, y: nodePosition.y };
                }
            } else {
                // 没有父节点，使用默认位置计算
                targetFlowPosition = { x: nodePosition.x + nodeWidth + 10, y: nodePosition.y };
            }
            // --- END NEW ---

            const screenPosition = reactFlowInstance.flowToScreenPosition(targetFlowPosition);
            setPopoverPosition({ top: screenPosition.y, left: screenPosition.x });

            // Open the popover
            setIsPopoverOpen(true);
        }
        // 添加对Loop类型节点的支持
        else if (node.type === 'loop') {
            setSelectedNode(node);
            setEditFormData({ ...node.data });

            // 循环节点不需要Vars/Dev
            setVarEntries([]);
            setDevEntries([]);

            // 计算弹出位置
            const nodeRect = flowNode.measured;
            const nodePosition = flowNode.position ? flowNode.position : { x: 0, y: 0 };
            const nodeWidth = nodeRect?.width || 180;

            // 设置弹出位置
            const targetFlowPosition = { x: nodePosition.x + nodeWidth + 10, y: nodePosition.y };
            const screenPosition = reactFlowInstance.flowToScreenPosition(targetFlowPosition);
            setPopoverPosition({ top: screenPosition.y, left: screenPosition.x });

            // 打开弹出窗口
            setIsPopoverOpen(true);
        }
        // --- REMOVED else block that explicitly closed popover ---

    }, [
        isContextMenuOpen, setIsContextMenuOpen, setContextMenuEdge, // Added context menu state/setters
        isEdgePopoverOpen, handleSaveEdgeCondition,
        selectedNode, isPopoverOpen, handleApplyEdit,
        reactFlowInstance,
        setSelectedNode, setEditFormData, setVarEntries, setDevEntries, setIsPopoverOpen, setPopoverPosition,
        setIsEdgePopoverOpen, setSelectedEdge // Keep these from previous logic
    ]);

    // Pane click handler - ADD closing Context Menu
    const handlePaneClick = () => {
        // --- Close context menu first if open ---
        if (isContextMenuOpen) {
            setIsContextMenuOpen(false);
            setContextMenuEdge(null);
        }
        // --- Then handle other popovers ---
        else if (selectedNode && isPopoverOpen) {
            handleApplyEdit();
        }
        else if (isEdgePopoverOpen) {
            handleSaveEdgeCondition();
        }
        // --- Else reset everything if nothing was open ---
        else {
            setIsPopoverOpen(false); setSelectedNode(null); setPopoverPosition(null);
            setVarEntries([]); setDevEntries([]);
            setIsEdgePopoverOpen(false); setSelectedEdge(null);
            setIsContextMenuOpen(false); setContextMenuEdge(null); // Ensure context menu is reset here too
        }
    };

    // --- Restore addNode definition ---
    // --- MODIFIED: Add 'loop' type, LoopNodeData ---
    const addNode = useCallback((type: 'section' | 'skip' | 'end' | 'start' | 'loop') => {
        // --- END MODIFICATION ---
        // 如果尝试添加开始节点，直接返回
        if (type === 'start') {
            toast.info('不能添加多个开始节点');
            return;
        }

        if (!reactFlowInstance) { console.error("React Flow instance not available"); return; }

        // --- Calculate yamlIndex for new manual node ---
        // 使用 reactFlowInstance.getNodes() 获取最新状态
        const currentNodes = reactFlowInstance.getNodes();
        const sectionOrSkipNodeCount = currentNodes.filter(n => n.type === 'section' || n.type === 'skip').length;
        const newYamlIndex = sectionOrSkipNodeCount; // 从0开始计数，所以数量就是下一个索引
        // --- End calculation ---

        const screenX = (reactFlowWrapper.current?.clientWidth ?? window.innerWidth) / 2;
        const screenY = (reactFlowWrapper.current?.clientHeight ?? window.innerHeight) / 3;
        const position = reactFlowInstance.screenToFlowPosition({ x: screenX, y: screenY });

        let newNode: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>;

        if (type === 'section') {
            // const sectionNodeCount = nodes.filter(n => n.type === 'section').length; // 旧逻辑
            newNode = {
                id: `section-${Date.now()}`,
                type: 'section',
                position,
                data: { desc: `Section #${newYamlIndex + 1}`, size: 1, type: 'section', yamlIndex: newYamlIndex } // 添加 yamlIndex
            };
        } else if (type === 'skip') {
            newNode = { id: `skip-${Date.now()}`, type: 'skip', position, data: { size: 1, type: 'skip', yamlIndex: newYamlIndex } }; // 添加 yamlIndex
        } else if (type === 'end') {
            // End 节点不需要 yamlIndex
            newNode = { id: `end-${Date.now()}`, type: 'end', position, data: { type: 'end' } };
        } else if (type === 'loop') {
            newNode = { id: `loop-${Date.now()}`, type: 'loop', position, data: { type: 'loop', loopCondition: 'true' } };
        } else {
            return; // 未知类型 (实际上 Start 已被过滤)
        }

        // 添加新节点
        const newNodeId = newNode.id;
        setNodes(prevNodes => [...prevNodes, newNode]);

        // 延迟执行以确保新节点已经被添加
        setTimeout(() => {
            const currentNodes = reactFlowInstance.getNodes();
            const startNode = currentNodes.find(node => node.type === 'start');
            if (!startNode) return;

            const nonStartNodes = currentNodes.filter(node => node.type !== 'start');
            const isFirstNonStartNode = nonStartNodes.length === 1 && nonStartNodes[0].id === newNodeId;

            if (isFirstNonStartNode) {
                console.log(`[addNode] 添加从开始节点到第一个节点的连接: ${startNode.id} -> ${newNodeId}`);
                const startEdge: Edge<EdgeData> = {
                    id: `edge-${startNode.id}-${newNodeId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                    source: startNode.id,
                    target: newNodeId,
                    data: {
                        isDefault: true,
                        priority: 0 // 第一条边优先级为0
                    },
                    type: 'condition'
                };
                setEdges(prevEdges => [...prevEdges, startEdge]);
            }
        }, 50);
    }, [reactFlowInstance, setNodes, setEdges, nodes]);

    // --- 移除 isNodeConnectedToStart 辅助函数 ---

    // --- Restore validateConnectivity definition ---
    const validateConnectivity = useCallback(() => {
        const startNode = nodes.find(node => node.type === 'start');
        if (!startNode) {
            return { isValid: false, unconnectedNodeIds: nodes.map(n => n.id) };
        }

        // 执行可达性分析，从开始节点出发
        const reachableNodes = new Set<string>();
        const queue: string[] = [startNode.id];
        reachableNodes.add(startNode.id);

        while (queue.length > 0) {
            const currentNodeId = queue.shift()!;
            const outgoingEdges = edges.filter(edge => edge.source === currentNodeId);

            for (const edge of outgoingEdges) {
                const targetId = edge.target;
                if (!reachableNodes.has(targetId)) {
                    reachableNodes.add(targetId);
                    queue.push(targetId);
                }
            }
        }

        // 收集未连接的节点ID
        const unconnectedNodeIds = nodes
            .filter(node => !reachableNodes.has(node.id))
            .map(node => node.id);

        return {
            isValid: unconnectedNodeIds.length === 0,
            unconnectedNodeIds
        };
    }, [nodes, edges]);

    // 禁止删除开始节点
    const onNodesDelete = useCallback((nodesToRemove: Node[]) => {
        // 过滤掉开始节点，避免被删除
        const filteredNodes = nodesToRemove.filter(node => node.type !== 'start');

        if (filteredNodes.length < nodesToRemove.length) {
            toast.info('开始节点不能被删除');
        }

        // 如果过滤后没有节点需要被删除，阻止删除操作
        return filteredNodes.length === 0;
    }, []);

    // --- 添加手动布局函数 ---
    const handleLayout = useCallback((direction = 'TB') => {
        console.log(`[handleLayout] Triggered with direction: ${direction}`); // Log trigger
        // 使用 reactFlowInstance.getNodes() 和 reactFlowInstance.getEdges() 获取当前状态
        const currentNodes = reactFlowInstance.getNodes();
        const currentEdges = reactFlowInstance.getEdges();
        console.log(`[handleLayout] Nodes before layout (${currentNodes.length}):`, JSON.stringify(currentNodes.map(n => ({ id: n.id, type: n.type, pos: n.position }))));

        const layouted = getLayoutedElements(
            currentNodes as Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>[], // 明确类型
            currentEdges as Edge<EdgeData>[], // 明确类型
            direction
        );

        console.log(`[handleLayout] Nodes after layout (${layouted.nodes.length}):`, JSON.stringify(layouted.nodes.map(n => ({ id: n.id, type: n.type, pos: n.position }))));

        // setNodes 现在接收精确类型，无需额外转换
        console.log("[handleLayout] Calling setNodes and setEdges...");
        setNodes(layouted.nodes);
        setEdges(layouted.edges);

        // 手动布局后fitView可能需要调整
        console.log("[handleLayout] Requesting fitView...");
        window.requestAnimationFrame(() => {
            reactFlowInstance?.fitView();
            console.log("[handleLayout] fitView completed.");
        });
    }, [reactFlowInstance, setNodes, setEdges]);


    // ... useMemo hooks for nodesWithEditingState and edgesWithSelection ...
    // --- REMOVED edgesWithSelection, combined into edgesWithHighlight ---
    // const edgesWithSelection = useMemo(() => { ... }, []);

    // --- 添加回调：用于添加连接节点 ---
    const handleAddConnectedNode = useCallback((sourceNodeId: string, newNodeType: 'section' | 'skip' | 'end' | 'loop') => {
        // --- END MODIFICATION ---
        if (!reactFlowInstance) return;

        // --- MODIFIED: Add LoopNodeData to union ---
        const sourceNode = reactFlowInstance.getNode(sourceNodeId);
        // --- END MODIFICATION ---
        if (!sourceNode) return;

        // 计算新节点位置 (大致在下方)
        const sourcePos = sourceNode.position || { x: 0, y: 0 };
        const sourceHeight = sourceNode.measured?.height || 75; // 使用测量高度或默认值
        const position = {
            x: sourcePos.x,
            y: sourcePos.y + sourceHeight + 80 // 间距 80
        };

        // 计算新节点的 yamlIndex (基于当前 section/skip 数量)
        const currentNodes = reactFlowInstance.getNodes();
        const sectionOrSkipNodeCount = currentNodes.filter(n => n.type === 'section' || n.type === 'skip').length;
        const newYamlIndex = sectionOrSkipNodeCount;

        let newNode: Node<SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData>;
        const newNodeId = `${newNodeType}-${Date.now()}`;

        // --- NEW: 检查源节点是否有 parentId (在 LoopNode 内) ---
        const parentId = sourceNode.parentId;
        const isInsideLoop = !!parentId;
        // --- END NEW ---

        if (newNodeType === 'section') {
            newNode = {
                id: newNodeId,
                type: 'section',
                position,
                // --- NEW: 如果源节点在循环内，保持新节点也在循环内 ---
                ...(isInsideLoop ? { parentId, extent: 'parent' } : {}),
                // --- END NEW ---
                data: { desc: `新 Section`, size: 1, type: 'section', yamlIndex: newYamlIndex }
            };
        } else if (newNodeType === 'skip') {
            newNode = {
                id: newNodeId,
                type: 'skip',
                position,
                // --- NEW: 如果源节点在循环内，保持新节点也在循环内 ---
                ...(isInsideLoop ? { parentId, extent: 'parent' } : {}),
                // --- END NEW ---
                data: { size: 1, type: 'skip', yamlIndex: newYamlIndex }
            };
        } else if (newNodeType === 'end') { // end
            newNode = {
                id: newNodeId,
                type: 'end',
                position,
                // --- NEW: 如果源节点在循环内，保持新节点也在循环内 ---
                ...(isInsideLoop ? { parentId, extent: 'parent' } : {}),
                // --- END NEW ---
                data: { type: 'end' }
            };
        } else if (newNodeType === 'loop') {
            newNode = {
                id: newNodeId,
                type: 'loop',
                position,
                // --- NEW: 如果源节点在循环内，保持新节点也在循环内 ---
                ...(isInsideLoop ? { parentId, extent: 'parent' } : {}),
                // --- END NEW ---
                data: { type: 'loop', loopCondition: 'true' }
            };
        }

        // --- Refactor: Create basic edge, use helper for priority/default ---
        // 创建新边 (移除硬编码的 priority/isDefault)
        const newEdge: Edge<EdgeData> = {
            id: `edge-${sourceNodeId}-${newNodeId}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
            source: sourceNodeId,
            target: newNodeId,
            type: 'condition', // 或其他默认边类型
            // data: { isDefault: true, priority: 0 } // REMOVED
            data: { condition: "true" } // Default condition
        };

        // 更新状态 (先加 Node, 再用 helper 更新 Edge)
        setNodes((nds) => nds.concat(newNode));
        setEdges((currentEdges) => updateEdgesForSource(sourceNodeId, [...currentEdges, newEdge]));
        // --- End Refactor ---

        // 可选：自动布局或FitView
        // setTimeout(() => reactFlowInstance.fitView({ duration: 300 }), 50);

    }, [reactFlowInstance, setNodes, setEdges]);

    // --- MOVED UP: Definition before usage in useMemo deps ---
    // --- Handler for adding child nodes inside a LoopNode ---
    const handleAddChildNode = useCallback((parentId: string, newNodeType: 'section' | 'skip') => {
        // --- REMOVED: Log call ---
        // console.log('[handleAddChildNode] Called with parentId:', parentId, 'type:', newNodeType);
        // --- END REMOVED ---
        if (!reactFlowInstance) {
            console.error("React Flow instance not available for adding child node");
            return;
        }

        const parentNode = reactFlowInstance.getNode(parentId);
        // --- REMOVED: Log parent node ---
        // console.log('[handleAddChildNode] Found parentNode:', parentNode);
        // --- END REMOVED ---
        if (!parentNode || parentNode.type !== 'loop') {
            console.error("Parent node not found or is not a LoopNode");
            return;
        }

        // Calculate initial position relative to parent (e.g., top-center inside)
        const initialPosition = { x: 50, y: 50 }; // Simple initial offset

        // Calculate yamlIndex (following current pattern, may need refinement for nested logic)
        const currentNodes = reactFlowInstance.getNodes();
        const sectionOrSkipNodeCount = currentNodes.filter(n => n.type === 'section' || n.type === 'skip').length;
        const newYamlIndex = sectionOrSkipNodeCount;

        let newNode: Node<SectionNodeData | SkipNodeData>;
        const newNodeId = `${newNodeType}-child-${Date.now()}`;

        if (newNodeType === 'section') {
            newNode = {
                id: newNodeId,
                type: 'section',
                position: initialPosition,
                // --- Key additions for child nodes ---
                // --- MODIFIED: Use parentId instead of parentNode (Re-apply) ---
                parentId: parentId,
                // --- END MODIFICATION ---
                extent: 'parent',
                // --- End key additions ---
                data: { desc: `Child Section`, size: 1, type: 'section', yamlIndex: newYamlIndex }
            };
        } else { // skip
            newNode = {
                id: newNodeId,
                type: 'skip',
                position: initialPosition,
                // --- Key additions for child nodes ---
                // --- MODIFIED: Use parentId instead of parentNode (Re-apply) ---
                parentId: parentId,
                // --- END MODIFICATION ---
                extent: 'parent',
                // --- End key additions ---
                data: { size: 1, type: 'skip', yamlIndex: newYamlIndex }
            };
        }

        // --- REMOVED: Log new child node ---
        // console.log(`[handleAddChildNode] Adding new child node:`, newNode);
        // --- END REMOVED ---
        setNodes((nds) => nds.concat(newNode));

        // Optional: Maybe fit view or focus on parent after adding?
        // setTimeout(() => reactFlowInstance.fitView({ duration: 300, nodes: [parentNode] }), 50);

    }, [reactFlowInstance, setNodes]);
    // --- END MOVED UP ---


    // 修改 nodesWithEditingState 以添加 isHovered 和回调
    const nodesWithEditingState = useMemo(() => {
        // --- ADDED: Calculate which parents have children ---
        const childNodeParentIds = new Set(nodes.filter(n => n.parentId).map(n => n.parentId));
        // --- END ADDED ---

        return nodes.map((node, index) => {
            // --- MODIFIED: Refactor how callback is added --- Refactor data object construction
            // Create base data object first
            const baseData = {
                ...(node.data as SectionNodeData | SkipNodeData | EndNodeData | StartNodeData | LoopNodeData), // Type assertion
                isEditing: isPopoverOpen && selectedNode?.id === node.id,
                displayIndex: index, // 通用索引
                isHovered: node.id === hoveredNodeId, // <-- 添加悬停状态
                onAddConnectedNode: handleAddConnectedNode, // <-- 注入回调函数
            };

            // Conditionally add the child node callback
            if (node.type === 'loop') {
                // Use 'as any' temporarily to avoid complex type issues during refactor
                (baseData as any).onAddChildNode = handleAddChildNode;

                // 添加onUpdateNodeData回调函数
                (baseData as any).onUpdateNodeData = (nodeId: string, newData: Partial<LoopNodeData>) => {
                    console.log(`[onUpdateNodeData] Updating loop node ${nodeId} with:`, newData);

                    // 使用 setNodes 更新节点数据，确保类型兼容性
                    setNodes((prevNodes) => {
                        return prevNodes.map((node) => {
                            if (node.id === nodeId && node.type === 'loop') {
                                // 仅更新 loop 类型节点的数据，确保类型安全
                                return {
                                    ...node,
                                    data: {
                                        ...node.data,
                                        loopCondition: newData.loopCondition || node.data.loopCondition
                                    } as LoopNodeData // 明确类型断言
                                };
                            }
                            return node;
                        });
                    });
                };

                (baseData as any).hasChildren = childNodeParentIds.has(node.id);
            }
            // --- END MODIFICATION ---

            return {
                ...node,
                data: baseData // Assign the modified baseData object
            };
        });
        // 确保 handleAddConnectedNode 在依赖项中
        // --- MODIFIED: Add handleAddChildNode to dependency array ---
        // --- RE-APPLY FIX: Remove childNodeParentIds from deps --- Fixes runtime error
    }, [nodes, selectedNode, isPopoverOpen, hoveredNodeId, handleAddConnectedNode, handleAddChildNode, setNodes]);
    // --- END MODIFICATION ---

    // --- NEW: Memoized Edges with Highlighting ---
    const edgesWithHighlight = useMemo(() => {
        // 创建节点ID到父节点ID的映射，用于检查子节点到父节点的边
        const nodeParentMap = new Map<string, string>();
        nodes.forEach(node => {
            if (node.parentId) {
                nodeParentMap.set(node.id, node.parentId);
            }
        });

        // 首先过滤掉从子节点指向父节点的边
        let filteredEdges = edges.filter(edge => {
            // 检查源节点是否有parentId，如果有，检查目标是否是它的父节点
            const sourceParentId = nodeParentMap.get(edge.source);
            if (sourceParentId && sourceParentId === edge.target) {
                // console.log(`[edgesWithHighlight] Filtered out edge from child ${edge.source} to parent ${edge.target}`);
                return false; // 过滤掉这条边
            }
            return true; // 保留其他边
        });

        // --- 处理Loop节点内部的反向连接 ---
        // 1. 找出所有内部边（相同父节点的子节点之间的边）
        const internalEdges = filteredEdges.filter(edge => {
            const sourceParentId = nodeParentMap.get(edge.source);
            const targetParentId = nodeParentMap.get(edge.target);
            return sourceParentId && targetParentId && sourceParentId === targetParentId;
        });

        // 2. 找出反向连接对 (如 A->B 和 B->A)
        const connectionMap = new Map<string, Edge<EdgeData>[]>();

        // 对内部边进行分组，按照"小节点ID-大节点ID"的方式生成唯一键
        // 这样A->B和B->A会映射到同一个键
        internalEdges.forEach(edge => {
            // 确保源节点和目标节点ID按字母顺序排序，生成唯一的连接键
            const [smallerId, largerId] = [edge.source, edge.target].sort();
            const connectionKey = `${smallerId}-${largerId}`;

            if (!connectionMap.has(connectionKey)) {
                connectionMap.set(connectionKey, []);
            }
            connectionMap.get(connectionKey)!.push(edge);
        });

        // 3. 对于每一组反向连接，只保留优先级最高的一条边
        const edgesToKeep = new Set<string>();

        connectionMap.forEach((edgeGroup, connectionKey) => {
            if (edgeGroup.length > 1) {
                // 检查是否存在反向连接 (A->B 和 B->A)
                const hasReverseConnection = edgeGroup.some(e1 =>
                    edgeGroup.some(e2 => e1.source === e2.target && e1.target === e2.source)
                );

                if (hasReverseConnection) {
                    // 按优先级排序 (数字越小优先级越高)
                    edgeGroup.sort((a, b) => {
                        const priorityA = a.data?.priority ?? Number.MAX_SAFE_INTEGER;
                        const priorityB = b.data?.priority ?? Number.MAX_SAFE_INTEGER;
                        return priorityA - priorityB;
                    });

                    // 保留优先级最高的边
                    const highestPriorityEdge = edgeGroup[0];
                    edgesToKeep.add(highestPriorityEdge.id);

                    // 记录过滤情况 (移除的边与保留的边)
                    const removedEdges = edgeGroup.slice(1).map(e => e.id).join(', ');
                    // console.log(`[edgesWithHighlight] Found reverse connection in loop: ${connectionKey}. Keeping edge ${highestPriorityEdge.id}, removing: ${removedEdges}`);
                } else {
                    // 如果不是反向连接，保留所有边
                    edgeGroup.forEach(edge => edgesToKeep.add(edge.id));
                }
            } else {
                // 只有一条边，直接保留
                edgeGroup.forEach(edge => edgesToKeep.add(edge.id));
            }
        });

        // 4. 应用过滤 - 保留非内部边和已标记为保留的内部边
        filteredEdges = filteredEdges.filter(edge => {
            const sourceParentId = nodeParentMap.get(edge.source);
            const targetParentId = nodeParentMap.get(edge.target);
            const isInternalEdge = sourceParentId && targetParentId && sourceParentId === targetParentId;

            // 如果是内部边，检查是否在保留列表中
            if (isInternalEdge) {
                return edgesToKeep.has(edge.id);
            }

            // 非内部边保留
            return true;
        });

        // 然后处理选中状态
        let processedEdges = filteredEdges.map(edge => ({
            ...edge,
            selected: selectedEdge?.id === edge.id || contextMenuEdge?.id === edge.id,
        }));

        // 然后基于悬停状态添加动画
        processedEdges = processedEdges.map(edge => ({
            ...edge,
            animated: edge.source === hoveredNodeId || edge.target === hoveredNodeId
        }));

        return processedEdges;
    }, [edges, nodes, selectedEdge, contextMenuEdge, hoveredNodeId]); // 添加 nodes 到依赖数组
    // --- End Memoized Edges ---


    // --- NEW: Handler for Modify option in Context Menu ---
    const handleModifyEdge = useCallback(() => {
        if (!contextMenuEdge) return;
        // ... existing code ...
        const edgeToModify = contextMenuEdge;
        const position = contextMenuPosition;
        setIsContextMenuOpen(false);
        setContextMenuEdge(null);
        setSelectedEdge(edgeToModify);
        setEdgeCondition(edgeToModify.data?.condition || "true");
        setEdgePopoverPosition(position);
        setIsEdgePopoverOpen(true);
    }, [contextMenuEdge, contextMenuPosition, setIsContextMenuOpen, setSelectedEdge, setEdgeCondition, setEdgePopoverPosition, setIsEdgePopoverOpen, setContextMenuEdge]);

    // --- NEW: Handler for Delete option in Context Menu ---
    const handleDeleteEdge = useCallback(() => {
        if (!contextMenuEdge) return;
        // ... (Implementation assumed correct)
        const edgeToDeleteId = contextMenuEdge.id;
        const deletedEdgeSourceId = contextMenuEdge.source;
        setIsContextMenuOpen(false);
        setContextMenuEdge(null);
        setEdges((eds) => {
            const remainingEdges = eds.filter((edge) => edge.id !== edgeToDeleteId);
            return updateEdgesForSource(deletedEdgeSourceId, remainingEdges);
        });
    }, [contextMenuEdge, setEdges, setIsContextMenuOpen, setContextMenuEdge]);

    // 添加处理优先级调整的函数
    const handleIncreasePriority = useCallback(() => {
        if (!contextMenuEdge) return;
        // ... (Implementation assumed correct)
        const sourceId = contextMenuEdge.source;
        setEdges(eds => {
            const sourceEdges = eds.filter(e => e.source === sourceId);
            sourceEdges.sort((a, b) => (a.data?.priority ?? 0) - (b.data?.priority ?? 0));
            const currentIndex = sourceEdges.findIndex(e => e.id === contextMenuEdge.id);
            if (currentIndex <= 0) return eds;
            const edgeToIncrease = sourceEdges[currentIndex];
            const edgeToDecrease = sourceEdges[currentIndex - 1];
            const priorityToSetForIncrease = edgeToDecrease.data?.priority ?? 0;
            const priorityToSetForDecrease = edgeToIncrease.data?.priority ?? 0;
            const swappedEdges = eds.map(edge => {
                if (edge.id === edgeToIncrease.id) return { ...edge, data: { ...edge.data, priority: priorityToSetForIncrease } };
                if (edge.id === edgeToDecrease.id) return { ...edge, data: { ...edge.data, priority: priorityToSetForDecrease } };
                return edge;
            });
            return updateEdgesForSource(sourceId, swappedEdges);
        });
        setIsContextMenuOpen(false);
        setContextMenuEdge(null);
    }, [contextMenuEdge, setEdges, setIsContextMenuOpen, setContextMenuEdge]);

    const handleDecreasePriority = useCallback(() => {
        if (!contextMenuEdge) return;
        // ... (Implementation assumed correct)
        const sourceId = contextMenuEdge.source;
        setEdges(eds => {
            const sourceEdges = eds.filter(e => e.source === sourceId);
            sourceEdges.sort((a, b) => (a.data?.priority ?? 0) - (b.data?.priority ?? 0));
            const currentIndex = sourceEdges.findIndex(e => e.id === contextMenuEdge.id);
            if (currentIndex === -1 || currentIndex >= sourceEdges.length - 1) return eds;
            const edgeToDecrease = sourceEdges[currentIndex];
            const edgeToIncrease = sourceEdges[currentIndex + 1];
            const priorityToSetForDecrease = edgeToIncrease.data?.priority ?? 0;
            const priorityToSetForIncrease = edgeToDecrease.data?.priority ?? 0;
            const swappedEdges = eds.map(edge => {
                if (edge.id === edgeToDecrease.id) return { ...edge, data: { ...edge.data, priority: priorityToSetForDecrease } };
                if (edge.id === edgeToIncrease.id) return { ...edge, data: { ...edge.data, priority: priorityToSetForIncrease } };
                return edge;
            });
            return updateEdgesForSource(sourceId, swappedEdges);
        });
        setIsContextMenuOpen(false);
        setContextMenuEdge(null);
    }, [contextMenuEdge, setEdges, setIsContextMenuOpen, setContextMenuEdge]);

    // --- Restore Return Statement and Closing Braces ---
    return (
        <div ref={reactFlowWrapper} className="w-full h-full">
            <ReactFlow
                nodes={nodesWithEditingState}
                edges={edgesWithHighlight}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
                fitView
                onNodeClick={onNodeClick}
                onEdgeClick={handleEdgeClick}
                onPaneClick={handlePaneClick}
                onNodesDelete={onNodesDelete}
                onEdgesDelete={() => { /* Keep empty or implement later */ }}
                onNodeMouseEnter={onNodeMouseEnter}
                onNodeMouseLeave={onNodeMouseLeave}
            >
                <Controls />
                <MiniMap pannable={true} />
                <Background />

                {/* Node Popover - Restore basic structure */}
                <Popover open={isPopoverOpen} onOpenChange={setIsPopoverOpen}>
                    <PopoverContent
                        sideOffset={10}
                        align="start"
                        className={cn(
                            "z-50 bg-background shadow-md rounded-lg border border-gray-200 p-0",
                            selectedNode?.data.type === 'skip' ? 'w-50' : 'w-96'
                        )}
                        style={{
                            position: 'absolute',
                            top: `${popoverPosition?.top ?? 0}px`,
                            left: `${popoverPosition?.left ?? 0}px`,
                        }}
                        onInteractOutside={(e) => { if (selectedNode && isPopoverOpen) { handleApplyEdit(); } else { /* basic reset */ setIsPopoverOpen(false); setSelectedNode(null); } }}
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        {/* --- Restore Full Popover Content --- */}
                        {selectedNode && (
                            <ScrollArea className="max-h-[60vh] px-4 py-5">
                                <div className="grid gap-5">
                                    <div className="grid gap-4">
                                        {selectedNode.data.type === 'section' && (
                                            <div className="grid grid-cols-4 items-center gap-x-4 gap-y-4">
                                                <Label htmlFor="edit-desc" className="text-right col-span-1 text-sm text-muted-foreground">📝 描述</Label>
                                                <Input id="edit-desc" name="desc" value={editFormData.desc || ''} onChange={handleEditFormChange} className="col-span-3 h-9 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0" />
                                                <Label htmlFor="edit-size" className="text-right col-span-1 text-sm text-muted-foreground">📏 大小</Label>
                                                <Input id="edit-size" name="size" type="number" value={editFormData.size || 0} onChange={handleEditFormChange} className="col-span-1 h-9 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0" />
                                                <Input id="edit-label" name="Label" value={editFormData.Label || ''} onChange={handleEditFormChange} placeholder="🏷️ 标签 (可选)" className="col-span-2 h-9 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0" />

                                                {/* Dev Section */}
                                                <>
                                                    <div className="col-span-4 flex justify-between items-center border-t pt-4 mt-1">
                                                        <Label className="text-base font-medium flex items-center"><Settings2 className="h-4 w-4 mr-2 text-gray-500" />设备 (Dev)</Label>
                                                        <Button type="button" variant="ghost" size="icon" onClick={handleAddDevice} className="text-blue-600 hover:text-blue-800 h-6 w-6" title="添加设备">
                                                            <PlusCircle className="h-4 w-4" />
                                                        </Button>
                                                    </div>
                                                    <div className="col-span-4 space-y-3">
                                                        {devEntries.map((deviceEntry, deviceIndex) => (
                                                            <div key={deviceEntry.id} className={`${deviceIndex > 0 ? 'border-t border-slate-100 pt-3' : ''}`}>
                                                                <div className="flex items-center space-x-2 mb-1.5">
                                                                    <Input
                                                                        placeholder="⚙️ 设备名"
                                                                        value={deviceEntry.deviceName}
                                                                        onChange={(e) => handleDeviceNameChange(deviceEntry.id, e.target.value)}
                                                                        className="h-9 text-sm font-medium flex-1 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                                    />
                                                                    <Button
                                                                        type="button"
                                                                        variant="ghost"
                                                                        size="icon"
                                                                        onClick={() => handleAddField(deviceEntry.id)}
                                                                        className="text-green-600 hover:text-green-800 h-6 w-6 flex-shrink-0"
                                                                        title="添加字段"
                                                                    >
                                                                        <PlusCircle className="h-4 w-4" />
                                                                    </Button>
                                                                </div>
                                                                <div className="space-y-2">
                                                                    {deviceEntry.fields.map((fieldEntry, fieldIndex) => (
                                                                        <div key={fieldEntry.id} className="flex items-center space-x-2">
                                                                            <Input
                                                                                placeholder="🔑 字段名"
                                                                                value={fieldEntry.key}
                                                                                onChange={(e) => handleDevFieldChange(deviceEntry.id, fieldEntry.id, 'key', e.target.value)}
                                                                                className="h-8 text-xs w-[30%] flex-shrink-0 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                                            />
                                                                            <span className="text-gray-400">:</span>
                                                                            <Input
                                                                                placeholder="∑ 表达式"
                                                                                value={fieldEntry.value}
                                                                                onChange={(e) => handleDevFieldChange(deviceEntry.id, fieldEntry.id, 'value', e.target.value)}
                                                                                className="h-8 text-xs flex-1 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                                            />
                                                                        </div>
                                                                    ))}
                                                                </div>
                                                            </div>
                                                        ))}
                                                    </div>
                                                </>

                                                {/* Vars Section */}
                                                <>
                                                    <div className="col-span-4 flex justify-between items-center border-t pt-4 mt-1">
                                                        <Label className="text-base font-medium flex items-center"><Variable className="h-4 w-4 mr-2 text-gray-500" />变量 (Vars)</Label>
                                                        <Button type="button" variant="ghost" size="icon" onClick={() => setVarEntries(prev => [...prev, { id: Date.now(), key: '', value: '' }])} className="text-blue-600 hover:text-blue-800 h-6 w-6" title="添加变量">
                                                            <PlusCircle className="h-4 w-4" />
                                                        </Button>
                                                    </div>
                                                    <div className="col-span-4 space-y-2">
                                                        {varEntries.map((entry, index) => (
                                                            <div key={entry.id} className="flex items-center space-x-2">
                                                                <Input
                                                                    placeholder="🏷️ 变量名"
                                                                    value={entry.key}
                                                                    onChange={(e) => handleVarChange(entry.id, 'key', e.target.value)}
                                                                    className="h-9 text-sm w-[30%] flex-shrink-0 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                                />
                                                                <span className="text-gray-400">:</span>
                                                                <Input
                                                                    placeholder="📄 表达式/值"
                                                                    value={entry.value}
                                                                    onChange={(e) => handleVarChange(entry.id, 'value', e.target.value)}
                                                                    className="h-9 text-sm flex-1 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                                />
                                                            </div>
                                                        ))}
                                                    </div>
                                                </>
                                            </div>
                                        )}
                                        {selectedNode.data.type === 'skip' && (
                                            <div className="flex items-center gap-2">
                                                <Label htmlFor="edit-skip" className="shrink-0 whitespace-nowrap text-sm text-muted-foreground">⏭️ 跳过字节</Label>
                                                <Input
                                                    id="edit-skip"
                                                    name="size"
                                                    type="number"
                                                    value={(editFormData as Partial<SkipNodeData>).size || 0}
                                                    onChange={handleEditFormChange}
                                                    className="h-9 flex-grow focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                />
                                            </div>
                                        )}
                                        {selectedNode.data.type === 'loop' && (
                                            <div className="flex flex-col gap-4">
                                                <div className="flex items-center gap-2">
                                                    <Label htmlFor="edit-loop-condition" className="shrink-0 whitespace-nowrap text-sm text-muted-foreground">🔄 循环条件</Label>
                                                    <Input
                                                        id="edit-loop-condition"
                                                        name="loopCondition"
                                                        value={(editFormData as Partial<LoopNodeData>).loopCondition || 'true'}
                                                        onChange={handleEditFormChange}
                                                        className="h-9 flex-grow focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                                        placeholder="输入条件表达式，例如: Vars.counter > 0"
                                                    />
                                                </div>
                                                <div className="text-xs text-gray-500 px-2">
                                                    <p>提示：循环将在条件为<strong>true</strong>时继续执行</p>
                                                    <p>示例：<code>Vars.count &lt; 10</code>、<code>Bytes[0] == 0x01</code></p>
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </ScrollArea>
                        )}
                        {/* --- End Restore Full Popover Content --- */}
                    </PopoverContent>
                </Popover>

                {/* Edge Condition Editor Popover - Restore basic structure */}
                <Popover open={isEdgePopoverOpen} onOpenChange={setIsEdgePopoverOpen}>
                    <PopoverContent
                        sideOffset={5}
                        className="w-80 z-[100] bg-background shadow-md rounded-lg border border-gray-200"
                        style={{
                            position: 'fixed',
                            top: `${edgePopoverPosition?.top ?? 0}px`,
                            left: `${edgePopoverPosition?.left ?? 0}px`,
                        }}
                        onInteractOutside={(e) => { if (selectedEdge && isEdgePopoverOpen) { handleSaveEdgeCondition(); } else { setIsEdgePopoverOpen(false); setSelectedEdge(null); } }}
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        {/* 恢复完整的条件编辑UI */}
                        {selectedEdge && (
                            <div className="p-4 flex flex-col gap-4">
                                <div className="flex flex-col gap-2">
                                    <div className="text-sm font-medium flex items-center">
                                        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="mr-2 h-4 w-4 text-blue-600"><path d="M12 20h9"></path><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path></svg>
                                        <span style={{ writingMode: 'horizontal-tb' }}>编辑连接条件</span>
                                    </div>
                                    <div className="text-xs text-gray-500">
                                        设置条件表达式，决定何时应该沿着这条边进行流程转换
                                    </div>
                                </div>

                                <div className="flex flex-col gap-2">
                                    <Label htmlFor="edge-condition" className="text-xs">
                                        条件表达式
                                    </Label>
                                    <Textarea
                                        id="edge-condition"
                                        placeholder="输入表达式，例如：x > 10"
                                        value={edgeCondition}
                                        onChange={(e) => setEdgeCondition(e.target.value)}
                                        className="w-full h-20 resize-none text-sm border-gray-300 focus-visible:border-blue-500 focus-visible:ring-0 focus-visible:ring-offset-0"
                                    />
                                </div>

                                <div className="flex justify-end gap-2 pt-2">
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={() => {
                                            setIsEdgePopoverOpen(false);
                                            setSelectedEdge(null);
                                        }}
                                    >
                                        取消
                                    </Button>
                                    <Button
                                        type="button"
                                        size="sm"
                                        onClick={handleSaveEdgeCondition}
                                    >
                                        保存
                                    </Button>
                                </div>
                            </div>
                        )}
                    </PopoverContent>
                </Popover>

                {/* Edge Context Menu Popover - Restore basic structure */}
                <Popover open={isContextMenuOpen} onOpenChange={setIsContextMenuOpen}>
                    <PopoverContent
                        className="w-auto p-1 z-[101]"
                        style={{
                            position: 'fixed',
                            top: `${contextMenuPosition?.top ?? 0}px`,
                            left: `${contextMenuPosition?.left ?? 0}px`,
                        }}
                        onInteractOutside={() => { setIsContextMenuOpen(false); setContextMenuEdge(null); }}
                        onOpenAutoFocus={(e) => e.preventDefault()}
                    >
                        {/* 恢复完整的边菜单内容 */}
                        {contextMenuEdge && (
                            <div className="flex flex-col py-1 text-sm">
                                <button
                                    className="px-4 py-1.5 text-left hover:bg-slate-100 rounded-sm flex items-center whitespace-nowrap"
                                    onClick={handleModifyEdge}
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 mr-2 text-blue-600"><path d="M12 20h9"></path><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path></svg>
                                    <span style={{ writingMode: 'horizontal-tb' }}>修改条件</span>
                                </button>
                                <button
                                    className="px-4 py-1.5 text-left hover:bg-slate-100 rounded-sm flex items-center whitespace-nowrap"
                                    onClick={handleIncreasePriority}
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 mr-2 text-green-600"><path d="m5 12 7-7 7 7"></path><path d="M12 19V5"></path></svg>
                                    <span style={{ writingMode: 'horizontal-tb' }}>提高优先级</span>
                                </button>
                                <button
                                    className="px-4 py-1.5 text-left hover:bg-slate-100 rounded-sm flex items-center whitespace-nowrap"
                                    onClick={handleDecreasePriority}
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 mr-2 text-amber-600"><path d="M12 5v14"></path><path d="m5 12 7 7 7-7"></path></svg>
                                    <span style={{ writingMode: 'horizontal-tb' }}>降低优先级</span>
                                </button>
                                <button
                                    className="px-4 py-1.5 text-left hover:bg-slate-100 rounded-sm flex items-center whitespace-nowrap text-red-600"
                                    onClick={handleDeleteEdge}
                                >
                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 mr-2"><path d="M3 6h18"></path><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"></path><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"></path><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line></svg>
                                    <span style={{ writingMode: 'horizontal-tb' }}>删除连接</span>
                                </button>
                            </div>
                        )}
                    </PopoverContent>
                </Popover>

            </ReactFlow>
        </div>
    );
})); // Close forwardRef and memo
// --- End Restore ---

// --- FlowActionButtons Component (Assume exists below) ---
// ...

// --- OrchestrationEditor Component (Restore Skeleton) ---
export default function OrchestrationEditor() {
    const initialData = useLoaderData<LoaderData>();
    console.log("OrchestrationEditor initialData:", initialData); // <--- 添加日志
    const navigate = useNavigate();
    const { versionId } = useParams<{ versionId: string }>();
    const flowCanvasRef = useRef<FlowCanvasHandle>(null);

    // State managed by the parent component - Initialize correctly
    const [yamlContent, setYamlContent] = useState(initialData?.yamlConfig || "");
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [error, setError] = useState<string | null>(initialData?.error || null);
    // Add other states like yamlForModal, yamlValidationError if needed
    const [yamlForModal, setYamlForModal] = useState(initialData?.yamlConfig || "");
    const [yamlValidationError, setYamlValidationError] = useState<string | null>(null);

    // --- Remove keyboard shortcut useEffect from here ---

    // --- IMPORTANT: Restore Callbacks (handleYamlChange, handleSave, handleApplyYaml, triggerAddNode, triggerLayout etc.) ---
    // Placeholder - Add your callback implementations here
    const handleYamlChange = useCallback((newYaml: string) => {
        console.log("YAML changed in canvas (placeholder)", newYaml);
        setYamlContent(newYaml); // Basic update
    }, []);

    const handleBackToProtocol = useCallback(() => {
        const protocolId = initialData.protocol?.id || initialData.version?.protocolId;
        if (protocolId) {
            navigate(`/protocols/${protocolId}`);
        } else {
            toast.error("无法确定协议ID，无法返回。");
            // 作为备选，可以导航到协议列表页
            // navigate("/protocols");
        }
    }, [navigate, initialData]);

    const handleSave = useCallback(async () => {
        if (!versionId) {
            toast.error("无效的版本ID");
            return;
        }
        // --- Use protocol.name and version.version ---
        const protocolName = initialData.protocol?.name; // <-- Use protocol name
        const version = initialData.version?.version;
        if (!protocolName || !version) {
            toast.error("无法获取协议名称或版本号"); // <-- Updated error message
            return;
        }
        // --- End get protocol name and version ---

        try {
            setIsSubmitting(true);

            // --- Pass protocolName and version to getYamlString ---
            const currentYaml = flowCanvasRef.current?.getYamlString(protocolName, version); // <-- Pass protocolName
            console.log("执行保存 - 获取到的YAML内容长度:", currentYaml?.length || 0);
            console.log("执行保存 - 获取到的YAML前200个字符:", currentYaml?.substring(0, 200));

            if (!currentYaml) {
                toast.error("无法从流程图生成YAML内容");
                setIsSubmitting(false); // Don't forget to reset submitting state
                return;
            }

            // 解析YAML为JSON对象
            let parsedYaml;
            try {
                parsedYaml = yaml.load(currentYaml); // 使用 currentYaml
                console.log("解析后的YAML对象类型:", typeof parsedYaml);
                if (typeof parsedYaml === 'object' && parsedYaml !== null) {
                    console.log("解析后的YAML对象结构:",
                        `顶层键数量: ${Object.keys(parsedYaml as object).length}, ` +
                        `首键: ${Object.keys(parsedYaml as object)[0]}`);

                    // 检查第一个键下的内容是否为数组
                    const firstKey = Object.keys(parsedYaml as object)[0];
                    const firstValue = (parsedYaml as any)[firstKey];
                    if (Array.isArray(firstValue)) {
                        console.log(`首键 "${firstKey}" 的值是数组，长度为 ${firstValue.length}`);
                        if (firstValue.length > 0) {
                            console.log("数组第一项类型:", typeof firstValue[0]);
                            console.log("数组第一项内容:", JSON.stringify(firstValue[0]).substring(0, 200));
                        }
                    } else {
                        console.warn(`警告: 首键 "${firstKey}" 的值不是数组，而是 ${typeof firstValue}`);
                    }
                }
                console.log("解析后的YAML对象JSON:", JSON.stringify(parsedYaml).substring(0, 500) + "...");
            } catch (parseError) {
                console.error("YAML解析错误:", parseError);
                toast.error(`YAML解析错误: ${parseError instanceof Error ? parseError.message : '未知错误'}`);
                setIsSubmitting(false); // Don't forget to reset submitting state
                return;
            }

            // 发送解析后的JSON对象到API
            console.log("准备发送到API的数据类型:", typeof parsedYaml);
            console.log("准备发送到API的数据结构:", Object.prototype.toString.call(parsedYaml));

            const response = await API.versions.updateDefinition(versionId, parsedYaml);
            console.log("API响应:", response);

            if (response.error) {
                throw new Error(response.error);
            }

            // 成功保存后重新获取最新数据验证
            console.log("保存成功，准备重新获取数据验证...");
            const verifyResponse = await API.versions.getDefinition(versionId);
            console.log("验证响应:", verifyResponse);

            if (verifyResponse.data) {
                console.log("验证数据类型:", typeof verifyResponse.data);
                console.log("验证数据结构:", Object.prototype.toString.call(verifyResponse.data));
                if (typeof verifyResponse.data === 'object' && verifyResponse.data !== null) {
                    console.log("验证数据键:", Object.keys(verifyResponse.data as object).join(", "));

                    // 检查数据是否与发送的相同
                    const origKeys = Object.keys(parsedYaml as object);
                    const newKeys = Object.keys(verifyResponse.data as object);
                    if (JSON.stringify(origKeys) !== JSON.stringify(newKeys)) {
                        console.warn("警告: 保存前后的键不完全相同", {
                            原始键: origKeys,
                            新键: newKeys
                        });
                    } else {
                        console.log("键匹配: 保存前后的顶层键相同");
                    }

                    // 转换验证数据为YAML以便对比
                    try {
                        const verifyYaml = yaml.dump(verifyResponse.data, { lineWidth: -1, sortKeys: false });
                        console.log("验证数据转YAML (前200字符):", verifyYaml.substring(0, 200));
                        // --- 使用 currentYaml 进行比较 ---
                        const isYamlSimilar = isSameYamlContent(currentYaml, verifyYaml);
                        console.log("YAML内容相似性检查:", isYamlSimilar ? "相似" : "不相似");
                    } catch (dumpError) {
                        console.error("验证数据转YAML失败:", dumpError);
                    }
                }
            } else {
                console.warn("警告: 验证响应中没有数据");
            }

            toast.success("定义已成功保存");

            // --- 更新 yamlContent 状态以反映保存后的内容 ---
            setYamlContent(currentYaml);

        } catch (error) {
            console.error("保存失败:", error);
            toast.error(`保存失败: ${error instanceof Error ? error.message : '未知错误'}`);
            setError(`保存失败: ${error instanceof Error ? error.message : '未知错误'}`);
        } finally {
            setIsSubmitting(false);
        }
        // --- 移除 yamlContent 依赖 ---
    }, [versionId, initialData.protocol, initialData.version, setYamlContent, setError]); // <-- Add initialData dependencies

    // --- Add keyboard shortcut for Ctrl+S to save (moved after handleSave declaration) ---
    useEffect(() => {
        const handleKeyDown = (event: KeyboardEvent) => {
            // Check for Ctrl+S or Command+S
            if ((event.ctrlKey || event.metaKey) && event.key === 's') {
                event.preventDefault(); // Prevent browser's save dialog
                handleSave();

                // Add visual feedback that save was triggered
                toast.info("保存中...", { duration: 1000 });
            }
        };

        // Add event listener
        window.addEventListener('keydown', handleKeyDown);

        // Clean up
        return () => {
            window.removeEventListener('keydown', handleKeyDown);
        };
    }, [handleSave]); // Include handleSave in dependencies
    // --- End keyboard shortcut implementation ---

    const handleApplyYaml = useCallback(() => {
        try {
            // 验证YAML格式
            try {
                yaml.load(yamlForModal);
                setYamlValidationError(null);
            } catch (e) {
                console.error("YAML解析错误:", e);
                setYamlValidationError((e as Error).message);
                toast.error(`YAML格式错误: ${(e as Error).message}`);
                return;
            }

            // 应用新的YAML到流程图
            handleYamlChange(yamlForModal);
            toast.success("YAML已成功应用到流程图");
        } catch (error) {
            console.error("应用YAML失败:", error);
            toast.error(`应用YAML失败: ${error instanceof Error ? error.message : '未知错误'}`);
        }
    }, [yamlForModal, handleYamlChange]);

    // --- MODIFIED: Add 'loop' type ---
    const triggerAddNode = useCallback((type: 'section' | 'skip' | 'end' | 'start' | 'loop') => {
        // --- END MODIFICATION ---
        console.log("Trigger add node (placeholder)", type);
        flowCanvasRef.current?.addNode(type);
    }, []);

    const triggerLayout = useCallback((direction: 'TB' | 'LR') => {
        console.log("Trigger layout (placeholder)", direction);
        flowCanvasRef.current?.triggerLayout(direction);
    }, []);

    // --- IMPORTANT: Restore useEffects (e.g., for yamlForModal update, keyboard shortcuts) ---
    useEffect(() => {
        setYamlForModal(yamlContent);
    }, [yamlContent]);

    // --- Basic Error/Loading handling ---
    if (!initialData?.version && !initialData?.error) {
        return (
            <Card className="text-center py-12">
                <CardHeader><CardTitle>无法加载</CardTitle><CardDescription>无法加载版本和定义数据。</CardDescription></CardHeader>
                <CardContent><Button onClick={() => navigate(-1)} variant="outline">返回</Button></CardContent>
            </Card>
        );
    }

    // --- IMPORTANT: Restore YAML Editor Sheet Trigger/Content ---
    const yamlEditButton = (
        // Add the full <Sheet> structure here
        <Sheet>
            <SheetTrigger asChild>
                {/* REMOVED TooltipProvider/Tooltip wrapper */}
                <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                        console.log("[yamlEditButton onClick] Triggered!"); // Log trigger
                        // --- Use protocol.name and version.version ---
                        const protocolName = initialData.protocol?.name; // <-- Use protocol name
                        const version = initialData.version?.version;
                        if (!protocolName || !version) {
                            toast.error("无法获取协议名称或版本号以生成YAML"); // <-- Updated error message
                            return;
                        }
                        // --- End get protocol name and version ---

                        // --- Pass protocolName and version to getYamlString ---
                        const latestYaml = flowCanvasRef.current?.getYamlString(protocolName, version);

                        if (!latestYaml) {
                            console.warn("[yamlEditButton onClick] getYamlString returned undefined, using fallback.");
                            toast.info("无法获取最新YAML，显示当前编辑器内容。");
                        } else {
                            console.log("[yamlEditButton onClick] latestYaml length:", latestYaml.length);
                            console.log("[yamlEditButton onClick] latestYaml content (first 100 chars):", latestYaml.substring(0, 100));
                            setYamlForModal(latestYaml);
                        }
                    }}
                    className="w-9 px-0"
                    title="编辑 YAML"
                >
                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-code"><polyline points="16 18 22 12 16 6"></polyline><polyline points="8 6 2 12 8 18"></polyline></svg>
                </Button>
                {/* END REMOVED TooltipProvider/Tooltip wrapper */}
            </SheetTrigger>
            <SheetContent className="w-[600px] sm:max-w-[70vw] flex flex-col" side="right">
                <SheetHeader>
                    <SheetTitle>编辑 YAML 定义</SheetTitle>
                    <SheetDescription>
                        直接修改协议的 YAML 定义。更改将在应用后反映到流程图中。
                    </SheetDescription>
                </SheetHeader>
                <ScrollArea className="flex-grow my-4 min-h-0">
                    <div className="grid gap-4 py-4">
                        {yamlValidationError && (
                            <div className="p-3 bg-red-100 border border-red-300 text-red-700 rounded-md text-sm">
                                YAML 格式错误: {yamlValidationError}
                            </div>
                        )}
                        <Textarea
                            value={yamlForModal}
                            onChange={(e) => setYamlForModal(e.target.value)}
                            className="h-[60vh] font-mono text-sm"
                            placeholder="在此输入 YAML..."
                        />
                    </div>
                </ScrollArea>
                <SheetFooter>
                    <SheetClose asChild>
                        <Button type="submit" onClick={handleApplyYaml}>应用 YAML</Button>
                    </SheetClose>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    );

    // --- Return JSX Structure ---
    return (
        <div className="absolute inset-x-0 bottom-0 top-14 flex flex-col overflow-hidden">
            {/* IMPORTANT: Restore Error display div */}
            {error && (
                <div className="absolute top-4 left-1/2 transform -translate-x-1/2 z-[100] bg-red-100 border border-red-400 text-red-700 px-4 py-2 rounded shadow-lg max-w-md text-center">
                    错误: {error}
                    <Button variant="ghost" size="sm" className="ml-2" onClick={() => setError(null)}>×</Button>
                </div>
            )}

            {/* Flow Canvas Area */}
            <div className="flex-grow overflow-auto relative min-h-0">
                <ReactFlowProvider>
                    {/* IMPORTANT: Restore FlowActionButtons */}
                    <FlowActionButtons
                        onSave={handleSave}
                        onAddNode={triggerAddNode}
                        onLayout={triggerLayout}
                        isSubmitting={isSubmitting}
                        yamlModalTrigger={yamlEditButton} // Pass the trigger/sheet here
                        onBackToProtocol={handleBackToProtocol} // 新增传递回调
                    />
                    <FlowCanvas
                        ref={flowCanvasRef}
                        initialYaml={yamlContent}
                        onYamlChange={handleYamlChange}
                        versionId={versionId}
                    />
                </ReactFlowProvider>
            </div>
        </div>
    );
}

// --- IMPORTANT: Ensure FlowActionButtons definition is present below or imported ---
// Assuming FlowActionButtons is defined somewhere like this (restore if needed):
interface FlowActionButtonsProps {
    onSave: () => void;
    onAddNode: (type: 'section' | 'skip' | 'end' | 'start' | 'loop') => void; // <-- Added 'loop'
    onLayout: (direction: 'TB' | 'LR') => void;
    isSubmitting: boolean;
    yamlModalTrigger: React.ReactNode;
    onBackToProtocol?: () => void; // 新增返回回调
}

const FlowActionButtons = ({ onSave, onAddNode, onLayout, isSubmitting, yamlModalTrigger, onBackToProtocol }: FlowActionButtonsProps) => {
    return (
        <div className="absolute top-4 right-4 z-20 flex space-x-2">
            {/* 新增返回按钮 */}
            {onBackToProtocol && (
                <TooltipProvider>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button size="sm" variant="outline" onClick={onBackToProtocol} className="w-9 px-0">
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-arrow-left"><path d="m12 19-7-7 7-7" /><path d="M19 12H5" /></svg>
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent side="bottom">
                            <p>返回协议详情</p>
                        </TooltipContent>
                    </Tooltip>
                </TooltipProvider>
            )}

            {/* Use the provided yamlModalTrigger */}
            {yamlModalTrigger}

            {/* Vertical layout button with tooltip */}
            <TooltipProvider>
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button size="sm" variant="outline" onClick={() => onLayout('TB')} className="w-9 px-0">
                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-align-vertical-justify-center"><rect width="14" height="6" x="5" y="13" rx="2" /><rect width="10" height="6" x="7" y="1" rx="2" /><path d="M12 9v4" /></svg>
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">
                        <p>垂直布局</p>
                    </TooltipContent>
                </Tooltip>
            </TooltipProvider>

            {/* Horizontal layout button with tooltip */}
            <TooltipProvider>
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button size="sm" variant="outline" onClick={() => onLayout('LR')} className="w-9 px-0">
                            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-align-horizontal-justify-center"><rect width="6" height="14" x="1" y="5" rx="2" /><rect width="6" height="10" x="13" y="7" rx="2" /><path d="M7 12h10" /></svg>
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">
                        <p>水平布局</p>
                    </TooltipContent>
                </Tooltip>
            </TooltipProvider>

            {/* Save button with icon */}
            <TooltipProvider>
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button size="sm" variant="secondary" onClick={onSave} disabled={isSubmitting} className="w-9 px-0">
                            {isSubmitting ?
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="animate-spin"><path d="M21 12a9 9 0 1 1-6.219-8.56" /></svg> :
                                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-save"><path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" /><polyline points="17 21 17 13 7 13 7 21" /><polyline points="7 3 7 8 15 8" /></svg>
                            }
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom">
                        <p>保存定义 (Ctrl+S)</p>
                    </TooltipContent>
                </Tooltip>
            </TooltipProvider>

            {/* Add node dropdown with icon and tooltip */}
            <DropdownMenu>
                <TooltipProvider>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <DropdownMenuTrigger asChild>
                                <Button size="sm" variant="secondary" className="w-9 px-0">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-plus"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
                                </Button>
                            </DropdownMenuTrigger>
                        </TooltipTrigger>
                        <TooltipContent side="bottom">
                            <p>添加节点</p>
                        </TooltipContent>
                    </Tooltip>
                </TooltipProvider>
                <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => onAddNode('section')}>
                        📦 添加 Section 节点
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onAddNode('skip')}>
                        ⏭️ 添加 Skip 节点
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onAddNode('end')}>
                        🏁 添加 End 节点
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onAddNode('loop')}>
                        🔄 添加 Loop 节点
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
        </div>
    );
};

// --- Helper Function to Update Priorities and Default flags for a Source ---
const updateEdgesForSource = (sourceId: string, currentEdges: Edge<EdgeData>[]): Edge<EdgeData>[] => {
    const sourceEdges = currentEdges.filter(edge => edge.source === sourceId);
    const otherEdges = currentEdges.filter(edge => edge.source !== sourceId);

    // Sort edges from the specific source by their current priority
    // Handle potentially undefined priorities during sorting
    sourceEdges.sort((a, b) => {
        const prioA = a.data?.priority;
        const prioB = b.data?.priority;
        if (prioA === undefined && prioB === undefined) return 0;
        if (prioA === undefined) return Infinity; // Treat undefined as highest
        if (prioB === undefined) return -Infinity;
        return prioA - prioB;
    });

    // Re-assign sequential priorities and set isDefault
    const updatedSourceEdges = sourceEdges.map((edge, index) => ({
        ...edge,
        data: {
            ...edge.data,
            priority: index,
            isDefault: index === 0
        }
    }));

    // Combine updated source edges with edges from other sources
    return [...otherEdges, ...updatedSourceEdges];
};
// --- End Helper Function ---

