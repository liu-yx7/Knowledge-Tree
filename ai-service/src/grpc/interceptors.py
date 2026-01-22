"""gRPC interceptors for authentication and logging."""

import logging
import time
from typing import Any, Callable

import grpc

from src.auth.jwt import AccessTokenClaims, JWTValidationError, validate_access_token
from src.config import get_settings

logger = logging.getLogger(__name__)

# Public endpoints that don't require authentication
PUBLIC_ENDPOINTS = frozenset([
    "/memos.api.v1.AIService/GetAIConfig",
    "/memos.api.v1.AIService/ListProviders",
])


class AuthInterceptor(grpc.ServerInterceptor):
    """Interceptor for JWT authentication."""

    def __init__(self):
        self.settings = get_settings()

    def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        method = handler_call_details.method

        # Skip auth for public endpoints
        if method in PUBLIC_ENDPOINTS:
            return continuation(handler_call_details)

        # Extract token from metadata
        metadata = dict(handler_call_details.invocation_metadata or [])
        auth_header = metadata.get("authorization", "")

        if not auth_header.startswith("Bearer "):
            return self._unauthenticated_handler()

        token = auth_header[7:]  # Remove "Bearer " prefix

        # Verify token
        try:
            claims = validate_access_token(token)
        except JWTValidationError as e:
            logger.warning(f"JWT validation failed: {e.message}")
            return self._unauthenticated_handler()

        # Store claims in handler for servicer to access
        handler = continuation(handler_call_details)
        if handler is not None:
            return _AuthenticatedHandler(handler, claims)

        return handler

    def _unauthenticated_handler(self) -> grpc.RpcMethodHandler:
        """Return handler that rejects with UNAUTHENTICATED."""
        def _abort(request, context):
            context.set_code(grpc.StatusCode.UNAUTHENTICATED)
            context.set_details("Invalid or missing authentication token")
            return None

        return grpc.unary_unary_rpc_method_handler(_abort)


class _AuthenticatedHandler(grpc.RpcMethodHandler):
    """Wrapper handler that injects user claims into context."""

    def __init__(self, handler: grpc.RpcMethodHandler, claims: AccessTokenClaims):
        self._handler = handler
        self._claims = claims

        # Copy attributes from wrapped handler
        self.request_streaming = handler.request_streaming
        self.response_streaming = handler.response_streaming
        self.request_deserializer = handler.request_deserializer
        self.response_serializer = handler.response_serializer

        # Wrap the appropriate handler method
        if handler.unary_unary:
            self.unary_unary = self._wrap_unary_unary(handler.unary_unary)
        if handler.unary_stream:
            self.unary_stream = self._wrap_unary_stream(handler.unary_stream)
        if handler.stream_unary:
            self.stream_unary = self._wrap_stream_unary(handler.stream_unary)
        if handler.stream_stream:
            self.stream_stream = self._wrap_stream_stream(handler.stream_stream)

    def _wrap_unary_unary(self, method):
        def wrapper(request, context):
            context._user_claims = self._claims
            return method(request, context)
        return wrapper

    def _wrap_unary_stream(self, method):
        def wrapper(request, context):
            context._user_claims = self._claims
            return method(request, context)
        return wrapper

    def _wrap_stream_unary(self, method):
        def wrapper(request_iterator, context):
            context._user_claims = self._claims
            return method(request_iterator, context)
        return wrapper

    def _wrap_stream_stream(self, method):
        def wrapper(request_iterator, context):
            context._user_claims = self._claims
            return method(request_iterator, context)
        return wrapper


class LoggingInterceptor(grpc.ServerInterceptor):
    """Interceptor for request logging."""

    def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Any],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> Any:
        method = handler_call_details.method
        start_time = time.time()

        logger.info(f"gRPC request: {method}")

        handler = continuation(handler_call_details)

        if handler is not None:
            return _LoggingHandler(handler, method, start_time)

        return handler


class _LoggingHandler(grpc.RpcMethodHandler):
    """Wrapper handler for logging."""

    def __init__(self, handler: grpc.RpcMethodHandler, method: str, start_time: float):
        self._handler = handler
        self._method = method
        self._start_time = start_time

        self.request_streaming = handler.request_streaming
        self.response_streaming = handler.response_streaming
        self.request_deserializer = handler.request_deserializer
        self.response_serializer = handler.response_serializer

        if handler.unary_unary:
            self.unary_unary = self._wrap_unary_unary(handler.unary_unary)
        if handler.unary_stream:
            self.unary_stream = handler.unary_stream
        if handler.stream_unary:
            self.stream_unary = handler.stream_unary
        if handler.stream_stream:
            self.stream_stream = handler.stream_stream

    def _wrap_unary_unary(self, method):
        def wrapper(request, context):
            try:
                response = method(request, context)
                duration = time.time() - self._start_time
                logger.info(f"gRPC response: {self._method} - {duration:.3f}s")
                return response
            except Exception as e:
                duration = time.time() - self._start_time
                logger.error(f"gRPC error: {self._method} - {duration:.3f}s - {e}")
                raise
        return wrapper
