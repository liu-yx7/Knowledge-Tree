import { BotIcon, PlusIcon, Settings2Icon, Trash2Icon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { toast } from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ChatInterface from "@/components/ChatInterface";
import ConfirmDialog from "@/components/ConfirmDialog";
import CreateConversationDialog from "@/components/CreateConversationDialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import useResponsiveWidth from "@/hooks/useResponsiveWidth";

const AIChat = observer(() => {
  const t = useTranslate();
  const { lg } = useResponsiveWidth();
  const [searchParams, setSearchParams] = useSearchParams();
  const conversationId = searchParams.get("c") ? parseInt(searchParams.get("c")!) : null;
  const [deleteId, setDeleteId] = useState<number | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);

  useEffect(() => {
    aiStore.fetchProviders();
    aiStore.fetchConversations();
  }, []);

  useEffect(() => {
    if (conversationId && conversationId !== aiStore.currentConversation?.id) {
      aiStore.loadConversation(conversationId);
    } else if (!conversationId) {
      aiStore.clearCurrentConversation();
    }
  }, [conversationId]);

  const handleSelectConversation = (id: number) => {
    setSearchParams({ c: id.toString() });
  };

  const handleDeleteConversation = async () => {
    if (!deleteId) return;
    
    try {
      await aiStore.deleteConversation(deleteId);
      if (conversationId === deleteId) {
        setSearchParams({});
      }
      toast.success(t("common.deleted"));
    } catch (error: any) {
      console.error("Failed to delete conversation:", error);
      toast.error(error.details || t("common.error"));
    } finally {
      setDeleteId(null);
    }
  };

  const handleNewChat = () => {
    // Clear current conversation to show empty chat
    setSearchParams({});
    aiStore.clearCurrentConversation();
  };

  const handleCreateWithCustom = async (name: string, provider: string, model: string, systemPrompt?: string) => {
    try {
      const conversation = await aiStore.createConversation(name, provider, model, systemPrompt);
      setSearchParams({ c: conversation.id.toString() });
      setCreateDialogOpen(false);
      toast.success(t("common.created"));
    } catch (error: any) {
      console.error("Failed to create conversation:", error);
      toast.error(error.details || t("common.error"));
    }
  };

  const availableProviders = aiStore.getAvailableProviders();

  return (
    <div className="w-full h-full flex overflow-hidden bg-background">
      {/* Sidebar */}
      <div
        className={cn(
          "border-r flex flex-col bg-sidebar transition-all",
          lg ? "w-72" : "w-56",
        )}
      >
        <div className="p-4 border-b bg-sidebar-accent">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-xl font-bold flex items-center gap-2">
              <BotIcon className="w-6 h-6" />
              {t("ai.title")}
            </h1>
          </div>
          <div className="flex gap-2">
            <Button
              className="flex-1"
              onClick={handleNewChat}
              disabled={availableProviders.length === 0}
            >
              <PlusIcon className="w-4 h-4 mr-2" />
              {t("ai.new-conversation")}
            </Button>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => setCreateDialogOpen(true)}
                  disabled={availableProviders.length === 0}
                >
                  <Settings2Icon className="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {t("ai.create-custom-conversation")}
              </TooltipContent>
            </Tooltip>
          </div>
          {availableProviders.length === 0 && (
            <p className="text-xs text-muted-foreground mt-2">
              {t("ai.no-providers-configured")}
            </p>
          )}
        </div>

        <div className="flex-1 overflow-y-auto">
          {aiStore.isLoadingConversations ? (
            <div className="p-4 text-sm text-muted-foreground">{t("common.loading")}</div>
          ) : aiStore.conversations.length === 0 ? (
            <div className="p-4 text-sm text-muted-foreground">{t("ai.no-conversations")}</div>
          ) : (
            <div className="p-2 space-y-1">
              {aiStore.conversations.map((conv) => (
                <div
                  key={conv.id}
                  className={cn(
                    "p-3 rounded-lg cursor-pointer hover:bg-sidebar-accent transition-colors group relative",
                    conversationId === conv.id && "bg-sidebar-accent"
                  )}
                  onClick={() => handleSelectConversation(conv.id)}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <h3 className="font-medium truncate text-sm">{conv.name}</h3>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground mt-1">
                        <span>{conv.messageCount} {t("ai.messages")}</span>
                      </div>
                    </div>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="opacity-0 group-hover:opacity-100 transition-opacity h-6 w-6 absolute right-2 top-2"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeleteId(conv.id);
                      }}
                    >
                      <Trash2Icon className="w-3 h-3" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div className="flex-1 flex flex-col min-w-0">
        <ChatInterface onConversationCreated={(id) => setSearchParams({ c: id.toString() })} />
      </div>

      {/* Dialogs */}
      <ConfirmDialog
        open={!!deleteId}
        onOpenChange={(open) => !open && setDeleteId(null)}
        title={t("ai.delete-conversation-title")}
        description={t("ai.delete-conversation-description")}
        confirmLabel={t("common.delete")}
        cancelLabel={t("common.cancel")}
        onConfirm={handleDeleteConversation}
        confirmVariant="destructive"
      />

      <CreateConversationDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onConfirm={handleCreateWithCustom}
        providers={availableProviders}
      />
    </div>
  );
});

export default AIChat;
