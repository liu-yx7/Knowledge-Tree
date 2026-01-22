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


class AuthInterceptor(grpc.ServerInterceptor):
    """gRPC server interceptor for authentication.
    
    This interceptor:
    1. Extracts the Authorization header from metadata
    2. Validates JWT tokens locally or PATs via Go backend
    3. Adds user claims to the context for servicers to use
    4. Returns UNAUTHENTICATED error if validation fails
    """

    def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        """Intercept incoming RPC calls for authentication."""
        method = handler_call_details.method
        
        # Skip auth for public methods
        if method in PUBLIC_METHODS:
            return continuation(handler_call_details)

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
                # PAT validation requires calling Go backend
                # For synchronous interceptor, we'll reject PATs and require JWT
                # In production, consider using async interceptor or caching
                raise JWTValidationError("Personal Access Tokens not supported for AI service. Use JWT access token.")
            else:
                claims = validate_access_token(token)

            # Check if user is archived
            if claims.status == "ARCHIVED":
                return self._unauthenticated_handler("User account is archived")

            # Store claims in handler_call_details for servicer to access
            handler_call_details.user_claims = claims  # type: ignore

        except JWTValidationError as e:
            logger.warning(f"Authentication failed: {e.message}")
            return self._unauthenticated_handler(e.message)

        return continuation(handler_call_details)

    def _unauthenticated_handler(self, message: str):
        """Return a handler that sends UNAUTHENTICATED error."""
        def handler(request, context):
            context.set_code(grpc.StatusCode.UNAUTHENTICATED)
            context.set_details(message)
            return None
        return grpc.unary_unary_rpc_method_handler(handler)


class AuthContext:
    """Context holder for authenticated user information.
    
    Used to pass user claims through the gRPC context.
    """
    
    _claims_key = "auth_claims"

    @classmethod
    def set_claims(cls, context: grpc.ServicerContext, claims: AccessTokenClaims) -> None:
        """Store claims in the servicer context."""
        setattr(context, cls._claims_key, claims)

    @classmethod
    def get_claims(cls, context: grpc.ServicerContext) -> AccessTokenClaims | None:
        """Retrieve claims from the servicer context."""
        return getattr(context, cls._claims_key, None)

    @classmethod
    def require_claims(cls, context: grpc.ServicerContext) -> AccessTokenClaims:
        """Get claims or raise UNAUTHENTICATED error."""
        claims = cls.get_claims(context)
        if claims is None:
            context.set_code(grpc.StatusCode.UNAUTHENTICATED)
            context.set_details("Not authenticated")
            raise grpc.RpcError()
        return claims
