"""Tests for AI microservice configuration."""

import os
import pytest

from src.config import Settings, get_settings


class TestSettings:
    """Test configuration settings."""

    def test_default_settings(self):
        """Test default settings values."""
        settings = Settings()
        assert settings.grpc_port == 50051
        assert settings.http_port == 8000
        assert settings.default_provider == "openai"
        assert settings.default_model == "gpt-4o-mini"

    def test_environment_override(self, monkeypatch):
        """Test settings can be overridden via environment."""
        monkeypatch.setenv("GRPC_PORT", "50052")
        monkeypatch.setenv("OPENAI_API_KEY", "test-key")
        
        # Clear cached settings
        get_settings.cache_clear()
        
        settings = get_settings()
        assert settings.grpc_port == 50052
        assert settings.openai_api_key == "test-key"
        
        # Reset
        get_settings.cache_clear()

    def test_rag_settings(self):
        """Test RAG configuration defaults."""
        settings = Settings()
        assert settings.rag_enabled is True
        assert settings.embedding_model == "text-embedding-3-small"
        assert settings.embedding_dimensions == 1536
        assert settings.rag_top_k == 5
