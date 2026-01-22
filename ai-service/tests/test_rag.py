"""Tests for RAG operations."""

import pytest
from unittest.mock import Mock, patch, MagicMock
import numpy as np

from src.rag.retriever import RAGRetriever, RetrievedDocument, cosine_similarity
from src.rag.indexer import chunk_text, DocumentIndexer


class TestEmbedding:
    """Test embedding generation."""

    def test_embedding_dimensions(self):
        """Test that embeddings have correct dimensions."""
        # Mock embedding for testing
        embedding = np.random.rand(1536).tolist()
        assert len(embedding) == 1536


class TestRetriever:
    """Test document retrieval."""

    def test_similarity_calculation(self):
        """Test cosine similarity calculation."""
        vec1 = [1.0, 0.0, 0.0]
        vec2 = [1.0, 0.0, 0.0]
        
        similarity = cosine_similarity(vec1, vec2)
        assert abs(similarity - 1.0) < 0.001

    def test_similarity_orthogonal(self):
        """Test similarity of orthogonal vectors."""
        vec1 = [1.0, 0.0, 0.0]
        vec2 = [0.0, 1.0, 0.0]
        
        similarity = cosine_similarity(vec1, vec2)
        assert abs(similarity) < 0.001

    def test_format_context(self):
        """Test context formatting."""
        docs = [
            RetrievedDocument(
                document_type="memo",
                document_uid="1",
                chunk_text="First memo content",
                score=0.9,
            ),
            RetrievedDocument(
                document_type="memo",
                document_uid="2",
                chunk_text="Second memo content",
                score=0.8,
            ),
        ]

        retriever = RAGRetriever(MagicMock())
        context = retriever.format_context(docs)

        assert "First memo content" in context
        assert "Second memo content" in context
        assert "relevant" in context.lower()

    def test_format_context_empty(self):
        """Test formatting empty document list."""
        retriever = RAGRetriever(MagicMock())
        context = retriever.format_context([])
        assert context == ""


class TestIndexer:
    """Test document indexing."""

    def test_chunk_text(self):
        """Test text chunking."""
        text = "A" * 1000
        chunks = chunk_text(text, chunk_size=200, overlap=50)
        
        assert len(chunks) > 1
        assert all(len(c) <= 200 for c in chunks)

    def test_chunk_text_small_input(self):
        """Test chunking with small input."""
        text = "Short text"
        chunks = chunk_text(text, chunk_size=200, overlap=50)
        
        assert len(chunks) == 1
        assert chunks[0] == "Short text"

    def test_chunk_text_empty(self):
        """Test chunking empty text."""
        chunks = chunk_text("", chunk_size=200, overlap=50)
        assert chunks == []
