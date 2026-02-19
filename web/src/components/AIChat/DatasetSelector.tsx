// ==================== DatasetSelector 组件 ====================
// Dataset 选择下拉组件，支持多选

import { useState } from "react";
import { Database, ChevronDown, Check, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import {
  useChatSettings,
  useUpdateDatasetSelection,
  getDatasetDisplayName,
  formatDocumentCount,
} from "@/hooks/useChatSettingsQueries";

interface DatasetSelectorProps {
  /** 紧凑模式，用于侧边栏 */
  compact?: boolean;
  /** 自定义类名 */
  className?: string;
  /** 选择 Dataset 后的回调 */
  onDatasetChange?: (datasetIds: string[]) => void;
}

const DatasetSelector = ({ compact = false, className, onDatasetChange }: DatasetSelectorProps) => {
  const [open, setOpen] = useState(false);

  // 获取聊天设置（包含可用 Dataset 和已选 Dataset）
  const { data: settings, isLoading } = useChatSettings();

  // 更新 Dataset 选择
  const updateSelection = useUpdateDatasetSelection();

  const availableDatasets = settings?.availableDatasets || [];
  const selectedIds = settings?.datasetIds || [];

  // 生成显示文本
  const getDisplayText = () => {
    if (selectedIds.length === 0) {
      return "选择知识库";
    }
    if (selectedIds.length === 1) {
      const dataset = availableDatasets.find((d) => d.id === selectedIds[0]);
      return dataset ? getDatasetDisplayName(dataset) : "1 个知识库";
    }
    return `${selectedIds.length} 个知识库`;
  };

  const handleToggleDataset = async (datasetId: string) => {
    const newSelection = selectedIds.includes(datasetId)
      ? selectedIds.filter((id) => id !== datasetId)
      : [...selectedIds, datasetId];

    try {
      await updateSelection.mutateAsync(newSelection);
      onDatasetChange?.(newSelection);
    } catch (error) {
      console.error("更新 Dataset 选择失败:", error);
    }
  };

  const handleSelectAll = async () => {
    const allIds = availableDatasets.map((d) => d.id);
    try {
      await updateSelection.mutateAsync(allIds);
      onDatasetChange?.(allIds);
    } catch (error) {
      console.error("全选 Dataset 失败:", error);
    }
  };

  const handleClearAll = async () => {
    try {
      await updateSelection.mutateAsync([]);
      onDatasetChange?.([]);
    } catch (error) {
      console.error("清空 Dataset 选择失败:", error);
    }
  };

  // 无可用 Dataset 时显示禁用状态
  if (availableDatasets.length === 0 && !isLoading) {
    return (
      <Button
        variant="ghost"
        size={compact ? "sm" : "default"}
        className={cn("gap-1 text-muted-foreground cursor-not-allowed", className)}
        disabled
      >
        <Database className={cn(compact ? "w-3 h-3" : "w-4 h-4")} />
        <span className={cn(compact ? "text-xs" : "text-sm")}>暂无知识库</span>
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
            updateSelection.isPending && "opacity-50",
            className
          )}
          disabled={isLoading || updateSelection.isPending}
        >
          {isLoading || updateSelection.isPending ? (
            <Loader2 className={cn("animate-spin", compact ? "w-3 h-3" : "w-4 h-4")} />
          ) : (
            <Database className={cn(compact ? "w-3 h-3" : "w-4 h-4")} />
          )}
          <span className={cn("truncate max-w-[100px]", compact ? "text-xs" : "text-sm")}>{getDisplayText()}</span>
          <ChevronDown className={cn("opacity-50", compact ? "w-3 h-3" : "w-4 h-4")} />
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="start" className="w-[240px]">
        {/* 快捷操作 */}
        <div className="flex items-center justify-between px-2 py-1.5">
          <span className="text-xs text-muted-foreground">知识库</span>
          <div className="flex gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs"
              onClick={handleSelectAll}
              disabled={selectedIds.length === availableDatasets.length}
            >
              全选
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs"
              onClick={handleClearAll}
              disabled={selectedIds.length === 0}
            >
              清空
            </Button>
          </div>
        </div>

        <DropdownMenuSeparator />

        {/* Dataset 列表 */}
        {availableDatasets.map((dataset) => {
          const isSelected = selectedIds.includes(dataset.id);
          return (
            <DropdownMenuItem
              key={dataset.id}
              onClick={() => handleToggleDataset(dataset.id)}
              className="flex items-center justify-between cursor-pointer"
            >
              <div className="flex flex-col flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm truncate">{getDatasetDisplayName(dataset)}</span>
                  {dataset.isDefault && (
                    <span className="text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded">默认</span>
                  )}
                </div>
                <span className="text-xs text-muted-foreground">{formatDocumentCount(dataset.documentCount)}</span>
              </div>
              {isSelected && <Check className="w-4 h-4 text-primary shrink-0" />}
            </DropdownMenuItem>
          );
        })}

        {availableDatasets.length === 0 && (
          <div className="py-4 text-center text-sm text-muted-foreground">暂无可用知识库</div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

export default DatasetSelector;
