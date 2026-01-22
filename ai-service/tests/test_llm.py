"""Tests for LLM providers."""

import pytest
from unittest.mock import MagicMock, patch

from src.llm.base import ChatMessage, LLMResponse
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
        mock_response.choices = [MagicMock(message=MagicMock(content="Hello!"))]
        mock_response.usage.total_tokens = 10
        mock_client.chat.completions.create.return_value = mock_response

        # Test
        provider = OpenAIProvider(api_key="test-key")
        messages = [ChatMessage(role="user", content="Hi")]
        result = provider.complete(messages)

        assert result.content == "Hello!"
        assert result.token_count == 10
        mock_client.chat.completions.create.assert_called_once()

    @patch("src.llm.openai.OpenAI")
    def test_stream(self, mock_openai_class):
        """Test streaming completion."""
        mock_client = MagicMock()
        mock_openai_class.return_value = mock_client

        # Mock streaming response
        mock_chunk1 = MagicMock()
        mock_chunk1.choices = [MagicMock(delta=MagicMock(content="Hel"))]
        mock_chunk2 = MagicMock()
        mock_chunk2.choices = [MagicMock(delta=MagicMock(content="lo!"))]
        
        mock_client.chat.completions.create.return_value = iter([mock_chunk1, mock_chunk2])

        # Test
        provider = OpenAIProvider(api_key="test-key")
        messages = [ChatMessage(role="user", content="Hi")]
        chunks = list(provider.stream(messages))

        assert chunks == ["Hel", "lo!"]


class TestAnthropicProvider:
    """Test Anthropic provider."""

    @patch("src.llm.anthropic.Anthropic")
    def test_complete(self, mock_anthropic_class):
        """Test completion generation."""
        from src.llm.anthropic import AnthropicProvider

        mock_client = MagicMock()
        mock_anthropic_class.return_value = mock_client

        mock_response = MagicMock()
        mock_response.content = [MagicMock(text="Hello from Claude!")]
        mock_response.usage.input_tokens = 5
        mock_response.usage.output_tokens = 3
        mock_client.messages.create.return_value = mock_response

        provider = AnthropicProvider(api_key="test-key")
        messages = [ChatMessage(role="user", content="Hi")]
        result = provider.complete(messages)

        assert result.content == "Hello from Claude!"
        assert result.token_count == 8
