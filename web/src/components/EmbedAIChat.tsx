import { BotIcon, PlusIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
  PopoverAnchor,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ChatInterface from "./ChatInterface";

const EmbedAIChat = observer(() => {
  const t = useTranslate();
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    if (isOpen) {
      aiStore.fetchProviders();
      aiStore.fetchConversations();
    }
  }, [isOpen]);

  const handleNewChat = () => {
    aiStore.clearCurrentConversation();
  };

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button
          className={cn(
            "fixed bottom-6 right-6 z-50 h-14 w-14 rounded-full shadow-lg transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]",
            isOpen ? "opacity-0 scale-50 pointer-events-none" : "opacity-100 scale-100"
          )}
          size="icon"
        >
          <BotIcon className="h-8 w-8" />
        </Button>
      </PopoverTrigger>
      <PopoverContent 
        className="w-[400px] h-[600px] p-0 flex flex-col gap-0 overflow-hidden rounded-2xl shadow-2xl border-border origin-bottom-right data-[state=open]:duration-500 data-[state=closed]:duration-300 data-[state=open]:zoom-in-50 data-[state=closed]:zoom-out-50" 
        side="top" 
        align="end"
      >
        <div className="px-4 py-3 border-b flex flex-row items-center justify-between bg-muted/30">
          <div className="flex items-center gap-2 font-medium">
            <BotIcon className="w-4 h-4" />
            {t("ai.title")}
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleNewChat}
            title={t("ai.new-conversation")}
            className="h-7 w-7"
          >
            <PlusIcon className="w-4 h-4" />
          </Button>
        </div>
        <div className="flex-1 min-h-0 bg-background">
          <ChatInterface 
            hideHeader 
            className="h-full"
          />
        </div>
      </PopoverContent>
    </Popover>
  );
});

export default EmbedAIChat;
