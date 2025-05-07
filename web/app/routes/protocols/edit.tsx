import { useEffect, useState } from "react";
import { useParams, useNavigate, useLoaderData } from "react-router";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import type { Protocol, Route } from "../../+types/protocols";
import { API } from "../../api"; // 导入 API

export const clientLoader = async ({ params }: { params: { protocolId: string } }) => {
    const protocolId = params.protocolId;

    try {
        // 使用 API.protocols.getById 替换 fetch
        const response = await API.protocols.getById(protocolId);
        if (response.status === 200 && response.data) {
            // API.protocols.getById 返回 { protocol: Protocol } 结构
            return response.data;
        }
        // 处理 API 返回的错误或通用错误
        return { protocol: null, error: response.error || '加载失败' };
    } catch (error) {
        console.error('加载协议详情出错:', error);
        // 处理网络或其他意外错误
        return { protocol: null, error: String(error) };
    }
};


export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const protocol = data?.protocol as Protocol;
    return [
        { title: protocol ? `${protocol.name} - 编辑协议` : '编辑协议 - 网关管理' },
        { name: "description", content: protocol?.description || '编辑协议基本信息' },
    ];
};

export default function EditProtocol() {
    const navigate = useNavigate();
    const data = useLoaderData<typeof clientLoader>();
    const { protocolId } = useParams<{ protocolId: string }>();
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [isSubmitting, setIsSubmitting] = useState(false);

    useEffect(() => {
        if (data?.protocol) {
            setName(data.protocol.name || "");
            setDescription(data.protocol.description || "");
        }
    }, [data]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSubmitting(true);

        if (!name.trim()) {
            toast.error("协议名称不能为空");
            setIsSubmitting(false);
            return;
        }

        try {
            // 使用 API.protocols.update 替换 fetch
            const response = await API.protocols.update(protocolId!, {
                name,
                description,
            });

            if (response.error) {
                // 处理 API 返回的错误
                throw new Error(response.error);
            }

            toast.success("协议更新成功");
            navigate(`/protocols/${protocolId}`);
        } catch (error) {
            console.error("更新协议失败:", error);
            // 显示更具体的错误信息
            toast.error(`更新协议失败: ${error instanceof Error ? error.message : String(error)}`);
        } finally {
            setIsSubmitting(false);
        }
    };

    if (!data?.protocol) {
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
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <div className="flex justify-between items-start">
                    <div>
                        <h1 className="text-2xl font-bold">编辑协议</h1>
                        <p className="text-muted-foreground">修改协议的基本信息</p>
                    </div>
                    <Button asChild variant="outline" size="sm">
                        <Link to={`/protocols/${protocolId}`}>
                            <ArrowLeftIcon className="mr-2 h-4 w-4" /> 返回详情
                        </Link>
                    </Button>
                </div>

                <Card>
                    <form onSubmit={handleSubmit}>
                        <CardHeader>
                            <CardTitle>基本信息</CardTitle>
                            <CardDescription>修改协议的名称和描述</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <div className="space-y-2">
                                <Label htmlFor="name">协议名称</Label>
                                <Input
                                    id="name"
                                    placeholder="输入协议名称"
                                    value={name}
                                    onChange={(e) => setName(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="description">描述</Label>
                                <Textarea
                                    id="description"
                                    placeholder="描述此协议的用途和特点..."
                                    value={description}
                                    onChange={(e) => setDescription(e.target.value)}
                                    rows={5}
                                />
                            </div>
                        </CardContent>
                        <CardFooter className="flex justify-between">
                            <Button
                                type="button"
                                variant="outline"
                                onClick={() => navigate(`/protocols/${protocolId}`)}
                            >
                                取消
                            </Button>
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting ? "保存中..." : "保存更改"}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    );
}
