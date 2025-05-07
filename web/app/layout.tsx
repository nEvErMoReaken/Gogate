import { NavLink, Outlet } from "react-router";
import { Toaster } from "./components/ui/sonner";
import { Icon } from "./components/Icon";
import { cn } from "~/lib/utils";
import { Button } from "./components/ui/button";
import { useState } from "react";

export default function Layout() {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <div className="grid h-screen grid-cols-[auto_1fr]">
      <aside
        className={cn(
          "h-full border-r transition-all",
          collapsed ? "w-16" : "w-64"
        )}
      >
        <div className="flex h-16 items-center border-b px-4">
          {!collapsed && <h1 className="text-lg font-semibold">Gagate</h1>}
          <Button
            variant="ghost"
            size="icon"
            className={cn("ml-auto", collapsed && "mx-auto")}
            onClick={() => setCollapsed(!collapsed)}
          >
            <Icon name={collapsed ? "panel-right" : "panel-left"} />
          </Button>
        </div>
        <div className="py-2">
          <NavLink
            to="/"
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2 px-4 py-2 transition-colors hover:bg-muted/50",
                isActive && "bg-muted",
                collapsed && "justify-center px-0"
              )
            }
            end
          >
            <Icon name="home" />
            {!collapsed && <span>首页</span>}
          </NavLink>
          <NavLink
            to="/gateways"
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2 px-4 py-2 transition-colors hover:bg-muted/50",
                isActive && "bg-muted",
                collapsed && "justify-center px-0"
              )
            }
          >
            <Icon name="router" />
            {!collapsed && <span>网关配置</span>}
          </NavLink>
          <NavLink
            to="/protocols"
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2 px-4 py-2 transition-colors hover:bg-muted/50",
                isActive && "bg-muted",
                collapsed && "justify-center px-0"
              )
            }
          >
            <Icon name="layers" />
            {!collapsed && <span>协议管理</span>}
          </NavLink>
          <NavLink
            to="/settings"
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2 px-4 py-2 transition-colors hover:bg-muted/50",
                isActive && "bg-muted",
                collapsed && "justify-center px-0"
              )
            }
          >
            <Icon name="settings" />
            {!collapsed && <span>系统设置</span>}
          </NavLink>
        </div>
      </aside>
      <main className="flex flex-col">
        <header className="flex h-16 items-center justify-between border-b px-6">
          <h1 className="text-lg font-semibold">
            <Outlet context="title" />
          </h1>
          <div className="flex items-center gap-4"></div>
        </header>
        <div className="flex-1 overflow-auto p-6">
          <Outlet />
        </div>
      </main>
      <Toaster position="top-right" />
    </div>
  );
}
