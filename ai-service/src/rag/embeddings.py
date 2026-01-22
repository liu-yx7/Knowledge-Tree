"""Embedding models for RAG."""

import logging
from abc import ABC, abstractmethod
from typing import Any

import numpy as np
from openai import OpenAI

from src.config import get_settings

logger = logging.getLogger(__name__)


class BaseEmbedding(ABC):
    """Abstract base class for embedding models."""

    @abstractmethod
    def embed(self, text: str) -> list[float]:
        """Generate embedding for a single text.
        
        Args:
            text: Text to embed
            
        Returns:
            Embedding vector as list of floats
        """
        pass

    @abstractmethod
    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for multiple texts.
        
        Args:
            texts: List of texts to embed
            
        Returns:
            List of embedding vectors
        """
        pass

    @property
    @abstractmethod
    def dimensions(self) -> int:
        """Return the dimension of embeddings produced by this model."""
        pass


class OpenAIEmbedding(BaseEmbedding):
    """OpenAI embedding model."""

    def __init__(
        self,
        model_name: str = "text-embedding-3-small",
        dimensions: int = 1536,
        api_key: str | None = None,
    ):
        """Initialize OpenAI embedding model.
        
        Args:
            model_name: OpenAI embedding model name
            dimensions: Embedding dimensions (for text-embedding-3-* models)
            api_key: OpenAI API key (defaults to settings)
        """
        settings = get_settings()
        self.model_name = model_name
        self._dimensions = dimensions
        self.client = OpenAI(api_key=api_key or settings.openai_api_key)

    @property
    def dimensions(self) -> int:
        return self._dimensions

    def embed(self, text: str) -> list[float]:
        """Generate embedding for a single text."""
        response = self.client.embeddings.create(
            model=self.model_name,
            input=text,
            dimensions=self._dimensions,
        )
        return response.data[0].embedding

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for multiple texts."""
        response = self.client.embeddings.create(
            model=self.model_name,
            input=texts,
            dimensions=self._dimensions,
        )
        return [item.embedding for item in response.data]


class LocalEmbedding(BaseEmbedding):
    """Local embedding model using sentence-transformers."""

    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        """Initialize local embedding model.
        
        Args:
            model_name: Sentence transformer model name
        """
        try:
            from sentence_transformers import SentenceTransformer
        except ImportError:
            raise ImportError(
                "sentence-transformers is required for local embeddings. "
                "Install with: pip install sentence-transformers"
            )

        self.model_name = model_name
        self.model = SentenceTransformer(model_name)
        self._dimensions = self.model.get_sentence_embedding_dimension()

    @property
    def dimensions(self) -> int:
        return self._dimensions

    def embed(self, text: str) -> list[float]:
        """Generate embedding for a single text."""
        embedding = self.model.encode(text, convert_to_numpy=True)
        return embedding.tolist()

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """Generate embeddings for multiple texts."""
        embeddings = self.model.encode(texts, convert_to_numpy=True)
        return embeddings.tolist()


# Singleton instance cache
_embedding_model: BaseEmbedding | None = None


def get_embedding_model(
    provider: str | None = None,
    model_name: str | None = None,
    **kwargs: Any,
) -> BaseEmbedding:
    """Get or create an embedding model instance.
    
    Args:
        provider: Embedding provider ("openai" or "local")
        model_name: Model name to use
        **kwargs: Additional arguments for the model
        
    Returns:
        Embedding model instance
    """
    global _embedding_model

    settings = get_settings()
    provider = provider or settings.embedding_provider

    # Return cached model if available and matches
    if _embedding_model is not None:
        return _embedding_model

    if provider == "openai":
        _embedding_model = OpenAIEmbedding(
            model_name=model_name or settings.embedding_model,
            dimensions=kwargs.get("dimensions", settings.embedding_dimensions),
        )
    elif provider == "local":
        _embedding_model = LocalEmbedding(
            model_name=model_name or "all-MiniLM-L6-v2",
        )
    else:
        raise ValueError(f"Unknown embedding provider: {provider}")

    logger.info(f"Initialized {provider} embedding model")
    return _embedding_model


def cosine_similarity(vec1: list[float], vec2: list[float]) -> float:
    """Calculate cosine similarity between two vectors.
    
    Args:
        vec1: First vector
        vec2: Second vector
        
    Returns:
        Cosine similarity score between -1 and 1
    """
    a = np.array(vec1)
    b = np.array(vec2)
    
    dot_product = np.dot(a, b)
    norm_a = np.linalg.norm(a)
    norm_b = np.linalg.norm(b)
    
    if norm_a == 0 or norm_b == 0:
        return 0.0
    
    return float(dot_product / (norm_a * norm_b))


def chunk_text(
    text: str,
    chunk_size: int = 500,
    overlap: int = 50,
) -> list[str]:
    """Split text into overlapping chunks.
    
    Args:
        text: Text to chunk
        chunk_size: Maximum size of each chunk
        overlap: Number of characters to overlap between chunks
        
    Returns:
        List of text chunks
    """
    if len(text) <= chunk_size:
        return [text]

    chunks = []
    start = 0
    
    while start < len(text):
        end = start + chunk_size
        chunk = text[start:end]
        chunks.append(chunk)
        start = end - overlap
        
        # Avoid infinite loop if overlap >= chunk_size
        if start <= chunks[-1] if len(chunks) > 1 else False:
            break

    return chunks
