import { BotIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { aiStore } from "@/store";
import { useTranslate } from "@/utils/i18n";
import ChatInterface from "@/components/ChatInterface";

const AIChat = observer(() => {
  const t = useTranslate();
  const [searchParams, setSearchParams] = useSearchParams();
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

  const handleNewChat = () => {
    // Clear current conversation to show empty chat
    setSearchParams({});
    aiStore.clearCurrentConversation();
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
            onClick={handleNewChat}
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
        <ChatInterface onConversationCreated={(id) => setSearchParams({ c: id.toString() })} />
      </div>
    </div>
  );
});

export default AIChat;
