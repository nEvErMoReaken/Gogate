import React, { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";
import { GatewayAPI } from "~/lib/api";
import type { GatewayConfig } from "~/lib/types";
import { formatDate } from "~/lib/utils";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "~/components/ui/table";
import { Button } from "~/components/ui/button";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "~/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import { Skeleton } from "~/components/ui/skeleton";

export default function Gateways() {
  const [gateways, setGateways] = useState<GatewayConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedGateway, setSelectedGateway] = useState<string | null>(null);
  const navigate = useNavigate();

  // 加载网关配置列表
  const loadGateways = async () => {
    try {
      setLoading(true);
      const data = await GatewayAPI.getAll();
      setGateways(data);
    } catch (error) {
      toast.error("加载网关配置失败");
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 删除网关配置
  const deleteGateway = async () => {
    if (!selectedGateway) return;

    try {
      await GatewayAPI.delete(selectedGateway);
      toast.success("网关配置已删除");
      setSelectedGateway(null);
      // 重新加载列表
      loadGateways();
    } catch (error) {
      toast.error("删除网关配置失败");
      console.error(error);
    }
  };

  // 组件挂载时加载数据
  useEffect(() => {
    loadGateways();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold tracking-tight">网关配置</h2>
          <p className="text-muted-foreground">
            管理所有网关配置，包括基础配置和协议设置
          </p>
        </div>
        <Button onClick={() => navigate("/gateways/new")}>
          <Plus className="mr-2 h-4 w-4" />
          新建配置
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>网关配置列表</CardTitle>
          <CardDescription>所有可用的网关配置</CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            // 加载状态显示骨架屏
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>监听地址</TableHead>
                  <TableHead>端口</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {gateways.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center">
                      暂无数据
                    </TableCell>
                  </TableRow>
                ) : (
                  gateways.map((gateway) => (
                    <TableRow key={gateway.id}>
                      <TableCell className="font-medium">
                        <Link to={`/gateways/${gateway.id}`} className="hover:underline">
                          {gateway.name}
                        </Link>
                      </TableCell>
                      <TableCell>{gateway.listenAddress}</TableCell>
                      <TableCell>{gateway.port}</TableCell>
                      <TableCell>
                        <span
                          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                            gateway.enabled
                              ? "bg-green-100 text-green-800"
                              : "bg-gray-100 text-gray-800"
                          }`}
                        >
                          {gateway.enabled ? "启用" : "禁用"}
                        </span>
                      </TableCell>
                      <TableCell>{formatDate(gateway.createdAt)}</TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end space-x-2">
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => navigate(`/gateways/${gateway.id}`)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Dialog>
                            <DialogTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => setSelectedGateway(gateway.id)}
                              >
                                <Trash2 className="h-4 w-4 text-red-500" />
                              </Button>
                            </DialogTrigger>
                            <DialogContent>
                              <DialogHeader>
                                <DialogTitle>确认删除</DialogTitle>
                                <DialogDescription>
                                  您确定要删除网关配置 "{gateway.name}" 吗？此操作无法撤销。
                                </DialogDescription>
                              </DialogHeader>
                              <DialogFooter>
                                <DialogClose asChild>
                                  <Button variant="outline">取消</Button>
                                </DialogClose>
                                <Button variant="destructive" onClick={deleteGateway}>
                                  删除
                                </Button>
                              </DialogFooter>
                            </DialogContent>
                          </Dialog>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
} 