"""gRPC module for AI microservice."""

from src.grpc.server import create_grpc_server
from src.grpc.servicer import AIServiceServicer
from src.grpc.interceptors import (
    AuthInterceptor,
    LoggingInterceptor,
    ErrorHandlerInterceptor,
)

__all__ = [
    "create_grpc_server",
    "AIServiceServicer",
    "AuthInterceptor",
    "LoggingInterceptor",
    "ErrorHandlerInterceptor",
]
