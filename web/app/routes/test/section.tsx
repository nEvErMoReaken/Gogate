import { Form } from "react-router";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { useState } from "react";
import { API } from "../../api"; // 引入 API Client
// import type { Route } from "../../+types/protocols"; // Route type not used here

// ... meta function removed if Route is not used ...
/*
export const meta = ({ }: Route.MetaArgs): Array<Record<string, string>> => {
    return [
        { title: "测试部分接口" },
        { name: "description", content: "测试后端特定部分接口" },
    ];
};
*/

export default function TestSection() {
    const [text, setText] = useState('');
    const [result, setResult] = useState<any>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        setIsLoading(true);
        setError(null);
        setResult(null);

        // 使用 API Client
        const response = await API.test.section({ text });

        if (response.error) {
            console.error("测试接口出错:", response.error);
            setError(response.error);
        } else {
            setResult(response.data);
        }

        setIsLoading(false);

        /* 移除旧的 fetch 逻辑
        try {
            const response = await fetch("/api/v1/test/section", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ text }),
            });

            if (!response.ok) {
                let errorMessage = "请求测试接口失败";
                try {
                    const errData = await response.json();
                    errorMessage = errData.message || errData.error || errorMessage;
                } catch (e) { }
                throw new Error(errorMessage);
            }

            const data = await response.json();
            setResult(data);

        } catch (err: any) {
            console.error("测试接口出错:", err);
            setError(err.message || "请求测试接口时发生错误");
        } finally {
            setIsLoading(false);
        }
        */
    };

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <h1 className="text-xl font-semibold">测试部分接口</h1>
                <Form onSubmit={handleSubmit} className="space-y-4">
                    <Textarea
                        placeholder="输入要发送到测试接口的文本"
                        value={text}
                        onChange={(e) => setText(e.target.value)}
                        rows={5}
                    />
                    <Button type="submit" disabled={isLoading}>
                        {isLoading ? "发送中..." : "发送请求"}
                    </Button>
                </Form>

                {error && (
                    <div className="text-red-500 p-3 bg-red-100 border border-red-300 rounded">
                        错误: {error}
                    </div>
                )}

                {result && (
                    <div>
                        <h2 className="font-medium">接口响应:</h2>
                        <pre className="mt-2 p-3 bg-muted rounded overflow-auto text-sm">
                            {JSON.stringify(result, null, 2)}
                        </pre>
                    </div>
                )}
            </div>
        </div>
    );
}
