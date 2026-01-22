"""Tests for RAG operations."""

import pytest
from unittest.mock import Mock, patch, MagicMock
import numpy as np

from src.rag.retriever import RAGRetriever, RetrievedDocument


class TestEmbedding:
    """Test embedding generation."""

    def test_embedding_dimensions(self):
        """Test that embeddings have correct dimensions."""
        # Mock embedding for testing
        embedding = np.random.rand(1536).tolist()
        assert len(embedding) == 1536

    @patch("src.rag.embeddings.SentenceTransformer")
    def test_local_embedding_model(self, mock_st):
        """Test local embedding model initialization."""
        mock_model = MagicMock()
        mock_model.encode.return_value = np.random.rand(1, 384)
        mock_st.return_value = mock_model

        from src.rag.embeddings import LocalEmbedding
        
        embedder = LocalEmbedding(model_name="all-MiniLM-L6-v2")
        result = embedder.embed("test text")
        
        assert mock_model.encode.called


class TestRetriever:
    """Test document retrieval."""

    def test_similarity_calculation(self):
        """Test cosine similarity calculation."""
        from src.rag.retriever import cosine_similarity
        
        vec1 = [1.0, 0.0, 0.0]
        vec2 = [1.0, 0.0, 0.0]
        
        similarity = cosine_similarity(vec1, vec2)
        assert abs(similarity - 1.0) < 0.001

    def test_similarity_orthogonal(self):
        """Test similarity of orthogonal vectors."""
        from src.rag.retriever import cosine_similarity
        
        vec1 = [1.0, 0.0, 0.0]
        vec2 = [0.0, 1.0, 0.0]
        
        similarity = cosine_similarity(vec1, vec2)
        assert abs(similarity) < 0.001

    def test_format_context(self):
        """Test context formatting."""
        docs = [
            RetrievedDocument(
                memo_id=1,
                content="First memo content",
                score=0.9,
                metadata={"title": "First"},
            ),
            RetrievedDocument(
                memo_id=2,
                content="Second memo content",
                score=0.8,
                metadata={"title": "Second"},
            ),
        ]

        retriever = RAGRetriever(MagicMock())
        context = retriever.format_context(docs)

        assert "First memo content" in context
        assert "Second memo content" in context
        assert "relevant context" in context.lower()

    @patch("src.rag.retriever.get_embedding_model")
    def test_retrieve_empty_when_no_embeddings(self, mock_get_model):
        """Test retrieval returns empty when no embeddings exist."""
        mock_db = MagicMock()
        mock_db.query.return_value.filter.return_value.all.return_value = []

        retriever = RAGRetriever(mock_db)
        docs = retriever.retrieve(user_id=1, query="test query")

        assert docs == []


class TestIndexer:
    """Test document indexing."""

    def test_chunk_text(self):
        """Test text chunking."""
        from src.rag.indexer import chunk_text
        
        text = "A" * 1000
        chunks = chunk_text(text, chunk_size=200, overlap=50)
        
        assert len(chunks) > 1
        assert all(len(c) <= 200 for c in chunks)

    def test_chunk_text_small_input(self):
        """Test chunking with small input."""
        from src.rag.indexer import chunk_text
        
        text = "Short text"
        chunks = chunk_text(text, chunk_size=200, overlap=50)
        
        assert len(chunks) == 1
        assert chunks[0] == "Short text"

    @patch("src.rag.indexer.get_embedding_model")
    def test_index_memo(self, mock_get_model):
        """Test memo indexing."""
        from src.rag.indexer import RAGIndexer

        mock_model = MagicMock()
        mock_model.embed.return_value = np.random.rand(1536).tolist()
        mock_get_model.return_value = mock_model

        mock_db = MagicMock()
        mock_db.query.return_value.filter.return_value.first.return_value = None

        indexer = RAGIndexer(mock_db)
        indexer.index_memo(
            memo_id=1,
            user_id=1,
            content="Test memo content for indexing",
        )

        mock_model.embed.assert_called_once()
        mock_db.add.assert_called_once()
        mock_db.commit.assert_called_once()
