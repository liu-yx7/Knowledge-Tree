"""LLM provider factory."""

from src.config import get_settings
from src.llm.base import LLMProvider
from src.llm.openai import DeepSeekProvider, OpenAIProvider


def get_llm_provider(name: str | None = None) -> LLMProvider:
    """Get an LLM provider by name.
    
    Args:
        name: Provider name ("openai" or "deepseek"). If None, uses default from settings.
        
    Returns:
        LLMProvider instance
        
    Raises:
        ValueError: If provider is unknown or not configured
    """
    settings = get_settings()
    name = name or settings.default_provider

    if name == "openai":
        if not settings.openai_api_key:
            raise ValueError("OpenAI API key not configured")
        return OpenAIProvider()

    if name == "deepseek":
        if not settings.deepseek_api_key:
            raise ValueError("DeepSeek API key not configured")
        return DeepSeekProvider()

    raise ValueError(f"Unknown LLM provider: {name}")


def get_available_providers() -> list[dict]:
    """Get list of available (configured) providers with their models.
    
    Returns:
        List of provider info dicts with name, display_name, and models
    """
    settings = get_settings()
    providers = []

    if settings.openai_api_key:
        providers.append({
            "name": "openai",
            "display_name": "OpenAI",
            "models": OpenAIProvider.AVAILABLE_MODELS,
        })

    if settings.deepseek_api_key:
        providers.append({
            "name": "deepseek",
            "display_name": "DeepSeek",
            "models": DeepSeekProvider.AVAILABLE_MODELS,
        })

    return providers
