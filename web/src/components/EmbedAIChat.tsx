import { BotIcon, MinusIcon, PlusIcon, PanelRight, AppWindow } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ChatInterface from "./ChatInterface";

const EmbedAIChat = observer(() => {
  const t = useTranslate();
  const { isChatOpen, chatViewMode } = aiStore;

  useEffect(() => {
    if (isChatOpen) {
      aiStore.fetchProviders();
      aiStore.fetchConversations();
    }
  }, [isChatOpen]);

  const handleNewChat = () => {
    aiStore.clearCurrentConversation();
  };

  const toggleViewMode = () => {
    aiStore.setChatViewMode(chatViewMode === "floating" ? "sidebar" : "floating");
  };

  const handleOpenChange = (open: boolean) => {
    aiStore.setChatOpen(open);
  };

  const renderHeader = () => (
    <div className="px-4 py-3 border-b flex flex-row items-center justify-between bg-muted/30">
      <div className="flex items-center gap-2 font-medium">
        <BotIcon className="w-4 h-4" />
        {t("ai.title")}
      </div>
      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleViewMode}
          title={chatViewMode === "floating" ? "Switch to sidebar" : "Switch to floating"}
          className="h-7 w-7"
        >
          {chatViewMode === "floating" ? (
            <PanelRight className="w-4 h-4" />
          ) : (
            <AppWindow className="w-4 h-4" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={handleNewChat}
          title={t("ai.new-conversation")}
          className="h-7 w-7"
        >
          <PlusIcon className="w-4 h-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={() => handleOpenChange(false)}
          title={t("common.close")}
          className="h-7 w-7"
        >
          <MinusIcon className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  const renderContent = () => (
    <>
      {renderHeader()}
      <div className="flex-1 min-h-0 bg-background">
        <ChatInterface hideHeader className="h-full" />
      </div>
    </>
  );

  if (chatViewMode === "sidebar") {
    return (
      <>
        <Button
          className={cn(
            "fixed bottom-6 right-6 z-50 h-14 w-14 rounded-full shadow-lg transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]",
            isChatOpen
              ? "opacity-0 scale-50 pointer-events-none"
              : "opacity-100 scale-100"
          )}
          size="icon"
          onClick={() => handleOpenChange(true)}
        >
          <BotIcon className="h-8 w-8" />
        </Button>
        <div
          className={cn(
            "fixed top-0 right-0 z-50 h-full w-[400px] bg-background shadow-2xl border-l transition-transform duration-300 ease-in-out flex flex-col",
            isChatOpen ? "translate-x-0" : "translate-x-full"
          )}
        >
          {renderContent()}
        </div>
      </>
    );
  }

  return (
    <Popover open={isChatOpen} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          className={cn(
            "fixed bottom-6 right-6 z-50 h-14 w-14 rounded-full shadow-lg transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]",
            isChatOpen
              ? "opacity-0 scale-50 pointer-events-none"
              : "opacity-100 scale-100"
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
        onInteractOutside={(e) => e.preventDefault()}
      >
        {renderContent()}
      </PopoverContent>
    </Popover>
  );
});

export default EmbedAIChat;
