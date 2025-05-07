import { Link, useLoaderData, useParams, useNavigate } from "react-router";
import type { Protocol, ProtocolVersion, Route } from "../../+types/protocols";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
    CardFooter,
} from "@/components/ui/card";
import {
    Table,
    TableBody,
    TableCaption,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    ArrowRightIcon,
    Pencil1Icon,
    RowsIcon,
    PlusCircledIcon,
    ExclamationTriangleIcon,
    GearIcon,
    TrashIcon,
    PlayIcon,
    DownloadIcon
} from "@radix-ui/react-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { API, useApiGet } from "../../api";
import { toast } from "sonner";
import { useState, useEffect, useRef } from "react";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
    Tabs,
    TabsList,
    TabsTrigger,
    TabsContent,
} from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import {
    Accordion,
    AccordionContent,
    AccordionItem,
    AccordionTrigger,
} from "@/components/ui/accordion";
import yaml from 'js-yaml';

// --- 辅助函数：格式化纳秒时间 ---
function formatDurationNs(nanoseconds: number | null | undefined): string {
    if (nanoseconds === null || nanoseconds === undefined) {
        return '- ms'; // 或者返回其他占位符
    }

    // 检查是否接近或等于 MaxInt64 (除以1M后仍是天文数字)
    // 9223372036854775807 / 1_000_000 > 9_000_000_000_000
    if (nanoseconds > 9_000_000_000_000_000) {
        return '> 290 years'; // 明确指出异常
    }

    if (nanoseconds < 0) {
        return 'Invalid Time'; // 处理可能的负值
    }

    if (nanoseconds < 1000) {
        return `${nanoseconds} ns`;
    }
    if (nanoseconds < 1_000_000) {
        return `${(nanoseconds / 1000).toFixed(1)} µs`; // 微秒保留1位小数
    }
    if (nanoseconds < 1_000_000_000) {
        return `${(nanoseconds / 1_000_000).toFixed(3)} ms`; // 毫秒保留3位小数
    }
    return `${(nanoseconds / 1_000_000_000).toFixed(3)} s`; // 秒保留3位小数
}
// --- 辅助函数结束 ---

