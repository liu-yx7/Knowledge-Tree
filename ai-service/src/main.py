"""Main entry point for the AI microservice."""

import asyncio
import logging
import signal
import sys
from concurrent import futures

import grpc
from grpc_reflection.v1alpha import reflection

from src.config import settings
from src.db.database import engine, Base, init_db
from src.grpc.server import create_grpc_server
from src.webhooks.handler import WebhookHandler

# Configure logging
logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL.upper()),
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)


async def serve():
    """Start the gRPC server."""
    # Initialize database
    logger.info("Initializing database...")
    init_db()
    logger.info("Database initialized")

    # Create gRPC server
    server = create_grpc_server()

    # Add server reflection for debugging
    service_names = (
        "memos.api.v1.AIService",
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    # Start server
    listen_addr = f"[::]:{settings.GRPC_PORT}"
    server.add_insecure_port(listen_addr)
    
    logger.info(f"Starting AI microservice on {listen_addr}")
    await server.start()

    # Setup graceful shutdown
    async def shutdown(sig):
        logger.info(f"Received signal {sig.name}, shutting down...")
        await server.stop(grace=5)
        logger.info("Server stopped")

    loop = asyncio.get_event_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(
            sig,
            lambda s=sig: asyncio.create_task(shutdown(s)),
        )

    logger.info("AI microservice is ready to accept requests")
    await server.wait_for_termination()


def main():
    """Main entry point."""
    logger.info("=" * 50)
    logger.info("Memos AI Microservice")
    logger.info("=" * 50)
    logger.info(f"Environment: {settings.ENVIRONMENT}")
    logger.info(f"gRPC Port: {settings.GRPC_PORT}")
    logger.info(f"Database: {settings.DATABASE_URL.split('@')[-1] if '@' in settings.DATABASE_URL else 'sqlite'}")
    logger.info(f"Default LLM Provider: {settings.DEFAULT_LLM_PROVIDER}")
    logger.info("=" * 50)

    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    except Exception as e:
        logger.error(f"Server error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
