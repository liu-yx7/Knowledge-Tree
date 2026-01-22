"""Webhook handlers for Go backend events."""

import hashlib
import hmac
import logging
from typing import Any

from aiohttp import web

from src.config import get_settings
from src.db.database import get_db_context
from src.rag import DocumentIndexer

logger = logging.getLogger(__name__)


def verify_webhook_signature(payload: bytes, signature: str) -> bool:
    """Verify webhook signature from Go backend.
    
    Args:
        payload: Raw request body
        signature: Signature from X-Webhook-Signature header
        
    Returns:
        True if signature is valid
    """
    settings = get_settings()
    if not settings.webhook_secret:
        logger.warning("Webhook secret not configured, skipping verification")
        return True

    expected = hmac.new(
        settings.webhook_secret.encode(),
        payload,
        hashlib.sha256,
    ).hexdigest()

    return hmac.compare_digest(f"sha256={expected}", signature)


async def handle_memo_webhook(request: web.Request) -> web.Response:
    """Handle memo create/update/delete webhooks.
    
    Webhook payload structure:
    {
        "action": "create" | "update" | "delete",
        "memo": {
            "uid": "...",
            "creator_id": 123,
            "content": "...",
            ...
        }
    }
    """
    # Verify signature
    payload = await request.read()
    signature = request.headers.get("X-Webhook-Signature", "")
    
    if not verify_webhook_signature(payload, signature):
        logger.warning("Invalid webhook signature")
        return web.Response(status=401, text="Invalid signature")

    try:
        data: dict[str, Any] = await request.json()
    except Exception as e:
        logger.error(f"Invalid webhook payload: {e}")
        return web.Response(status=400, text="Invalid JSON")

    action = data.get("action")
    memo = data.get("memo", {})
    memo_uid = memo.get("uid")
    user_id = memo.get("creator_id")
    content = memo.get("content", "")

    if not memo_uid:
        return web.Response(status=400, text="Missing memo UID")

    logger.info(f"Received memo webhook: action={action}, uid={memo_uid}")

    with get_db_context() as db:
        indexer = DocumentIndexer(db)

        if action in ("create", "update"):
            if user_id and content:
                indexer.index_memo(user_id, memo_uid, content)
        elif action == "delete":
            indexer.delete_memo_index(memo_uid)

    return web.Response(status=200, text="OK")


async def handle_attachment_webhook(request: web.Request) -> web.Response:
    """Handle attachment create/delete webhooks.
    
    For attachments, the Go backend should extract text content
    and send it in the webhook payload.
    
    Webhook payload structure:
    {
        "action": "create" | "delete",
        "attachment": {
            "uid": "...",
            "creator_id": 123,
            "extracted_text": "...",  # Only for create
            ...
        }
    }
    """
    # Verify signature
    payload = await request.read()
    signature = request.headers.get("X-Webhook-Signature", "")
    
    if not verify_webhook_signature(payload, signature):
        logger.warning("Invalid webhook signature")
        return web.Response(status=401, text="Invalid signature")

    try:
        data: dict[str, Any] = await request.json()
    except Exception as e:
        logger.error(f"Invalid webhook payload: {e}")
        return web.Response(status=400, text="Invalid JSON")

    action = data.get("action")
    attachment = data.get("attachment", {})
    attachment_uid = attachment.get("uid")
    user_id = attachment.get("creator_id")
    content = attachment.get("extracted_text", "")

    if not attachment_uid:
        return web.Response(status=400, text="Missing attachment UID")

    logger.info(f"Received attachment webhook: action={action}, uid={attachment_uid}")

    with get_db_context() as db:
        indexer = DocumentIndexer(db)

        if action == "create" and user_id and content:
            indexer.index_attachment(user_id, attachment_uid, content)
        elif action == "delete":
            from src.rag.vector_store import delete_document_embeddings
            delete_document_embeddings(db, "attachment", attachment_uid)

    return web.Response(status=200, text="OK")


async def handle_health(request: web.Request) -> web.Response:
    """Health check endpoint."""
    return web.Response(status=200, text="OK")


def create_webhook_app() -> web.Application:
    """Create aiohttp application for webhook handling."""
    app = web.Application()
    
    app.router.add_post("/webhooks/memo", handle_memo_webhook)
    app.router.add_post("/webhooks/attachment", handle_attachment_webhook)
    app.router.add_get("/health", handle_health)
    
    return app
