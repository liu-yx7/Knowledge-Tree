"""AI Conversation model."""

import uuid
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING

from sqlalchemy import BigInteger, Boolean, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from src.db.database import Base

if TYPE_CHECKING:
    from src.db.models.message import Message


class RowStatus(str, Enum):
    NORMAL = "NORMAL"
    ARCHIVED = "ARCHIVED"


class Conversation(Base):
    """AI conversation model.
    
    Matches Go struct: store/ai_conversation.go:AIConversation
    """

    __tablename__ = "ai_conversations"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    user_id: Mapped[int] = mapped_column(BigInteger, nullable=False, index=True)
    title: Mapped[str] = mapped_column(String(512), nullable=False, default="New Chat")
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    updated_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    row_status: Mapped[str] = mapped_column(String(16), nullable=False, default=RowStatus.NORMAL.value)
    model: Mapped[str] = mapped_column(String(128), nullable=False, default="")
    provider: Mapped[str] = mapped_column(String(64), nullable=False, default="")
    
    # RAG settings for this conversation
    rag_enabled: Mapped[bool] = mapped_column(Boolean, default=True)
    system_prompt: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Relationships
    messages: Mapped[list["Message"]] = relationship(
        "Message",
        back_populates="conversation",
        cascade="all, delete-orphan",
        order_by="Message.created_ts",
    )

    __table_args__ = (
        Index("idx_ai_conversation_user_id", "user_id"),
        Index("idx_ai_conversation_created_ts", "created_ts"),
    )
