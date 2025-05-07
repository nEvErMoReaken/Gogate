import { useState } from "react";
import { useNavigate, useParams, useLoaderData } from "react-router";
import { Link } from "react-router";
import type { Protocol, Route } from "../../../+types/protocols";
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
import { API } from "../../../api";

export const clientLoader = async ({ params }: { params: { protocolId: string } }) => {
    const protocolId = params.protocolId;

    try {
        const response = await API.protocols.getById(protocolId);
        if (response.error) {
            return { protocol: null, error: response.error };
        }
        return response.data || { protocol: null };
    } catch (error) {
        console.error('加载协议详情出错:', error);
        return { protocol: null, error: String(error) };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const protocol = data?.protocol as Protocol;
    return [
        { title: protocol ? `${protocol.name} - 新建版本` : '新建版本 - 网关管理' },
        { name: "description", content: '创建协议的新版本' },
    ];
};

export default function NewProtocolVersion() {
    const navigate = useNavigate();
    const data = useLoaderData<typeof clientLoader>();
    const { protocolId } = useParams<{ protocolId: string }>();
    const protocol = data?.protocol;

    const [version, setVersion] = useState("1.0.0");
    const [description, setDescription] = useState("");
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSubmitting(true);

        if (!version.trim()) {
            toast.error("版本号不能为空");
            setIsSubmitting(false);
            return;
        }

        try {
            const response = await API.protocols.versions.create(protocolId!, {
                version,
                description
            });

            if (response.error) {
                const expectedErrorMsg = "该协议下已存在相同的版本号";
                // console.log("Received error:", response.error);
                // console.log("Expected error:", expectedErrorMsg);
                // console.log("Do they include?", response.error.includes(expectedErrorMsg)); // 移除调试日志

                if (response.error.includes(expectedErrorMsg)) {
                    toast.error(`版本号 \"${version}\" 已存在，请使用不同的版本号。`);
                } else {
                    toast.error(`创建版本失败: ${response.error}`);
                }
            } else {
                toast.success("版本创建成功");
                navigate(`/protocols/${protocolId}`);
            }
        } catch (error) {
            console.error("创建版本请求失败:", error);
            if (error instanceof Error && error.message.includes("Failed to fetch")) {
                toast.error("网络连接错误，请检查网络后重试。");
            } else {
                toast.error("创建版本时发生未知错误，请稍后重试。");
            }
        } finally {
            setIsSubmitting(false);
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

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <div className="flex justify-between items-start">
                    <div>
                        <h1 className="text-2xl font-bold">{protocol.name}</h1>
                        <p className="text-muted-foreground">新建版本</p>
                    </div>
                    <Button asChild variant="outline" size="sm">
                        <Link to={`/protocols/${protocolId}`}>
                            <ArrowLeftIcon className="mr-2 h-4 w-4" /> 返回版本列表
                        </Link>
                    </Button>
                </div>

                <Card>
                    <form onSubmit={handleSubmit}>
                        <CardHeader>
                            <CardTitle>版本信息</CardTitle>
                            <CardDescription>填写新版本的详细信息。</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4 pb-6">
                            <div className="space-y-2">
                                <Label htmlFor="version">版本号</Label>
                                <Input
                                    id="version"
                                    placeholder="输入版本号 (如 1.0.0)"
                                    value={version}
                                    onChange={(e) => setVersion(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="description">描述</Label>
                                <Textarea
                                    id="description"
                                    placeholder="描述此版本的变更内容..."
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
                                onClick={() => navigate(`/protocols/${protocolId}/versions`)}
                            >
                                取消
                            </Button>
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting ? "创建中..." : "创建版本"}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    );
}
