# -*- coding: utf-8 -*-
"""API v1 protocol buffer package."""

from src.proto.api.v1.ai_service_pb2 import (
    AIConfig,
    Conversation,
    CreateConversationRequest,
    DeleteConversationRequest,
    DeleteMemoIndexRequest,
    GetAIConfigRequest,
    GetConversationRequest,
    IndexMemoRequest,
    IndexMemoResponse,
    ListConversationsRequest,
    ListConversationsResponse,
    ListMessagesRequest,
    ListMessagesResponse,
    ListProvidersRequest,
    ListProvidersResponse,
    Message,
    Provider,
    SendMessageRequest,
    SendMessageResponse,
    StreamMessageRequest,
    StreamMessageResponse,
    UpdateConversationRequest,
)
from src.proto.api.v1.ai_service_pb2_grpc import (
    AIServiceServicer,
    AIServiceStub,
    add_AIServiceServicer_to_server,
)

__all__ = [
    # Messages
    "AIConfig",
    "Conversation",
    "CreateConversationRequest",
    "DeleteConversationRequest",
    "DeleteMemoIndexRequest",
    "GetAIConfigRequest",
    "GetConversationRequest",
    "IndexMemoRequest",
    "IndexMemoResponse",
    "ListConversationsRequest",
    "ListConversationsResponse",
    "ListMessagesRequest",
    "ListMessagesResponse",
    "ListProvidersRequest",
    "ListProvidersResponse",
    "Message",
    "Provider",
    "SendMessageRequest",
    "SendMessageResponse",
    "StreamMessageRequest",
    "StreamMessageResponse",
    "UpdateConversationRequest",
    # gRPC
    "AIServiceServicer",
    "AIServiceStub",
    "add_AIServiceServicer_to_server",
]
