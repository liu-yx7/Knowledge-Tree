"""Document embedding model for RAG."""

import uuid
from datetime import datetime

from pgvector.sqlalchemy import Vector
from sqlalchemy import BigInteger, Index, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from src.config import get_settings
from src.db.database import Base


class DocumentEmbedding(Base):
    """Document embedding model for RAG vector search.
    
    Stores chunked document content with vector embeddings for similarity search.
    """

    __tablename__ = "document_embeddings"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    user_id: Mapped[int] = mapped_column(BigInteger, nullable=False, index=True)
    
    # Document reference
    document_type: Mapped[str] = mapped_column(String(32), nullable=False)  # "memo" or "attachment"
    document_uid: Mapped[str] = mapped_column(String(256), nullable=False, index=True)
    
    # Chunk information
    chunk_index: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    chunk_text: Mapped[str] = mapped_column(Text, nullable=False)
    
    # Vector embedding (dimensions from settings)
    embedding: Mapped[list[float]] = mapped_column(Vector(get_settings().embedding_dimensions), nullable=False)
    
    # Metadata
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    updated_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))

    __table_args__ = (
        Index("idx_embedding_user_id", "user_id"),
        Index("idx_embedding_document", "document_type", "document_uid"),
    )
