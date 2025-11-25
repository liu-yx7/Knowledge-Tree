import { BotIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ChatInterface from "@/components/ChatInterface";
import CreateConversationDialog from "@/components/CreateConversationDialog";

const AIChat = observer(() => {
  const t = useTranslate();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const conversationId = searchParams.get("c") ? parseInt(searchParams.get("c")!) : null;

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

  const handleDeleteConversation = async (id: number, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(t("ai.confirm-delete-conversation"))) return;
    
    try {
      await aiStore.deleteConversation(id);
      if (conversationId === id) {
        setSearchParams({});
      }
    } catch (error) {
      console.error("Failed to delete conversation:", error);
    }
  };

  const handleCreateConversation = async (name: string, provider: string, model: string, systemPrompt?: string) => {
    try {
      const conversation = await aiStore.createConversation(name, provider, model, systemPrompt);
      setShowCreateDialog(false);
      setSearchParams({ c: conversation.id.toString() });
    } catch (error) {
      console.error("Failed to create conversation:", error);
    }
  };

  const availableProviders = aiStore.getAvailableProviders();

  return (
    <div className="w-full h-full flex overflow-hidden bg-background">
      {/* Sidebar */}
      <div className="w-80 border-r flex flex-col bg-sidebar">
        <div className="p-4 border-b bg-sidebar-accent">
          <div className="flex items-center justify-between mb-4">
            <h1 className="text-xl font-bold flex items-center gap-2">
              <BotIcon className="w-6 h-6" />
              {t("ai.title")}
            </h1>
          </div>
          <Button
            className="w-full"
            onClick={() => setShowCreateDialog(true)}
            disabled={availableProviders.length === 0}
          >
            <PlusIcon className="w-4 h-4 mr-2" />
            {t("ai.new-conversation")}
          </Button>
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
                    "p-3 rounded-lg cursor-pointer hover:bg-sidebar-accent transition-colors group",
                    conversationId === conv.id && "bg-sidebar-accent"
                  )}
                  onClick={() => handleSelectConversation(conv.id)}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <h3 className="font-medium truncate">{conv.name}</h3>
                      <p className="text-xs text-muted-foreground truncate">
                        {conv.llmProvider} · {conv.llmModel}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {conv.messageCount} {t("ai.messages")}
                      </p>
                    </div>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="opacity-0 group-hover:opacity-100 transition-opacity h-8 w-8"
                      onClick={(e) => handleDeleteConversation(conv.id, e)}
                    >
                      <Trash2Icon className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div className="flex-1 flex flex-col">
        {aiStore.currentConversation ? (
          <ChatInterface />
        ) : (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center space-y-4">
              <BotIcon className="w-16 h-16 mx-auto text-muted-foreground" />
              <div className="space-y-2">
                <h2 className="text-2xl font-bold">{t("ai.welcome")}</h2>
                <p className="text-muted-foreground">{t("ai.select-or-create-conversation")}</p>
              </div>
              {availableProviders.length > 0 && (
                <Button onClick={() => setShowCreateDialog(true)}>
                  <PlusIcon className="w-4 h-4 mr-2" />
                  {t("ai.new-conversation")}
                </Button>
              )}
            </div>
          </div>
        )}
      </div>

      <CreateConversationDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
        onConfirm={handleCreateConversation}
        providers={availableProviders}
      />
    </div>
  );
});

export default AIChat;
