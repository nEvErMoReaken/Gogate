import { useState } from "react";
import { useNavigate } from "react-router";
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

export const meta = () => {
    return [
        { title: "创建新协议 - 网关管理" },
        { name: "description", content: "创建新的协议配置" },
    ];
};

export default function NewProtocol() {
    const navigate = useNavigate();
    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSubmitting(true);

        if (!name.trim()) {
            toast.error("协议名称不能为空");
            setIsSubmitting(false);
            return;
        }

        try {
            const response = await fetch('/api/v1/protocols', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name,
                    description,
                    config: {
                        parser: { type: '', config: {} },
                        connector: { type: '', config: {} },
                        dispatcher: { repeat_data_filter: [] },
                        strategy: [],
                        version: '1.0.0',
                        log: {
                            log_path: '/var/log/gateway',
                            max_size: 10,
                            max_backups: 5,
                            max_age: 30,
                            compress: true,
                            level: 'info',
                            buffer_size: 256,
                            flush_interval_secs: 5,
                        },
                    }
                }),
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const result = await response.json();
            toast.success("协议创建成功");
            navigate(`/protocols/${result.id}`);
        } catch (error) {
            console.error("创建协议失败:", error);
            toast.error("创建协议失败，请稍后重试");
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <div className="flex justify-between items-start">
                    <div>
                        <h1 className="text-2xl font-bold">创建新协议</h1>
                        <p className="text-muted-foreground">创建新的协议配置</p>
                    </div>
                    <Button asChild variant="outline" size="sm">
                        <Link to="/">
                            <ArrowLeftIcon className="mr-2 h-4 w-4" /> 返回列表
                        </Link>
                    </Button>
                </div>

                <Card>
                    <form onSubmit={handleSubmit}>
                        <CardHeader>
                            <CardTitle>协议基本信息</CardTitle>
                            <CardDescription>请输入新协议的基本信息</CardDescription>
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
                                onClick={() => navigate("/")}
                            >
                                取消
                            </Button>
                            <Button type="submit" disabled={isSubmitting}>
                                {isSubmitting ? "创建中..." : "创建协议"}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    );
}
