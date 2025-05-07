import React, { memo, useCallback } from 'react';
import type { NodeProps } from '@xyflow/react';
import { Handle, Position, NodeResizeControl, useNodeId } from '@xyflow/react';
import { cn } from '@/lib/utils';
import { Plus } from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { LoopNodeData } from './orchestration'; // Adjust path if LoopNodeData is moved

// --- MODIFIED: Add id and onAddChildNode to props type ---
// --- Remove extends NodeProps to potentially fix TS constraint error ---
interface LoopNodeComponentProps /* extends NodeProps<LoopNodeData> */ {
    id: string; // Ensure id is explicitly typed
    data: LoopNodeData & {
        onAddChildNode?: (parentId: string, type: 'section' | 'skip') => void;
        hasChildren?: boolean; // <-- Add hasChildren flag
    };
}
// --- END MODIFICATION ---

// LoopNode Component Implementation
// --- MODIFIED: Update props destructuring ---
const LoopNodeComponent = ({ id, data }: LoopNodeComponentProps) => {
    // 使用useNodeId获取自动的节点ID
    const nodeId = useNodeId() || id;

    return (
        <>
            {/* Handles for incoming and outgoing connections */}
            <Handle type="target" position={Position.Top} className="!top-[-5px]" />

            {/* Main Node Container */}
            <div
                className={cn(
                    'loop-node',
                    'bg-blue-50/70',
                    'border-2',
                    'border-dashed',
                    'border-blue-400',
                    'rounded-md',
                    'p-4',
                    'shadow-sm',
                    'min-w-[250px]',
                    'min-h-[180px]',
                    'w-full',
                    'h-full',
                    'relative', // Needed for the wrapper div's absolute positioning
                    'flex',
                    'flex-col',
                    'items-center',
                    'pointer-events-none',
                    'pt-6'
                )}
            >
                {/* Loop Condition Display */}
                <div className="absolute top-1 left-1/2 -translate-x-1/2 bg-blue-100 text-blue-800 text-xs font-medium px-2 py-0.5 rounded pointer-events-auto">
                    Loop While: {data.loopCondition}
                </div>

                {/* Placeholder for future content inside the loop */}
                <div className="flex-grow w-full flex items-center justify-center">
                    {!data.hasChildren && (
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <button
                                    className={cn(
                                        'absolute',
                                        'top-1/2',
                                        'left-1/2',
                                        '-translate-x-1/2',
                                        '-translate-y-1/2',
                                        'p-1',
                                        'rounded-full',
                                        'bg-blue-200',
                                        'text-blue-700',
                                        'hover:bg-blue-300',
                                        'transition-colors',
                                        'z-10',
                                        'pointer-events-auto'
                                    )}
                                    title="Add node inside loop"
                                    onClick={(e) => e.stopPropagation()}
                                >
                                    <Plus className="h-4 w-4" />
                                </button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent sideOffset={5}>
                                <DropdownMenuItem onSelect={() => data.onAddChildNode?.(id, 'section')}>
                                    添加 Section 节点
                                </DropdownMenuItem>
                                <DropdownMenuItem onSelect={() => data.onAddChildNode?.(id, 'skip')}>
                                    添加 Skip 节点
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    )}
                </div>

                {/* --- NEW: Wrapper Div for Resize Control --- */}
                <div
                    style={{
                        position: 'absolute',
                        bottom: -5, // Anchor wrapper to bottom-right of parent
                        right: 11,
                        width: 'auto', // Let content determine size initially
                        height: 'auto',
                        pointerEvents: 'auto', // Allow interaction with the control inside
                    }}
                >
                    <NodeResizeControl
                        // position prop might not be needed/used now
                        style={{
                            // Position relative to the new wrapper div
                            position: 'relative', // Use relative positioning within the wrapper
                            display: 'block', // Ensure it takes space
                            width: 16,
                            height: 16,
                            background: 'transparent',
                            border: 'none',
                            borderBottom: '3px solid #60a5fa',
                            borderRight: '3px solid #60a5fa',
                            borderBottomRightRadius: '6px',
                            cursor: 'nwse-resize',
                            // Remove absolute positioning relative to canvas
                            // bottom: 2,
                            // right: 2,
                            // zIndex: 10, // zIndex might be needed on wrapper instead
                        }}
                        minWidth={250} // These constraints still apply to the node itself
                        minHeight={180}
                    />
                </div>
                {/* --- END NEW: Wrapper Div --- */}

            </div> {/* End Main Node Container */}

            <Handle type="source" position={Position.Bottom} className="!bottom-[-5px]" />
        </>
    );
};

// --- NEW: Custom comparison function for LoopNode ---
const loopNodePropsAreEqual = (prevProps: any, nextProps: any): boolean => {
    // Compare props that affect visual rendering
    if (
        prevProps.id !== nextProps.id ||
        prevProps.data.type !== nextProps.data.type ||
        prevProps.data.loopCondition !== nextProps.data.loopCondition ||
        prevProps.data.isHovered !== nextProps.data.isHovered // Assuming isHovered might affect style/visibility
    ) {
        return false; // Props are different, re-render
    }

    // Ignore comparison of the onAddChildNode callback reference
    // If all relevant props are the same, skip re-render
    return true;
};
// --- END NEW ---

// Memoize the component for performance
// --- MODIFIED: Apply custom comparison function ---
export const LoopNode = memo(LoopNodeComponent, loopNodePropsAreEqual);
// --- END MODIFICATION ---
