import { useState } from "react";
import { ArrowUp, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import ModelSelector from "./ModelSelector";
import DatasetSelector from "./DatasetSelector";
import ChatOptions from "./ChatOptions";

interface AIChatInputProps {
  onSend: (content: string) => void;
  /** 中止当前流式请求 */
  onAbort?: () => void;
  disabled: boolean;
  /** 正在流式传输中（显示中止按钮） */
  isStreaming?: boolean;
  compact?: boolean;
  /** 是否显示工具栏（模型选择、Dataset 选择、设置） */
  showToolbar?: boolean;
}

const AIChatInput = ({ onSend, onAbort, disabled, isStreaming = false, compact = false, showToolbar = true }: AIChatInputProps) => {
  const [inputMessage, setInputMessage] = useState("");

  const handleSend = () => {
    if (!inputMessage.trim()) return;
    onSend(inputMessage);
    setInputMessage("");
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className={cn(compact ? "p-2" : "p-4", "shrink-0")}>
      <div className={cn(compact ? "" : "max-w-3xl mx-auto")}>
        <div className="relative border rounded-xl bg-background shadow-sm">
          <Textarea
            placeholder="Type your message..."
            value={inputMessage}
            onChange={(e) => setInputMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            className={cn(
              "resize-none border-0 focus-visible:ring-0 focus-visible:ring-offset-0 rounded-xl",
              compact ? "min-h-[60px] max-h-[120px] pr-10 pb-10 text-sm" : "min-h-[80px] max-h-[200px] pr-14 pb-12",
            )}
            disabled={disabled || isStreaming}
          />

          {/* 底部工具栏 */}
          <div className={cn("absolute left-2 right-2 flex items-center justify-between", compact ? "bottom-1" : "bottom-2")}>
            {/* 左侧：模型选择、Dataset 选择、设置 */}
            {showToolbar ? (
              <div className="flex items-center gap-1">
                <ModelSelector compact={compact} />
                <DatasetSelector compact={compact} />
                <ChatOptions compact={compact} />
              </div>
            ) : (
              <div />
            )}

            {/* 右侧：发送/中止按钮 */}
            {isStreaming ? (
              // 中止按钮
              <Button
                onClick={onAbort}
                variant="destructive"
                size="icon"
                className={cn("rounded-lg", compact ? "h-6 w-6" : "h-7 w-7")}
                title="Stop generating"
              >
                <Square className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
              </Button>
            ) : (
              // 发送按钮
              <Button
                onClick={handleSend}
                disabled={!inputMessage.trim() || disabled}
                size="icon"
                className={cn("rounded-lg", compact ? "h-6 w-6" : "h-7 w-7")}
              >
                <ArrowUp className={compact ? "w-3 h-3" : "w-3.5 h-3.5"} />
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default AIChatInput;
