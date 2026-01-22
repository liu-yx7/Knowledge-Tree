"""Chat service for AI conversations."""

import logging
from dataclasses import dataclass
from datetime import datetime

from sqlalchemy.orm import Session

from src.config import get_settings
from src.db.models import Conversation, Message, MessageRole
from src.llm import get_llm_provider
from src.llm.base import ChatMessage, LLMResponse
from src.rag import RAGRetriever

logger = logging.getLogger(__name__)


@dataclass
class ChatResult:
    """Result of a chat interaction."""

    user_message: Message
    assistant_message: Message
    rag_context_used: bool


class ChatService:
    """Service for handling AI chat interactions."""

    def __init__(self, db: Session):
        self.db = db
        self.settings = get_settings()

    def send_message(
        self,
        conversation: Conversation,
        user_id: int,
        content: str,
    ) -> ChatResult:
        """Send a message and get AI response.
        
        Args:
            conversation: The conversation to send message in
            user_id: ID of the user sending the message
            content: Message content
            
        Returns:
            ChatResult with user and assistant messages
        """
        now = int(datetime.utcnow().timestamp())

        # Create user message
        user_msg = Message(
            conversation_id=conversation.id,
            role=MessageRole.USER.value,
            content=content,
            created_ts=now,
            token_count=0,
        )
        self.db.add(user_msg)

        # Build message history
        messages = []
        rag_context_used = False

        # Add RAG context if enabled
        if conversation.rag_enabled and self.settings.rag_enabled:
            retriever = RAGRetriever(self.db)
            docs = retriever.retrieve(user_id, content)
            if docs:
                context_text = retriever.format_context(docs)
                messages.append(ChatMessage(role="system", content=context_text))
                rag_context_used = True

        # Add conversation history
        for msg in conversation.messages:
            role = "user" if msg.role == MessageRole.USER.value else "assistant"
            messages.append(ChatMessage(role=role, content=msg.content))

        # Add current user message
        messages.append(ChatMessage(role="user", content=content))

        # Get LLM response
        provider = get_llm_provider(conversation.provider)
        response = provider.complete(messages, model=conversation.model)

        # Create assistant message
        assistant_msg = Message(
            conversation_id=conversation.id,
            role=MessageRole.ASSISTANT.value,
            content=response.content,
            created_ts=int(datetime.utcnow().timestamp()),
            token_count=response.token_count,
        )
        self.db.add(assistant_msg)

        # Update conversation
        conversation.updated_ts = int(datetime.utcnow().timestamp())

        # Auto-generate title if needed
        if conversation.title == "New Chat" and len(conversation.messages) == 0:
            conversation.title = content[:50] + ("..." if len(content) > 50 else "")

        self.db.commit()
        self.db.refresh(user_msg)
        self.db.refresh(assistant_msg)

        logger.info(
            f"Chat completed: conversation={conversation.uid}, "
            f"tokens={response.token_count}, rag={rag_context_used}"
        )

        return ChatResult(
            user_message=user_msg,
            assistant_message=assistant_msg,
            rag_context_used=rag_context_used,
        )

    def stream_message(
        self,
        conversation: Conversation,
        user_id: int,
        content: str,
    ):
        """Stream a message response (generator).
        
        Args:
            conversation: The conversation to send message in
            user_id: ID of the user sending the message
            content: Message content
            
        Yields:
            Chunks of the assistant's response
        """
        # Build message history
        messages = []

        # Add RAG context if enabled
        if conversation.rag_enabled and self.settings.rag_enabled:
            retriever = RAGRetriever(self.db)
            docs = retriever.retrieve(user_id, content)
            if docs:
                context_text = retriever.format_context(docs)
                messages.append(ChatMessage(role="system", content=context_text))

        # Add conversation history
        for msg in conversation.messages:
            role = "user" if msg.role == MessageRole.USER.value else "assistant"
            messages.append(ChatMessage(role=role, content=msg.content))

        # Add current user message
        messages.append(ChatMessage(role="user", content=content))

        # Stream LLM response
        provider = get_llm_provider(conversation.provider)
        yield from provider.stream(messages, model=conversation.model)
