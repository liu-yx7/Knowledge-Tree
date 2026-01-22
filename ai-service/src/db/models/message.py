"""AI Message model."""

import uuid
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING

from sqlalchemy import BigInteger, ForeignKey, Index, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from src.db.database import Base

if TYPE_CHECKING:
    from src.db.models.conversation import Conversation


class MessageRole(str, Enum):
    USER = "USER"
    ASSISTANT = "ASSISTANT"
    SYSTEM = "SYSTEM"


class Message(Base):
    """AI message model.
    
    Matches Go struct: store/ai_message.go:AIMessage
    """

    __tablename__ = "ai_messages"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    conversation_id: Mapped[int] = mapped_column(BigInteger, ForeignKey("ai_conversations.id", ondelete="CASCADE"), nullable=False)
    role: Mapped[str] = mapped_column(String(16), nullable=False)
    content: Mapped[str] = mapped_column(Text, nullable=False)
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    token_count: Mapped[int] = mapped_column(Integer, nullable=False, default=0)

    # Relationship
    conversation: Mapped["Conversation"] = relationship("Conversation", back_populates="messages")

    __table_args__ = (
        Index("idx_ai_message_conversation_id", "conversation_id"),
        Index("idx_ai_message_created_ts", "created_ts"),
    )
