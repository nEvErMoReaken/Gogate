import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useLoaderData } from 'react-router';
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { PlusCircledIcon, Cross2Icon, TrashIcon, ArrowLeftIcon, ShuffleIcon } from "@radix-ui/react-icons";
import { Form } from "@/components/ui/form";
import {
    useForm,
    Controller,
    type SubmitHandler
} from "react-hook-form";
import type { Protocol, GatewayConfig, Route } from "../../+types/protocols";
import { API } from "../../api";
import YAML from 'js-yaml';

// --- TypeScript Interfaces (Mirroring Go structs) ---

interface GoLogConfig {
    log_path: string;
    max_size: number;
    max_backups: number;
    max_age: number;
    compress: boolean;
    level: string;
    buffer_size: number;
    flush_interval_secs: number;
}

interface GoStrategyConfig {
    type: string;
    enable: boolean;
    filter: string[];
    config: Record<string, any>; // map[string]interface{}
}

interface GoParserConfig {
    config: Record<string, any>; // map[string]interface{}
}

interface GoConnectorConfig {
    type: string;
    config: Record<string, any>; // map[string]interface{}
}

// Interface GatewayConfig is now imported

// Default empty state matching the structure
const initialConfigState: GatewayConfig = {
    parser: { config: {} },
    connector: { type: '', config: {} },
    strategy: [],
    version: '',
    log: {
        log_path: './gateway.log',
        max_size: 512,
        max_backups: 1000,
        max_age: 365,
        compress: true,
        level: 'info',
        buffer_size: 0,
        flush_interval_secs: 0,
    },
};

// Define LoaderData type based on API response
interface LoaderData {
    protocol: Protocol | null;
    error?: string;
}

// Use API in clientLoader
export const clientLoader = async ({ params }: { params: { protocolId: string } }): Promise<LoaderData> => {
    const protocolId = params.protocolId;
    if (!protocolId) {
        return { protocol: null, error: 'Missing protocolId' };
    }

    try {
        const response = await API.protocols.getById(protocolId);
        if (response.error || !response.data?.protocol) {
            return { protocol: null, error: response.error || '加载协议失败' };
        }
        // API returns { protocol: Protocol }, so return that structure
        return { protocol: response.data.protocol };
    } catch (error) {
        console.error('加载协议详情出错:', error);
        return { protocol: null, error: String(error) };
    }
};

export const meta = ({ data }: Route.MetaArgs): Array<Record<string, string>> => {
    const protocol = data?.protocol as Protocol | null;
    return [
        { title: protocol ? `${protocol.name} - Configure Protocol` : 'Configure Protocol - Gateway Management' },
        { name: "description", content: protocol?.description || 'Configure protocol settings' },
    ];
};

// --- Helper Function to Clean Empty Objects/Arrays ---
function cleanEmptyObjectsAndArrays(data: any): any {
    if (Array.isArray(data)) {
        // Clean array elements recursively, filter out undefined results
        const cleanedArray = data.map(cleanEmptyObjectsAndArrays).filter(item => item !== undefined);
        // Return undefined if the array becomes empty after cleaning
        return cleanedArray.length > 0 ? cleanedArray : undefined;
    } else if (typeof data === 'object' && data !== null) {
        const cleanedObject: Record<string, any> = {};
        let hasContent = false;
        for (const key in data) {
            if (Object.prototype.hasOwnProperty.call(data, key)) {
                const value = data[key];
                const cleanedValue = cleanEmptyObjectsAndArrays(value);

                // Keep the key only if the cleaned value is not undefined
                if (cleanedValue !== undefined) {
                    cleanedObject[key] = cleanedValue;
                    hasContent = true;
                }
            }
        }
        // Return undefined if the object becomes empty after cleaning
        return hasContent ? cleanedObject : undefined;
    }
    // Return primitive values (and non-empty arrays/objects from recursive calls)
    return data;
}

