"""LLM provider interface."""

from abc import ABC, abstractmethod
from dataclasses import dataclass


@dataclass
class ChatMessage:
    """A chat message."""
    role: str  # "user", "assistant", "system"
    content: str


@dataclass
class CompletionResponse:
    """LLM completion response."""
    content: str
    token_count: int
    finish_reason: str


class LLMProvider(ABC):
    """Abstract base class for LLM providers."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Provider name."""
        pass

    @abstractmethod
    def complete(
        self,
        messages: list[ChatMessage],
        model: str | None = None,
        max_tokens: int | None = None,
        temperature: float = 0.7,
    ) -> CompletionResponse:
        """Generate a completion."""
        pass

    @abstractmethod
    def get_available_models(self) -> list[str]:
        """Return list of available models for this provider."""
        pass
