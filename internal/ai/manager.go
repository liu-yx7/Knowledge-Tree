package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/usememos/memos/store"
)

// Manager manages LLM providers
type Manager struct {
	store     *store.Store
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewManager creates a new LLM manager
func NewManager(store *store.Store) *Manager {
	return &Manager{
		store:     store,
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers a new provider
func (m *Manager) RegisterProvider(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.Name()] = provider
}

// GetProvider returns a provider by name
func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return provider, nil
}

// ListProviders returns all registered providers
func (m *Manager) ListProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	providers := make([]Provider, 0, len(m.providers))
	for _, provider := range m.providers {
		providers = append(providers, provider)
	}
	return providers
}

// SendMessage sends a message using the specified provider
func (m *Manager) SendMessage(ctx context.Context, providerName string, req ChatRequest) (ChatResponse, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return ChatResponse{}, err
	}
	return provider.SendMessage(ctx, req)
}

// StreamMessage streams a message using the specified provider
func (m *Manager) StreamMessage(ctx context.Context, providerName string, req ChatRequest) (<-chan string, <-chan error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		errCh := make(chan error, 1)
		errCh <- err
		close(errCh)
		contentCh := make(chan string)
		close(contentCh)
		return contentCh, errCh
	}
	return provider.StreamMessage(ctx, req)
}
