"""RAG context retrieval."""

import logging
import math
from dataclasses import dataclass, field
from typing import Any

from openai import OpenAI
from sqlalchemy.orm import Session

from src.config import get_settings
from src.rag.vector_store import search_similar_documents

logger = logging.getLogger(__name__)


def cosine_similarity(vec1: list[float], vec2: list[float]) -> float:
    """Calculate cosine similarity between two vectors.
    
    Args:
        vec1: First vector
        vec2: Second vector
        
    Returns:
        Cosine similarity score between -1 and 1
    """
    if len(vec1) != len(vec2):
        raise ValueError("Vectors must have the same length")
    
    dot_product = sum(a * b for a, b in zip(vec1, vec2))
    norm1 = math.sqrt(sum(a * a for a in vec1))
    norm2 = math.sqrt(sum(b * b for b in vec2))
    
    if norm1 == 0 or norm2 == 0:
        return 0.0
    
    return dot_product / (norm1 * norm2)


@dataclass
class RetrievedDocument:
    """A document retrieved for RAG context."""

    # Primary fields (new API)
    document_type: str = ""  # "memo" or "attachment"
    document_uid: str = ""
    chunk_text: str = ""
    score: float = 0.0
    chunk_index: int = 0
    
    # Legacy fields for backward compatibility with tests
    memo_id: int | None = None
    content: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    
    def __post_init__(self):
        """Handle legacy field mappings."""
        # Map legacy fields to new fields if provided
        if self.content and not self.chunk_text:
            self.chunk_text = self.content
        if self.memo_id and not self.document_uid:
            self.document_uid = str(self.memo_id)
            self.document_type = "memo"


class RAGRetriever:
    """Retrieves relevant context for RAG-augmented generation."""

    def __init__(self, db: Session):
        self.db = db
        self.settings = get_settings()
        self.openai_client = OpenAI(api_key=self.settings.openai_api_key)

    def retrieve(
        self,
        user_id: int,
        query: str,
        top_k: int | None = None,
        threshold: float | None = None,
    ) -> list[RetrievedDocument]:
        """Retrieve relevant documents for a query.
        
        Args:
            user_id: User ID to filter documents
            query: Query text to search for
            top_k: Number of results to return (default from settings)
            threshold: Minimum similarity threshold (default from settings)
            
        Returns:
            List of retrieved documents sorted by relevance
        """
        top_k = top_k or self.settings.rag_top_k
        threshold = threshold or self.settings.rag_similarity_threshold

        # Generate query embedding
        response = self.openai_client.embeddings.create(
            model=self.settings.embedding_model,
            input=query,
            dimensions=self.settings.embedding_dimensions,
        )
        query_embedding = response.data[0].embedding

        # Search for similar documents
        results = search_similar_documents(
            db=self.db,
            user_id=user_id,
            query_embedding=query_embedding,
            top_k=top_k,
            threshold=threshold,
        )

        documents = [
            RetrievedDocument(
                document_type=r["document_type"],
                document_uid=r["document_uid"],
                chunk_text=r["chunk_text"],
                score=r["score"],
                chunk_index=r["chunk_index"],
            )
            for r in results
        ]

        logger.info(f"Retrieved {len(documents)} documents for user {user_id}")
        return documents

    def format_context(self, documents: list[RetrievedDocument]) -> str:
        """Format retrieved documents as context for LLM prompt.
        
        Args:
            documents: List of retrieved documents
            
        Returns:
            Formatted context string to prepend to user message
        """
        if not documents:
            return ""

        context_parts = ["The following information from your notes may be relevant:\n"]

        for i, doc in enumerate(documents, 1):
            source = "memo" if doc.document_type == "memo" else "attachment"
            context_parts.append(f"[{i}] From {source}:")
            context_parts.append(doc.chunk_text)
            context_parts.append("")

        return "\n".join(context_parts)
