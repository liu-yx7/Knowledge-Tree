"""gRPC server factory."""

import logging
from concurrent import futures

import grpc
from grpc import aio

from src.config import get_settings
from src.grpc.servicers.ai_servicer import AIServiceServicer
from src.proto.api.v1 import ai_service_pb2_grpc

logger = logging.getLogger(__name__)

# Get settings
settings = get_settings()


def create_grpc_server() -> aio.Server:
    """Create and configure the gRPC server."""
    # Create async server without interceptors for now (interceptors need async implementation)
    server = aio.server(
        options=[
            ("grpc.max_send_message_length", 50 * 1024 * 1024),  # 50MB
            ("grpc.max_receive_message_length", 50 * 1024 * 1024),  # 50MB
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
