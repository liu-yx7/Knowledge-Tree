"""AI Service gRPC servicer implementation."""

import logging
from datetime import datetime

import grpc
from google.protobuf import empty_pb2, timestamp_pb2
from sqlalchemy.orm import Session

from src.auth.jwt import AccessTokenClaims
from src.config import get_settings
from src.db.database import get_db_context
from src.db.models import Conversation, Message, MessageRole, RowStatus
from src.llm import get_available_providers, get_llm_provider
from src.llm.base import ChatMessage
from src.rag import RAGRetriever

# Import generated protobuf classes (will be generated from proto files)
# For now, we'll define placeholder types that match the proto definitions
# These should be replaced with actual imports after running buf generate

logger = logging.getLogger(__name__)


class AIServiceServicer:
    """gRPC servicer for AI Service.
    
    Implements all methods defined in ai_service.proto.
    """

    def __init__(self):
        self.settings = get_settings()

    def _get_user_claims(self, context: grpc.ServicerContext) -> AccessTokenClaims:
        """Extract user claims from context (set by auth interceptor)."""
        # The auth interceptor stores claims in handler_call_details
        # We need to retrieve them from the context
        claims = getattr(context, "_user_claims", None)
        if claims is None:
            context.set_code(grpc.StatusCode.UNAUTHENTICATED)
            context.set_details("Not authenticated")
            raise Exception("Not authenticated")
        return claims

    def _conversation_to_proto(self, conv: Conversation, include_messages: bool = False) -> dict:
        """Convert Conversation model to proto response dict."""
        result = {
            "id": conv.uid,
            "user": f"users/{conv.user_id}",
            "title": conv.title,
            "create_time": self._timestamp_from_unix(conv.created_ts),
            "update_time": self._timestamp_from_unix(conv.updated_ts),
            "model": conv.model,
            "provider": conv.provider,
        }
        if include_messages:
            result["messages"] = [self._message_to_proto(m) for m in conv.messages]
        return result

    def _message_to_proto(self, msg: Message) -> dict:
        """Convert Message model to proto response dict."""
        role_map = {
            MessageRole.USER.value: 1,  # USER = 1
            MessageRole.ASSISTANT.value: 2,  # ASSISTANT = 2
            MessageRole.SYSTEM.value: 3,  # SYSTEM = 3
        }
        return {
            "id": msg.uid,
            "role": role_map.get(msg.role, 0),
            "content": msg.content,
            "create_time": self._timestamp_from_unix(msg.created_ts),
            "token_count": msg.token_count,
        }

    def _timestamp_from_unix(self, unix_ts: int) -> timestamp_pb2.Timestamp:
        """Convert Unix timestamp to protobuf Timestamp."""
        ts = timestamp_pb2.Timestamp()
        ts.FromSeconds(unix_ts)
        return ts

    def CreateConversation(self, request, context: grpc.ServicerContext):
        """Create a new AI conversation."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            now = int(datetime.utcnow().timestamp())
            
            conv = Conversation(
                user_id=claims.user_id,
                title=request.title or "New Chat",
                model=request.model or self.settings.default_model,
                provider=request.provider or self.settings.default_provider,
                created_ts=now,
                updated_ts=now,
            )
            db.add(conv)
            db.commit()
            db.refresh(conv)
            
            logger.info(f"Created conversation {conv.uid} for user {claims.user_id}")
            return self._conversation_to_proto(conv)

    def ListConversations(self, request, context: grpc.ServicerContext):
        """List all conversations for the current user."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            query = db.query(Conversation).filter(
                Conversation.user_id == claims.user_id,
                Conversation.row_status == RowStatus.NORMAL.value,
            ).order_by(Conversation.updated_ts.desc())
            
            # Pagination
            page_size = request.page_size or 20
            if page_size > 100:
                page_size = 100
            
            # Simple offset-based pagination using page_token as offset
            offset = 0
            if request.page_token:
                try:
                    offset = int(request.page_token)
                except ValueError:
                    pass
            
            conversations = query.offset(offset).limit(page_size + 1).all()
            
            # Check if there are more results
            has_more = len(conversations) > page_size
            if has_more:
                conversations = conversations[:page_size]
            
            return {
                "conversations": [self._conversation_to_proto(c) for c in conversations],
                "next_page_token": str(offset + page_size) if has_more else "",
            }

    def GetConversation(self, request, context: grpc.ServicerContext):
        """Get a specific conversation with messages."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            conv = db.query(Conversation).filter(
                Conversation.uid == request.conversation_id,
                Conversation.user_id == claims.user_id,
            ).first()
            
            if not conv:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"Conversation not found: {request.conversation_id}")
                return None
            
            return self._conversation_to_proto(conv, include_messages=True)

    def UpdateConversation(self, request, context: grpc.ServicerContext):
        """Update conversation metadata."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            conv = db.query(Conversation).filter(
                Conversation.uid == request.conversation_id,
                Conversation.user_id == claims.user_id,
            ).first()
            
            if not conv:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"Conversation not found: {request.conversation_id}")
                return None
            
            # Update fields if provided
            if request.title:
                conv.title = request.title
            if request.model:
                conv.model = request.model
            if request.provider:
                conv.provider = request.provider
            
            conv.updated_ts = int(datetime.utcnow().timestamp())
            db.commit()
            db.refresh(conv)
            
            return self._conversation_to_proto(conv)

    def DeleteConversation(self, request, context: grpc.ServicerContext):
        """Delete a conversation and all its messages."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            conv = db.query(Conversation).filter(
                Conversation.uid == request.conversation_id,
                Conversation.user_id == claims.user_id,
            ).first()
            
            if not conv:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"Conversation not found: {request.conversation_id}")
                return None
            
            db.delete(conv)  # Cascade deletes messages
            db.commit()
            
            logger.info(f"Deleted conversation {request.conversation_id}")
            return empty_pb2.Empty()

    def SendMessage(self, request, context: grpc.ServicerContext):
        """Send a message and get AI response."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            # Get conversation
            conv = db.query(Conversation).filter(
                Conversation.uid == request.conversation_id,
                Conversation.user_id == claims.user_id,
            ).first()
            
            if not conv:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"Conversation not found: {request.conversation_id}")
                return None
            
            now = int(datetime.utcnow().timestamp())
            
            # Create user message
            user_msg = Message(
                conversation_id=conv.id,
                role=MessageRole.USER.value,
                content=request.content,
                created_ts=now,
                token_count=0,
            )
            db.add(user_msg)
            
            # Build message history for LLM
            messages = []
            
            # Add system prompt if RAG is enabled
            if conv.rag_enabled and self.settings.rag_enabled:
                retriever = RAGRetriever(db)
                docs = retriever.retrieve(claims.user_id, request.content)
                if docs:
                    context_text = retriever.format_context(docs)
                    messages.append(ChatMessage(role="system", content=context_text))
            
            # Add conversation history
            for msg in conv.messages:
                role = "user" if msg.role == MessageRole.USER.value else "assistant"
                messages.append(ChatMessage(role=role, content=msg.content))
            
            # Add current user message
            messages.append(ChatMessage(role="user", content=request.content))
            
            # Get LLM response
            try:
                provider = get_llm_provider(conv.provider)
                response = provider.complete(messages, model=conv.model)
            except Exception as e:
                logger.error(f"LLM error: {e}")
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"LLM error: {str(e)}")
                return None
            
            # Create assistant message
            assistant_msg = Message(
                conversation_id=conv.id,
                role=MessageRole.ASSISTANT.value,
                content=response.content,
                created_ts=int(datetime.utcnow().timestamp()),
                token_count=response.token_count,
            )
            db.add(assistant_msg)
            
            # Update conversation timestamp
            conv.updated_ts = int(datetime.utcnow().timestamp())
            
            # Auto-generate title from first message if needed
            if conv.title == "New Chat" and len(conv.messages) == 0:
                conv.title = request.content[:50] + ("..." if len(request.content) > 50 else "")
            
            db.commit()
            db.refresh(user_msg)
            db.refresh(assistant_msg)
            
            return {
                "user_message": self._message_to_proto(user_msg),
                "assistant_message": self._message_to_proto(assistant_msg),
            }

    def ListMessages(self, request, context: grpc.ServicerContext):
        """List all messages in a conversation."""
        claims = self._get_user_claims(context)
        
        with get_db_context() as db:
            # Verify conversation ownership
            conv = db.query(Conversation).filter(
                Conversation.uid == request.conversation_id,
                Conversation.user_id == claims.user_id,
            ).first()
            
            if not conv:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"Conversation not found: {request.conversation_id}")
                return None
            
            query = db.query(Message).filter(
                Message.conversation_id == conv.id,
            ).order_by(Message.created_ts.asc())
            
            # Pagination
            page_size = request.page_size or 50
            if page_size > 200:
                page_size = 200
            
            offset = 0
            if request.page_token:
                try:
                    offset = int(request.page_token)
                except ValueError:
                    pass
            
            messages = query.offset(offset).limit(page_size + 1).all()
            
            has_more = len(messages) > page_size
            if has_more:
                messages = messages[:page_size]
            
            return {
                "messages": [self._message_to_proto(m) for m in messages],
                "next_page_token": str(offset + page_size) if has_more else "",
            }

    def GetAIConfig(self, request, context: grpc.ServicerContext):
        """Get AI configuration (public endpoint)."""
        providers = get_available_providers()
        
        return {
            "enabled": self.settings.ai_enabled and len(providers) > 0,
            "providers": [
                {
                    "name": p["name"],
                    "display_name": p["display_name"],
                    "models": p["models"],
                }
                for p in providers
            ],
            "default_provider": self.settings.default_provider,
            "default_model": self.settings.default_model,
        }

    def ListProviders(self, request, context: grpc.ServicerContext):
        """List available LLM providers (public endpoint)."""
        providers = get_available_providers()
        
        return {
            "providers": [
                {
                    "name": p["name"],
                    "display_name": p["display_name"],
                    "models": p["models"],
                }
                for p in providers
            ],
        }

    def StreamMessage(self, request, context: grpc.ServicerContext):
        """Send a message and stream the response (not implemented yet)."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details("Streaming not implemented yet")
        return

    def IndexMemo(self, request, context: grpc.ServicerContext):
        """Index a memo for RAG retrieval."""
        claims = self._get_user_claims(context)
        
        from src.rag.indexer import DocumentIndexer
        
        with get_db_context() as db:
            indexer = DocumentIndexer(db)
            chunks_indexed = indexer.index_memo(
                user_id=claims.user_id,
                memo_uid=request.memo_uid,
                content=request.content,
            )
            
            return {
                "success": True,
                "chunks_indexed": chunks_indexed,
            }

    def DeleteMemoIndex(self, request, context: grpc.ServicerContext):
        """Delete a memo from the RAG index."""
        self._get_user_claims(context)  # Verify auth
        
        from src.rag.indexer import DocumentIndexer
        
        with get_db_context() as db:
            indexer = DocumentIndexer(db)
            chunks_deleted = indexer.delete_memo_index(request.memo_uid)
            
            return {
                "success": True,
                "chunks_indexed": chunks_deleted,  # Reusing field for deleted count
            }
