import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FormField, FormItem, FormLabel, FormControl, FormMessage, Form } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useNavigate, useParams } from "react-router";
import { API } from "../../../api";
import { useState, useEffect, useRef } from "react";
import { toast } from "sonner";
import type { Route } from "../../../+types/protocols";
import { ArrowLeftIcon } from "@radix-ui/react-icons";
import { Link } from "react-router";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

// JSON编辑器
import { useMonaco } from "@monaco-editor/react";
import Editor from "@monaco-editor/react";

// 定义表单验证规则
const formSchema = z.object({
    name: z.string().min(1, { message: "名称不能为空" }),
    description: z.string().optional(),
    // 初始时content为空对象
    content: z.any().optional()
});

type FormValues = z.infer<typeof formSchema>;

export const meta = ({ params }: Route.MetaArgs): Array<Record<string, string>> => {
    return [
        { title: "新建全局映射 - 网关管理" },
        { name: "description", content: "创建新的全局映射" }
    ];
};

export default function NewGlobalMap() {
    const navigate = useNavigate();
    const { protocolId } = useParams<{ protocolId: string }>();
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [activeTab, setActiveTab] = useState("basic");
    const [jsonContent, setJsonContent] = useState("{}");
    const monaco = useMonaco();
    const editorMounted = useRef(false);

    // 设置默认表单值
    const defaultValues: FormValues = {
        name: "",
        description: "",
        content: {}
    };

    // 初始化表单
    const form = useForm<FormValues>({
        resolver: zodResolver(formSchema),
        defaultValues,
    });

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

    // 编辑器挂载状态管理
    const handleEditorDidMount = () => {
        editorMounted.current = true;
    };

    // 处理标签切换，确保编辑器已挂载
    useEffect(() => {
        // 避免在非JSON标签页尝试初始化编辑器
        if (activeTab !== "json") {
            return;
        }
    }, [activeTab]);

    // 表单提交处理
    const onSubmit = async (values: FormValues) => {
        if (!protocolId) {
            toast.error("无法获取协议ID");
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
            // 调用API创建全局映射
            const response = await API.protocols.globalmaps.create(protocolId, {
                name: values.name,
                description: values.description,
                content: values.content || {}
            });

            if (response.error) {
                toast.error(`创建全局映射失败: ${response.error}`);
            } else {
                toast.success("全局映射创建成功");
                // 创建成功后返回协议详情页
                navigate(`/protocols/${protocolId}`);
            }
        } catch (error) {
            console.error("创建全局映射时出错:", error);
            toast.error("创建全局映射时发生错误");
        } finally {
            setIsSubmitting(false);
        }
    };

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
                        <Link to={`/protocols/${protocolId}`}>
                            <ArrowLeftIcon className="mr-1 h-4 w-4" /> 返回
                        </Link>
                    </Button>
                    <h1 className="text-2xl font-bold">新建全局映射</h1>
                </div>

                <Card>
                    <CardHeader>
                        <CardTitle>全局映射信息</CardTitle>
                        <CardDescription>
                            创建一个新的全局映射，用于存储全局配置数据。
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
                                                {activeTab === "json" && (
                                                    <Editor
                                                        height="400px"
                                                        defaultLanguage="json"
                                                        value={jsonContent}
                                                        onChange={handleJsonChange}
                                                        onMount={handleEditorDidMount}
                                                        options={{
                                                            minimap: { enabled: false },
                                                            scrollBeyondLastLine: false,
                                                            automaticLayout: true
                                                        }}
                                                        loading={<div className="p-4 text-center">加载编辑器...</div>}
                                                    />
                                                )}
                                            </div>
                                            <p className="text-sm text-muted-foreground">
                                                编辑全局映射的JSON内容。请确保是有效的JSON格式。
                                            </p>
                                        </div>
                                    </TabsContent>

                                    <div className="flex justify-end">
                                        <Button type="submit" disabled={isSubmitting}>
                                            {isSubmitting ? "创建中..." : "创建全局映射"}
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