export const clientLoader = async ({ params }: { params: { protocolId: string } }) => {
    const protocolId = params.protocolId;

    try {
        // 使用API.ts中定义的方法
        const response = await API.protocols.getById(protocolId);
        if (response.error) {
            // 创建一个模拟数据作为备份
            const mockData = {
                protocol: {
                    id: protocolId,
                    name: "模拟协议详情",
                    description: "这是一个模拟协议详情，用于演示目的",
                    createdAt: new Date().toISOString(),
                    updatedAt: new Date().toISOString()
                }
            };
            console.warn('API请求失败，使用模拟数据');
            return mockData;
        }
        return response.data;
    } catch (error) {
        console.error('加载协议详情出错:', error);
        // 返回通用模拟数据
        return {
            protocol: {
                id: protocolId || 'unknown',
                name: "模拟协议(备用)",
                description: "备用模拟数据",
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString()
            }
        };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const protocol = data?.protocol as Protocol;
    return [
        { title: protocol ? `${protocol.name} - 协议详情` : '协议详情 - 网关管理' },
        { name: "description", content: protocol?.description || '查看协议详细信息' },
    ];
};

export default function ProtocolDetail() {
    const data = useLoaderData<typeof clientLoader>();
    const { protocolId } = useParams<{ protocolId: string }>();
    const navigate = useNavigate();
    const protocol = data?.protocol;
    const [isDeleting, setIsDeleting] = useState(false);
    const [isDeletingGlobalMap, setIsDeletingGlobalMap] = useState(false);
    const [selectedVersion, setSelectedVersion] = useState<ProtocolVersion | null>(null);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [selectedGlobalMapId, setSelectedGlobalMapId] = useState<string>("none");
    const [isRunningTest, setIsRunningTest] = useState(false);
    const [testResults, setTestResults] = useState<any>(null);
    const [hexData, setHexData] = useState("0102030405"); // 默认十六进制数据
    const [isResultsDialogOpen, setIsResultsDialogOpen] = useState(false);
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const [isConfigDialogOpen, setIsConfigDialogOpen] = useState(false);
    const [configError, setConfigError] = useState<string | null>(null);
    const [configDebugInfo, setConfigDebugInfo] = useState<any>(null);

    // 添加点击外部关闭下拉菜单的事件处理
    useEffect(() => {
        function handleClickOutside(event: MouseEvent) {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setIsDropdownOpen(false);
            }
        }

        // 添加全局点击事件
        document.addEventListener("mousedown", handleClickOutside);
        return () => {
            // 清理事件
            document.removeEventListener("mousedown", handleClickOutside);
        };
    }, []);

    const {
        data: versions,
        isLoading: isLoadingVersions,
        error: versionsError
    } = useApiGet(() => {
        if (!protocolId) return Promise.resolve({ status: 400, error: 'Missing protocolId' });
        return API.protocols.versions.getAll(protocolId);
    }, [protocolId]);

    const {
        data: globalmaps,
        isLoading: isLoadingGlobalMaps,
        error: globalmapsError
    } = useApiGet(() => {
        if (!protocolId) return Promise.resolve({ status: 400, error: 'Missing protocolId' });
        return API.protocols.globalmaps.getAll(protocolId);
    }, [protocolId]);

    const handleDeleteVersion = async (versionId: string, versionNumber: string) => {
        if (!window.confirm(`确定要删除版本 "${versionNumber}" 吗？此操作无法撤销。`)) {
            return;
        }

        setIsDeleting(true);
        try {
            const response = await API.versions.delete(versionId);
            if (response.error) {
                toast.error(`删除版本 ${versionNumber} 失败: ${response.error}`);
            } else {
                toast.success(`版本 ${versionNumber} 已成功删除`);
                window.location.reload();
            }
        } catch (error) {
            console.error(`删除版本 ${versionNumber} 时出错:`, error);
            toast.error(`删除版本 ${versionNumber} 时发生网络或未知错误`);
        } finally {
            setIsDeleting(false);
        }
    };

    const handleDeleteGlobalMap = async (globalMapId: string, globalMapName: string) => {
        if (!window.confirm(`确定要删除全局映射 "${globalMapName}" 吗？此操作无法撤销。`)) {
            return;
        }

        setIsDeletingGlobalMap(true);
        try {
            const response = await API.globalmaps.delete(globalMapId);
            if (response.error) {
                toast.error(`删除全局映射 ${globalMapName} 失败: ${response.error}`);
            } else {
                toast.success(`全局映射 ${globalMapName} 已成功删除`);
                window.location.reload();
            }
        } catch (error) {
            console.error(`删除全局映射 ${globalMapName} 时出错:`, error);
            toast.error(`删除全局映射 ${globalMapName} 时发生网络或未知错误`);
        } finally {
            setIsDeletingGlobalMap(false);
        }
    };

    const handleRunTest = async () => {
        if (!selectedVersion) return;

        // 验证十六进制数据格式
        if (!hexData.match(/^[0-9A-Fa-f]+$/)) {
            toast.error('请输入有效的十六进制数据 (0-9, A-F)');
            return;
        }

        // 十六进制长度必须是偶数
        if (hexData.length % 2 !== 0) {
            toast.error('十六进制数据长度必须是偶数');
            return;
        }

        setIsRunningTest(true);
        setTestResults(null);
        // --- 声明变量以在 catch 块中访问 ---
        let combinedConfig: any = null;
        let errorTextFromResponse: string | null = null;

        try {
            // 获取协议配置
            if (!protocolId) {
                toast.error('无法获取协议ID');
                return;
            }

            const protocolConfigResponse = await API.protocols.getById(protocolId);
            if (protocolConfigResponse.error) {
                toast.error(`获取协议配置失败: ${protocolConfigResponse.error}`);
                return;
            }

            const protocolConfig = protocolConfigResponse.data?.protocol?.config || {};
            console.log('协议配置数据:', protocolConfig);

            // 获取版本配置
            const versionResponse = await API.versions.getDefinition(selectedVersion.id);
            if (versionResponse.error) {
                toast.error(`获取版本配置失败: ${versionResponse.error}`);
                return;
            }

            // 获取全局映射数据（如果选择了的话）
            let globalMapData = null;
            if (selectedGlobalMapId && selectedGlobalMapId !== "none") {
                const globalMapResponse = await API.globalmaps.getById(selectedGlobalMapId);
                if (globalMapResponse.error) {
                    toast.error(`获取全局映射数据失败: ${globalMapResponse.error}`);
                    return;
                }
                // 确保数据存在再访问content字段
                if (globalMapResponse.data) {
                    globalMapData = globalMapResponse.data.content;
                } else {
                    toast.error('全局映射数据无效');
                    return;
                }
            }

            // 准备测试数据
            const definition = versionResponse.data;

            // 提供更详细的错误信息
            if (!definition) {
                const errorMessage = `版本配置数据无效: 没有接收到版本 ${selectedVersion.version} 的配置数据`;
                console.error(errorMessage, versionResponse);
                setConfigError(errorMessage);
                // 修改：设置调试信息时包含更多上下文
                setConfigDebugInfo({
                    message: errorMessage,
                    details: {
                        versionResponse: versionResponse,
                        protocolConfig: protocolConfigResponse.data?.protocol?.config || {},
                    }
                });
                setIsConfigDialogOpen(true);
                toast.error(errorMessage);
                return; // 需要 return 退出函数
            }

            // --- 修改开始: 提取动态根键下的段配置数组 ---
            let sectionConfigs: any[] = [];

            // 检查definition是否为对象
            if (typeof definition === 'object' && definition !== null && !Array.isArray(definition)) {
                const rootKeys = Object.keys(definition);

                // 如果有根键，获取第一个键下的数据
                if (rootKeys.length > 0) {
                    const dynamicRootKey = rootKeys[0]; // 例如 "gw05-ats-mq_1.0.2"
                    console.log(`找到动态根键: ${dynamicRootKey}`);

                    const potentialConfigs = definition[dynamicRootKey];

                    // 检查根键下的值是否为数组
                    if (Array.isArray(potentialConfigs)) {
                        sectionConfigs = potentialConfigs;
                        console.log(`成功从动态根键 ${dynamicRootKey} 下提取了 ${sectionConfigs.length} 个段配置`);
                    } else {
                        console.warn(`动态根键 ${dynamicRootKey} 下的值不是数组，类型: ${typeof potentialConfigs}`);
                    }
                } else {
                    console.warn("定义对象没有键");
                }
            }

            // 回退：检查传统的definition.config
            if (sectionConfigs.length === 0 && definition.config) {
                if (Array.isArray(definition.config)) {
                    sectionConfigs = definition.config;
                    console.log("使用传统 definition.config 数组作为段配置");
                } else {
                    console.warn("definition.config 存在但不是数组，类型:", typeof definition.config);
                }
            }

            // 验证提取的段配置
            if (sectionConfigs.length === 0) {
                const errorMessage = `版本配置数据无效: 版本 ${selectedVersion.version} 的配置为空`;
                console.error(errorMessage, definition);
                setConfigError(errorMessage);
                // 修改：设置调试信息时包含更多上下文
                setConfigDebugInfo({
                    message: errorMessage,
                    details: {
                        rawDefinition: definition,
                        extractedConfigs: '未找到有效的段配置数组',
                        protocolConfig: protocolConfigResponse.data?.protocol?.config || {},
                    }
                });
                setIsConfigDialogOpen(true);
                toast.error(errorMessage);
                return; // 需要 return 退出函数
            }
            // --- 修改结束 ---

            // 控制台记录有效配置，便于调试
            console.log('版本配置数据:', definition);
            console.log('提取的段配置:', sectionConfigs);
            console.log('协议配置数据:', protocolConfig);
            console.log('全局映射数据:', globalMapData);

            // 合并协议配置和版本配置
            // 注意：这里简单组合，实际组合规则可能需要根据后端接口要求调整
            // --- 将 combinedConfig 赋值给外部变量 ---
            combinedConfig = {
                sectionConfigs: sectionConfigs, // 使用提取的段配置数组
                hexPayload: hexData,
                globalMap: globalMapData,
                protocolConfig: protocolConfig,
                initialVars: {} // 添加空的initialVars
            };

            console.log('发送到后端的组合配置:', combinedConfig);

            // 调用测试API
            const response = await fetch('/api/v1/test/section', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(combinedConfig),
            });

            if (!response.ok) {
                // --- 读取错误文本并存储到外部变量 ---
                errorTextFromResponse = await response.text();
                // 抛出错误，由 catch 处理
                throw new Error(`测试API请求失败: ${response.status} ${errorTextFromResponse}`);
            }

            const results = await response.json();
            setTestResults(results);
            toast.success('测试执行成功');

            // 测试成功后关闭参数对话框
            setIsDialogOpen(false);

            // 显示测试结果对话框
            setIsResultsDialogOpen(true);

            // 控制台记录
            console.log('测试结果:', results);

        } catch (error) {
            console.error('执行测试出错:', error);
            const errorMessage = error instanceof Error ? error.message : '未知错误';
            setConfigError(`执行测试失败: ${errorMessage}`);

            // --- 构建更详细的调试信息对象 ---
            const debugDetails: Record<string, any> = {
                errorMessage: errorMessage,
                requestPayload: combinedConfig, // 包含发送的请求体
                stackTrace: error instanceof Error ? error.stack : undefined,
            };

            // 尝试解析后端返回的错误体
            if (errorTextFromResponse) {
                try {
                    debugDetails.backendError = JSON.parse(errorTextFromResponse);
                } catch (parseError) {
                    debugDetails.backendErrorRaw = errorTextFromResponse; // 解析失败则存储原始文本
                }
            }
            // --- 修改结束 ---

            setConfigDebugInfo(debugDetails); // 设置包含详细信息的对象
            setIsConfigDialogOpen(true);
            toast.error(`执行测试失败: ${errorMessage}`);
        } finally {
            setIsRunningTest(false);
        }
    };

    const openRunDialog = (version: ProtocolVersion) => {
        setSelectedVersion(version);
        setSelectedGlobalMapId("none");
        setHexData("0102030405"); // 重置为默认值
        setIsDialogOpen(true);
    };

    const handleExportProtocolConfig = () => {
        // 1. 确保 protocol 对象存在
        if (!protocol) {
            toast.error("协议数据不可用，无法导出。");
            return;
        }

        // 2. 使用类型守卫检查 config 是否存在且不为 null
        // 我们需要断言 protocol 的类型，以便 TypeScript 知道 config 可能存在
        const fullProtocol = protocol as Protocol; // Protocol 是应包含 config 的完整类型

        if (!fullProtocol.config || typeof fullProtocol.config !== 'object') {
            toast.error("协议配置数据格式不正确或不存在，无法导出。");
            return;
        }

        // 3. 检查 config 是否为空对象
        if (Object.keys(fullProtocol.config).length === 0) {
            toast.info("协议配置为空，无需导出。");
            return;
        }

        try {
            // 现在可以安全使用 fullProtocol.config
            const yamlString = yaml.dump(fullProtocol.config, { indent: 2, lineWidth: -1, sortKeys: false });
            const blob = new Blob([yamlString], { type: 'application/x-yaml' });

            let protocolNameForFile = 'protocol';
            // protocol.name 应该总是存在的，但为了安全起见也做检查
            if (fullProtocol.name && fullProtocol.name.trim() !== '') {
                protocolNameForFile = fullProtocol.name.replace(/[^a-z0-9_.-]/gi, '_').toLowerCase();
            }

            const downloadUrl = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = downloadUrl;
            a.download = `${protocolNameForFile}_config.yaml`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(downloadUrl);
            toast.success("协议配置已成功导出为 YAML 文件。");

        } catch (error) {
            console.error("导出协议配置失败:", error);
            toast.error(`导出协议配置失败: ${error instanceof Error ? error.message : '未知错误'}`);
        }
    };

    if (!protocol) {
        return (
            <Card className="text-center py-12">
                <CardHeader>
                    <CardTitle>未找到协议</CardTitle>
                    <CardDescription>该协议可能已被删除或不存在</CardDescription>
                </CardHeader>
                <CardContent>
                    <Button asChild variant="outline">
                        <Link to="/protocols">返回协议列表</Link>
                    </Button>
                </CardContent>
            </Card>
        );
    }

    const renderVersionList = () => {
        if (isLoadingVersions) {
            return (
                <div className="space-y-2 mt-4">
                    {[...Array(3)].map((_, i) => (
                        <Skeleton key={i} className="h-10 w-full bg-muted/60" />
                    ))}
                </div>
            );
        }

        if (versionsError) {
            return (
                <div className="text-red-600 bg-red-50 border border-red-200 rounded-md p-4 mt-4 flex items-center">
                    <ExclamationTriangleIcon className="h-5 w-5 mr-2 flex-shrink-0" />
                    <span>加载版本列表失败: {versionsError}</span>
                </div>
            );
        }

        return (
            <Table>
                {!versions || versions.length === 0 && (
                    <TableCaption>暂无版本信息。</TableCaption>
                )}
                <TableHeader>
                    <TableRow>
                        <TableHead>版本号</TableHead>
                        <TableHead>描述</TableHead>
                        <TableHead>创建时间</TableHead>
                        <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {versions && versions.map((version) => (
                        <TableRow key={version.id}>
                            <TableCell className="font-medium">{version.version}</TableCell>
                            <TableCell className="text-muted-foreground">{version.description || '-'}</TableCell>
                            <TableCell className="text-muted-foreground">{new Date(version.createdAt || Date.now()).toLocaleString()}</TableCell>
                            <TableCell className="text-right space-x-1">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2 text-green-600 hover:text-green-700 hover:bg-green-50"
                                    title="运行测试"
                                    onClick={() => openRunDialog(version)}
                                >
                                    <PlayIcon className="mr-1 h-4 w-4" /> 运行
                                </Button>
                                <Button asChild variant="ghost" size="sm" className="h-8 px-2">
                                    <Link to={`/versions/${version.id}/edit`} title="编辑版本信息">
                                        <Pencil1Icon className="mr-1 h-4 w-4" /> 编辑
                                    </Link>
                                </Button>
                                <Button asChild variant="ghost" size="sm" className="h-8 px-2">
                                    <Link to={`/versions/${version.id}/orchestration`} title="协议编排">
                                        <RowsIcon className="mr-1 h-4 w-4" /> 编排
                                    </Link>
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2 text-blue-600 hover:text-blue-700 hover:bg-blue-50"
                                    title="导出编排配置"
                                    onClick={() => {
                                        if (protocolId && version.id) {
                                            const exportUrl = `/api/v1/protocols/${protocolId}/versions/${version.id}/export?exportType=definition`;
                                            window.open(exportUrl, '_blank');
                                            toast.info(`开始导出版本 ${version.version} 的编排配置YAML文件...`);
                                        } else {
                                            toast.error("无法导出：缺少协议ID或版本ID。");
                                        }
                                    }}
                                >
                                    <DownloadIcon className="mr-1 h-4 w-4" /> 导出
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2 text-red-600 hover:text-red-700 hover:bg-red-50"
                                    title="删除版本"
                                    disabled={isDeleting}
                                    onClick={() => handleDeleteVersion(version.id, version.version)}
                                >
                                    <TrashIcon className="mr-1 h-4 w-4" /> 删除
                                </Button>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        );
    }

    const renderGlobalMapList = () => {
        if (isLoadingGlobalMaps) {
            return (
                <div className="space-y-2 mt-4">
                    {[...Array(3)].map((_, i) => (
                        <Skeleton key={i} className="h-10 w-full bg-muted/60" />
                    ))}
                </div>
            );
        }

        if (globalmapsError) {
            return (
                <div className="text-red-600 bg-red-50 border border-red-200 rounded-md p-4 mt-4 flex items-center">
                    <ExclamationTriangleIcon className="h-5 w-5 mr-2 flex-shrink-0" />
                    <span>加载全局映射列表失败: {globalmapsError}</span>
                </div>
            );
        }

        return (
            <Table>
                {!globalmaps || globalmaps.length === 0 && (
                    <TableCaption>暂无全局映射信息。</TableCaption>
                )}
                <TableHeader>
                    <TableRow>
                        <TableHead>名称</TableHead>
                        <TableHead>描述</TableHead>
                        <TableHead>创建时间</TableHead>
                        <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {globalmaps && globalmaps.map((globalmap) => (
                        <TableRow key={globalmap.id}>
                            <TableCell className="font-medium">{globalmap.name}</TableCell>
                            <TableCell className="text-muted-foreground">{globalmap.description || '-'}</TableCell>
                            <TableCell className="text-muted-foreground">{new Date(globalmap.createdAt || Date.now()).toLocaleString()}</TableCell>
                            <TableCell className="text-right space-x-1">
                                <Button asChild variant="ghost" size="sm" className="h-8 px-2">
                                    <Link to={`/globalmaps/${globalmap.id}/edit`} title="编辑全局映射">
                                        <Pencil1Icon className="mr-1 h-4 w-4" /> 编辑
                                    </Link>
                                </Button>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="h-8 px-2 text-red-600 hover:text-red-700 hover:bg-red-50"
                                    title="删除全局映射"
                                    disabled={isDeletingGlobalMap}
                                    onClick={() => handleDeleteGlobalMap(globalmap.id, globalmap.name)}
                                >
                                    <TrashIcon className="mr-1 h-4 w-4" /> 删除
                                </Button>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        );
    }

    // 渲染测试结果的函数
    const renderTestResults = () => {
        if (!testResults) return null;

        // 字节块可视化所需数据
        const totalBytesString = testResults.totalBytes || '';
        const finalCursor = testResults.finalCursor ?? 0; // finalCursor 可能为 0 或 null/undefined
        const bytesArray = totalBytesString.match(/.{1,2}/g) || []; // 分割为字节数组

        return (
            <Tabs defaultValue="summary" className="w-full">
                <TabsList className="grid grid-cols-4 mb-4">
                    <TabsTrigger value="summary">概览</TabsTrigger>
                    <TabsTrigger value="sections">段处理</TabsTrigger>
                    <TabsTrigger value="dispatcher">调度器</TabsTrigger>
                    <TabsTrigger value="raw">原始数据</TabsTrigger>
                </TabsList>

                <TabsContent value="summary">
                    <Card className="mb-4">
                        <CardHeader className="pb-2">
                            <CardTitle className="text-sm font-medium">处理字节 (总共: {bytesArray.length} 字节)</CardTitle>
                            {/* 添加图例说明 */}
                            <CardDescription className="text-xs pt-1">
                                <span className="inline-block w-3 h-3 bg-green-100 border border-green-300 mr-1 align-middle"></span> 已解析
                                <span className="inline-block w-3 h-3 bg-gray-100 border border-gray-300 ml-3 mr-1 align-middle"></span> 未解析
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            {/* 替换原来的 ScrollArea 和 pre */}
                            <div className="flex flex-wrap gap-1 p-2 border rounded-md bg-muted/20">
                                {bytesArray.length > 0 ? (
                                    bytesArray.map((byteHex: string, index: number) => {
                                        const isParsed = index < finalCursor;
                                        const bgColor = isParsed ? 'bg-green-100 hover:bg-green-200' : 'bg-gray-100 hover:bg-gray-200';
                                        const borderColor = isParsed ? 'border-green-300' : 'border-gray-300';
                                        const textColor = isParsed ? 'text-green-900' : 'text-gray-700';

                                        return (
                                            <span
                                                key={index}
                                                title={`字节 ${index} (值: 0x${byteHex}) - ${isParsed ? '已解析' : '未解析'}`}
                                                className={`inline-block px-1.5 py-0.5 border rounded text-xs font-mono transition-colors ${bgColor} ${borderColor} ${textColor}`}
                                            >
                                                {byteHex}
                                            </span>
                                        );
                                    })
                                ) : (
                                    <span className="text-xs text-muted-foreground italic">无字节数据</span>
                                )}
                            </div>
                        </CardContent>
                    </Card>

                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <Card>
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm font-medium">性能指标</CardTitle>
                            </CardHeader>
                            <CardContent>
                                <div className="flex items-center justify-between">
                                    <span className="text-sm">处理时间</span>
                                    <Badge variant="outline" className="ml-auto font-mono">
                                        {formatDurationNs(testResults.processingTime)}
                                    </Badge>
                                </div>
                                <div className="flex items-center justify-between mt-2">
                                    <span className="text-sm">生成点数</span>
                                    <Badge variant="outline" className="ml-auto font-mono">
                                        {testResults.points?.length || 0}
                                    </Badge>
                                </div>
                            </CardContent>
                        </Card>

                        <Card className="md:col-span-2">
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm font-medium">数据点</CardTitle>
                            </CardHeader>
                            <CardContent className="p-0">
                                <Table>
                                    <TableHeader>
                                        <TableRow>
                                            <TableHead>设备</TableHead>
                                            <TableHead>字段</TableHead>
                                            <TableHead className="text-right">值</TableHead>
                                        </TableRow>
                                    </TableHeader>
                                    <TableBody>
                                        {testResults.points && testResults.points.length > 0 ? (
                                            testResults.points.map((point: any, index: number) => (
                                                <TableRow key={index}>
                                                    <TableCell className="font-medium">{point.Device}</TableCell>
                                                    <TableCell>
                                                        {Object.keys(point.Field || {}).join(', ')}
                                                    </TableCell>
                                                    <TableCell className="text-right font-mono">
                                                        {Object.values(point.Field || {}).join(', ')}
                                                    </TableCell>
                                                </TableRow>
                                            ))
                                        ) : (
                                            <TableRow>
                                                <TableCell colSpan={3} className="text-center text-muted-foreground">
                                                    没有数据点
                                                </TableCell>
                                            </TableRow>
                                        )}
                                    </TableBody>
                                </Table>
                            </CardContent>
                        </Card>

                        <Card className="md:col-span-3">
                            <CardHeader className="pb-2">
                                <CardTitle className="text-sm font-medium">变量状态</CardTitle>
                            </CardHeader>
                            <CardContent>
                                {Object.keys(testResults.finalVars || {}).length > 0 ? (
                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                                        {Object.entries(testResults.finalVars || {}).map(([key, value]: [string, any], index: number) => (
                                            <div key={index} className="flex items-center justify-between border p-2 rounded-md">
                                                <span className="text-sm font-medium">{key}</span>
                                                <Badge variant="secondary" className="font-mono text-xs">
                                                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                                </Badge>
                                            </div>
                                        ))}
                                    </div>
                                ) : (
                                    <div className="text-center text-muted-foreground">
                                        没有变量数据
                                    </div>
                                )}
                            </CardContent>
                        </Card>
                    </div>
                </TabsContent>

                <TabsContent value="sections">
                    {/* --- 添加新的基于 processingSteps 的渲染逻辑 --- */}
                    {testResults.processingSteps && testResults.processingSteps.length > 0 ? (
                        <>
                            <div className="flex items-center gap-4 mb-4 text-xs">
                                <span className="font-medium">图例:</span>
                                <div className="flex items-center">
                                    <span className="h-3 w-3 inline-block bg-green-50 border border-green-200 mr-1 rounded-sm"></span>
                                    <span>新增变量</span>
                                </div>
                                <div className="flex items-center">
                                    <span className="h-3 w-3 inline-block bg-blue-50 border border-blue-200 mr-1 rounded-sm"></span>
                                    <span>修改变量</span>
                                </div>
                                <div className="flex items-center">
                                    <span className="h-3 w-3 inline-block bg-red-50 border border-red-200 mr-1 rounded-sm"></span>
                                    <span>错误</span>
                                </div>
                            </div>
                            <Accordion type="single" collapsible className="w-full space-y-2">
                                {testResults.processingSteps.map((step: any, index: number) => (
                                    <AccordionItem key={index} value={`step-${index}`}>
                                        <AccordionTrigger className={`flex justify-between items-center text-sm px-3 py-2 rounded-md hover:no-underline ${step.error ? 'bg-red-50 hover:bg-red-100' : 'bg-muted/50 hover:bg-muted/80'}`}>
                                            <div className="flex items-center gap-2">
                                                <Badge variant={step.error ? "destructive" : "secondary"} className="w-6 h-6 flex items-center justify-center p-0">{index + 1}</Badge>
                                                <span className="font-medium">{step.nodeLabel || '未知节点'}</span>
                                                <span className="text-xs text-muted-foreground">
                                                    ({step.startIndex} <ArrowRightIcon className="inline h-3 w-3 mx-0.5" /> {step.endIndex})
                                                </span>
                                            </div>
                                            {step.error ? (
                                                <span className="text-xs text-red-600 mr-2 flex items-center">
                                                    <ExclamationTriangleIcon className="h-4 w-4 mr-1" /> 错误
                                                </span>
                                            ) : (
                                                <Badge variant="outline" className="font-mono text-xs h-5 px-1.5 mr-2">
                                                    {step.consumedBytes || '无消耗'}
                                                </Badge>
                                            )}
                                        </AccordionTrigger>
                                        <AccordionContent className="px-3 pt-3 border rounded-b-md mt-[-2px]">
                                            {step.error && (
                                                <div className="mb-3 p-2 border border-red-200 bg-red-50 text-red-800 rounded text-xs">
                                                    <strong className="font-medium">错误信息:</strong> {step.error}
                                                </div>
                                            )}
                                            <div className="text-xs mb-3">
                                                <strong className="font-medium">消耗字节:</strong>
                                                <span className="ml-2 font-mono p-1 bg-background border rounded text-muted-foreground">
                                                    {step.consumedBytes || '-'}
                                                </span>
                                                <span className="ml-2 text-muted-foreground">
                                                    (光标: {step.startIndex} &rarr; {step.endIndex})
                                                </span>
                                            </div>

                                            {/* 处理前变量 */}
                                            <div className="mb-3">
                                                <strong className="font-medium text-xs mb-1 block">处理前变量:</strong>
                                                {Object.keys(step.varsBefore || {}).length > 0 ? (
                                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2 p-2 border rounded bg-muted/20">
                                                        {Object.entries(step.varsBefore || {}).map(([key, value]: [string, any], vIndex: number) => (
                                                            <div key={vIndex} className="flex items-center justify-between border p-1 rounded-md bg-background text-xs">
                                                                <span className="font-medium mr-1 truncate" title={key}>{key}</span>
                                                                <Badge variant="outline" className="font-mono text-xs px-1 whitespace-nowrap">
                                                                    {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                                                                </Badge>
                                                            </div>
                                                        ))}
                                                    </div>
                                                ) : (
                                                    <div className="text-xs text-center text-muted-foreground p-2 border rounded bg-muted/20 italic">
                                                        无变量
                                                    </div>
                                                )}
                                            </div>

                                            {/* 变量变化 */}
                                            <div>
                                                <strong className="font-medium text-xs mb-1 block">变量变化 (新增/修改):</strong>
                                                {(() => {
                                                    // 计算变量变化
                                                    const varChanges: {
                                                        key: string;
                                                        oldValue: any;
                                                        newValue: any;
                                                        type: 'added' | 'modified';
                                                    }[] = [];

                                                    // 检查所有 varsAfter 中的键
                                                    Object.entries(step.varsAfter || {}).forEach(([key, newValue]) => {
                                                        const beforeVars = step.varsBefore || {};

                                                        // 检查键是否存在于 varsBefore
                                                        if (!(key in beforeVars)) {
                                                            // 新增的变量
                                                            varChanges.push({
                                                                key,
                                                                oldValue: undefined,
                                                                newValue,
                                                                type: 'added'
                                                            });
                                                        }
                                                        // 检查值是否发生变化
                                                        else if (JSON.stringify(beforeVars[key]) !== JSON.stringify(newValue)) {
                                                            // 修改的变量
                                                            varChanges.push({
                                                                key,
                                                                oldValue: beforeVars[key],
                                                                newValue,
                                                                type: 'modified'
                                                            });
                                                        }
                                                    });

                                                    if (varChanges.length > 0) {
                                                        return (
                                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-2 p-2 border rounded bg-muted/20">
                                                                {varChanges.map((change, cIndex) => (
                                                                    <div key={cIndex} className={`flex flex-col border p-1 rounded-md
                                                                        ${change.type === 'added' ? 'bg-green-50 border-green-200' : 'bg-blue-50 border-blue-200'}`}>
                                                                        <div className="flex items-center justify-between mb-1">
                                                                            <span className={`font-medium truncate ${change.type === 'added' ? 'text-green-700' : 'text-blue-700'}`} title={change.key}>
                                                                                {change.key}
                                                                                <Badge className={`ml-1 ${change.type === 'added' ? 'bg-green-200 text-green-800' : 'bg-blue-200 text-blue-800'}`}>
                                                                                    {change.type === 'added' ? '新增' : '修改'}
                                                                                </Badge>
                                                                            </span>
                                                                        </div>
                                                                        {change.type === 'modified' && (
                                                                            <div className="flex items-center text-xs text-red-500 line-through mb-1">
                                                                                <span className="font-mono bg-red-50 px-1 rounded">
                                                                                    {typeof change.oldValue === 'object' ? JSON.stringify(change.oldValue) : String(change.oldValue)}
                                                                                </span>
                                                                            </div>
                                                                        )}
                                                                        <div className="flex items-center text-xs text-green-600">
                                                                            <span className="font-mono bg-green-50 px-1 rounded">
                                                                                {typeof change.newValue === 'object' ? JSON.stringify(change.newValue) : String(change.newValue)}
                                                                            </span>
                                                                        </div>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        );
                                                    } else {
                                                        return (
                                                            <div className="text-xs text-center text-muted-foreground p-2 border rounded bg-muted/20 italic">
                                                                无变量变化
                                                            </div>
                                                        );
                                                    }
                                                })()}
                                            </div>
                                        </AccordionContent>
                                    </AccordionItem>
                                ))}
                            </Accordion>
                        </>
                    ) : (
                        <div className="text-center p-4 border rounded-md text-muted-foreground">
                            没有详细的处理步骤信息。
                        </div>
                    )}
                </TabsContent>

                <TabsContent value="dispatcher">
                    {testResults.dispatcherResults && testResults.dispatcherResults.length > 0 ? (
                        <div className="space-y-4">
                            {testResults.dispatcherResults.map((result: any, index: number) => (
                                <Card key={index}>
                                    <CardHeader className="pb-2">
                                        <CardTitle className="text-base">{result.strategyName || `策略 ${index + 1}`}</CardTitle>
                                        <CardDescription>
                                            处理点数: <Badge variant="secondary">{result.points?.length || 0}</Badge>
                                        </CardDescription>
                                    </CardHeader>
                                    <CardContent className="p-0">
                                        {result.points && result.points.length > 0 ? (
                                            <Table>
                                                <TableHeader>
                                                    <TableRow>
                                                        <TableHead>设备</TableHead>
                                                        <TableHead>字段</TableHead>
                                                        <TableHead className="text-right">值</TableHead>
                                                    </TableRow>
                                                </TableHeader>
                                                <TableBody>
                                                    {result.points.map((point: any, pointIdx: number) => (
                                                        <TableRow key={pointIdx}>
                                                            <TableCell className="font-medium">{point.Device}</TableCell>
                                                            <TableCell>
                                                                {Object.keys(point.Field || {}).join(', ')}
                                                            </TableCell>
                                                            <TableCell className="text-right font-mono">
                                                                {Object.values(point.Field || {}).join(', ')}
                                                            </TableCell>
                                                        </TableRow>
                                                    ))}
                                                </TableBody>
                                            </Table>
                                        ) : (
                                            <div className="p-4 text-center text-muted-foreground">
                                                没有处理点数据
                                            </div>
                                        )}
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    ) : (
                        <div className="text-center p-4 border rounded-md text-muted-foreground">
                            没有调度器结果数据
                        </div>
                    )}
                </TabsContent>

                <TabsContent value="raw">
                    <Card>
                        <CardHeader>
                            <CardTitle className="text-sm">原始JSON数据</CardTitle>
                            <CardDescription>完整的测试结果响应</CardDescription>
                        </CardHeader>
                        <CardContent>
                            <ScrollArea className="h-[400px] rounded-md border p-4">
                                <pre className="text-xs font-mono whitespace-pre-wrap">
                                    {JSON.stringify(testResults, null, 2)}
                                </pre>
                            </ScrollArea>
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        );
    };

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <div className="flex justify-between items-start">
                    <div>
                        <h1 className="text-2xl font-bold">{protocol.name}</h1>
                        <p className="text-muted-foreground">协议详情</p>
                    </div>
                    <div className="flex space-x-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleExportProtocolConfig}
                            title="导出协议配置 (YAML)"
                        >
                            <DownloadIcon className="mr-2 h-4 w-4" />
                            导出配置
                        </Button>
                        <Button asChild variant="outline" size="sm">
                            <Link to={`/protocols/${protocol.id}/edit-config`} title="修改配置">
                                <GearIcon className="mr-2 h-4 w-4" /> 配置
                            </Link>
                        </Button>
                        <Button asChild variant="outline" size="sm">
                            <Link to={`/protocols/${protocol.id}/edit`}>
                                <Pencil1Icon className="mr-2 h-4 w-4" /> 编辑
                            </Link>
                        </Button>
                    </div>
                </div>

                <Card>
                    <CardHeader className="flex flex-row items-center justify-between">
                        <div>
                            <CardTitle>版本列表</CardTitle>
                            <CardDescription>管理该协议的不同版本。</CardDescription>
                        </div>
                        <Button asChild size="sm">
                            <Link to={`/protocols/${protocolId}/versions/new`}>
                                <PlusCircledIcon className="mr-2 h-4 w-4" /> 新建版本
                            </Link>
                        </Button>
                    </CardHeader>
                    <CardContent>
                        {renderVersionList()}
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader className="flex flex-row items-center justify-between">
                        <div>
                            <CardTitle>全局映射列表</CardTitle>
                            <CardDescription>管理该协议的全局映射。</CardDescription>
                        </div>
                        <Button asChild size="sm">
                            <Link to={`/protocols/${protocolId}/globalmaps/new`}>
                                <PlusCircledIcon className="mr-2 h-4 w-4" /> 新建全局映射
                            </Link>
                        </Button>
                    </CardHeader>
                    <CardContent>
                        {renderGlobalMapList()}
                    </CardContent>
                </Card>
            </div>

            {/* 参数设置对话框 */}
            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle>运行协议测试</DialogTitle>
                        <DialogDescription>
                            选择要使用的全局映射和测试数据，然后运行测试。
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <Tabs defaultValue="params">
                            <TabsList className="grid grid-cols-2">
                                <TabsTrigger value="params">测试参数</TabsTrigger>
                                <TabsTrigger value="config">当前配置</TabsTrigger>
                            </TabsList>

                            <TabsContent value="params" className="space-y-4">
                                <div>
                                    <Label htmlFor="version" className="mb-2 block">版本</Label>
                                    <div id="version" className="font-medium text-sm">
                                        {selectedVersion?.version} - {selectedVersion?.description || '无描述'}
                                    </div>
                                </div>

                                <div>
                                    <Label htmlFor="globalmap" className="mb-2 block">全局映射</Label>
                                    <div className="relative" ref={dropdownRef}>
                                        {/* 使用单独的按钮触发下拉菜单，避免aria-hidden问题 */}
                                        <Button
                                            variant="outline"
                                            className="w-full justify-between"
                                            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                                        >
                                            {selectedGlobalMapId === 'none' ? '不使用全局映射' :
                                                globalmaps?.find(map => map.id === selectedGlobalMapId)?.name || '选择全局映射'}
                                        </Button>

                                        {/* 自定义下拉内容 */}
                                        <div
                                            className={`absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-md ${isDropdownOpen ? 'block' : 'hidden'}`}
                                        >
                                            <div
                                                className="p-2 hover:bg-accent hover:text-accent-foreground cursor-pointer"
                                                onClick={() => {
                                                    setSelectedGlobalMapId('none');
                                                    setIsDropdownOpen(false);
                                                }}
                                            >
                                                不使用全局映射
                                            </div>

                                            {globalmaps && globalmaps.map(gmap => (
                                                <div
                                                    key={gmap.id}
                                                    className="p-2 hover:bg-accent hover:text-accent-foreground cursor-pointer"
                                                    onClick={() => {
                                                        setSelectedGlobalMapId(gmap.id);
                                                        setIsDropdownOpen(false);
                                                    }}
                                                >
                                                    {gmap.name} - {gmap.description || '无描述'}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                </div>

                                <div>
                                    <Label htmlFor="hexdata" className="mb-2 block">十六进制测试数据</Label>
                                    <Input
                                        id="hexdata"
                                        placeholder="输入十六进制数据 (例如: 0102030405)"
                                        value={hexData}
                                        onChange={(e) => setHexData(e.target.value.replace(/[^0-9A-Fa-f]/g, ''))}
                                        className="font-mono"
                                    />
                                    <p className="text-xs text-muted-foreground mt-1">
                                        使用十六进制格式 (0-9, A-F)，长度必须是偶数
                                    </p>
                                </div>
                            </TabsContent>

                            <TabsContent value="config" className="space-y-4">
                                <div>
                                    <div className="flex justify-between items-center mb-2">
                                        <Label className="font-medium">调度器配置</Label>
                                        <Button variant="ghost" size="sm" asChild className="h-6 px-2">
                                            <Link to={`/protocols/${protocolId}/edit-config`}>
                                                <Pencil1Icon className="h-3 w-3 mr-1" /> 编辑
                                            </Link>
                                        </Button>
                                    </div>
                                    <div className="rounded-md border p-3 bg-muted/30 text-sm">
                                        {(() => {
                                            // 安全地提取配置，添加类型检查以避免TypeScript错误
                                            // @ts-ignore - 处理Protocol联合类型中的可选config属性
                                            const dispatcherConfig = protocol?.config?.dispatcher;
                                            const repeatDataFilters = dispatcherConfig?.repeat_data_filter || [];
                                            const hasFilters = Array.isArray(repeatDataFilters) && repeatDataFilters.length > 0;

                                            return (
                                                <div className="mb-2">
                                                    <h4 className="text-xs font-medium text-muted-foreground mb-1">重复数据过滤</h4>
                                                    {!hasFilters ? (
                                                        <p className="text-xs italic">无过滤规则</p>
                                                    ) : (
                                                        <div className="space-y-1">
                                                            {repeatDataFilters.map((filter: any, index: number) => (
                                                                <div key={index} className="grid grid-cols-2 gap-2 text-xs">
                                                                    <div>设备: <span className="font-mono">{filter.dev_filter || '.*'}</span></div>
                                                                    <div>遥测: <span className="font-mono">{filter.tele_filter || '.*'}</span></div>
                                                                </div>
                                                            ))}
                                                        </div>
                                                    )}
                                                </div>
                                            );
                                        })()}
                                    </div>
                                </div>

                                <div>
                                    <Label className="font-medium mb-2 block">策略配置</Label>
                                    <div className="rounded-md border p-3 bg-muted/30 text-sm">
                                        {(() => {
                                            // 安全地提取配置，添加类型检查以避免TypeScript错误
                                            // @ts-ignore - 处理Protocol联合类型中的可选config属性
                                            const strategies = protocol?.config?.strategy || [];
                                            const hasStrategies = Array.isArray(strategies) && strategies.length > 0;

                                            return !hasStrategies ? (
                                                <p className="text-xs italic">无策略配置</p>
                                            ) : (
                                                <div className="space-y-3">
                                                    {strategies.map((strategy: any, index: number) => (
                                                        <div key={index} className="border-b border-border last:border-0 pb-2 last:pb-0">
                                                            <div className="flex justify-between items-center mb-1">
                                                                <h4 className="text-xs font-medium">{strategy.type || '未命名策略'}</h4>
                                                                <span className={`text-xs ${strategy.enable ? 'text-green-600' : 'text-red-600'}`}>
                                                                    {strategy.enable ? '已启用' : '已禁用'}
                                                                </span>
                                                            </div>
                                                            <div className="text-xs text-muted-foreground">过滤规则:</div>
                                                            {!strategy.filter ||
                                                                (Array.isArray(strategy.filter) && strategy.filter.length === 0) ? (
                                                                <p className="text-xs italic">无过滤规则</p>
                                                            ) : (
                                                                <div className="mt-1 space-y-1">
                                                                    {Array.isArray(strategy.filter) &&
                                                                        strategy.filter.map((filter: any, filterIdx: number) => (
                                                                            <div key={filterIdx} className="grid grid-cols-2 gap-2 text-xs">
                                                                                <div>设备: <span className="font-mono">{filter.dev_filter || '.*'}</span></div>
                                                                                <div>遥测: <span className="font-mono">{filter.tele_filter || '.*'}</span></div>
                                                                            </div>
                                                                        ))}
                                                                </div>
                                                            )}
                                                        </div>
                                                    ))}
                                                </div>
                                            );
                                        })()}
                                    </div>
                                </div>

                                <div className="text-xs text-muted-foreground">
                                    <p>这些配置将用于测试过程中的数据处理。点击"编辑"按钮可修改配置。</p>
                                </div>
                            </TabsContent>
                        </Tabs>
                    </div>
                    <DialogFooter>
                        <Button
                            type="button"
                            variant="secondary"
                            onClick={() => setIsDialogOpen(false)}
                            disabled={isRunningTest}
                        >
                            取消
                        </Button>
                        <Button
                            type="button"
                            onClick={handleRunTest}
                            disabled={isRunningTest}
                        >
                            {isRunningTest ? (
                                <>
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    处理中...
                                </>
                            ) : (
                                <>
                                    <PlayIcon className="mr-2 h-4 w-4" />
                                    运行测试
                                </>
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* 测试结果对话框 */}
            <Dialog open={isResultsDialogOpen} onOpenChange={setIsResultsDialogOpen}>
                <DialogContent className="sm:max-w-3xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>测试结果</DialogTitle>

                    </DialogHeader>
                    <div className="py-2">
                        {renderTestResults()}
                    </div>
                    <DialogFooter>
                        <Button
                            type="button"
                            onClick={() => setIsResultsDialogOpen(false)}
                        >
                            关闭
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* 配置详情对话框 */}
            <AlertDialog open={isConfigDialogOpen} onOpenChange={setIsConfigDialogOpen}>
                <AlertDialogContent className="max-w-3xl">
                    <AlertDialogHeader>
                        <AlertDialogTitle>配置详情 / 调试信息</AlertDialogTitle>
                    </AlertDialogHeader>

                    <div className="py-4 max-h-[60vh] overflow-y-auto pr-2">
                        {configDebugInfo ? (
                            <Accordion type="multiple" className="w-full space-y-2">
                                {(configDebugInfo.backendError || configDebugInfo.backendErrorRaw) && (
                                    <AccordionItem value="backend-error">
                                        <AccordionTrigger className="text-sm font-medium hover:no-underline bg-muted/50 px-3 rounded-t-md">后端错误详情</AccordionTrigger>
                                        <AccordionContent className="px-3 pt-3 border border-t-0 rounded-b-md">
                                            <pre className="text-xs whitespace-pre-wrap break-all bg-background p-2 rounded">
                                                {configDebugInfo.backendError ?
                                                    JSON.stringify(configDebugInfo.backendError, null, 2) :
                                                    configDebugInfo.backendErrorRaw}
                                            </pre>
                                        </AccordionContent>
                                    </AccordionItem>
                                )}

                                {configDebugInfo.requestPayload && (
                                    <AccordionItem value="request-payload">
                                        <AccordionTrigger className="text-sm font-medium hover:no-underline bg-muted/50 px-3 rounded-t-md">请求体 (Request Payload)</AccordionTrigger>
                                        <AccordionContent className="px-3 pt-3 border border-t-0 rounded-b-md">
                                            <ScrollArea className="h-[200px] rounded border bg-background p-2">
                                                <pre className="text-xs whitespace-pre-wrap break-all">
                                                    {JSON.stringify(configDebugInfo.requestPayload, null, 2)}
                                                </pre>
                                            </ScrollArea>
                                        </AccordionContent>
                                    </AccordionItem>
                                )}

                                {configDebugInfo.stackTrace && (
                                    <AccordionItem value="stack-trace">
                                        <AccordionTrigger className="text-sm font-medium hover:no-underline bg-muted/50 px-3 rounded-t-md">前端堆栈跟踪 (Stack Trace)</AccordionTrigger>
                                        <AccordionContent className="px-3 pt-3 border border-t-0 rounded-b-md">
                                            <ScrollArea className="h-[200px] rounded border bg-background p-2">
                                                <pre className="text-xs whitespace-pre-wrap break-all">
                                                    {configDebugInfo.stackTrace}
                                                </pre>
                                            </ScrollArea>
                                        </AccordionContent>
                                    </AccordionItem>
                                )}

                                {configDebugInfo.details && (
                                    <AccordionItem value="other-details">
                                        <AccordionTrigger className="text-sm font-medium hover:no-underline bg-muted/50 px-3 rounded-t-md">其他细节</AccordionTrigger>
                                        <AccordionContent className="px-3 pt-3 border border-t-0 rounded-b-md">
                                            <ScrollArea className="h-[200px] rounded border bg-background p-2">
                                                <pre className="text-xs whitespace-pre-wrap break-all">
                                                    {JSON.stringify(configDebugInfo.details, null, 2)}
                                                </pre>
                                            </ScrollArea>
                                        </AccordionContent>
                                    </AccordionItem>
                                )}

                            </Accordion>
                        ) : (
                            <div className="text-center text-muted-foreground p-4">没有可用的调试信息。</div>
                        )}

                        <div className="mt-6 text-sm text-muted-foreground border-t pt-4">
                            <p className="font-medium mb-2">常见问题和解决方案:</p>
                            <ul className="list-disc pl-5 mt-2 space-y-1">
                                <li>确保版本配置已正确保存</li>
                                <li>检查配置是否包含有效的段定义</li>
                                <li>验证每个段是否包含所需的字段（desc, size, Dev等）</li>
                                <li>确保协议配置正确设置（可在右上角"配置"按钮查看）</li>
                                <li>检查协议配置和版本配置是否匹配</li>
                                <li>版本配置需要是有效的段配置数组，协议配置则包含解析器、连接器等全局设置</li>
                                <li>如果问题持续，请尝试重新编辑并保存配置</li>
                            </ul>
                        </div>
                    </div>

                    <AlertDialogFooter>
                        <AlertDialogCancel>关闭</AlertDialogCancel>
                        <AlertDialogAction asChild>
                            <Button
                                variant="default"
                                onClick={() => {
                                    if (selectedVersion) {
                                        navigate(`/versions/${selectedVersion.id}/orchestration`);
                                    }
                                }}
                            >
                                <Pencil1Icon className="mr-2 h-4 w-4" />
                                编辑配置
                            </Button>
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}
