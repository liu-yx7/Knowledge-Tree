"""Webhook HTTP server."""

from src.webhooks.handlers import create_webhook_app

# Alias for backward compatibility
create_webhook_server = create_webhook_app

__all__ = ["create_webhook_server", "create_webhook_app"]
