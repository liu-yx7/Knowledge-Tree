"""RAG vector store operations using pgvector."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from src.config import get_settings


def search_similar_documents(
    db: Session,
    user_id: int,
    query_embedding: list[float],
    top_k: int = 5,
    threshold: float = 0.7,
) -> list[dict[str, Any]]:
    """Search for similar documents using pgvector cosine similarity.
    
    Args:
        db: Database session
        user_id: User ID to filter documents
        query_embedding: Query embedding vector
        top_k: Number of results to return
        threshold: Minimum similarity threshold (cosine similarity)
        
    Returns:
        List of matching documents with scores
    """
    # Convert embedding to PostgreSQL vector format
    embedding_str = "[" + ",".join(str(x) for x in query_embedding) + "]"

    query = text("""
        SELECT 
            uid,
            document_type,
            document_uid,
            chunk_index,
            chunk_text,
            1 - (embedding <=> :embedding::vector) as similarity
        FROM document_embeddings
        WHERE user_id = :user_id
          AND 1 - (embedding <=> :embedding::vector) >= :threshold
        ORDER BY embedding <=> :embedding::vector
        LIMIT :top_k
    """)

    result = db.execute(
        query,
        {
            "user_id": user_id,
            "embedding": embedding_str,
            "threshold": threshold,
            "top_k": top_k,
        },
    )

    rows = result.fetchall()

    return [
        {
            "uid": row.uid,
            "document_type": row.document_type,
            "document_uid": row.document_uid,
            "chunk_index": row.chunk_index,
            "chunk_text": row.chunk_text,
            "score": float(row.similarity),
        }
        for row in rows
    ]


def delete_document_embeddings(
    db: Session,
    document_type: str,
    document_uid: str,
) -> int:
    """Delete all embeddings for a document.
    
    Args:
        db: Database session
        document_type: Type of document ("memo" or "attachment")
        document_uid: UID of the document
        
    Returns:
        Number of deleted rows
    """
    query = text("""
        DELETE FROM document_embeddings
        WHERE document_type = :document_type AND document_uid = :document_uid
    """)

    result = db.execute(
        query,
        {"document_type": document_type, "document_uid": document_uid},
    )
    db.commit()

    return result.rowcount
