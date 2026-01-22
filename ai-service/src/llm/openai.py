"""OpenAI LLM provider."""

from openai import OpenAI

from src.config import get_settings
from src.llm.base import ChatMessage, CompletionResponse, LLMProvider


class OpenAIProvider(LLMProvider):
    """OpenAI LLM provider."""

    AVAILABLE_MODELS = [
        "gpt-4o",
        "gpt-4o-mini",
        "gpt-4-turbo",
        "gpt-4",
        "gpt-3.5-turbo",
    ]

    def __init__(self, api_key: str | None = None, base_url: str | None = None):
        settings = get_settings()
        self.client = OpenAI(
            api_key=api_key or settings.openai_api_key,
            base_url=base_url or settings.openai_base_url,
        )
        self._default_model = settings.default_model

    @property
    def name(self) -> str:
        return "openai"

    def complete(
        self,
        messages: list[ChatMessage],
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float = 0.7,
    ) -> CompletionResponse:
        model = model or self._default_model

        openai_messages = [
            {"role": msg.role, "content": msg.content}
            for msg in messages
        ]

        response = self.client.chat.completions.create(
            model=model,
            messages=openai_messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )

        choice = response.choices[0]
        usage = response.usage

        return CompletionResponse(
            content=choice.message.content or "",
            token_count=usage.total_tokens if usage else 0,
            finish_reason=choice.finish_reason or "stop",
        )

    def get_available_models(self) -> list[str]:
        return self.AVAILABLE_MODELS


class DeepSeekProvider(LLMProvider):
    """DeepSeek LLM provider (OpenAI-compatible API)."""

    AVAILABLE_MODELS = [
        "deepseek-chat",
        "deepseek-coder",
    ]

    def __init__(self):
        settings = get_settings()
        self.client = OpenAI(
            api_key=settings.deepseek_api_key,
            base_url=settings.deepseek_base_url,
        )

    @property
    def name(self) -> str:
        return "deepseek"

    def complete(
        self,
        messages: list[ChatMessage],
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float = 0.7,
    ) -> CompletionResponse:
        model = model or "deepseek-chat"

        openai_messages = [
            {"role": msg.role, "content": msg.content}
            for msg in messages
        ]

        response = self.client.chat.completions.create(
            model=model,
            messages=openai_messages,
            max_tokens=max_tokens,
            temperature=temperature,
        )

        choice = response.choices[0]
        usage = response.usage

        return CompletionResponse(
            content=choice.message.content or "",
            token_count=usage.total_tokens if usage else 0,
            finish_reason=choice.finish_reason or "stop",
        )

    def get_available_models(self) -> list[str]:
        return self.AVAILABLE_MODELS
