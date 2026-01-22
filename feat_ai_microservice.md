# Feature: AI Microservice with RAG (LlamaIndex)

## Overview

This document provides a comprehensive implementation plan for migrating the AI feature to a standalone Python microservice with RAG (Retrieval-Augmented Generation) capabilities using LlamaIndex. The service will enable AI to reference user's memos and attachments when generating responses.

### Key Design Decisions

1. **Nginx Routing**: API Gateway routes `/api/v1/ai/*` directly to Python service
2. **Independent JWT Validation**: Python service validates JWT tokens independently (same secret as Go backend)
3. **Separate Database**: Python service uses PostgreSQL with pgvector for conversations, messages, and vector embeddings
4. **LlamaIndex RAG**: Full LlamaIndex integration for document indexing and retrieval
5. **Data Sync via Webhooks**: Go backend notifies Python service of memo/attachment changes

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Aliyun Cloud Server                                │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                        Nginx (Port 443)                                 │ │
│  │  ┌──────────────────────────────────────────────────────────────────┐  │ │
│  │  │ SSL Termination + Routing                                        │  │ │
│  │  │                                                                   │  │ │
│  │  │  /api/v1/ai/*  ─────────────────▶  Python AI Service (8000)     │  │ │
│  │  │  /api/v1/*     ─────────────────▶  Go Backend (8081)            │  │ │
│  │  │  /memos.api.*  ─────────────────▶  Go Backend (8081)            │  │ │
│  │  │  /*            ─────────────────▶  Go Backend (8081)            │  │ │
│  │  └──────────────────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
│  ┌─────────────────────────────┐    ┌─────────────────────────────────────┐ │
│  │      Go Backend             │    │       Python AI Service             │ │
│  │      (Port 8081)            │    │         (Port 8000)                 │ │
│  │                             │    │                                     │ │
│  │  - User/Memo/Attachment API │───▶│  - JWT Validation (same secret)    │ │
│  │  - Webhook on data changes  │    │  - Conversation CRUD               │ │
│  │                             │    │  - RAG Query + LLM Generation      │ │
│  │  ┌───────────────────────┐  │    │  - Document Indexing               │ │
│  │  │ SQLite/MySQL/Postgres │  │    │                                     │ │
│  │  │ - users               │  │    │  ┌───────────────────────────────┐ │ │
│  │  │ - memos               │  │    │  │ PostgreSQL + pgvector         │ │ │
│  │  │ - attachments         │  │    │  │ - ai_conversations            │ │ │
│  │  └───────────────────────┘  │    │  │ - ai_messages                 │ │ │
│  └─────────────────────────────┘    │  │ - document_embeddings         │ │ │
│                                      │  └───────────────────────────────┘ │ │
│                                      └─────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## JWT Token Structure (Reference)

The Go backend generates JWT tokens with the following structure. Python service must validate these identically.

### Access Token Claims (V2)

```json
{
  "type": "access",
  "role": "USER|HOST|ADMIN",
  "status": "NORMAL|ARCHIVED",
  "username": "john_doe",
  "iss": "memos",
  "aud": ["user.access-token"],
  "sub": "123",
  "iat": 1737500000,
  "exp": 1737500900
}
```

### Token Validation Rules

1. **Algorithm**: HS256 (HMAC-SHA256)
2. **Key ID**: Must be "v1" in header
3. **Issuer**: Must be "memos"
4. **Audience**: Must contain "user.access-token"
5. **Expiration**: Must not be expired (15-minute lifetime)
6. **Type**: Must be "access"

### Personal Access Token (PAT)

- Format: `memos_pat_<random_string>`
- Stored as SHA-256 hash in database
- Python service cannot validate PATs (requires Go database access)
- **Decision**: PAT validation will use auth_request subrequest to Go backend

---

## Phase 1: Project Setup & Infrastructure

### 1.1 Create Python Project Structure

```
ai-service/
├── pyproject.toml              # Project configuration (Poetry/PDM)
├── Dockerfile
├── docker-compose.yml          # Local development
├── alembic.ini                 # Database migrations
├── .env.example
├── README.md
│
├── alembic/                    # Database migrations
│   ├── env.py
│   └── versions/
│       └── 001_initial_schema.py
│
├── proto/                      # Protocol Buffer definitions
│   ├── buf.yaml
│   ├── buf.gen.yaml
│   └── api/v1/
│       └── ai_service.proto    # AI service proto (copy from Go)
│
├── src/
│   ├── __init__.py
│   ├── main.py                 # gRPC server entry point
│   ├── config.py               # Configuration management
│   │
│   ├── proto/                  # Generated protobuf code
│   │   └── api/v1/
│   │       ├── ai_service_pb2.py
│   │       └── ai_service_pb2_grpc.py
│   │
│   ├── grpc/                   # gRPC layer
│   │   ├── __init__.py
│   │   ├── server.py           # gRPC server setup
│   │   ├── interceptors.py     # Auth, logging interceptors
│   │   └── servicers/          # gRPC service implementations
│   │       ├── __init__.py
│   │       └── ai_servicer.py  # AIService implementation
│   │
│   ├── auth/                   # Authentication
│   │   ├── __init__.py
│   │   ├── jwt.py              # JWT validation (matching Go)
│   │   └── interceptor.py      # gRPC auth interceptor
│   │
│   ├── db/                     # Database layer
│   │   ├── __init__.py
│   │   ├── database.py         # SQLAlchemy setup
│   │   ├── models/             # SQLAlchemy models
│   │   │   ├── __init__.py
│   │   │   ├── conversation.py
│   │   │   ├── message.py
│   │   │   └── embedding.py
│   │   └── repositories/       # Data access
│   │       ├── __init__.py
│   │       ├── conversation.py
│   │       └── message.py
│   │
│   ├── llm/                    # LLM providers
│   │   ├── __init__.py
│   │   ├── base.py             # Provider interface
│   │   ├── openai.py           # OpenAI provider
│   │   └── deepseek.py         # DeepSeek provider
│   │
│   ├── rag/                    # RAG components
│   │   ├── __init__.py
│   │   ├── indexer.py          # Document indexing
│   │   ├── retriever.py        # Context retrieval
│   │   ├── loaders/            # Document loaders
│   │   │   ├── __init__.py
│   │   │   ├── memo.py         # Memo loader
│   │   │   ├── pdf.py          # PDF attachment loader
│   │   │   └── image.py        # Image captioning
│   │   └── vector_store.py     # pgvector integration
│   │
│   ├── services/               # Business logic
│   │   ├── __init__.py
│   │   ├── conversation.py     # Conversation service
│   │   ├── chat.py             # Chat/message service
│   │   └── indexing.py         # Indexing service
│   │
│   └── webhooks/               # Webhook handlers (HTTP endpoint)
│       ├── __init__.py
│       ├── server.py           # Lightweight HTTP server for webhooks
│       └── handlers.py         # Memo/attachment change handlers
│
└── tests/
    ├── __init__.py
    ├── conftest.py             # Pytest fixtures
    ├── test_auth.py
    ├── test_servicers.py
    └── test_rag.py
```

### 1.2 pyproject.toml

```toml
[project]
name = "memos-ai-service"
version = "0.1.0"
description = "AI microservice for Memos with RAG capabilities (gRPC)"
requires-python = ">=3.11"
dependencies = [
    # gRPC
    "grpcio>=1.60.0",
    "grpcio-tools>=1.60.0",
    "grpcio-reflection>=1.60.0",
    "grpcio-health-checking>=1.60.0",

    # Database
    "sqlalchemy>=2.0.25",
    "asyncpg>=0.29.0",
    "alembic>=1.13.1",
    "pgvector>=0.2.4",
    "psycopg2-binary>=2.9.9",

    # Authentication
    "pyjwt>=2.8.0",
    "cryptography>=42.0.0",

    # LlamaIndex & LLM
    "llama-index>=0.10.0",
    "llama-index-vector-stores-postgres>=0.1.0",
    "llama-index-embeddings-openai>=0.1.0",
    "llama-index-llms-openai>=0.1.0",
    "openai>=1.10.0",

    # Document processing
    "pypdf>=4.0.0",
    "python-magic>=0.4.27",

    # HTTP server for webhooks
    "aiohttp>=3.9.0",

    # Utilities
    "pydantic>=2.5.0",
    "pydantic-settings>=2.1.0",
    "httpx>=0.26.0",
    "tenacity>=8.2.3",

    # Protobuf
    "protobuf>=4.25.0",
]

[project.optional-dependencies]
dev = [
    "pytest>=7.4.0",
    "pytest-asyncio>=0.23.0",
    "pytest-cov>=4.1.0",
    "grpcio-testing>=1.60.0",
    "ruff>=0.1.0",
    "mypy>=1.8.0",
    "mypy-protobuf>=3.5.0",
]

[tool.ruff]
line-length = 120
target-version = "py311"

[tool.ruff.lint]
select = ["E", "F", "I", "N", "W", "UP"]

[tool.mypy]
python_version = "3.11"
strict = true
plugins = ["mypy_protobuf.main"]
```

### 1.3 Protocol Buffer Definition (proto/api/v1/ai_service.proto)

Use the same proto file from Go backend to ensure compatibility:

```protobuf
// Copy from: proto/api/v1/ai_service.proto
// This ensures Go and Python use identical message formats

syntax = "proto3";

package memos.api.v1;

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "gen/api/v1";

// AIService provides AI chat functionality.
service AIService {
  // CreateConversation creates a new AI conversation.
  rpc CreateConversation(CreateConversationRequest) returns (Conversation);

  // ListConversations lists all conversations for the current user.
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);

  // GetConversation gets a specific conversation with messages.
  rpc GetConversation(GetConversationRequest) returns (Conversation);

  // DeleteConversation deletes a conversation and all its messages.
  rpc DeleteConversation(DeleteConversationRequest) returns (google.protobuf.Empty);

  // UpdateConversation updates conversation metadata (e.g., title).
  rpc UpdateConversation(UpdateConversationRequest) returns (Conversation);

  // SendMessage sends a message and gets AI response.
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);

  // ListMessages lists all messages in a conversation.
  rpc ListMessages(ListMessagesRequest) returns (ListMessagesResponse);

  // GetAIConfig returns available AI providers and models.
  rpc GetAIConfig(GetAIConfigRequest) returns (GetAIConfigResponse);
}

// ... (rest of the proto definitions - same as existing)
```

### 1.4 Buf Configuration (proto/buf.yaml)

```yaml
version: v1
breaking:
  use:
    - FILE
lint:
  use:
    - DEFAULT
```

### 1.5 Buf Generation Config (proto/buf.gen.yaml)

```yaml
version: v1
plugins:
  # Python protobuf
  - plugin: buf.build/protocolbuffers/python
    out: ../src/proto
  # Python gRPC
  - plugin: buf.build/grpc/python
    out: ../src/proto
  # Python type stubs (optional, for mypy)
  - plugin: buf.build/community/nipunn1313-mypy
    out: ../src/proto
```

### 1.6 Configuration (src/config.py)

```python
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
```

---

## Phase 2: Authentication Layer

### 2.1 JWT Validation (src/auth/jwt.py)

```python
"""JWT validation matching Go backend implementation.

This module implements JWT validation that is compatible with the Go backend's
token generation. The token structure and validation rules must match exactly.

Go backend reference: server/auth/token.go
"""

from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

import jwt
from jwt.exceptions import InvalidTokenError

from src.config import get_settings


class JWTValidationError(Exception):
    """Raised when JWT validation fails."""

    def __init__(self, message: str, details: str | None = None):
        self.message = message
        self.details = details
        super().__init__(message)


@dataclass
class AccessTokenClaims:
    """Claims from a validated access token.

    Matches Go struct: server/auth/token.go:AccessTokenClaims
    """

    user_id: int
    username: str
    role: str  # USER, HOST, ADMIN
    status: str  # NORMAL, ARCHIVED
    issued_at: datetime
    expires_at: datetime


def validate_access_token(token: str) -> AccessTokenClaims:
    """Validate a JWT access token and return claims.

    Validation rules (must match Go backend):
    1. Algorithm must be HS256
    2. Key ID (kid) in header must be "v1"
    3. Issuer must be "memos"
    4. Audience must contain "user.access-token"
    5. Token must not be expired
    6. Token type must be "access"

    Args:
        token: The JWT token string (without "Bearer " prefix)

    Returns:
        AccessTokenClaims with user information

    Raises:
        JWTValidationError: If token is invalid
    """
    settings = get_settings()

    try:
        # First, decode header without verification to check kid
        unverified_header = jwt.get_unverified_header(token)

        # Validate key ID (matches Go: verifyJWTKeyFunc)
        kid = unverified_header.get("kid")
        if kid != settings.jwt_key_id:
            raise JWTValidationError(
                "Invalid token",
                f"Unexpected key ID: {kid}, expected: {settings.jwt_key_id}"
            )

        # Validate algorithm
        alg = unverified_header.get("alg")
        if alg != settings.jwt_algorithm:
            raise JWTValidationError(
                "Invalid token",
                f"Unexpected algorithm: {alg}, expected: {settings.jwt_algorithm}"
            )

        # Decode and verify token
        payload: dict[str, Any] = jwt.decode(
            token,
            settings.jwt_secret,
            algorithms=[settings.jwt_algorithm],
            issuer=settings.jwt_issuer,
            audience=settings.jwt_audience,
            options={
                "require": ["exp", "iat", "sub", "iss", "aud"],
                "verify_exp": True,
                "verify_iat": True,
            },
        )

        # Validate token type (matches Go: ParseAccessTokenV2)
        token_type = payload.get("type")
        if token_type != "access":
            raise JWTValidationError(
                "Invalid token type",
                f"Expected 'access', got '{token_type}'"
            )

        # Extract user ID from subject
        try:
            user_id = int(payload["sub"])
        except (ValueError, TypeError) as e:
            raise JWTValidationError("Invalid user ID in token", str(e))

        # Build claims object
        return AccessTokenClaims(
            user_id=user_id,
            username=payload.get("username", ""),
            role=payload.get("role", "USER"),
            status=payload.get("status", "NORMAL"),
            issued_at=datetime.fromtimestamp(payload["iat"], tz=timezone.utc),
            expires_at=datetime.fromtimestamp(payload["exp"], tz=timezone.utc),
        )

    except jwt.ExpiredSignatureError:
        raise JWTValidationError("Token expired")
    except jwt.InvalidAudienceError:
        raise JWTValidationError("Invalid audience")
    except jwt.InvalidIssuerError:
        raise JWTValidationError("Invalid issuer")
    except InvalidTokenError as e:
        raise JWTValidationError("Invalid token", str(e))


def is_personal_access_token(token: str) -> bool:
    """Check if token is a Personal Access Token (PAT).

    PATs start with 'memos_pat_' prefix.
    """
    return token.startswith("memos_pat_")
```

### 2.2 gRPC Auth Interceptor (src/auth/interceptor.py)

```python
"""gRPC authentication interceptor."""

import logging
from typing import Any, Callable

import grpc
import httpx

from src.auth.jwt import (
    AccessTokenClaims,
    JWTValidationError,
    is_personal_access_token,
    validate_access_token,
)
from src.config import get_settings

logger = logging.getLogger(__name__)

# Metadata key for storing user claims
USER_CLAIMS_KEY = "user_claims"

# Methods that don't require authentication
PUBLIC_METHODS = frozenset([
    "/memos.api.v1.AIService/GetAIConfig",
])


def extract_bearer_token(metadata: tuple[tuple[str, str], ...]) -> str | None:
    """Extract Bearer token from gRPC metadata."""
    for key, value in metadata:
        if key.lower() == "authorization":
            parts = value.split(" ", 1)
            if len(parts) == 2 and parts[0].lower() == "bearer":
                return parts[1]
    return None


async def validate_pat_with_go_backend(token: str) -> AccessTokenClaims:
    """Validate PAT by calling Go backend.

    Since PATs are stored in the Go backend's database, we need to
    call the Go backend to validate them.
    """
    settings = get_settings()

    async with httpx.AsyncClient() as client:
        try:
            response = await client.get(
                f"{settings.go_backend_url}/api/v1/auth/status",
                headers={"Authorization": f"Bearer {token}"},
                timeout=5.0,
            )

            if response.status_code == 200:
                data = response.json()
                return AccessTokenClaims(
                    user_id=data["user"]["id"],
                    username=data["user"]["username"],
                    role=data["user"]["role"],
                    status="NORMAL",
                    issued_at=None,
                    expires_at=None,
                )
            else:
                raise JWTValidationError("Invalid Personal Access Token")

        except httpx.RequestError as e:
            raise JWTValidationError(f"Cannot validate PAT: {str(e)}")


class AuthInterceptor(grpc.aio.ServerInterceptor):
    """gRPC server interceptor for authentication.

    This interceptor:
    1. Extracts the Authorization header from metadata
    2. Validates JWT tokens locally or PATs via Go backend
    3. Adds user claims to the context for servicers to use
    4. Returns UNAUTHENTICATED error if validation fails
    """

    async def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        """Intercept incoming RPC calls for authentication."""
        method = handler_call_details.method

        # Skip auth for public methods
        if method in PUBLIC_METHODS:
            return await continuation(handler_call_details)

        # Extract token from metadata
        metadata = dict(handler_call_details.invocation_metadata or [])
        auth_header = metadata.get("authorization", "")

        token = None
        if auth_header.lower().startswith("bearer "):
            token = auth_header[7:]

        if not token:
            # Return an error handler that sends UNAUTHENTICATED
            return self._unauthenticated_handler("Missing authorization header")

        try:
            # Validate token
            if is_personal_access_token(token):
                # PAT validation requires async call to Go backend
                # For simplicity, we'll handle this in the servicer
                # and pass a flag indicating PAT validation is needed
                claims = await validate_pat_with_go_backend(token)
            else:
                claims = validate_access_token(token)

            # Check if user is archived
            if claims.status == "ARCHIVED":
                return self._unauthenticated_handler("User account is archived")

            # Store claims in context for servicer to access
            # We'll use a custom context variable
            handler_call_details.user_claims = claims

        except JWTValidationError as e:
            logger.warning(f"Authentication failed: {e.message}")
            return self._unauthenticated_handler(e.message)

        return await continuation(handler_call_details)

    def _unauthenticated_handler(self, message: str):
        """Return a handler that sends UNAUTHENTICATED error."""
        async def handler(request, context):
            await context.abort(grpc.StatusCode.UNAUTHENTICATED, message)
        return grpc.unary_unary_rpc_method_handler(handler)


class AuthContext:
    """Context holder for authenticated user information.

    Used to pass user claims through the gRPC context.
    """

    _claims_key = "auth_claims"

    @classmethod
    def set_claims(cls, context: grpc.aio.ServicerContext, claims: AccessTokenClaims) -> None:
        """Store claims in the servicer context."""
        # gRPC Python doesn't have built-in context values like Go
        # We use a workaround by storing in the context object
        setattr(context, cls._claims_key, claims)

    @classmethod
    def get_claims(cls, context: grpc.aio.ServicerContext) -> AccessTokenClaims | None:
        """Retrieve claims from the servicer context."""
        return getattr(context, cls._claims_key, None)

    @classmethod
    def require_claims(cls, context: grpc.aio.ServicerContext) -> AccessTokenClaims:
        """Get claims or raise UNAUTHENTICATED error."""
        claims = cls.get_claims(context)
        if claims is None:
            raise grpc.RpcError(grpc.StatusCode.UNAUTHENTICATED, "Not authenticated")
        return claims
```

---

## Phase 3: Database Layer

### 3.1 Database Setup (src/db/database.py)

```python
"""Database configuration and session management."""

from collections.abc import Generator
from contextlib import contextmanager

from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from src.config import get_settings


class Base(DeclarativeBase):
    """Base class for SQLAlchemy models."""
    pass


# Create engine (synchronous for gRPC)
settings = get_settings()
engine = create_engine(
    settings.database_url,
    echo=settings.debug,
    pool_size=10,
    max_overflow=20,
    pool_pre_ping=True,
)

# Session factory
SessionLocal = sessionmaker(
    bind=engine,
    autocommit=False,
    autoflush=False,
)


def get_db() -> Generator[Session, None, None]:
    """Get database session."""
    db = SessionLocal()
    try:
        yield db
        db.commit()
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


@contextmanager
def get_db_context() -> Generator[Session, None, None]:
    """Context manager for database session."""
    db = SessionLocal()
    try:
        yield db
        db.commit()
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def init_db() -> None:
    """Initialize database (create tables)."""
    # Enable pgvector extension
    with engine.connect() as conn:
        conn.execute("CREATE EXTENSION IF NOT EXISTS vector")
        conn.commit()
    Base.metadata.create_all(bind=engine)


def close_db() -> None:
    """Close database connections."""
    engine.dispose()
```

### 3.2 Database Models (src/db/models/)

**src/db/models/conversation.py**

```python
"""AI Conversation model."""

import uuid
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING

from sqlalchemy import BigInteger, Boolean, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from src.db.database import Base

if TYPE_CHECKING:
    from src.db.models.message import Message


class RowStatus(str, Enum):
    NORMAL = "NORMAL"
    ARCHIVED = "ARCHIVED"


class Conversation(Base):
    """AI conversation model.

    Matches Go struct: store/ai_conversation.go:AIConversation
    """

    __tablename__ = "ai_conversations"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    user_id: Mapped[int] = mapped_column(BigInteger, nullable=False, index=True)
    title: Mapped[str] = mapped_column(String(512), nullable=False, default="New Chat")
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    updated_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    row_status: Mapped[RowStatus] = mapped_column(String(16), nullable=False, default=RowStatus.NORMAL)
    model: Mapped[str] = mapped_column(String(128), nullable=False, default="")
    provider: Mapped[str] = mapped_column(String(64), nullable=False, default="")

    # RAG settings for this conversation
    rag_enabled: Mapped[bool] = mapped_column(Boolean, default=True)
    system_prompt: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Relationships
    messages: Mapped[list["Message"]] = relationship(
        "Message",
        back_populates="conversation",
        cascade="all, delete-orphan",
        order_by="Message.created_ts",
    )

    __table_args__ = (
        Index("idx_ai_conversation_user_id", "user_id"),
        Index("idx_ai_conversation_created_ts", "created_ts"),
    )
```

**src/db/models/message.py**

```python
"""AI Message model."""

import uuid
from datetime import datetime
from enum import Enum
from typing import TYPE_CHECKING

from sqlalchemy import BigInteger, ForeignKey, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from src.db.database import Base

if TYPE_CHECKING:
    from src.db.models.conversation import Conversation


class MessageRole(str, Enum):
    USER = "user"
    ASSISTANT = "assistant"
    SYSTEM = "system"


class Message(Base):
    """AI message model.

    Matches Go struct: store/ai_message.go:AIMessage
    """

    __tablename__ = "ai_messages"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))
    conversation_id: Mapped[int] = mapped_column(
        BigInteger,
        ForeignKey("ai_conversations.id", ondelete="CASCADE"),
        nullable=False,
    )
    role: Mapped[MessageRole] = mapped_column(String(16), nullable=False)
    content: Mapped[str] = mapped_column(Text, nullable=False, default="")
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    token_count: Mapped[int] = mapped_column(BigInteger, nullable=False, default=0)

    # RAG context (JSON array of referenced document IDs)
    rag_context: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Relationships
    conversation: Mapped["Conversation"] = relationship("Conversation", back_populates="messages")

    __table_args__ = (
        Index("idx_ai_message_conversation_id", "conversation_id"),
        Index("idx_ai_message_created_ts", "created_ts"),
    )
```

**src/db/models/embedding.py**

```python
"""Document embedding model for RAG."""

import uuid
from datetime import datetime
from enum import Enum

from pgvector.sqlalchemy import Vector
from sqlalchemy import BigInteger, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from src.db.database import Base
from src.config import get_settings


class DocumentType(str, Enum):
    MEMO = "memo"
    ATTACHMENT = "attachment"


class DocumentEmbedding(Base):
    """Document embedding for RAG retrieval.

    Stores vector embeddings of user documents (memos, attachments)
    for semantic search.
    """

    __tablename__ = "document_embeddings"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    uid: Mapped[str] = mapped_column(String(256), unique=True, nullable=False, default=lambda: str(uuid.uuid4()))

    # Document reference
    user_id: Mapped[int] = mapped_column(BigInteger, nullable=False, index=True)
    document_type: Mapped[DocumentType] = mapped_column(String(32), nullable=False)
    document_uid: Mapped[str] = mapped_column(String(256), nullable=False)  # memo UID or attachment UID

    # Chunk information (for long documents)
    chunk_index: Mapped[int] = mapped_column(BigInteger, nullable=False, default=0)
    chunk_text: Mapped[str] = mapped_column(Text, nullable=False)

    # Embedding vector
    embedding: Mapped[list[float]] = mapped_column(
        Vector(get_settings().embedding_dimensions),
        nullable=False,
    )

    # Metadata
    created_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))
    updated_ts: Mapped[int] = mapped_column(BigInteger, nullable=False, default=lambda: int(datetime.utcnow().timestamp()))

    __table_args__ = (
        Index("idx_document_embedding_user_id", "user_id"),
        Index("idx_document_embedding_document", "document_type", "document_uid"),
    )
```

---

## Phase 4: gRPC Server & Servicers

### 4.1 gRPC Server Setup (src/grpc/server.py)

```python
"""gRPC server setup and lifecycle management."""

import logging
from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc
from grpc_reflection.v1alpha import reflection

from src.auth.interceptor import AuthInterceptor
from src.config import get_settings
from src.grpc.interceptors import LoggingInterceptor, RecoveryInterceptor
from src.grpc.servicers.ai_servicer import AIServiceServicer
from src.proto.api.v1 import ai_service_pb2, ai_service_pb2_grpc

logger = logging.getLogger(__name__)


def create_server() -> grpc.Server:
    """Create and configure gRPC server with all services."""
    settings = get_settings()

    # Create interceptor chain (order matters: first added = outermost)
    interceptors = [
        RecoveryInterceptor(),      # Catch panics
        LoggingInterceptor(),       # Log requests
        AuthInterceptor(),          # Authenticate
    ]

    # Create server with thread pool
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=settings.grpc_max_workers),
        interceptors=interceptors,
        options=[
            ("grpc.max_send_message_length", 50 * 1024 * 1024),  # 50MB
            ("grpc.max_receive_message_length", 50 * 1024 * 1024),  # 50MB
        ],
    )

    # Register AI service
    ai_servicer = AIServiceServicer()
    ai_service_pb2_grpc.add_AIServiceServicer_to_server(ai_servicer, server)

    # Register health check service
    health_servicer = health.HealthServicer()
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    # Set health status
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_servicer.set(
        ai_service_pb2.DESCRIPTOR.services_by_name["AIService"].full_name,
        health_pb2.HealthCheckResponse.SERVING,
    )

    # Enable reflection for debugging (disable in production if needed)
    service_names = (
        ai_service_pb2.DESCRIPTOR.services_by_name["AIService"].full_name,
        health_pb2.DESCRIPTOR.services_by_name["Health"].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    # Bind to port
    address = f"{settings.grpc_host}:{settings.grpc_port}"
    server.add_insecure_port(address)
    logger.info(f"gRPC server configured on {address}")

    return server


def serve() -> None:
    """Start gRPC server and block until termination."""
    settings = get_settings()
    server = create_server()

    server.start()
    logger.info(f"gRPC server started on {settings.grpc_host}:{settings.grpc_port}")

    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down gRPC server...")
        server.stop(grace=5)
        logger.info("gRPC server stopped")
```

### 4.2 gRPC Interceptors (src/grpc/interceptors.py)

```python
"""gRPC server interceptors for logging and error handling."""

import logging
import time
import traceback
from typing import Any, Callable

import grpc

logger = logging.getLogger(__name__)


class LoggingInterceptor(grpc.ServerInterceptor):
    """Interceptor that logs all RPC calls."""

    def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        """Log incoming RPC calls."""
        method = handler_call_details.method
        start_time = time.time()

        logger.info(f"gRPC call started: {method}")

        try:
            response = continuation(handler_call_details)
            elapsed = (time.time() - start_time) * 1000
            logger.info(f"gRPC call completed: {method} ({elapsed:.2f}ms)")
            return response
        except Exception as e:
            elapsed = (time.time() - start_time) * 1000
            logger.error(f"gRPC call failed: {method} ({elapsed:.2f}ms) - {e}")
            raise


class RecoveryInterceptor(grpc.ServerInterceptor):
    """Interceptor that recovers from panics/exceptions."""

    def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        """Catch unhandled exceptions and return INTERNAL error."""
        try:
            return continuation(handler_call_details)
        except grpc.RpcError:
            # Re-raise gRPC errors (already properly formatted)
            raise
        except Exception as e:
            logger.exception(f"Unhandled exception in {handler_call_details.method}")

            # Return a handler that sends INTERNAL error
            def error_handler(request, context):
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Internal server error: {str(e)}")
                return None

            return grpc.unary_unary_rpc_method_handler(error_handler)
```

### 4.3 AI Service Servicer (src/grpc/servicers/ai_servicer.py)

```python
"""gRPC AIService implementation."""

import json
import logging
import uuid
from datetime import datetime

import grpc
from google.protobuf import empty_pb2, timestamp_pb2

from src.auth.interceptor import AuthContext
from src.auth.jwt import AccessTokenClaims
from src.config import get_settings
from src.db.database import get_db_context
from src.db.models.conversation import Conversation, RowStatus
from src.db.models.message import Message, MessageRole
from src.llm import get_llm_provider
from src.llm.base import ChatMessage
from src.proto.api.v1 import ai_service_pb2, ai_service_pb2_grpc
from src.rag.retriever import RAGRetriever

logger = logging.getLogger(__name__)


class AIServiceServicer(ai_service_pb2_grpc.AIServiceServicer):
    """Implementation of AIService gRPC service."""

    def _get_user_claims(self, context: grpc.ServicerContext) -> AccessTokenClaims:
        """Get authenticated user claims from context."""
        claims = AuthContext.get_claims(context)
        if claims is None:
            context.abort(grpc.StatusCode.UNAUTHENTICATED, "Not authenticated")
        return claims

    def _timestamp_from_unix(self, unix_ts: int) -> timestamp_pb2.Timestamp:
        """Convert Unix timestamp to protobuf Timestamp."""
        ts = timestamp_pb2.Timestamp()
        ts.FromSeconds(unix_ts)
        return ts

    def _conversation_to_proto(
        self,
        conv: Conversation,
        username: str,
        include_messages: bool = False,
    ) -> ai_service_pb2.Conversation:
        """Convert database Conversation to protobuf."""
        proto = ai_service_pb2.Conversation(
            id=conv.uid,
            user=f"users/{username}",
            title=conv.title,
            model=conv.model,
            provider=conv.provider,
            create_time=self._timestamp_from_unix(conv.created_ts),
            update_time=self._timestamp_from_unix(conv.updated_ts),
        )

        if include_messages:
            for msg in conv.messages:
                proto.messages.append(self._message_to_proto(msg))

        return proto

    def _message_to_proto(self, msg: Message) -> ai_service_pb2.Message:
        """Convert database Message to protobuf."""
        role_map = {
            MessageRole.USER: ai_service_pb2.MessageRole.USER,
            MessageRole.ASSISTANT: ai_service_pb2.MessageRole.ASSISTANT,
            MessageRole.SYSTEM: ai_service_pb2.MessageRole.SYSTEM,
        }
        return ai_service_pb2.Message(
            id=msg.uid,
            role=role_map.get(msg.role, ai_service_pb2.MessageRole.MESSAGE_ROLE_UNSPECIFIED),
            content=msg.content,
            create_time=self._timestamp_from_unix(msg.created_ts),
            token_count=msg.token_count,
        )

    def CreateConversation(
        self,
        request: ai_service_pb2.CreateConversationRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.Conversation:
        """Create a new AI conversation."""
        claims = self._get_user_claims(context)
        settings = get_settings()

        title = request.title or "New Chat"
        model = request.model or settings.default_model
        provider = request.provider or settings.default_provider

        with get_db_context() as db:
            conv = Conversation(
                uid=str(uuid.uuid4()),
                user_id=claims.user_id,
                title=title,
                model=model,
                provider=provider,
                rag_enabled=True,
            )
            db.add(conv)
            db.flush()
            db.refresh(conv)

            return self._conversation_to_proto(conv, claims.username)

    def ListConversations(
        self,
        request: ai_service_pb2.ListConversationsRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.ListConversationsResponse:
        """List all conversations for the current user."""
        claims = self._get_user_claims(context)

        with get_db_context() as db:
            conversations = (
                db.query(Conversation)
                .filter(Conversation.user_id == claims.user_id)
                .filter(Conversation.row_status == RowStatus.NORMAL)
                .order_by(Conversation.updated_ts.desc())
                .all()
            )

            response = ai_service_pb2.ListConversationsResponse()
            for conv in conversations:
                response.conversations.append(
                    self._conversation_to_proto(conv, claims.username)
                )
            return response

    def GetConversation(
        self,
        request: ai_service_pb2.GetConversationRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.Conversation:
        """Get a specific conversation with messages."""
        claims = self._get_user_claims(context)

        with get_db_context() as db:
            conv = (
                db.query(Conversation)
                .filter(Conversation.uid == request.conversation_id)
                .first()
            )

            if not conv:
                context.abort(grpc.StatusCode.NOT_FOUND, "Conversation not found")

            if conv.user_id != claims.user_id:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "Access denied")

            return self._conversation_to_proto(conv, claims.username, include_messages=True)

    def DeleteConversation(
        self,
        request: ai_service_pb2.DeleteConversationRequest,
        context: grpc.ServicerContext,
    ) -> empty_pb2.Empty:
        """Delete a conversation and all its messages."""
        claims = self._get_user_claims(context)

        with get_db_context() as db:
            conv = (
                db.query(Conversation)
                .filter(Conversation.uid == request.conversation_id)
                .first()
            )

            if not conv:
                context.abort(grpc.StatusCode.NOT_FOUND, "Conversation not found")

            if conv.user_id != claims.user_id:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "Access denied")

            db.delete(conv)

        return empty_pb2.Empty()

    def UpdateConversation(
        self,
        request: ai_service_pb2.UpdateConversationRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.Conversation:
        """Update conversation metadata."""
        claims = self._get_user_claims(context)

        with get_db_context() as db:
            conv = (
                db.query(Conversation)
                .filter(Conversation.uid == request.conversation_id)
                .first()
            )

            if not conv:
                context.abort(grpc.StatusCode.NOT_FOUND, "Conversation not found")

            if conv.user_id != claims.user_id:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "Access denied")

            if request.title:
                conv.title = request.title
            if request.model:
                conv.model = request.model
            if request.provider:
                conv.provider = request.provider

            conv.updated_ts = int(datetime.utcnow().timestamp())
            db.flush()
            db.refresh(conv)

            return self._conversation_to_proto(conv, claims.username, include_messages=True)

    def SendMessage(
        self,
        request: ai_service_pb2.SendMessageRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.SendMessageResponse:
        """Send a message and get AI response."""
        claims = self._get_user_claims(context)
        settings = get_settings()

        with get_db_context() as db:
            # Get conversation
            conv = (
                db.query(Conversation)
                .filter(Conversation.uid == request.conversation_id)
                .first()
            )

            if not conv:
                context.abort(grpc.StatusCode.NOT_FOUND, "Conversation not found")

            if conv.user_id != claims.user_id:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "Access denied")

            # Save user message
            user_msg = Message(
                uid=str(uuid.uuid4()),
                conversation_id=conv.id,
                role=MessageRole.USER,
                content=request.content,
            )
            db.add(user_msg)
            db.flush()

            # Get RAG context if enabled
            rag_context = ""
            rag_docs = []
            if conv.rag_enabled and settings.rag_enabled:
                try:
                    retriever = RAGRetriever(db)
                    rag_docs = retriever.retrieve(
                        user_id=claims.user_id,
                        query=request.content,
                    )
                    rag_context = retriever.format_context(rag_docs)
                except Exception as e:
                    logger.warning(f"RAG retrieval failed: {e}")

            # Build LLM messages
            llm_messages = self._build_llm_messages(conv, rag_context)

            # Call LLM
            provider = get_llm_provider(conv.provider or None)
            model = conv.model or settings.default_model

            try:
                llm_response = provider.complete(
                    messages=llm_messages,
                    model=model,
                )
            except Exception as e:
                logger.error(f"LLM call failed: {e}")
                context.abort(grpc.StatusCode.INTERNAL, f"AI generation failed: {str(e)}")

            # Save assistant message
            rag_context_json = json.dumps([
                {"type": doc.document_type, "uid": doc.document_uid, "score": doc.score}
                for doc in rag_docs
            ]) if rag_docs else None

            assistant_msg = Message(
                uid=str(uuid.uuid4()),
                conversation_id=conv.id,
                role=MessageRole.ASSISTANT,
                content=llm_response.content,
                token_count=llm_response.token_count,
                rag_context=rag_context_json,
            )
            db.add(assistant_msg)

            # Update conversation title if first exchange
            if len(conv.messages) <= 1:
                title = request.content[:50]
                if len(request.content) > 50:
                    title += "..."
                conv.title = title

            conv.updated_ts = int(datetime.utcnow().timestamp())
            db.flush()
            db.refresh(user_msg)
            db.refresh(assistant_msg)

            return ai_service_pb2.SendMessageResponse(
                user_message=self._message_to_proto(user_msg),
                assistant_message=self._message_to_proto(assistant_msg),
            )

    def _build_llm_messages(
        self,
        conv: Conversation,
        rag_context: str,
    ) -> list[ChatMessage]:
        """Build message list for LLM including history and RAG context."""
        messages: list[ChatMessage] = []

        # System prompt
        system_content = conv.system_prompt or "You are a helpful AI assistant."
        if rag_context:
            system_content += f"\n\n{rag_context}\n\nUse the above context from the user's notes to help answer their question when relevant."

        messages.append(ChatMessage(role="system", content=system_content))

        # Conversation history (limit to last 20 messages)
        history = conv.messages[-20:]
        for msg in history:
            messages.append(ChatMessage(
                role=msg.role.value,
                content=msg.content,
            ))

        return messages

    def ListMessages(
        self,
        request: ai_service_pb2.ListMessagesRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.ListMessagesResponse:
        """List all messages in a conversation."""
        claims = self._get_user_claims(context)

        with get_db_context() as db:
            conv = (
                db.query(Conversation)
                .filter(Conversation.uid == request.conversation_id)
                .first()
            )

            if not conv:
                context.abort(grpc.StatusCode.NOT_FOUND, "Conversation not found")

            if conv.user_id != claims.user_id:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "Access denied")

            response = ai_service_pb2.ListMessagesResponse()
            for msg in conv.messages:
                response.messages.append(self._message_to_proto(msg))
            return response

    def GetAIConfig(
        self,
        request: ai_service_pb2.GetAIConfigRequest,
        context: grpc.ServicerContext,
    ) -> ai_service_pb2.GetAIConfigResponse:
        """Get AI configuration (public endpoint)."""
        settings = get_settings()

        providers = []

        if settings.openai_api_key:
            providers.append(ai_service_pb2.AIProvider(
                name="openai",
                display_name="OpenAI",
                models=["gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"],
            ))

        if settings.deepseek_api_key:
            providers.append(ai_service_pb2.AIProvider(
                name="deepseek",
                display_name="DeepSeek",
                models=["deepseek-chat", "deepseek-coder"],
            ))

        return ai_service_pb2.GetAIConfigResponse(
            enabled=settings.ai_enabled and len(providers) > 0,
            providers=providers,
            default_provider=settings.default_provider,
            default_model=settings.default_model,
        )
```

### 4.4 Main Entry Point (src/main.py)

```python
"""Application entry point - runs gRPC server and HTTP webhook server."""

import asyncio
import logging
import signal
import sys
import threading
from concurrent.futures import ThreadPoolExecutor

from src.config import get_settings
from src.db.database import close_db, init_db
from src.grpc.server import create_server
from src.webhooks.server import create_webhook_server

# Configure logging
logging.basicConfig(
    level=logging.DEBUG if get_settings().debug else logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


def run_grpc_server(server):
    """Run gRPC server in a thread."""
    settings = get_settings()
    server.start()
    logger.info(f"gRPC server started on {settings.grpc_host}:{settings.grpc_port}")
    server.wait_for_termination()


async def run_webhook_server():
    """Run webhook HTTP server."""
    settings = get_settings()
    app = create_webhook_server()

    from aiohttp import web
    runner = web.AppRunner(app)
    await runner.setup()

    site = web.TCPSite(runner, settings.http_host, settings.http_port)
    await site.start()
    logger.info(f"Webhook HTTP server started on {settings.http_host}:{settings.http_port}")

    # Keep running
    while True:
        await asyncio.sleep(3600)


def main():
    """Main entry point."""
    settings = get_settings()

    logger.info("Starting Memos AI Service...")

    # Initialize database
    logger.info("Initializing database...")
    init_db()
    logger.info("Database initialized")

    # Create gRPC server
    grpc_server = create_server()

    # Handle shutdown signals
    def shutdown(signum, frame):
        logger.info("Received shutdown signal...")
        grpc_server.stop(grace=5)
        close_db()
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    # Start gRPC server in background thread
    grpc_thread = threading.Thread(target=run_grpc_server, args=(grpc_server,), daemon=True)
    grpc_thread.start()

    # Run webhook server in main thread (async)
    try:
        asyncio.run(run_webhook_server())
    except KeyboardInterrupt:
        logger.info("Shutting down...")
        grpc_server.stop(grace=5)
        close_db()


if __name__ == "__main__":
    main()
```

---

## Phase 5: RAG Implementation

### 5.1 Vector Store (src/rag/vector_store.py)

```python
"""PostgreSQL pgvector store for document embeddings."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from src.config import get_settings


async def search_similar_documents(
    db: Session,
    user_id: int,
    query_embedding: list[float],
    top_k: int = 5,
    threshold: float = 0.7,
) -> list[dict[str, Any]]:
    """Search for similar documents using pgvector.

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
```

### 5.2 RAG Retriever (src/rag/retriever.py)

```python
"""RAG context retrieval."""

import logging
from dataclasses import dataclass

from openai import OpenAI
from sqlalchemy.orm import Session

from src.config import get_settings
from src.rag.vector_store import search_similar_documents

logger = logging.getLogger(__name__)


@dataclass
class RetrievedDocument:
    """A document retrieved for RAG context."""

    document_type: str  # "memo" or "attachment"
    document_uid: str
    chunk_text: str
    score: float
    chunk_index: int


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
        """Retrieve relevant documents for a query."""
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
        """Format retrieved documents as context for LLM prompt."""
        if not documents:
            return ""

        context_parts = ["The following information from your notes may be relevant:\n"]

        for i, doc in enumerate(documents, 1):
            source = "memo" if doc.document_type == "memo" else "attachment"
            context_parts.append(f"[{i}] From {source}:")
            context_parts.append(doc.chunk_text)
            context_parts.append("")

        return "\n".join(context_parts)
```

---

## Phase 6: LLM Integration

### 6.1 LLM Provider Interface (src/llm/base.py)

```python
"""LLM provider interface."""

from abc import ABC, abstractmethod
from dataclasses import dataclass


@dataclass
class ChatMessage:
    """A chat message."""
    role: str  # "user", "assistant", "system"
    content: str


@dataclass
class CompletionResponse:
    """LLM completion response."""
    content: str
    token_count: int
    finish_reason: str


class LLMProvider(ABC):
    """Abstract base class for LLM providers."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Provider name."""
        pass

    @abstractmethod
    def complete(
        self,
        messages: list[ChatMessage],
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float = 0.7,
    ) -> CompletionResponse:
        """Generate a completion."""
        pass
```

### 6.2 OpenAI Provider (src/llm/openai.py)

```python
"""OpenAI LLM provider."""

from openai import OpenAI

from src.config import get_settings
from src.llm.base import ChatMessage, CompletionResponse, LLMProvider


class OpenAIProvider(LLMProvider):
    """OpenAI LLM provider."""

    def __init__(self):
        settings = get_settings()
        self.client = OpenAI(
            api_key=settings.openai_api_key,
            base_url=settings.openai_base_url,
        )
        self._default_model = settings.default_model

    @property
    def name(self) -> str:
        return "openai"

    def complete(
        self,
        messages: list[ChatMessage],
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float = 0.7,
    ) -> CompletionResponse:
        model = model or self._default_model

        openai_messages = [
            {"role": msg.role, "content": msg.content}
            for msg in messages
        ]

        response = self.client.chat.completions.create(
            model=model,
            messages=openai_messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )

        choice = response.choices[0]
        usage = response.usage

        return CompletionResponse(
            content=choice.message.content or "",
            token_count=usage.total_tokens if usage else 0,
            finish_reason=choice.finish_reason or "stop",
        )
```

### 6.3 Provider Factory (src/llm/**init**.py)

```python
"""LLM provider factory."""

from src.config import get_settings
from src.llm.base import LLMProvider
from src.llm.openai import OpenAIProvider


def get_llm_provider(name: str | None = None) -> LLMProvider:
    """Get an LLM provider by name."""
    settings = get_settings()
    name = name or settings.default_provider

    if name == "openai":
        if not settings.openai_api_key:
            raise ValueError("OpenAI API key not configured")
        return OpenAIProvider()

    if name == "deepseek":
        if not settings.deepseek_api_key:
            raise ValueError("DeepSeek API key not configured")
        # DeepSeek uses OpenAI-compatible API
        from src.llm.openai import OpenAIProvider
        provider = OpenAIProvider()
        provider.client.base_url = settings.deepseek_base_url
        provider.client.api_key = settings.deepseek_api_key
        return provider

    raise ValueError(f"Unknown LLM provider: {name}")
```

---

## Phase 7: Webhook Handlers

### 7.1 Webhook HTTP Server (src/webhooks/server.py)

```python
"""Lightweight HTTP server for webhooks from Go backend."""

import hashlib
import hmac
import json
import logging

from aiohttp import web

from src.config import get_settings
from src.db.database import get_db_context
from src.rag.indexer import DocumentIndexer

logger = logging.getLogger(__name__)


def verify_signature(payload: bytes, signature: str, secret: str) -> bool:
    """Verify webhook signature using HMAC-SHA256."""
    expected = hmac.new(secret.encode(), payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)


async def handle_memo_webhook(request: web.Request) -> web.Response:
    """Handle memo create/update/delete webhooks."""
    settings = get_settings()

    # Verify signature
    signature = request.headers.get("X-Webhook-Signature", "")
    body = await request.read()

    if settings.webhook_secret and not verify_signature(body, signature, settings.webhook_secret):
        return web.json_response({"error": "Invalid signature"}, status=401)

    data = json.loads(body)
    event = data.get("event")

    with get_db_context() as db:
        indexer = DocumentIndexer(db)

        if event == "memo.deleted":
            count = indexer.delete_memo_index(data["memo_uid"])
            logger.info(f"Deleted index for memo {data['memo_uid']}: {count} chunks")
            return web.json_response({"status": "ok", "deleted_chunks": count})

        if event in ("memo.created", "memo.updated"):
            count = indexer.index_memo(
                user_id=data["user_id"],
                memo_uid=data["memo_uid"],
                content=data.get("content", ""),
            )
            logger.info(f"Indexed memo {data['memo_uid']}: {count} chunks")
            return web.json_response({"status": "ok", "indexed_chunks": count})

    return web.json_response({"error": f"Unknown event: {event}"}, status=400)


async def handle_health(request: web.Request) -> web.Response:
    """Health check endpoint."""
    return web.json_response({"status": "healthy"})


def create_webhook_server() -> web.Application:
    """Create aiohttp application for webhooks."""
    app = web.Application()
    app.router.add_post("/webhooks/memo", handle_memo_webhook)
    app.router.add_get("/health", handle_health)
    return app
```

---

## Phase 8: Nginx Configuration for gRPC

### 8.1 Production Nginx Config

```nginx
# /etc/nginx/conf.d/memos.conf

# Go backend (HTTP/1.1 and HTTP/2)
upstream go_backend {
    server 127.0.0.1:8081;
    keepalive 32;
}

# Python AI gRPC service (HTTP/2 required for gRPC)
upstream ai_grpc {
    server 127.0.0.1:50051;
    keepalive 16;
}

# Python AI webhook service (HTTP/1.1)
upstream ai_webhooks {
    server 127.0.0.1:8000;
    keepalive 8;
}

server {
    listen 80;
    server_name memos.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl;
    http2 on;  # Enable HTTP/2 (required for gRPC)
    server_name memos.yourdomain.com;

    # SSL Configuration
    ssl_certificate     /etc/nginx/ssl/memos.crt;
    ssl_certificate_key /etc/nginx/ssl/memos.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Logging
    access_log /var/log/nginx/memos_access.log;
    error_log  /var/log/nginx/memos_error.log;

    # gRPC AI Service routes (memos.api.v1.AIService/*)
    # These are called by the frontend via Connect RPC
    location ~ ^/memos\.api\.v1\.AIService/ {
        # gRPC proxy settings
        grpc_pass grpc://ai_grpc;

        # Pass original headers including Authorization
        grpc_set_header Host $host;
        grpc_set_header X-Real-IP $remote_addr;
        grpc_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        grpc_set_header X-Forwarded-Proto $scheme;

        # Timeouts for AI responses (LLM can be slow)
        grpc_read_timeout 120s;
        grpc_send_timeout 120s;

        # Error handling
        error_page 502 = /grpc_error_502;
        error_page 503 = /grpc_error_503;
        error_page 504 = /grpc_error_504;
    }

    # Internal webhook endpoint (Go backend → Python AI service)
    location /internal/ai/webhooks/ {
        # Only allow internal access
        allow 127.0.0.1;
        deny all;

        proxy_pass http://ai_webhooks/webhooks/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # All other gRPC routes → Go Backend
    location ~ ^/memos\.api\.v1\. {
        grpc_pass grpc://go_backend;
        grpc_set_header Host $host;
        grpc_set_header X-Real-IP $remote_addr;
        grpc_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        grpc_set_header X-Forwarded-Proto $scheme;
    }

    # REST API routes → Go Backend
    location /api/v1/ {
        proxy_pass http://go_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Authorization $http_authorization;
        proxy_set_header Connection "";
    }

    # Frontend & other routes → Go Backend
    location / {
        proxy_pass http://go_backend;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
    }

    # gRPC error pages
    location = /grpc_error_502 {
        internal;
        default_type application/grpc;
        add_header grpc-status 14;  # UNAVAILABLE
        add_header grpc-message "AI service unavailable";
        return 204;
    }

    location = /grpc_error_503 {
        internal;
        default_type application/grpc;
        add_header grpc-status 14;  # UNAVAILABLE
        add_header grpc-message "AI service temporarily unavailable";
        return 204;
    }

    location = /grpc_error_504 {
        internal;
        default_type application/grpc;
        add_header grpc-status 4;  # DEADLINE_EXCEEDED
        add_header grpc-message "AI request timeout";
        return 204;
    }
}
```

### 8.2 Key Nginx Configuration Notes

| Setting                                   | Purpose                                    |
| ----------------------------------------- | ------------------------------------------ |
| `http2 on`                                | Required for gRPC (uses HTTP/2 framing)    |
| `grpc_pass grpc://`                       | Routes to gRPC upstream (not `proxy_pass`) |
| `grpc_read_timeout 120s`                  | Long timeout for LLM responses             |
| `location ~ ^/memos\.api\.v1\.AIService/` | Regex match for AI service methods         |
| `grpc_set_header`                         | Pass headers to gRPC backend               |
| Error pages with `grpc-status`            | Return proper gRPC error codes             |

---

## Phase 9: Docker Deployment

### 9.1 AI Service Dockerfile

```dockerfile
# ai-service/Dockerfile
FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    libpq-dev \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
COPY pyproject.toml .
RUN pip install --no-cache-dir .

# Copy application code
COPY src/ src/
COPY alembic/ alembic/
COPY alembic.ini .

# Create non-root user
RUN useradd -m -u 1000 appuser && chown -R appuser:appuser /app
USER appuser

# Expose ports
EXPOSE 50051  # gRPC
EXPOSE 8000   # HTTP webhooks

CMD ["python", "-m", "src.main"]
```

### 9.2 Docker Compose (Production)

```yaml
# docker-compose.yml
version: "3.8"

services:
  memos:
    image: ghcr.io/usememos/memos:latest
    container_name: memos
    restart: unless-stopped
    ports:
      - "127.0.0.1:8081:8081"
    volumes:
      - memos_data:/var/opt/memos
    environment:
      - MEMOS_DRIVER=sqlite
      - MEMOS_DATA=/var/opt/memos
      - MEMOS_AI_WEBHOOK_URL=http://ai-service:8000
      - MEMOS_AI_WEBHOOK_SECRET=${WEBHOOK_SECRET}
    networks:
      - memos_network
    depends_on:
      - ai-service

  ai-service:
    build:
      context: ./ai-service
      dockerfile: Dockerfile
    container_name: memos-ai
    restart: unless-stopped
    ports:
      - "127.0.0.1:50051:50051" # gRPC
      - "127.0.0.1:8000:8000" # HTTP webhooks
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - POSTGRES_HOST=ai-db
      - POSTGRES_PORT=5432
      - POSTGRES_USER=memos_ai
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=memos_ai
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - DEEPSEEK_API_KEY=${DEEPSEEK_API_KEY}
      - GO_BACKEND_URL=http://memos:8081
      - WEBHOOK_SECRET=${WEBHOOK_SECRET}
      - GRPC_PORT=50051
      - HTTP_PORT=8000
    networks:
      - memos_network
    depends_on:
      ai-db:
        condition: service_healthy

  ai-db:
    image: pgvector/pgvector:pg16
    container_name: memos-ai-db
    restart: unless-stopped
    volumes:
      - ai_db_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=memos_ai
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=memos_ai
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U memos_ai -d memos_ai"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - memos_network

volumes:
  memos_data:
  ai_db_data:

networks:
  memos_network:
    driver: bridge
```

### 9.3 Environment Variables (.env.example)

```bash
# JWT Secret (MUST match Go backend MEMOS_JWT_SECRET)
JWT_SECRET=your-super-secret-jwt-key-min-32-chars

# Database
POSTGRES_PASSWORD=secure-password-here

# LLM Providers
OPENAI_API_KEY=sk-...
DEEPSEEK_API_KEY=sk-...

# Webhook Security
WEBHOOK_SECRET=webhook-secret-for-go-to-python
```

---

## Phase 10: Go Backend Changes

### 10.1 Add Webhook Dispatcher

```go
// plugin/webhook/ai_webhook.go
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AIWebhookClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewAIWebhookClient(baseURL, secret string) *AIWebhookClient {
	return &AIWebhookClient{
		baseURL: baseURL,
		secret:  secret,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

type MemoEvent struct {
	Event     string `json:"event"`
	Timestamp int64  `json:"timestamp"`
	UserID    int32  `json:"user_id"`
	MemoUID   string `json:"memo_uid"`
	Content   string `json:"content,omitempty"`
}

func (c *AIWebhookClient) NotifyMemoChange(ctx context.Context, event *MemoEvent) error {
	body, _ := json.Marshal(event)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/webhooks/memo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", c.sign(body))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *AIWebhookClient) sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
```

### 10.2 Remove Old AI Service Code

Delete or disable these files (AI now handled by Python service):

```
DELETE:
- server/router/api/v1/ai_service.go
- store/ai_conversation.go
- store/ai_message.go
- store/db/sqlite/ai_conversation.go
- store/db/sqlite/ai_message.go
- store/db/mysql/ai_conversation.go
- store/db/mysql/ai_message.go
- store/db/postgres/ai_conversation.go
- store/db/postgres/ai_message.go
```

---

## Implementation Checklist

### Phase 1: Project Setup

- [ ] Create `ai-service/` directory structure
- [ ] Create `pyproject.toml` with dependencies
- [ ] Create `src/config.py` configuration
- [ ] Copy `ai_service.proto` from Go backend
- [ ] Set up buf for Python code generation
- [ ] Generate Python protobuf code: `cd proto && buf generate`

### Phase 2: Authentication

- [ ] Implement `src/auth/jwt.py` (JWT validation matching Go)
- [ ] Implement `src/auth/interceptor.py` (gRPC auth interceptor)
- [ ] Write auth unit tests

### Phase 3: Database

- [ ] Create SQLAlchemy models
- [ ] Set up Alembic migrations
- [ ] Create initial migration with pgvector
- [ ] Test database connection

### Phase 4: gRPC Server

- [ ] Implement `src/grpc/server.py` (server setup)
- [ ] Implement `src/grpc/interceptors.py` (logging, recovery)
- [ ] Implement `src/grpc/servicers/ai_servicer.py` (all methods)
- [ ] Test with grpcurl

### Phase 5: RAG Implementation

- [ ] Implement `src/rag/vector_store.py` (pgvector queries)
- [ ] Implement `src/rag/retriever.py` (embedding + search)
- [ ] Implement `src/rag/indexer.py` (document indexing)

### Phase 6: LLM Integration

- [ ] Implement `src/llm/base.py` (provider interface)
- [ ] Implement `src/llm/openai.py` (OpenAI provider)
- [ ] Test LLM completion

### Phase 7: Webhooks

- [ ] Implement `src/webhooks/server.py` (aiohttp server)
- [ ] Implement memo webhook handler
- [ ] Test webhook signature verification

### Phase 8: Go Backend Changes

- [ ] Add webhook dispatcher to Go backend
- [ ] Integrate webhooks into memo service
- [ ] Remove old AI service code from Go

### Phase 9: Nginx Configuration

- [ ] Configure gRPC routing for AI service
- [ ] Configure HTTP routing for webhooks
- [ ] Test gRPC through Nginx

### Phase 10: Docker Deployment

- [ ] Create Dockerfile
- [ ] Create docker-compose.yml
- [ ] Test full stack locally
- [ ] Deploy to Aliyun server

---

## API Endpoints Summary

### gRPC Methods (Python AI Service)

| Method               | Path                                         | Auth | Description                    |
| -------------------- | -------------------------------------------- | ---- | ------------------------------ |
| `CreateConversation` | `/memos.api.v1.AIService/CreateConversation` | Yes  | Create conversation            |
| `ListConversations`  | `/memos.api.v1.AIService/ListConversations`  | Yes  | List conversations             |
| `GetConversation`    | `/memos.api.v1.AIService/GetConversation`    | Yes  | Get conversation               |
| `UpdateConversation` | `/memos.api.v1.AIService/UpdateConversation` | Yes  | Update conversation            |
| `DeleteConversation` | `/memos.api.v1.AIService/DeleteConversation` | Yes  | Delete conversation            |
| `SendMessage`        | `/memos.api.v1.AIService/SendMessage`        | Yes  | Send message + get AI response |
| `ListMessages`       | `/memos.api.v1.AIService/ListMessages`       | Yes  | List messages                  |
| `GetAIConfig`        | `/memos.api.v1.AIService/GetAIConfig`        | No   | Get AI configuration           |

### HTTP Endpoints (Python Webhooks)

| Method | Path             | Auth      | Description         |
| ------ | ---------------- | --------- | ------------------- |
| `POST` | `/webhooks/memo` | Signature | Memo change webhook |
| `GET`  | `/health`        | No        | Health check        |

---

## Environment Variables Summary

### Python AI Service

| Variable            | Required | Description                       |
| ------------------- | -------- | --------------------------------- |
| `JWT_SECRET`        | Yes      | Must match Go backend             |
| `GRPC_PORT`         | No       | gRPC port (default: 50051)        |
| `HTTP_PORT`         | No       | HTTP webhook port (default: 8000) |
| `POSTGRES_HOST`     | Yes      | PostgreSQL host                   |
| `POSTGRES_PORT`     | No       | PostgreSQL port (default: 5432)   |
| `POSTGRES_USER`     | Yes      | PostgreSQL user                   |
| `POSTGRES_PASSWORD` | Yes      | PostgreSQL password               |
| `POSTGRES_DB`       | Yes      | PostgreSQL database               |
| `OPENAI_API_KEY`    | No       | OpenAI API key                    |
| `DEEPSEEK_API_KEY`  | No       | DeepSeek API key                  |
| `GO_BACKEND_URL`    | Yes      | Go backend URL for PAT validation |
| `WEBHOOK_SECRET`    | Yes      | Webhook signature secret          |

### Go Backend (New Variables)

| Variable                  | Required | Description              |
| ------------------------- | -------- | ------------------------ |
| `MEMOS_AI_WEBHOOK_URL`    | No       | AI service webhook URL   |
| `MEMOS_AI_WEBHOOK_SECRET` | No       | Webhook signature secret |

---

## Testing Commands

```bash
# Generate Python protobuf code
cd ai-service/proto && buf generate

# Run Python AI service locally
cd ai-service && python -m src.main

# Test gRPC with grpcurl
grpcurl -plaintext -d '{}' localhost:50051 memos.api.v1.AIService/GetAIConfig

# Test authenticated endpoint
grpcurl -plaintext \
  -H "authorization: Bearer <jwt_token>" \
  -d '{"title": "Test Chat"}' \
  localhost:50051 memos.api.v1.AIService/CreateConversation

# Run tests
cd ai-service && pytest tests/ -v

# Check health
curl http://localhost:8000/health
```
