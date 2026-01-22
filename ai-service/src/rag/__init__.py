"""RAG module exports."""

from src.rag.embeddings import (
    BaseEmbedding,
    LocalEmbedding,
    OpenAIEmbedding,
    chunk_text,
    cosine_similarity,
    get_embedding_model,
)
from src.rag.indexer import DocumentIndexer
from src.rag.retriever import RAGRetriever, RetrievedDocument
from src.rag.vector_store import delete_document_embeddings, search_similar_documents

# Alias for backward compatibility
RAGIndexer = DocumentIndexer

__all__ = [
    # Embeddings
    "BaseEmbedding",
    "LocalEmbedding",
    "OpenAIEmbedding",
    "chunk_text",
    "cosine_similarity",
    "get_embedding_model",
    # Indexer
    "DocumentIndexer",
    "RAGIndexer",  # Alias
    # Retriever
    "RAGRetriever",
    "RetrievedDocument",
    # Vector store
    "search_similar_documents",
    "delete_document_embeddings",
]
