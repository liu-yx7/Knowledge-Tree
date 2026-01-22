"""AI Service business logic."""

import logging
import uuid
from datetime import datetime
from typing import AsyncIterator, Optional

from sqlalchemy.orm import Session

from src.db.models import AIConversation, AIMessage, DocumentEmbedding
from src.db.database import get_db
from src.llm import get_llm_provider, LLMProvider
from src.rag import RAGRetriever, DocumentIndexer

logger = logging.getLogger(__name__)


class AIService:
    """Service for AI conversation and RAG operations."""

    def __init__(self, db: Session):
        self.db = db
        self._rag_retriever: Optional[RAGRetriever] = None
        self._document_indexer: Optional[DocumentIndexer] = None

    @property
    def rag_retriever(self) -> RAGRetriever:
        if self._rag_retriever is None:
            self._rag_retriever = RAGRetriever(self.db)
        return self._rag_retriever

    @property
    def document_indexer(self) -> DocumentIndexer:
        if self._document_indexer is None:
            self._document_indexer = DocumentIndexer(self.db)
        return self._document_indexer

    # Conversation CRUD operations

    def create_conversation(
        self,
        creator_id: int,
        title: str,
        provider: str,
        model: str,
        system_prompt: Optional[str] = None,
    ) -> AIConversation:
        """Create a new AI conversation."""
        now = int(datetime.utcnow().timestamp())
        conversation = AIConversation(
            uid=str(uuid.uuid4()),
            creator_id=creator_id,
            title=title or "New Conversation",
            provider=provider,
            model=model,
            system_prompt=system_prompt,
            row_status="NORMAL",
            created_ts=now,
            updated_ts=now,
        )
        self.db.add(conversation)
        self.db.commit()
        self.db.refresh(conversation)
        logger.info(f"Created conversation {conversation.uid} for user {creator_id}")
        return conversation

    def get_conversation(self, uid: str, creator_id: int) -> Optional[AIConversation]:
        """Get a conversation by UID."""
        return (
            self.db.query(AIConversation)
            .filter(
                AIConversation.uid == uid,
                AIConversation.creator_id == creator_id,
            )
            .first()
        )

    def list_conversations(
        self,
        creator_id: int,
        page_size: int = 20,
        offset: int = 0,
        include_archived: bool = False,
    ) -> list[AIConversation]:
        """List conversations for a user."""
        query = self.db.query(AIConversation).filter(
            AIConversation.creator_id == creator_id
        )
        if not include_archived:
            query = query.filter(AIConversation.row_status == "NORMAL")
        
        return (
            query.order_by(AIConversation.updated_ts.desc())
            .offset(offset)
            .limit(page_size)
            .all()
        )

    def update_conversation(
        self,
        uid: str,
        creator_id: int,
        title: Optional[str] = None,
        system_prompt: Optional[str] = None,
        row_status: Optional[str] = None,
    ) -> Optional[AIConversation]:
        """Update a conversation."""
        conversation = self.get_conversation(uid, creator_id)
        if not conversation:
            return None

        if title is not None:
            conversation.title = title
        if system_prompt is not None:
            conversation.system_prompt = system_prompt
        if row_status is not None:
            conversation.row_status = row_status
        conversation.updated_ts = int(datetime.utcnow().timestamp())

        self.db.commit()
        self.db.refresh(conversation)
        return conversation

    def delete_conversation(self, uid: str, creator_id: int) -> bool:
        """Delete a conversation and its messages."""
        conversation = self.get_conversation(uid, creator_id)
        if not conversation:
            return False

        self.db.delete(conversation)
        self.db.commit()
        logger.info(f"Deleted conversation {uid}")
        return True

    # Message operations

    def create_message(
        self,
        conversation_id: int,
        role: str,
        content: str,
        token_count: int = 0,
    ) -> AIMessage:
        """Create a new message in a conversation."""
        message = AIMessage(
            uid=str(uuid.uuid4()),
            conversation_id=conversation_id,
            role=role,
            content=content,
            token_count=token_count,
            created_ts=int(datetime.utcnow().timestamp()),
        )
        self.db.add(message)
        self.db.commit()
        self.db.refresh(message)
        return message

    def list_messages(
        self,
        conversation_id: int,
        page_size: int = 50,
        offset: int = 0,
    ) -> list[AIMessage]:
        """List messages in a conversation."""
        return (
            self.db.query(AIMessage)
            .filter(AIMessage.conversation_id == conversation_id)
            .order_by(AIMessage.created_ts.asc())
            .offset(offset)
            .limit(page_size)
            .all()
        )

    def get_conversation_history(
        self,
        conversation_id: int,
        limit: int = 20,
    ) -> list[dict]:
        """Get conversation history formatted for LLM."""
        messages = (
            self.db.query(AIMessage)
            .filter(AIMessage.conversation_id == conversation_id)
            .order_by(AIMessage.created_ts.desc())
            .limit(limit)
            .all()
        )
        # Reverse to get chronological order
        messages = list(reversed(messages))
        return [{"role": m.role, "content": m.content} for m in messages]

    # AI Chat operations

    async def send_message(
        self,
        conversation_uid: str,
        creator_id: int,
        content: str,
        use_rag: bool = False,
    ) -> tuple[AIMessage, AIMessage, list]:
        """Send a message and get AI response."""
        conversation = self.get_conversation(conversation_uid, creator_id)
        if not conversation:
            raise ValueError(f"Conversation {conversation_uid} not found")

        # Create user message
        user_message = self.create_message(
            conversation_id=conversation.id,
            role="user",
            content=content,
        )

        # Get RAG context if enabled
        retrieved_docs = []
        rag_context = ""
        if use_rag:
            retrieved_docs = self.rag_retriever.retrieve(
                query=content,
                user_id=creator_id,
                top_k=5,
            )
            if retrieved_docs:
                rag_context = self._format_rag_context(retrieved_docs)

        # Build messages for LLM
        messages = []
        if conversation.system_prompt:
            messages.append({"role": "system", "content": conversation.system_prompt})
        if rag_context:
            messages.append({
                "role": "system",
                "content": f"Use the following context to help answer:\n\n{rag_context}",
            })
        
        # Add conversation history
        history = self.get_conversation_history(conversation.id, limit=10)
        messages.extend(history)

        # Get LLM response
        llm = get_llm_provider(conversation.provider)
        response_content, token_count = await llm.chat(
            messages=messages,
            model=conversation.model,
        )

        # Create assistant message
        assistant_message = self.create_message(
            conversation_id=conversation.id,
            role="assistant",
            content=response_content,
            token_count=token_count,
        )

        # Update conversation timestamp
        conversation.updated_ts = int(datetime.utcnow().timestamp())
        self.db.commit()

        return user_message, assistant_message, retrieved_docs

    async def stream_message(
        self,
        conversation_uid: str,
        creator_id: int,
        content: str,
        use_rag: bool = False,
    ) -> AsyncIterator[tuple[str, Optional[AIMessage], Optional[AIMessage], list]]:
        """Stream AI response for a message."""
        conversation = self.get_conversation(conversation_uid, creator_id)
        if not conversation:
            raise ValueError(f"Conversation {conversation_uid} not found")

        # Create user message
        user_message = self.create_message(
            conversation_id=conversation.id,
            role="user",
            content=content,
        )

        # Get RAG context if enabled
        retrieved_docs = []
        rag_context = ""
        if use_rag:
            retrieved_docs = self.rag_retriever.retrieve(
                query=content,
                user_id=creator_id,
                top_k=5,
            )
            if retrieved_docs:
                rag_context = self._format_rag_context(retrieved_docs)

        # Yield initial response with user message
        yield ("start", user_message, None, retrieved_docs)

        # Build messages for LLM
        messages = []
        if conversation.system_prompt:
            messages.append({"role": "system", "content": conversation.system_prompt})
        if rag_context:
            messages.append({
                "role": "system",
                "content": f"Use the following context to help answer:\n\n{rag_context}",
            })
        
        history = self.get_conversation_history(conversation.id, limit=10)
        messages.extend(history)

        # Stream LLM response
        llm = get_llm_provider(conversation.provider)
        full_response = ""
        total_tokens = 0

        async for chunk, tokens in llm.stream_chat(
            messages=messages,
            model=conversation.model,
        ):
            full_response += chunk
            total_tokens = tokens
            yield ("chunk", None, None, [])

        # Create assistant message
        assistant_message = self.create_message(
            conversation_id=conversation.id,
            role="assistant",
            content=full_response,
            token_count=total_tokens,
        )

        # Update conversation timestamp
        conversation.updated_ts = int(datetime.utcnow().timestamp())
        self.db.commit()

        yield ("end", None, assistant_message, [])

    def _format_rag_context(self, docs: list) -> str:
        """Format retrieved documents as context string."""
        context_parts = []
        for i, doc in enumerate(docs, 1):
            context_parts.append(f"[{i}] {doc.chunk_text}")
        return "\n\n".join(context_parts)

    # RAG indexing operations

    def index_memo(
        self,
        memo_uid: str,
        user_id: int,
        content: str,
    ) -> int:
        """Index a memo for RAG retrieval."""
        return self.document_indexer.index_document(
            document_type="memo",
            document_uid=memo_uid,
            user_id=user_id,
            content=content,
        )

    def delete_memo_index(self, memo_uid: str) -> bool:
        """Delete a memo's index."""
        return self.document_indexer.delete_document(
            document_type="memo",
            document_uid=memo_uid,
        )
