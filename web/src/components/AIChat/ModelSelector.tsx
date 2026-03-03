// ==================== ModelSelector 组件 ====================
// 模型选择下拉组件，参考 Notion AI 风格

import { useState } from "react";
import { ChevronDown, Check, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import {
  useAvailableModels,
  useUserLLMPreference,
  useSetUserLLMPreference,
  getModelDisplayName,
  formatProviderName,
} from "@/hooks/useLLMQueries";
import type { LLMModel } from "@/types/proto/api/v1/llm_service_pb";

interface ModelSelectorProps {
  /** 紧凑模式，用于侧边栏 */
  compact?: boolean;
  /** 自定义类名 */
  className?: string;
  /** 选择模型后的回调 */
  onModelChange?: (modelId: string) => void;
}

const ModelSelector = ({ compact = false, className, onModelChange }: ModelSelectorProps) => {
  const [open, setOpen] = useState(false);

  // 获取可用模型列表
  const { data: models = [], isLoading: isLoadingModels } = useAvailableModels();

  // 获取用户当前选择的模型
  const { data: preference, isLoading: isLoadingPreference } = useUserLLMPreference();

  // 设置模型偏好
  const setPreference = useSetUserLLMPreference();

  const isLoading = isLoadingModels || isLoadingPreference;
  const currentModelId = preference?.modelId || "";

  // 查找当前选中的模型信息
  const currentModel = models.find((m) => m.modelId === currentModelId);
  const displayName = currentModel ? getModelDisplayName(currentModel) : "选择模型";

  // 按提供商分组模型
  const modelsByProvider = models.reduce(
    (acc, model) => {
      const provider = model.provider || "其他";
      if (!acc[provider]) {
        acc[provider] = [];
      }
      acc[provider].push(model);
      return acc;
    },
    {} as Record<string, LLMModel[]>
  );

  const handleSelectModel = async (modelId: string) => {
    if (modelId === currentModelId) {
      setOpen(false);
      return;
    }

    try {
      await setPreference.mutateAsync(modelId);
      onModelChange?.(modelId);
    } catch (error) {
      console.error("切换模型失败:", error);
    }
    setOpen(false);
  };

  // 无可用模型时显示禁用状态
  if (models.length === 0 && !isLoading) {
    return (
      <Button
        variant="ghost"
        size={compact ? "sm" : "default"}
        className={cn("gap-1 text-muted-foreground cursor-not-allowed", className)}
        disabled
      >
        <Sparkles className={cn(compact ? "w-3 h-3" : "w-4 h-4")} />
        <span className={cn(compact ? "text-xs" : "text-sm")}>模型未配置</span>
      </Button>
    );
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size={compact ? "sm" : "default"}
          className={cn(
            "gap-1 font-normal",
            compact ? "h-7 px-2" : "h-8 px-3",
            setPreference.isPending && "opacity-50",
            className
          )}
          disabled={isLoading || setPreference.isPending}
        >
          {isLoading || setPreference.isPending ? (
            <Loader2 className={cn("animate-spin", compact ? "w-3 h-3" : "w-4 h-4")} />
          ) : (
            <Sparkles className={cn(compact ? "w-3 h-3" : "w-4 h-4")} />
          )}
          <span className={cn("truncate max-w-[120px]", compact ? "text-xs" : "text-sm")}>{displayName}</span>
          <ChevronDown className={cn("opacity-50", compact ? "w-3 h-3" : "w-4 h-4")} />
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="start" className="w-[220px]">
        {Object.entries(modelsByProvider).map(([provider, providerModels], index) => (
          <div key={provider}>
            {index > 0 && <DropdownMenuSeparator />}
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              {formatProviderName(provider)}
            </DropdownMenuLabel>
            {providerModels.map((model) => (
              <DropdownMenuItem
                key={model.modelId}
                onClick={() => handleSelectModel(model.modelId)}
                className="flex items-center justify-between cursor-pointer"
              >
                <div className="flex flex-col">
                  <span className="text-sm">{getModelDisplayName(model)}</span>
                  {model.description && (
                    <span className="text-xs text-muted-foreground truncate max-w-[180px]">{model.description}</span>
                  )}
                </div>
                {model.modelId === currentModelId && <Check className="w-4 h-4 text-primary" />}
              </DropdownMenuItem>
            ))}
          </div>
        ))}

        {models.length === 0 && (
          <div className="py-4 text-center text-sm text-muted-foreground">暂无可用模型</div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

export default ModelSelector;