export default function ProtocolEditConfig() {
    const { protocolId } = useParams<{ protocolId: string }>();
    const navigate = useNavigate();
    const loaderData = useLoaderData<LoaderData>();
    const [config, setConfig] = useState<GatewayConfig>(initialConfigState);
    const [protocolName, setProtocolName] = useState<string>('');
    const [isLoading, setIsLoading] = useState<boolean>(true);
    const [isSaving, setIsSaving] = useState<boolean>(false);
    const [activeTab, setActiveTab] = useState<string>("parser");
    const [viewMode, setViewMode] = useState<'form' | 'yaml'>('form');
    const [yamlString, setYamlString] = useState<string>('');
    const [rawSubYaml, setRawSubYaml] = useState<Record<string, string>>({});

    const form = useForm<GatewayConfig>({
        defaultValues: initialConfigState,
        mode: 'onChange',
    });

    const mergeWithDefaults = useCallback((parsed: any): GatewayConfig => {
        if (typeof parsed !== 'object' || parsed === null) {
            console.warn("Parsed data is not an object, returning initial state.");
            return JSON.parse(JSON.stringify(initialConfigState));
        }
        const merged: GatewayConfig = {
            parser: {
                config: (parsed.parser && typeof parsed.parser.config === 'object' && parsed.parser.config !== null)
                    ? parsed.parser.config
                    : (initialConfigState.parser.config || {})
            },
            connector: { ...(initialConfigState.connector), ...(parsed.connector || {}) },
            strategy: Array.isArray(parsed.strategy) ? parsed.strategy.map((s: any) => ({
                type: s.type || '',
                enable: typeof s.enable === 'boolean' ? s.enable : true,
                filter: Array.isArray(s.filter) && s.filter.every((f: any) => typeof f === 'string')
                    ? s.filter
                    : (Array.isArray(s.filter) ? s.filter.map(String) : []),
                config: typeof s.config === 'object' && s.config !== null ? s.config : {},
            })) : [],
            version: typeof parsed.version === 'string' ? parsed.version : initialConfigState.version,
            log: { ...(initialConfigState.log), ...(parsed.log || {}) },
        };
        if (typeof merged.parser.config !== 'object' || merged.parser.config === null) merged.parser.config = {};
        if (typeof merged.connector.config !== 'object' || merged.connector.config === null) merged.connector.config = {};
        return merged;
    }, []);

    // 帮助指南函数，根据字段路径返回对应的帮助文本
    const getHelpText = (path: string): string => {
        if (path.includes('strategy') && path.includes('config')) {
            return "策略配置需要有效的YAML格式，使用键值对结构。例如：\nfilters:\n  - name: test\n    value: 123";
        }
        if (path === 'parser.config') {
            return "解析器配置，必须是有效的YAML格式。根据解析器类型填写适当的配置参数。";
        }
        if (path === 'connector.config') {
            return "连接器配置，必须是有效的YAML格式。根据连接器类型填写适当的配置参数。";
        }
        return "";
    };

    useEffect(() => {
        setIsLoading(true);
        if (loaderData?.protocol) {
            const fetchedProtocol = loaderData.protocol;
            setProtocolName(fetchedProtocol.name || 'Unknown Protocol');
            const initialMergedConfig = mergeWithDefaults(fetchedProtocol.config);
            setConfig(initialMergedConfig);
            form.reset(initialMergedConfig);
            setRawSubYaml({});
            try {
                setYamlString(YAML.dump(initialMergedConfig));
            } catch (e) {
                console.error("Initial YAML dump failed:", e);
                setYamlString("# Error generating initial YAML");
            }
        } else {
            const errorMsg = loaderData?.error || "Failed to load protocol data.";
            toast.error(`${errorMsg} Using default config.`);
            const defaultConfig = mergeWithDefaults({});
            setConfig(defaultConfig);
            form.reset(defaultConfig);
            setRawSubYaml({});
            try {
                setYamlString(YAML.dump(defaultConfig));
            } catch (e) { setYamlString("# Error generating default YAML"); }
        }
        setIsLoading(false);
    }, [loaderData, form, mergeWithDefaults]);

    const handleInputChange = (path: string, value: any) => {
        form.setValue(path as any, value, { shouldValidate: true, shouldDirty: true });
        const updatedFormConfig = form.getValues();
        const newConfigState = JSON.parse(JSON.stringify(updatedFormConfig));
        setConfig(newConfigState);
        try {
            setYamlString(YAML.dump(newConfigState));
        } catch (e) {
            console.error("YAML dump failed on form change:", e);
        }
    };

    const handleYamlSubfieldChange = (path: string, yamlInput: string) => {
        try {
            // 如果内容为空，则允许，视为空对象
            if (!yamlInput.trim()) {
                setRawSubYaml(prev => {
                    const next = { ...prev };
                    delete next[path];
                    return next;
                });
                form.clearErrors(path as any);
                form.setValue(path as any, {}, { shouldValidate: true, shouldDirty: true });
                const updatedFormConfig = form.getValues();
                setConfig(JSON.parse(JSON.stringify(updatedFormConfig)));
                try {
                    setYamlString(YAML.dump(form.getValues()));
                } catch (e) {
                    console.error("YAML dump failed on empty field:", e);
                }
                return;
            }

            // 尝试解析YAML
            let parsedValue;
            try {
                parsedValue = YAML.load(yamlInput);
            } catch (error: any) {
                // 格式化YAML解析错误，使其更友好
                const yamlError = `YAML解析错误: ${error.message || '无效的YAML格式'}`;
                throw new Error(yamlError);
            }

            // 确保解析结果是一个对象
            if (typeof parsedValue !== 'object' || parsedValue === null) {
                throw new Error("YAML必须表示一个对象 (键值对结构)");
            }

            const finalValue = parsedValue;

            setRawSubYaml(prev => {
                const next = { ...prev };
                delete next[path];
                return next;
            });
            form.clearErrors(path as any);
            form.setValue(path as any, finalValue, { shouldValidate: true, shouldDirty: true });
            const updatedFormConfig = form.getValues();
            setConfig(JSON.parse(JSON.stringify(updatedFormConfig)));
            try {
                setYamlString(YAML.dump(form.getValues()));
            } catch (e) {
                console.error("YAML dump failed on subfield change:", e);
            }

        } catch (error: any) {
            console.warn(`Invalid YAML for ${path}:`, error);

            // 记录YAML的原始内容，供后续调试
            setRawSubYaml(prev => ({ ...prev, [path]: yamlInput }));

            // 设置更友好的错误信息
            const friendlyMessage = error.message || '无效的YAML格式';
            form.setError(path as any, {
                type: "manual",
                message: friendlyMessage
            });

            // 保持原始输入内容，让用户可以修改
            form.setValue(path as any, yamlInput as any, { shouldValidate: false, shouldDirty: true });

            // 更新配置状态，但确保它是有效的JSON
            const updatedFormConfig = form.getValues();
            setConfig(JSON.parse(JSON.stringify(updatedFormConfig)));

            // 提示用户错误，确保注意到
            if (path.includes('strategy') && path.includes('config')) {
                // 从路径中提取策略索引
                const match = path.match(/strategy\.(\d+)\.config/);
                if (match && match[1]) {
                    const strategyIndex = parseInt(match[1]);
                    toast.error(`策略 ${strategyIndex + 1} 配置的YAML格式错误: ${friendlyMessage}`);
                } else {
                    toast.error(`策略配置的YAML格式错误: ${friendlyMessage}`);
                }
            } else {
                // 对于其他类型的配置
                toast.error(`${path.split('.').pop()} 配置的YAML格式错误: ${friendlyMessage}`);
            }
        }
    };

    const getYamlSubfieldValue = (path: string): string => {
        if (rawSubYaml[path] !== undefined) {
            return rawSubYaml[path];
        }
        const value = form.watch(path as any);

        if (value === null || value === undefined || (typeof value === 'object' && Object.keys(value).length === 0)) {
            return '';
        }

        try {
            return YAML.dump(value, { indent: 2 });
        } catch (e) {
            console.error(`Error dumping YAML for ${path}:`, e);
            return "# 显示 YAML 时出错";
        }
    };

    const handleToggleView = () => {
        if (viewMode === 'form') {
            // Form -> YAML: Clean the config before dumping
            try {
                const latestConfig = mergeWithDefaults(form.getValues());
                setConfig(latestConfig); // Keep internal state updated

                // Clean the object to remove empty {} and []
                const cleanedConfig = cleanEmptyObjectsAndArrays(latestConfig);

                // Dump the cleaned configuration
                setYamlString(YAML.dump(cleanedConfig || {}, { // Handle case where entire config becomes empty
                    skipInvalid: true, // Don't throw on complex types if any
                    indent: 2
                }));
                setViewMode('yaml');
            } catch (error) {
                console.error("Error converting form data to YAML:", error);
                toast.error("生成 YAML 视图失败。");
            }
        } else {
            // YAML -> Form: Parse yamlString (logic remains the same)
            try {
                const parsedConfig = YAML.load(yamlString);
                const validatedConfig = mergeWithDefaults(parsedConfig);
                setConfig(validatedConfig);
                form.reset(validatedConfig);
                setRawSubYaml({});
                setViewMode('form');
                toast.success("已切换到表单视图。YAML 解析成功。");
            } catch (error: any) {
                console.error("Error parsing YAML:", error);
                toast.error(`解析 YAML 失败: ${error.message || '未知错误'}。请修正 YAML 后再切换。`);
            }
        }
    };

    const onSubmit: SubmitHandler<GatewayConfig> = async (formData) => {
        setIsSaving(true);

        let configToSave: GatewayConfig | null = null;
        let sourceView = viewMode;

        try {
            if (viewMode === 'yaml') {
                const parsedConfig = YAML.load(yamlString);
                configToSave = mergeWithDefaults(parsedConfig);
            } else {
                // 手动触发表单验证
                const isFormValid = await form.trigger();
                console.log("表单验证状态:", isFormValid, form.formState);

                if (Object.keys(rawSubYaml).length > 0) {
                    const invalidPaths = Object.keys(rawSubYaml).join(', ');
                    throw new Error(`请在保存前修正以下字段中的无效 YAML: ${invalidPaths}`);
                }

                if (!isFormValid || !form.formState.isValid) {
                    // 记录详细的验证错误信息到控制台，帮助开发者调试
                    console.error("表单验证错误:", JSON.stringify(form.formState.errors, null, 2));

                    // 构建更具体的错误信息
                    let errorMessage = "请在保存前修正表单中的验证错误";
                    const errors = form.formState.errors;

                    // 处理策略配置错误
                    if (errors.strategy && Array.isArray(errors.strategy)) {
                        const strategyErrors = errors.strategy
                            .map((strategyError, index) => {
                                if (!strategyError) return null;
                                const fields = Object.keys(strategyError);
                                if (fields.length === 0) return null;
                                return `策略 ${index + 1}: ${fields.join(', ')}`;
                            })
                            .filter(Boolean);

                        if (strategyErrors.length > 0) {
                            errorMessage += `\n- 策略配置问题: ${strategyErrors.join('; ')}`;
                        }
                    }

                    // 处理其他字段错误
                    const otherErrors = Object.entries(errors)
                        .filter(([key]) => key !== 'strategy')
                        .map(([key, error]) => `${key}: ${(error as any)?.message || '验证失败'}`);

                    if (otherErrors.length > 0) {
                        errorMessage += `\n- 其他问题: ${otherErrors.join('; ')}`;
                    }

                    // 如果没有具体错误信息，但表单报告无效
                    if (Object.keys(errors).length === 0) {
                        errorMessage += "\n请检查表单中各个字段是否填写正确。如未找到具体错误，请尝试切换到YAML模式编辑。";

                        // 检查策略类型等必填字段
                        const strategies = form.getValues('strategy') || [];
                        const invalidStrategies = strategies
                            .map((strategy, index) => {
                                if (!strategy.type) return `策略 ${index + 1}: 缺少类型`;
                                return null;
                            })
                            .filter(Boolean);

                        if (invalidStrategies.length > 0) {
                            errorMessage += `\n- 检测到的问题: ${invalidStrategies.join('; ')}`;
                        }
                    }

                    toast.error(errorMessage);
                    setIsSaving(false);
                    return;
                }
                configToSave = mergeWithDefaults(formData);
            }

            if (!configToSave) {
                throw new Error("无法准备配置数据以供保存。");
            }

            if (!protocolId) throw new Error("缺少协议 ID。");
            const response = await API.protocols.updateConfig(protocolId, configToSave);
            if (response.error) throw new Error(response.error);

            toast.success(`协议配置保存成功 (来自 ${sourceView} 视图)。`);
            navigate(`/protocols/${protocolId}`);

        } catch (error: any) {
            console.error(`保存失败 (来自 ${sourceView} 视图):`, error);
            toast.error(`保存失败: ${error.message || '未知错误'}`);
        } finally {
            setIsSaving(false);
        }
    };

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-48">
                <div className="text-center">
                    <div className="w-8 h-8 border-4 border-t-blue-500 border-b-transparent border-l-transparent border-r-transparent rounded-full animate-spin mx-auto mb-4"></div>
                    <p className="text-muted-foreground">正在加载配置...</p>
                </div>
            </div>
        );
    }

    if (!loaderData?.protocol && !isLoading) {
        return (
            <Card className="text-center py-12">
                <CardHeader>
                    <CardTitle>加载协议失败</CardTitle>
                    <CardDescription>{loaderData?.error || '未能加载协议详情，请重试或检查协议是否存在。'}</CardDescription>
                </CardHeader>
                <CardContent>
                    <Button asChild variant="outline">
                        <a onClick={() => navigate('/protocols')}>返回协议列表</a>
                    </Button>
                </CardContent>
            </Card>
        );
    }

    const renderStrategy = (strategy: GoStrategyConfig, index: number) => {
        const strategyPath = `strategy.${index}` as const;
        const configPath = `${strategyPath}.config` as const;
        return (
            <Card key={index} className="mb-6 shadow-sm hover:shadow-md transition-shadow duration-200 ease-in-out">
                <CardHeader className="pb-4 pt-5">
                    <div className="flex justify-between items-start">
                        <CardTitle className="text-xl font-semibold">策略 {index + 1}</CardTitle>
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => removeStrategy(index)}
                            className="h-8 w-8 text-destructive opacity-70 hover:opacity-100 transition-opacity"
                        >
                            <TrashIcon className="h-4 w-4" />
                        </Button>
                    </div>
                </CardHeader>
                <CardContent className="space-y-6 pt-0 pb-5">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-x-6 gap-y-4 items-center pt-2">
                        <div className="md:col-span-2">
                            <Label htmlFor={`${strategyPath}.type`} className="mb-1.5 block font-medium">
                                类型<span className="text-red-500 ml-1">*</span>
                            </Label>
                            <Input
                                id={`${strategyPath}.type`}
                                {...form.register(`${strategyPath}.type`, {
                                    required: "策略类型是必填项",
                                    validate: value => value.trim() !== '' || "策略类型不能为空"
                                })}
                                onChange={e => handleInputChange(`${strategyPath}.type`, e.target.value)}
                                className={`${form.formState.errors?.strategy?.[index]?.type ? 'border-red-500' : 'border-border'} h-9`}
                                placeholder="例如: default_all, custom_js"
                            />
                            {form.formState.errors?.strategy?.[index]?.type && (
                                <p className="text-xs text-red-600 mt-1.5">
                                    {(form.formState.errors.strategy[index]!.type as any).message}
                                </p>
                            )}
                        </div>
                        <div className="flex items-center space-x-2 md:pt-5 justify-self-start md:justify-self-end">
                            <Controller
                                name={`${strategyPath}.enable`}
                                control={form.control}
                                render={({ field }: { field: any }) => (
                                    <Switch
                                        id={field.name}
                                        checked={field.value}
                                        onCheckedChange={(checked) => { field.onChange(checked); handleInputChange(field.name, checked); }}
                                        className="data-[state=checked]:bg-primary"
                                    />
                                )}
                            />
                            <Label htmlFor={`${strategyPath}.enable`} className="font-medium whitespace-nowrap">启用</Label>
                        </div>
                    </div>

                    <div>
                        <Label htmlFor={configPath} className="mb-1.5 block font-medium text-base">详细参数 (YAML)</Label>
                        <Textarea
                            id={configPath}
                            value={getYamlSubfieldValue(configPath)}
                            onChange={e => handleYamlSubfieldChange(configPath, e.target.value)}
                            rows={8}
                            placeholder="输入配置 (YAML格式)...
key: value
nested:
  attr: true"
                            className={`font-mono text-sm border rounded-md p-3 focus:outline-none focus:ring-2 focus:ring-ring ${form.formState.errors?.strategy?.[index]?.config ? 'border-red-500' : 'border-border'}`}
                        />
                        {form.formState.errors?.strategy?.[index]?.config && (
                            <p className="text-xs text-red-600 mt-1.5">
                                {(form.formState.errors.strategy[index]!.config as any).message}
                            </p>
                        )}
                        <p className="text-xs text-muted-foreground mt-1.5 px-1">{getHelpText(configPath)}</p>
                    </div>

                    <div>
                        <div className="flex justify-between items-center mb-2.5">
                            <Label className="font-medium text-base">标签过滤器</Label>
                            <Button type="button" variant="outline" size="sm" onClick={() => addFilterToStrategy(index)} className="h-8">
                                <PlusCircledIcon className="h-4 w-4 mr-1.5" /> 添加
                            </Button>
                        </div>
                        <div className="space-y-3">
                            {form.watch(`${strategyPath}.filter`)?.map((filterString, filterIndex) => {
                                const filterPath = `${strategyPath}.filter.${filterIndex}` as const;
                                return (
                                    <div key={filterPath} className="border rounded-lg p-3.5 relative bg-slate-50 dark:bg-slate-800/30">
                                        <div className="flex items-center">
                                            <Input
                                                id={filterPath}
                                                value={filterString}
                                                onChange={e => {
                                                    const currentFilters = form.getValues(`${strategyPath}.filter`) || [];
                                                    const updatedFilters = [...currentFilters];
                                                    updatedFilters[filterIndex] = e.target.value;
                                                    form.setValue(`${strategyPath}.filter`, updatedFilters, { shouldValidate: true, shouldDirty: true });
                                                    const newFormValues = form.getValues();
                                                    setConfig(JSON.parse(JSON.stringify(newFormValues)));
                                                    try {
                                                        setYamlString(YAML.dump(newFormValues));
                                                    } catch (dumpErr) { console.error("YAML dump failed on filter change:", dumpErr); }
                                                }}
                                                placeholder="输入标签过滤表达式 (例如: device_type == \'sensor\')"
                                                className="h-9 flex-grow mr-2"
                                            />
                                            <Button variant="ghost" size="icon" onClick={() => removeFilterFromStrategy(index, filterIndex)} className="h-7 w-7 text-destructive opacity-60 hover:opacity-100 focus-visible:opacity-100 shrink-0">
                                                <Cross2Icon className="h-3.5 w-3.5" />
                                            </Button>
                                        </div>
                                    </div>
                                );
                            })}
                            {!(form.watch(`${strategyPath}.filter`)?.length) &&
                                <div className="text-sm text-muted-foreground rounded-lg p-6 py-8 text-center border-2 border-dashed border-border bg-slate-50/50 dark:bg-slate-800/20">
                                    尚未添加标签过滤器。
                                    <Button type="button" variant="link" size="sm" onClick={() => addFilterToStrategy(index)} className="mt-1 h-auto p-0 text-primary">
                                        点击此处添加一个。
                                    </Button>
                                </div>
                            }
                        </div>
                    </div>
                </CardContent>
            </Card>
        );
    };

    const addStrategy = () => { const current = form.getValues('strategy') || []; handleInputChange('strategy', [...current, { type: '', enable: true, filter: [], config: {} }]); };
    const removeStrategy = (index: number) => { const current = form.getValues('strategy') || []; handleInputChange('strategy', current.filter((_, i) => i !== index)); };
    const addFilterToStrategy = (strategyIndex: number) => {
        const filterPath = `strategy.${strategyIndex}.filter` as const;
        const currentFilters = form.getValues(filterPath) || [];
        form.setValue(filterPath, [...currentFilters, ''], { shouldValidate: true, shouldDirty: true });
        const newFormValues = form.getValues();
        setConfig(JSON.parse(JSON.stringify(newFormValues)));
        try { setYamlString(YAML.dump(newFormValues)); } catch (e) { console.error("YAML dump on addFilter:", e); }
    };
    const removeFilterFromStrategy = (strategyIndex: number, filterIndex: number) => {
        const filterPath = `strategy.${strategyIndex}.filter` as const;
        const currentFilters = form.getValues(filterPath) || [];
        form.setValue(filterPath, currentFilters.filter((_, i) => i !== filterIndex), { shouldValidate: true, shouldDirty: true });
        const newFormValues = form.getValues();
        setConfig(JSON.parse(JSON.stringify(newFormValues)));
        try { setYamlString(YAML.dump(newFormValues)); } catch (e) { console.error("YAML dump on removeFilter:", e); }
    };

    return (
        <div className="p-6 mx-auto w-full max-w-7xl">
            <div className="space-y-6">
                <div className="flex justify-between items-start">
                    <div>
                        <h1 className="text-2xl font-bold">{protocolName || '加载中...'}</h1>
                        <p className="text-muted-foreground">协议配置</p>
                    </div>
                    <Button asChild variant="outline" size="sm">
                        <a onClick={() => navigate(`/protocols/${protocolId}`)}>
                            <ArrowLeftIcon className="mr-2 h-4 w-4" /> 返回详情
                        </a>
                    </Button>
                </div>

                <Card>
                    <CardHeader>
                        <div className="flex justify-between items-center">
                            <CardTitle>编辑协议配置</CardTitle>
                            <div className="flex gap-2">
                                <Button variant="outline" size="sm" onClick={handleToggleView}>
                                    <ShuffleIcon className="mr-2 h-4 w-4" />
                                    {viewMode === 'form' ? '切换到 YAML' : '切换到表单'}
                                </Button>
                            </div>
                        </div>
                        <CardDescription>协议: {protocolName || `ID: ${protocolId}`}</CardDescription>
                    </CardHeader>
                    <form onSubmit={form.handleSubmit(onSubmit)}>
                        <CardContent>
                            {viewMode === 'form' ? (
                                <Tabs defaultValue="parser" className="w-full" value={activeTab} onValueChange={setActiveTab}>
                                    <TabsList className="mb-4 grid w-full grid-cols-4">
                                        <TabsTrigger value="parser">解析器</TabsTrigger>
                                        <TabsTrigger value="connector">连接器</TabsTrigger>
                                        <TabsTrigger value="strategy">策略</TabsTrigger>
                                        <TabsTrigger value="log">日志</TabsTrigger>
                                    </TabsList>

                                    <TabsContent value="parser">
                                        <div className="space-y-4 mt-4">
                                            <div>
                                                <Label htmlFor="parser.config" className="mb-1 block">配置 (YAML)</Label>
                                                <Textarea
                                                    id="parser.config"
                                                    value={getYamlSubfieldValue("parser.config")}
                                                    onChange={e => handleYamlSubfieldChange("parser.config", e.target.value)}
                                                    rows={10}
                                                    placeholder="输入解析器配置 (YAML格式)...\nkey: value\n..."
                                                    className={`font-mono text-sm border rounded-md p-2 focus:outline-none focus:ring-2 focus:ring-ring ${form.formState.errors?.parser?.config ? 'border-red-500' : ''}`}
                                                />
                                                {form.formState.errors?.parser?.config && <p className="text-xs text-red-600 mt-1">{(form.formState.errors.parser.config as any).message}</p>}
                                                <p className="text-xs text-muted-foreground mt-1">{getHelpText("parser.config")}</p>
                                            </div>
                                        </div>
                                    </TabsContent>

                                    <TabsContent value="connector">
                                        <div className="space-y-4 mt-4">
                                            <div>
                                                <Label htmlFor="connector.type" className="mb-1 block">类型</Label>
                                                <Input id="connector.type" {...form.register("connector.type")} onChange={e => handleInputChange("connector.type", e.target.value)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="connector.config" className="mb-1 block">配置 (YAML)</Label>
                                                <Textarea
                                                    id="connector.config"
                                                    value={getYamlSubfieldValue("connector.config")}
                                                    onChange={e => handleYamlSubfieldChange("connector.config", e.target.value)}
                                                    rows={10}
                                                    placeholder="输入连接器配置 (YAML格式)..."
                                                    className={`font-mono text-sm border rounded-md p-2 focus:outline-none focus:ring-2 focus:ring-ring ${form.formState.errors?.connector?.config ? 'border-red-500' : ''}`}
                                                />
                                                {form.formState.errors?.connector?.config && <p className="text-xs text-red-600 mt-1">{(form.formState.errors.connector.config as any).message}</p>}
                                                <p className="text-xs text-muted-foreground mt-1">{getHelpText("connector.config")}</p>
                                            </div>
                                        </div>
                                    </TabsContent>

                                    <TabsContent value="strategy">
                                        <div className="space-y-4 mt-4">
                                            {form.watch('strategy')?.map((s, i) => renderStrategy(s, i))}
                                            <Button type="button" variant="outline" onClick={addStrategy} className="mt-2">
                                                <PlusCircledIcon className="h-4 w-4 mr-2" /> 添加策略
                                            </Button>
                                        </div>
                                    </TabsContent>

                                    <TabsContent value="log">
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-4 gap-y-4 mt-4">
                                            <div>
                                                <Label htmlFor="log.log_path" className="mb-1 block">日志路径</Label>
                                                <Input id="log.log_path" {...form.register("log.log_path")} onChange={e => handleInputChange("log.log_path", e.target.value)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.level" className="mb-1 block">日志级别</Label>
                                                <Input id="log.level" {...form.register("log.level")} onChange={e => handleInputChange("log.level", e.target.value)} placeholder="例如: info, debug, warn, error" />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.max_size" className="mb-1 block">最大大小 (MB)</Label>
                                                <Input id="log.max_size" type="number" {...form.register("log.max_size", { valueAsNumber: true })} onChange={e => handleInputChange("log.max_size", parseInt(e.target.value) || 0)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.max_backups" className="mb-1 block">最大备份数</Label>
                                                <Input id="log.max_backups" type="number" {...form.register("log.max_backups", { valueAsNumber: true })} onChange={e => handleInputChange("log.max_backups", parseInt(e.target.value) || 0)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.max_age" className="mb-1 block">最大保留天数</Label>
                                                <Input id="log.max_age" type="number" {...form.register("log.max_age", { valueAsNumber: true })} onChange={e => handleInputChange("log.max_age", parseInt(e.target.value) || 0)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.buffer_size" className="mb-1 block">缓冲区大小</Label>
                                                <Input id="log.buffer_size" type="number" {...form.register("log.buffer_size", { valueAsNumber: true })} onChange={e => handleInputChange("log.buffer_size", parseInt(e.target.value) || 0)} />
                                            </div>
                                            <div>
                                                <Label htmlFor="log.flush_interval_secs" className="mb-1 block">刷新间隔 (秒)</Label>
                                                <Input id="log.flush_interval_secs" type="number" {...form.register("log.flush_interval_secs", { valueAsNumber: true })} onChange={e => handleInputChange("log.flush_interval_secs", parseInt(e.target.value) || 0)} />
                                            </div>
                                            <div className="flex items-center space-x-2 md:col-span-1 pt-6">
                                                <Controller name="log.compress" control={form.control} render={({ field }: { field: any }) => (
                                                    <Switch id={field.name} checked={field.value} onCheckedChange={(checked) => { field.onChange(checked); handleInputChange(field.name, checked); }} />
                                                )} />
                                                <Label htmlFor="log.compress" className="mb-1">压缩</Label>
                                            </div>
                                        </div>
                                    </TabsContent>
                                </Tabs>
                            ) : (
                                <div className="mt-4 space-y-2">
                                    <Label htmlFor="yaml-editor" className="mb-1 block font-semibold">YAML 配置</Label>
                                    <Textarea id="yaml-editor" value={yamlString} onChange={(e) => setYamlString(e.target.value)} rows={30} placeholder="直接以 YAML 格式输入配置..." className="font-mono text-sm border rounded-md p-2 focus:outline-none focus:ring-2 focus:ring-ring" />
                                    <p className="text-xs text-muted-foreground">直接以 YAML 格式编辑配置。请确保结构符合要求。</p>
                                </div>
                            )}
                        </CardContent>
                        <CardFooter className="flex justify-between mt-6">
                            <Button type="button" variant="outline" onClick={() => navigate(`/protocols/${protocolId}`)}>取消</Button>
                            <Button type="submit" disabled={isSaving || isLoading || (viewMode === 'form' && Object.keys(rawSubYaml).length > 0) || (viewMode === 'form' && !form.formState.isDirty && form.formState.isSubmitted && !form.formState.isValid)}>
                                {isSaving ? '保存中...' : '保存配置'}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>
            </div>
        </div>
    );
}
