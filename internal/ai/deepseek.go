package ai

// DeepseekProvider implements the Provider interface for Deepseek
// Deepseek uses OpenAI-compatible API, so we can reuse the OpenAI provider
type DeepseekProvider struct {
	*OpenAIProvider
}

// NewDeepseekProvider creates a new Deepseek provider
func NewDeepseekProvider(apiKey string) *DeepseekProvider {
	return &DeepseekProvider{
		OpenAIProvider: NewOpenAIProvider(apiKey, "https://api.deepseek.com/v1"),
	}
}

func (p *DeepseekProvider) Name() string {
	return "deepseek"
}

func (p *DeepseekProvider) GetModels() []string {
	return []string{"deepseek-chat", "deepseek-coder"}
}
