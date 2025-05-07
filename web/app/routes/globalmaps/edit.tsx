import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FormField, FormItem, FormLabel, FormControl, FormMessage, Form } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { Link, useLoaderData, useNavigate, useParams } from "react-router";
import { API } from "../../api";
import { useState, useEffect } from "react";
import { toast } from "sonner";
import type { GlobalMap, Route } from "../../+types/protocols";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

// JSON编辑器
import { useMonaco } from "@monaco-editor/react";
import Editor from "@monaco-editor/react";

// 定义表单验证规则
const formSchema = z.object({
    name: z.string().min(1, { message: "名称不能为空" }),
    description: z.string().optional(),
    content: z.any().optional()
});

type FormValues = z.infer<typeof formSchema>;

// 客户端加载器
export const clientLoader = async ({ params }: { params: { globalMapId: string } }) => {
    const globalMapId = params.globalMapId;

    try {
        const response = await API.globalmaps.getById(globalMapId);
        if (response.error) {
            return { error: response.error };
        }
        return { globalMap: response.data };
    } catch (error) {
        console.error('加载全局映射详情出错:', error);
        return { error: '获取全局映射详情失败' };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const globalMap = data?.globalMap as GlobalMap;
    return [
        { title: globalMap ? `编辑 ${globalMap.name} - 全局映射` : '编辑全局映射 - 网关管理' },
        { name: "description", content: '编辑全局映射信息' },
    ];
};

export default function EditGlobalMap() {
    const { globalMapId } = useParams<{ globalMapId: string }>();
    const data = useLoaderData<typeof clientLoader>();
    const globalMap = data?.globalMap as GlobalMap;
    const error = data?.error;
    const navigate = useNavigate();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [activeTab, setActiveTab] = useState("basic");
    const [jsonContent, setJsonContent] = useState<string>("");
    const monaco = useMonaco();

    // 设置默认表单值
    const defaultValues: FormValues = {
        name: globalMap?.name || "",
        description: globalMap?.description || "",
        content: globalMap?.content || {}
    };

    // 初始化表单
    const form = useForm<FormValues>({
        resolver: zodResolver(formSchema),
        defaultValues,
    });

    // 当globalMap加载完成后，更新表单值和JSON编辑器内容
    useEffect(() => {
        if (globalMap) {
            form.reset({
                name: globalMap.name,
                description: globalMap.description || "",
                content: globalMap.content || {}
            });
            setJsonContent(JSON.stringify(globalMap.content || {}, null, 2));
        }
    }, [globalMap, form]);

    // JSON编辑器内容变更处理
    const handleJsonChange = (value: string | undefined) => {
        if (!value) return;
        setJsonContent(value);
        try {
            const contentObj = JSON.parse(value);
            form.setValue("content", contentObj);
        } catch (error) {
            // JSON格式错误，不更新表单值，但保留编辑器内容
            console.error("JSON解析错误:", error);
        }
    };

    // 表单提交处理
    const onSubmit = async (values: FormValues) => {
        if (!globalMapId) {
            toast.error("无法获取全局映射ID");
            return;
        }

        // 尝试解析JSON编辑器内容
        if (activeTab === "json") {
            try {
                const contentObj = JSON.parse(jsonContent);
                values.content = contentObj;
            } catch (error) {
                toast.error("JSON格式错误，请检查后重试");
                return;
            }
        }

        setIsSubmitting(true);

        try {
            // 调用API更新全局映射
            const response = await API.globalmaps.update(globalMapId, {
                name: values.name,
                description: values.description,
                content: values.content
            });

            if (response.error) {
                toast.error(`更新全局映射失败: ${response.error}`);
            } else {
                toast.success("全局映射更新成功");
                // 更新成功后返回协议详情页
                if (globalMap?.protocolId) {
                    navigate(`/protocols/${globalMap.protocolId}`);
                } else {
                    navigate("/protocols");
                }
            }
        } catch (error) {
            console.error("更新全局映射时出错:", error);
            toast.error("更新全局映射时发生错误");
        } finally {
            setIsSubmitting(false);
        }
    };

    // 如果出现错误
    if (error) {
        return (
            <div className="p-6 mx-auto w-full max-w-3xl">
                <Card className="text-center py-12">
                    <CardHeader>
                        <CardTitle>加载失败</CardTitle>
                        <CardDescription>{error}</CardDescription>
                    </CardHeader>
                    <CardContent>
                        <Button asChild variant="outline">
                            <Link to="/protocols">返回协议列表</Link>
                        </Button>
                    </CardContent>
                </Card>
            </div>
        );
    }

    // 如果数据尚未加载
    if (!globalMap) {
        return (
            <div className="p-6 mx-auto w-full max-w-3xl">
                <div className="space-y-6">
                    <Skeleton className="h-10 w-1/3" />
                    <Card>
                        <CardHeader>
                            <Skeleton className="h-7 w-1/4 mb-1" />
                            <Skeleton className="h-5 w-1/2" />
                        </CardHeader>
                        <CardContent className="space-y-6">
                            {[...Array(3)].map((_, i) => (
                                <div key={i} className="space-y-2">
                                    <Skeleton className="h-5 w-1/4" />
                                    <Skeleton className="h-10 w-full" />
                                </div>
                            ))}
                            <div className="flex justify-end">
                                <Skeleton className="h-10 w-1/4" />
                            </div>
                        </CardContent>
                    </Card>
                </div>
            </div>
        );
    }

    return (
        <div className="p-6 mx-auto w-full max-w-3xl">
            <div className="space-y-6">
                <div className="flex items-center">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 px-2 mr-2"
                        asChild
                    >
                        <Link to={globalMap.protocolId ? `/protocols/${globalMap.protocolId}` : "/protocols"}>
                            <ArrowLeftIcon className="mr-1 h-4 w-4" /> 返回
                        </Link>
                    </Button>
                    <h1 className="text-2xl font-bold">编辑全局映射</h1>
                </div>

                <Card>
                    <CardHeader>
                        <CardTitle>全局映射信息</CardTitle>
                        <CardDescription>
                            编辑全局映射信息及其内容
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <Tabs defaultValue="basic" value={activeTab} onValueChange={setActiveTab}>
                            <TabsList>
                                <TabsTrigger value="basic">基本信息</TabsTrigger>
                                <TabsTrigger value="json">JSON编辑器</TabsTrigger>
                            </TabsList>

                            <Form {...form}>
                                <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6 mt-4">
                                    <TabsContent value="basic">
                                        <div className="space-y-6">
                                            <FormField
                                                control={form.control}
                                                name="name"
                                                render={({ field }) => (
                                                    <FormItem>
                                                        <FormLabel>名称</FormLabel>
                                                        <FormControl>
                                                            <Input placeholder="全局映射名称" {...field} />
                                                        </FormControl>
                                                        <FormMessage />
                                                    </FormItem>
                                                )}
                                            />

                                            <FormField
                                                control={form.control}
                                                name="description"
                                                render={({ field }) => (
                                                    <FormItem>
                                                        <FormLabel>描述 (可选)</FormLabel>
                                                        <FormControl>
                                                            <Textarea
                                                                placeholder="全局映射的描述信息"
                                                                {...field}
                                                                value={field.value || ''}
                                                            />
                                                        </FormControl>
                                                        <FormMessage />
                                                    </FormItem>
                                                )}
                                            />
                                        </div>
                                    </TabsContent>

                                    <TabsContent value="json">
                                        <div className="space-y-2">
                                            <FormLabel>JSON内容</FormLabel>
                                            <div className="border rounded-md overflow-hidden">
                                                <Editor
                                                    height="400px"
                                                    defaultLanguage="json"
                                                    value={jsonContent}
                                                    onChange={handleJsonChange}
                                                    options={{
                                                        minimap: { enabled: false },
                                                        scrollBeyondLastLine: false,
                                                        automaticLayout: true
                                                    }}
                                                />
                                            </div>
                                            <p className="text-sm text-muted-foreground">
                                                编辑全局映射的JSON内容。请确保是有效的JSON格式。
                                            </p>
                                        </div>
                                    </TabsContent>

                                    <div className="flex justify-end">
                                        <Button type="submit" disabled={isSubmitting}>
                                            {isSubmitting ? "保存中..." : "保存修改"}
                                        </Button>
                                    </div>
                                </form>
                            </Form>
                        </Tabs>
                    </CardContent>
                </Card>
            </div>
        </div>
    );
}
