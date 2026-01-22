"""Application configuration management."""

from functools import lru_cache
from typing import Literal

from pydantic import Field, computed_field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )

    # Service
    service_name: str = "memos-ai-service"
    debug: bool = False

    # gRPC Server
    grpc_host: str = "0.0.0.0"
    grpc_port: int = 50051
    grpc_max_workers: int = 10

    # HTTP Server (for webhooks only)
    http_host: str = "0.0.0.0"
    http_port: int = 8000

    # JWT Authentication (MUST match Go backend)
    jwt_secret: str = Field(..., description="JWT secret, must match Go backend MEMOS_JWT_SECRET")
    jwt_issuer: str = "memos"
    jwt_audience: str = "user.access-token"
    jwt_algorithm: str = "HS256"
    jwt_key_id: str = "v1"

    # Database
    postgres_host: str = "localhost"
    postgres_port: int = 5432
    postgres_user: str = "memos_ai"
    postgres_password: str = Field(..., description="PostgreSQL password")
    postgres_db: str = "memos_ai"

    @computed_field
    @property
    def database_url(self) -> str:
        return f"postgresql://{self.postgres_user}:{self.postgres_password}@{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"

    @computed_field
    @property
    def database_url_async(self) -> str:
        return f"postgresql+asyncpg://{self.postgres_user}:{self.postgres_password}@{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"

    # Go Backend (for fetching attachments, PAT validation)
    go_backend_url: str = "http://localhost:8081"
    go_backend_internal_key: str = Field("", description="Internal API key for Go backend communication")

    # LLM Providers
    openai_api_key: str = Field("", description="OpenAI API key")
    openai_base_url: str = "https://api.openai.com/v1"
    deepseek_api_key: str = Field("", description="DeepSeek API key")
    deepseek_base_url: str = "https://api.deepseek.com/v1"

    # AI Configuration
    ai_enabled: bool = True
    default_provider: Literal["openai", "deepseek"] = "openai"
    default_model: str = "gpt-4o-mini"
    embedding_provider: Literal["openai", "local"] = "openai"
    embedding_model: str = "text-embedding-3-small"
    embedding_dimensions: int = 1536

    # RAG Configuration
    rag_enabled: bool = True
    rag_top_k: int = 5
    rag_similarity_threshold: float = 0.7
    rag_chunk_size: int = 512
    rag_chunk_overlap: int = 50

    # Webhook Security
    webhook_secret: str = Field("", description="Secret for validating webhooks from Go backend")


@lru_cache
def get_settings() -> Settings:
    """Get cached settings instance."""
    return Settings()
