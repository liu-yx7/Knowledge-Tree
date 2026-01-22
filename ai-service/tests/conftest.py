"""Pytest configuration and fixtures."""

import os

# Set required environment variables BEFORE importing anything else
os.environ.setdefault("JWT_SECRET", "test-jwt-secret-for-testing-only-32chars")
os.environ.setdefault("POSTGRES_PASSWORD", "test-password")
os.environ.setdefault("POSTGRES_HOST", "localhost")
os.environ.setdefault("POSTGRES_DB", "test_db")
os.environ.setdefault("OPENAI_API_KEY", "test-api-key")

import pytest
from datetime import datetime, UTC
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from src.db.database import Base


@pytest.fixture
def db_session():
    """Create an in-memory database session for testing."""
    engine = create_engine("sqlite:///:memory:")
    Base.metadata.create_all(engine)
    
    Session = sessionmaker(bind=engine)
    session = Session()
    
    yield session
    
    session.close()
    engine.dispose()


@pytest.fixture
def sample_conversation(db_session):
    """Create a sample conversation for testing."""
    from src.db.models import Conversation

    conv = Conversation(
        uid="test-conv-123",
        user_id=1,
        title="Test Conversation",
        provider="openai",
        model="gpt-4",
        rag_enabled=True,
        created_ts=int(datetime.now(UTC).timestamp()),
        updated_ts=int(datetime.now(UTC).timestamp()),
    )
    db_session.add(conv)
    db_session.commit()
    db_session.refresh(conv)
    
    return conv
