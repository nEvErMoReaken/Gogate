import { Outlet, Link } from "react-router";
import { Button } from "@/components/ui/button";
import { PlusCircledIcon, MixerHorizontalIcon } from "@radix-ui/react-icons";

export default function DashboardLayout() {
    return (
        <div className="relative min-h-screen flex flex-col bg-muted/40">
            {/* 顶部导航栏 - 添加背景和调整 */}
            <header className="sticky top-0 z-10 flex h-14 items-center gap-4 border-b bg-background px-6">
                <Link to="/" className="flex items-center gap-2 font-semibold text-lg mr-4">
                    <MixerHorizontalIcon className="h-5 w-5" />
                    <span>协议网关</span>
                </Link>
                {/* Navigation links can go here if needed */}
                <div className="ml-auto flex items-center gap-4">
                </div>
            </header>

            {/* 主内容区域 - 修改 main: 移除 flex-1 和 relative, overflow-hidden */}
            <main>
                <Outlet />
            </main>
        </div>
    );
}
