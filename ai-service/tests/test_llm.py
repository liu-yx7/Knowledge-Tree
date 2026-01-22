"""Tests for LLM providers."""

import pytest
from unittest.mock import MagicMock, patch

from src.llm.base import ChatMessage, CompletionResponse
from src.llm.openai import OpenAIProvider


class TestOpenAIProvider:
    """Test OpenAI provider."""

    @patch("src.llm.openai.OpenAI")
    def test_complete(self, mock_openai_class):
        """Test completion generation."""
        # Setup mock
        mock_client = MagicMock()
        mock_openai_class.return_value = mock_client
        
        mock_response = MagicMock()
        mock_response.choices = [MagicMock(message=MagicMock(content="Hello!"), finish_reason="stop")]
        mock_response.usage = MagicMock(total_tokens=10)
        mock_client.chat.completions.create.return_value = mock_response

        # Test
        provider = OpenAIProvider()
        messages = [ChatMessage(role="user", content="Hi")]
        result = provider.complete(messages)

        assert result.content == "Hello!"
        assert result.token_count == 10
        mock_client.chat.completions.create.assert_called_once()
