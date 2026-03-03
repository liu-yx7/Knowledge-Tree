// ==================== ChatOptions 组件 ====================
// 聊天选项面板，包含引用开关、推理开关

import { Settings2, Quote, Brain, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { useChatSettings, useUpdateChatSettings } from "@/hooks/useChatSettingsQueries";

interface ChatOptionsProps {
  /** 紧凑模式，用于侧边栏 */
  compact?: boolean;
  /** 自定义类名 */
  className?: string;
}

const ChatOptions = ({ compact = false, className }: ChatOptionsProps) => {
  // 获取聊天设置
  const { data: settings, isLoading } = useChatSettings();

  // 更新设置
  const updateSettings = useUpdateChatSettings();

  const quoteEnabled = settings?.quoteEnabled ?? true;
  const reasoningEnabled = settings?.reasoningEnabled ?? false;

  const handleToggleQuote = async (enabled: boolean) => {
    try {
      await updateSettings.mutateAsync({ quoteEnabled: enabled });
    } catch (error) {
      console.error("切换引用开关失败:", error);
    }
  };

  const handleToggleReasoning = async (enabled: boolean) => {
    try {
      await updateSettings.mutateAsync({ reasoningEnabled: enabled });
    } catch (error) {
      console.error("切换推理开关失败:", error);
    }
  };

  const isPending = updateSettings.isPending;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size={compact ? "sm" : "default"}
          className={cn(
            "gap-1",
            compact ? "h-7 w-7 p-0" : "h-8 w-8 p-0",
            className
          )}
          disabled={isLoading}
        >
          {isLoading ? (
            <Loader2 className={cn("animate-spin", compact ? "w-3 h-3" : "w-4 h-4")} />
          ) : (
            <Settings2 className={cn(compact ? "w-3 h-3" : "w-4 h-4")} />
          )}
        </Button>
      </PopoverTrigger>

      <PopoverContent align="end" className="w-[240px]">
        <div className="space-y-4">
          <div className="text-sm font-medium">对话选项</div>

          {/* 引用开关 */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Quote className="w-4 h-4 text-muted-foreground" />
              <div className="flex flex-col">
                <Label htmlFor="quote-switch" className="text-sm">
                  显示引用
                </Label>
                <span className="text-xs text-muted-foreground">在回答中显示来源引用</span>
              </div>
            </div>
            <Switch
              id="quote-switch"
              checked={quoteEnabled}
              onCheckedChange={handleToggleQuote}
              disabled={isPending}
            />
          </div>

          {/* 推理开关 */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Brain className="w-4 h-4 text-muted-foreground" />
              <div className="flex flex-col">
                <Label htmlFor="reasoning-switch" className="text-sm">
                  深度研究
                </Label>
                <span className="text-xs text-muted-foreground">启用深度推理模式</span>
              </div>
            </div>
            <Switch
              id="reasoning-switch"
              checked={reasoningEnabled}
              onCheckedChange={handleToggleReasoning}
              disabled={isPending}
            />
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
};

export default ChatOptions;
