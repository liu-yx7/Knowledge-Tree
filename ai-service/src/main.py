"""Main entry point for the AI microservice."""

import asyncio
import logging
import signal
import sys
from concurrent import futures

import grpc
from grpc_reflection.v1alpha import reflection

from src.config import get_settings
from src.db.database import init_db, close_db
from src.grpc.server import create_grpc_server
from src.webhooks.server import create_webhook_server

# Get settings
settings = get_settings()

# Configure logging
logging.basicConfig(
    level=logging.DEBUG if settings.debug else logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)


async def run_webhook_server():
    """Run the HTTP webhook server."""
    from aiohttp import web
    
    app = create_webhook_server()
    runner = web.AppRunner(app)
    await runner.setup()
    
    site = web.TCPSite(runner, settings.http_host, settings.http_port)
    await site.start()
    logger.info(f"Webhook HTTP server started on {settings.http_host}:{settings.http_port}")
    
    # Keep running
    while True:
        await asyncio.sleep(3600)


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
    listen_addr = f"[::]:{settings.grpc_port}"
    server.add_insecure_port(listen_addr)
    
    logger.info(f"Starting gRPC server on {listen_addr}")
    await server.start()

    # Setup graceful shutdown
    async def shutdown(sig):
        logger.info(f"Received signal {sig.name}, shutting down...")
        await server.stop(grace=5)
        close_db()
        logger.info("Server stopped")

    loop = asyncio.get_event_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(
            sig,
            lambda s=sig: asyncio.create_task(shutdown(s)),
        )

    logger.info(f"gRPC server started on port {settings.grpc_port}")
    
    # Start webhook server concurrently
    webhook_task = asyncio.create_task(run_webhook_server())
    
    logger.info("AI microservice is ready to accept requests")
    await server.wait_for_termination()


def main():
    """Main entry point."""
    logger.info("=" * 50)
    logger.info("Memos AI Microservice")
    logger.info("=" * 50)
    logger.info(f"Debug: {settings.debug}")
    logger.info(f"gRPC Port: {settings.grpc_port}")
    logger.info(f"HTTP Port: {settings.http_port}")
    logger.info(f"Database: {settings.postgres_host}:{settings.postgres_port}/{settings.postgres_db}")
    logger.info(f"Default Provider: {settings.default_provider}")
    logger.info(f"Default Model: {settings.default_model}")
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
