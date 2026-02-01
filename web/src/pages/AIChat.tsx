import { useNavigate } from "react-router-dom";
import { Bot } from "lucide-react";
import { AIChatMessages, AIChatInput, AIChatEmptyState } from "@/components/AIChat";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useSemanticSearch } from "@/hooks/useRAGFlowQueries";

const AIChat = () => {
  const navigate = useNavigate();
  const { mutateAsync: semanticSearch, isLoading: isSearching } = useSemanticSearch();

  const handleSendMessage = async (content: string) => {
    if (!content.trim()) return;

    try {
      const results = await semanticSearch({ query: content });
      console.log("Search Results:", results);
      // Handle displaying results in the UI
    } catch (error) {
      console.error("Failed to perform semantic search:", error);
    }
  };

  return (
    <div className="flex h-[calc(100vh-4rem)] sm:h-screen w-full">
      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <AIChatEmptyState 
          onNewChat={() => navigate("/ai")} 
          isLoading={isSearching} 
        />

        {/* Messages */}
        <ScrollArea className="flex-1 overflow-hidden">
          <AIChatMessages
            messages={[]}
            isLoading={false}
            isSending={isSearching}
          />
        </ScrollArea>

        {/* Input Area */}
        <AIChatInput
          onSend={handleSendMessage}
          disabled={isSearching}
        />
      </div>
    </div>
  );
};

export default AIChat;
