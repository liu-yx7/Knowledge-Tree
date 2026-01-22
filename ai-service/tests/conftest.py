"""Pytest configuration and fixtures."""

import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from src.db.models import Base


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
    from datetime import datetime

    conv = Conversation(
        uid="test-conv-123",
        user_id=1,
        title="Test Conversation",
        provider="openai",
        model="gpt-4",
        rag_enabled=True,
        created_ts=int(datetime.utcnow().timestamp()),
        updated_ts=int(datetime.utcnow().timestamp()),
    )
    db_session.add(conv)
    db_session.commit()
    db_session.refresh(conv)
    
    return conv
