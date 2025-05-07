import React from "react";
import { useNavigate } from "react-router";
import { GatewayAPI } from "~/lib/api";
import type { GatewayConfig } from "~/lib/types";
import { Button } from "~/components/ui/button";
import { ArrowLeft } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { toast } from "sonner";
import { Input } from "~/components/ui/input";
import { useForm } from "react-hook-form";
import type { SubmitHandler } from "react-hook-form";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { Checkbox } from "~/components/ui/checkbox";

// 网关配置表单类型
type GatewayFormValues = Omit<GatewayConfig, "id" | "createdAt" | "updatedAt" | "protocol">;

export default function NewGateway() {
  const navigate = useNavigate();
  const form = useForm<GatewayFormValues>({
    defaultValues: {
      name: "",
      description: "",
      listenAddress: "0.0.0.0",
      port: 8080,
      enabled: true,
    },
  });

  // 提交表单
  const onSubmit: SubmitHandler<GatewayFormValues> = async (data) => {
    try {
      const gateway = await GatewayAPI.create(data);
      toast.success("网关配置创建成功");
      navigate(`/gateways/${gateway.id}`);
    } catch (error) {
      toast.error("创建网关配置失败");
      console.error(error);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-2">
        <Button variant="ghost" size="icon" onClick={() => navigate("/gateways")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <h2 className="text-3xl font-bold tracking-tight">新增网关配置</h2>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>基础配置</CardTitle>
          <CardDescription>设置网关的基本参数</CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>网关名称</FormLabel>
                    <FormControl>
                      <Input placeholder="输入网关名称" {...field} />
                    </FormControl>
                    <FormDescription>网关的唯一标识名称</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>描述</FormLabel>
                    <FormControl>
                      <Input placeholder="输入网关描述" {...field} />
                    </FormControl>
                    <FormDescription>对网关用途的简要描述</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="grid gap-6 md:grid-cols-2">
                <FormField
                  control={form.control}
                  name="listenAddress"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>监听地址</FormLabel>
                      <FormControl>
                        <Input placeholder="0.0.0.0" {...field} />
                      </FormControl>
                      <FormDescription>网关监听的IP地址</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="port"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>端口</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          placeholder="8080"
                          {...field}
                          onChange={(e) => field.onChange(Number(e.target.value))}
                        />
                      </FormControl>
                      <FormDescription>网关监听的端口号</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="enabled"
                render={({ field }) => (
                  <FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <div className="space-y-1 leading-none">
                      <FormLabel>启用状态</FormLabel>
                      <FormDescription>
                        是否立即启用此网关配置
                      </FormDescription>
                    </div>
                  </FormItem>
                )}
              />

              <div className="flex justify-end space-x-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => navigate("/gateways")}
                >
                  取消
                </Button>
                <Button type="submit">创建网关</Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
} 