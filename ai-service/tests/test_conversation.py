"""Tests for conversation operations."""

import pytest
from datetime import datetime

from src.db.models import Conversation, Message


class TestConversation:
    """Test conversation CRUD operations."""

    def test_create_conversation(self, db_session):
        """Test creating a new conversation."""
        conv = Conversation(
            uid="conv-001",
            user_id=1,
            title="Test Chat",
            provider="openai",
            model="gpt-4",
            rag_enabled=True,
            created_ts=int(datetime.utcnow().timestamp()),
            updated_ts=int(datetime.utcnow().timestamp()),
        )
        db_session.add(conv)
        db_session.commit()

        result = db_session.query(Conversation).filter_by(uid="conv-001").first()
        assert result is not None
        assert result.title == "Test Chat"
        assert result.provider == "openai"
        assert result.rag_enabled is True

    def test_add_message_to_conversation(self, db_session, sample_conversation):
        """Test adding messages to a conversation."""
        msg = Message(
            uid="msg-001",
            conversation_id=sample_conversation.id,
            role="user",
            content="Hello, AI!",
            created_ts=int(datetime.utcnow().timestamp()),
        )
        db_session.add(msg)
        db_session.commit()

        result = db_session.query(Message).filter_by(uid="msg-001").first()
        assert result is not None
        assert result.role == "user"
        assert result.content == "Hello, AI!"
        assert result.conversation_id == sample_conversation.id

    def test_conversation_messages_relationship(self, db_session, sample_conversation):
        """Test conversation-messages relationship."""
        messages = [
            Message(
                uid=f"msg-{i}",
                conversation_id=sample_conversation.id,
                role="user" if i % 2 == 0 else "assistant",
                content=f"Message {i}",
                created_ts=int(datetime.utcnow().timestamp()),
            )
            for i in range(3)
        ]
        db_session.add_all(messages)
        db_session.commit()

        db_session.refresh(sample_conversation)
        assert len(sample_conversation.messages) == 3

    def test_delete_conversation_cascades(self, db_session, sample_conversation):
        """Test that deleting a conversation deletes its messages."""
        msg = Message(
            uid="msg-cascade",
            conversation_id=sample_conversation.id,
            role="user",
            content="Will be deleted",
            created_ts=int(datetime.utcnow().timestamp()),
        )
        db_session.add(msg)
        db_session.commit()

        db_session.delete(sample_conversation)
        db_session.commit()

        result = db_session.query(Message).filter_by(uid="msg-cascade").first()
        assert result is None
