import { Link } from "react-router";
import type { Route, Protocol } from "../+types/protocols"; // 引入 Protocol 类型
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
// 移除 Badge import: import { Badge } from "@/components/ui/badge";
import { ArrowRightIcon, PlusCircledIcon, ExclamationTriangleIcon, ArchiveIcon } from "@radix-ui/react-icons"; // 使用 ArchiveIcon 作为占位符
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert"; // 引入 Alert
import { Skeleton } from "../components/ui/skeleton"; // 引入 Skeleton
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Input } from "../components/ui/input";
import { SearchIcon } from "lucide-react";
import { API, useApiGet } from "../api"; // 引入 API Client 和 Hook
import { useState } from "react"; // 仍然需要 useState

export const meta = ({ }: Route.MetaArgs): Array<Record<string, string>> => {
  return [
    { title: "首页 - 协议网关" },
    { name: "description", content: "查看和管理网关协议" },
  ];
};

export default function Home() {
  // 使用 useApiGet Hook 获取数据
  const { data: protocols, isLoading, error } = useApiGet(() => API.protocols.getAll(), []);

  const [search, setSearch] = useState('');
  const [view, setView] = useState('grid');

  // 过滤协议 - 使用 hook 返回的 protocols || [] 保证类型安全
  const filteredProtocols = (protocols || []).filter(protocol =>
    protocol.name.toLowerCase().includes(search.toLowerCase()) ||
    (protocol.description && protocol.description.toLowerCase().includes(search.toLowerCase()))
  );

  // --- 渲染逻辑 ---

  // 加载状态 - 现代骨架屏效果
  const renderLoadingState = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {[...Array(8)].map((_, i) => (
        <Card key={i} className="overflow-hidden border border-border/30 bg-card/20 rounded-lg shadow-sm">
          <CardHeader className="p-4">
            <Skeleton className="h-5 w-3/4 mb-2.5 bg-muted animate-pulse rounded" />
            <Skeleton className="h-3.5 w-1/2 bg-muted animate-pulse rounded" />
          </CardHeader>
          <CardContent className="p-4 pt-0 space-y-2">
            <Skeleton className="h-3 w-full bg-muted animate-pulse rounded" />
            <Skeleton className="h-3 w-5/6 bg-muted animate-pulse rounded" />
          </CardContent>
          <CardFooter className="px-4 py-3 border-t border-border/20">
            <Skeleton className="h-3 w-1/3 bg-muted animate-pulse rounded" />
          </CardFooter>
        </Card>
      ))}
    </div>
  );

  // 错误状态 - 使用 hook 的 error
  const renderErrorState = () => (
    <Alert variant="destructive" className="mt-6 border-destructive/30 bg-destructive/5">
      <ExclamationTriangleIcon className="h-5 w-5" />
      <AlertTitle className="text-base font-medium">加载错误</AlertTitle>
      <AlertDescription className="text-sm">{error}</AlertDescription>
    </Alert>
  );

  // 空数据状态 - 在 !isLoading && protocols?.length === 0 时判断
  const renderEmptyState = () => (
    <div className="text-center py-16 px-6 border-2 border-dashed border-border/30 rounded-xl mt-6 bg-card/5">
      <div className="w-20 h-20 bg-primary/10 rounded-full flex items-center justify-center mx-auto mb-6">
        <ArchiveIcon className="h-10 w-10 text-primary" />
      </div>
      <h3 className="text-xl font-semibold text-foreground mb-2">尚未创建任何协议</h3>
      <p className="text-muted-foreground mb-8 max-w-md mx-auto">
        点击下方按钮，开始创建您的第一个网关协议配置。
      </p>
      <Button asChild size="lg" className="bg-primary hover:bg-primary/90 shadow-md">
        <Link to="/protocols/new">
          <PlusCircledIcon className="mr-2 h-5 w-5" /> 添加第一个协议
        </Link>
      </Button>
    </div>
  );

  // 搜索空结果状态 - 在 !isLoading && protocols && filteredProtocols.length === 0 时判断
  const renderEmptySearchState = () => (
    <div className="text-center py-12 px-6 border border-border/30 rounded-xl mt-6 bg-card/5">
      <SearchIcon className="w-10 h-10 text-muted-foreground/40 mx-auto mb-4" />
      <h3 className="text-lg font-medium text-foreground mb-2">未找到匹配的协议</h3>
      <p className="text-muted-foreground mb-4 max-w-md mx-auto">
        没有找到与"{search}"匹配的协议。请尝试使用其他关键词搜索。
      </p>
      <Button
        variant="outline"
        onClick={() => setSearch('')}
        className="border-primary/30 text-primary hover:bg-primary/10"
      >
        清除搜索
      </Button>
    </div>
  );

  // 正常数据状态 - 使用 hook 返回的 filteredProtocols
  const renderProtocolsGrid = () => (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 items-start">
      {filteredProtocols.map((protocol) => (
        <Link key={protocol.id} to={`/protocols/${protocol.id}`} className="block hover:no-underline">
          <Card
            className="group relative flex flex-col h-full bg-card rounded-xl shadow hover:shadow-lg transition-shadow duration-200 overflow-hidden"
          >
            <div className="py-3 px-4 flex flex-col">
              <div className="flex items-center mb-2">
                <ArchiveIcon className="h-4 w-4 mr-2 text-gray-400 dark:text-gray-500 flex-shrink-0" />
                <h3 className="font-semibold text-lg truncate text-foreground">{protocol.name}</h3>
              </div>
              <p className="text-sm text-muted-foreground mb-2 line-clamp-2 leading-snug">
                {protocol.description || <span className="italic">暂无描述</span>}
              </p>
              <div className="text-xs text-gray-400 dark:text-gray-500 mb-2 truncate">
                最后更新: {protocol.updatedAt ? new Date(protocol.updatedAt).toLocaleString() : '未知'}
              </div>
            </div>
          </Card>
        </Link>
      ))}
    </div>
  );

  // 列表视图 - 使用 hook 返回的 filteredProtocols
  const renderProtocolsList = () => (
    <div className="space-y-3">
      {filteredProtocols.map((protocol) => (
        <Card
          key={protocol.id}
          className="group flex flex-row items-center overflow-hidden bg-card border border-border/50 hover:border-primary/50 rounded-lg shadow-sm hover:shadow transition-all duration-200"
        >
          <div className="w-1.5 self-stretch bg-primary/0 group-hover:bg-primary/80 transition-colors"></div>
          <div className="flex-grow p-4 sm:p-5">
            <h3 className="text-base font-medium">{protocol.name}</h3>
            <p className="text-sm text-muted-foreground line-clamp-1 mt-1">
              {protocol.description || <span className="italic text-muted-foreground/70">暂无描述</span>}
            </p>
            <div className="inline-flex items-center px-2 py-0.5 mt-2 rounded-full text-xs font-medium bg-primary/10 text-primary">
              <span className="truncate font-mono">ID: {protocol.id}</span>
            </div>
          </div>
          <div className="pr-4">
            <Button
              asChild
              variant="ghost"
              size="sm"
              className="hover:bg-primary/10 hover:text-primary ripple"
            >
              <Link to={`/protocols/${protocol.id}`}>
                <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-1" />
              </Link>
            </Button>
          </div>
        </Card>
      ))}
    </div>
  );

  // 头部搜索和筛选区 - 使用 protocols?.length 判断
  const renderHeader = () => (
    <div className="mb-6">
      {!isLoading && !error && protocols && protocols.length > 0 && (
        <div className="flex flex-col sm:flex-row gap-4 items-center justify-between mb-5">
          <div className="relative w-full sm:w-64">
            <SearchIcon className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder="搜索协议..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 bg-background border-border/50 focus:border-primary/50"
            />
          </div>
          <div className="flex items-center gap-4">
            <Tabs value={view} onValueChange={setView} className="w-full sm:w-auto">
              <TabsList className="grid w-full grid-cols-2 sm:w-[180px]">
                <TabsTrigger value="grid" className="text-xs">卡片视图</TabsTrigger>
                <TabsTrigger value="list" className="text-xs">列表视图</TabsTrigger>
              </TabsList>
            </Tabs>
            <Button variant="default" asChild className="bg-primary hover:bg-primary/90 shadow-sm">
              <Link to="/protocols/new">
                <PlusCircledIcon className="mr-2 h-4 w-4" />
                新建协议
              </Link>
            </Button>
          </div>
        </div>
      )}
    </div>
  );

  return (
    <div className="p-6 mx-auto w-full max-w-7xl">
      <div className="space-y-6">
        {renderHeader()}

        <div className="p-1">
          {isLoading
            ? renderLoadingState()
            : error
              ? renderErrorState()
              : protocols && protocols.length === 0
                ? renderEmptyState()
                : protocols && filteredProtocols.length === 0
                  ? renderEmptySearchState()
                  : view === 'grid'
                    ? renderProtocolsGrid()
                    : renderProtocolsList()
          }
        </div>
      </div>
    </div>
  );
}

// Placeholder Icon component if CubeIcon is not available directly
function CubeIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
      <path d="m3.3 7 8.7 5 8.7-5" />
      <path d="M12 22V12" />
    </svg>
  )
}
