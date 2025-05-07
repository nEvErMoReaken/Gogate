import { /* Form, */ useLoaderData, useNavigate, useParams } from "react-router";
import type { Route, ProtocolVersion } from "../../+types/protocols";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { useState, useEffect } from "react";
import { API } from "../../api";

// 定义 Loader 返回类型，明确包含可能的 error
interface LoaderData {
    version: ProtocolVersion | null;
    error?: string;
}

export const clientLoader = async ({ params }: { params: { versionId: string } }) => {
    const versionId = params.versionId;

    try {
        const response = await API.versions.getById(versionId);
        if (response.error) {
            return { version: null, error: response.error };
        }
        return { version: response.data };
    } catch (error) {
        console.error('加载版本信息出错:', error);
        return { version: null, error: String(error) };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    // Cast to the corrected type
    const version = data?.version as Omit<ProtocolVersion, 'config'> | undefined;
    return [
        { title: version ? `编辑版本: ${version.version}` : (data?.error ? '编辑错误' : '编辑版本 - 网关管理') },
        { name: "description", content: '修改版本信息' },
    ];
};

export default function EditVersion() {
    const initialData = useLoaderData<LoaderData>();
    const navigate = useNavigate();
    const { versionId } = useParams<{ versionId: string }>();

    // State for version metadata
    const [version, setVersion] = useState<Partial<Omit<ProtocolVersion, 'config'>>>({});
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState<string | null>(null);
    const [loaderError, setLoaderError] = useState<string | null>(null);

    useEffect(() => {
        if (initialData?.error) {
            setLoaderError(initialData.error);
            setVersion({});
        } else if (initialData?.version) {
            setVersion(initialData.version);
            setLoaderError(null);
        } else {
            setLoaderError("无法加载版本数据。");
            setVersion({});
        }
    }, [initialData]);

    const handleInputChange = (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const { name, value } = event.target;
        setVersion(prev => ({ ...prev, [name]: value }));
    };

    const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (!versionId) {
            setSubmitError("无效的版本 ID");
            return;
        }

        setIsSubmitting(true);
        setSubmitError(null);

        const updateData = {
            // Only include fields that can be updated via this endpoint
            version: version.version || '',
            description: version.description || '',
        };

        // Use API Client (assuming it sends only version/description for this call)
        // Verify that API.versions.update matches this payload structure
        const response = await API.versions.update(versionId, updateData);

        if (response.error) {
            console.error("更新版本出错:", response.error);
            setSubmitError(response.error);
        } else {
            // toast.success("版本信息已更新！"); // Consider adding toast notifications
            navigate(`/protocols/${response.data?.protocolId || ''}`); // Navigate back to protocol detail or version detail?
        }

        setIsSubmitting(false);
    };

    // 如果 Loader 返回错误，显示错误信息
    if (loaderError) {
        return (
            <Card className="text-center py-12">
                <CardHeader>
                    <CardTitle>加载错误</CardTitle>
                    <CardDescription>{loaderError}</CardDescription>
                </CardHeader>
                <CardContent>
                    <Button onClick={() => navigate(-1)} variant="outline">返回</Button>
                </CardContent>
            </Card>
        );
    }

    // 如果 version 不存在（即使没有 loaderError），也显示错误
    if (!version || Object.keys(version).length === 0 && !loaderError) {
        return (
            <Card className="text-center py-12">
                <CardHeader>
                    <CardTitle>未找到版本</CardTitle>
                    <CardDescription>无法加载版本信息。</CardDescription>
                </CardHeader>
                <CardContent>
                    <Button onClick={() => navigate(-1)} variant="outline">返回</Button>
                </CardContent>
            </Card>
        );
    }

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <Card>
                    <form onSubmit={handleSubmit}>
                        <CardHeader>
                            <CardTitle>编辑版本</CardTitle>
                            <CardDescription>协议 ID: {initialData?.version?.protocolId || '未知'} / 版本 ID: {versionId}</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            {submitError && (
                                <div className="text-red-600 bg-red-50 border border-red-200 rounded-md p-3 text-sm mb-4">
                                    {submitError}
                                </div>
                            )}
                            <div className="space-y-1">
                                <Label htmlFor="version" className="block mb-1.5">版本号</Label>
                                <Input
                                    id="version"
                                    name="version"
                                    value={version.version || ''}
                                    onChange={handleInputChange}
                                    required
                                    disabled={isSubmitting}
                                />
                            </div>
                            <div className="space-y-1">
                                <Label htmlFor="description" className="block mb-1.5">描述</Label>
                                <Textarea
                                    id="description"
                                    name="description"
                                    value={version.description || ''}
                                    onChange={handleInputChange}
                                    rows={3}
                                    disabled={isSubmitting}
                                />
                            </div>
                        </CardContent>
                        <CardFooter className="flex justify-end mt-6">
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting ? '正在保存...' : '保存更改'}
                            </Button>
                            <Button type="button" variant="outline" className="ml-2" onClick={() => navigate(-1)} disabled={isSubmitting}>
                                取消
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    );
}
