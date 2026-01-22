"""gRPC server factory."""

import logging
from concurrent import futures

import grpc
from grpc import aio

from src.config import settings
from src.grpc.interceptors import (
    AuthInterceptor,
    LoggingInterceptor,
    ErrorHandlerInterceptor,
)
from src.grpc.servicer import AIServiceServicer
from src.proto.api.v1 import ai_service_pb2_grpc

logger = logging.getLogger(__name__)


def create_grpc_server() -> aio.Server:
    """Create and configure the gRPC server."""
    # Create interceptors
    interceptors = [
        ErrorHandlerInterceptor(),
        LoggingInterceptor(),
        AuthInterceptor(),
    ]

    # Create async server with interceptors
    server = aio.server(
        futures.ThreadPoolExecutor(max_workers=settings.GRPC_MAX_WORKERS),
        interceptors=interceptors,
        options=[
            ("grpc.max_send_message_length", settings.GRPC_MAX_MESSAGE_LENGTH),
            ("grpc.max_receive_message_length", settings.GRPC_MAX_MESSAGE_LENGTH),
            ("grpc.keepalive_time_ms", 30000),
            ("grpc.keepalive_timeout_ms", 10000),
            ("grpc.keepalive_permit_without_calls", True),
        ],
    )

    # Register services
    ai_servicer = AIServiceServicer()
    ai_service_pb2_grpc.add_AIServiceServicer_to_server(ai_servicer, server)

    logger.info("gRPC server created with AI service registered")
    return server
