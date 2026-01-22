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
    issued_at: datetime | None
    expires_at: datetime | None


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
