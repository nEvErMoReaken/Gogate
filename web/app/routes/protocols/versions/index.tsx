import { Link, useLoaderData, useParams } from "react-router";
import type { Protocol, ProtocolVersion, Route } from "../../../+types/protocols";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
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
    ArrowLeftIcon,
    ArrowRightIcon,
    Pencil1Icon,
    PlusCircledIcon,
    ExclamationTriangleIcon
} from "@radix-ui/react-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { API, useApiGet } from "../../../api";

export const clientLoader = async ({ params }: { params: { protocolId: string } }) => {
    const protocolId = params.protocolId;

    try {
        const response = await API.protocols.getById(protocolId);
        if (response.status === 200 && response.data) {
            return response.data;
        }
        return { protocol: null, error: response.error || '加载失败' };
    } catch (error) {
        console.error('加载协议详情出错:', error);
        return { protocol: null, error: String(error) };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const protocol = data?.protocol as Protocol;
    return [
        { title: protocol ? `${protocol.name} - 版本管理` : '版本管理 - 网关管理' },
        { name: "description", content: '管理协议的不同版本' },
    ];
};

export default function ProtocolVersionsIndex() {
    const data = useLoaderData<typeof clientLoader>();
    const { protocolId } = useParams<{ protocolId: string }>();
    const protocol = data?.protocol;

    const {
        data: versions,
        isLoading: isLoadingVersions,
        error: versionsError
    } = useApiGet(() => {
        if (!protocolId) return Promise.resolve({ status: 400, error: 'Missing protocolId' });
        return API.protocols.versions.getAll(protocolId);
    }, [protocolId]);

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

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-start">
                <div>
                    <h1 className="text-2xl font-bold">{protocol.name}</h1>
                    <p className="text-muted-foreground">版本管理</p>
                </div>
                <div className="flex space-x-2">
                    <Button asChild variant="outline" size="sm">
                        <Link to={`/protocols/${protocol.id}`}>
                            <ArrowLeftIcon className="mr-2 h-4 w-4" /> 返回协议
                        </Link>
                    </Button>
                    <Button asChild size="sm">
                        <Link to={`/protocols/${protocolId}/versions/new`}>
                            <PlusCircledIcon className="mr-2 h-4 w-4" /> 新建版本
                        </Link>
                    </Button>
                </div>
            </div>

            <Card>
                <CardHeader>
                    <CardTitle>协议版本列表</CardTitle>
                    <CardDescription>管理"{protocol.name}"的不同版本。</CardDescription>
                </CardHeader>
                <CardContent>
                    {isLoadingVersions ? (
                        <div className="space-y-2 mt-4">
                            {[...Array(3)].map((_, i) => (
                                <Skeleton key={i} className="h-10 w-full bg-muted/60" />
                            ))}
                        </div>
                    ) : versionsError ? (
                        <div className="text-red-600 bg-red-50 border border-red-200 rounded-md p-4 mt-4 flex items-center">
                            <ExclamationTriangleIcon className="h-5 w-5 mr-2 flex-shrink-0" />
                            <span>加载版本列表失败: {versionsError}</span>
                        </div>
                    ) : (
                        <Table>
                            {!versions || versions.length === 0 && (
                                <TableCaption>暂无版本信息。</TableCaption>
                            )}
                            <TableHeader>
                                <TableRow>
                                    <TableHead>版本号</TableHead>
                                    <TableHead>描述</TableHead>
                                    <TableHead>创建时间</TableHead>
                                    <TableHead>更新时间</TableHead>
                                    <TableHead className="text-right">操作</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {versions && versions.map((version) => (
                                    <TableRow key={version.id}>
                                        <TableCell className="font-medium">{version.version}</TableCell>
                                        <TableCell className="text-muted-foreground">{version.description || '-'}</TableCell>
                                        <TableCell className="text-muted-foreground">{new Date(version.createdAt || Date.now()).toLocaleString()}</TableCell>
                                        <TableCell className="text-muted-foreground">{new Date(version.updatedAt || Date.now()).toLocaleString()}</TableCell>
                                        <TableCell className="text-right space-x-2">
                                            <Button asChild variant="ghost" size="sm">
                                                <Link to={`/versions/${version.id}/edit`}>
                                                    <Pencil1Icon className="mr-2 h-4 w-4" /> 编辑
                                                </Link>
                                            </Button>
                                            <Button asChild variant="ghost" size="sm">
                                                <Link to={`/versions/${version.id}`}>
                                                    <ArrowRightIcon className="mr-2 h-4 w-4" /> 详情
                                                </Link>
                                            </Button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
