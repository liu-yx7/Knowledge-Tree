package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	apiKey      string
	apiEndpoint string
	models      []string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, apiEndpoint string) *OpenAIProvider {
	if apiEndpoint == "" {
		apiEndpoint = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:      apiKey,
		apiEndpoint: apiEndpoint,
		models:      []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) GetModels() []string {
	return p.models
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Temperature float32         `json:"temperature,omitempty"`
	MaxTokens   int32           `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiResponse struct {
	Choices []struct {
		Message openaiMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int32 `json:"total_tokens"`
	} `json:"usage"`
}

type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) SendMessage(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	messages := make([]openaiMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, msg := range req.Messages {
		messages = append(messages, openaiMessage{Role: msg.Role, Content: msg.Content})
	}

	reqBody := openaiRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.apiEndpoint+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return ChatResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResponse{}, fmt.Errorf("openai api error: %d - %s", resp.StatusCode, string(body))
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return ChatResponse{}, err
	}

	if len(openaiResp.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("no response from openai")
	}

	return ChatResponse{
		Content: openaiResp.Choices[0].Message.Content,
		Tokens:  openaiResp.Usage.TotalTokens,
	}, nil
}

func (p *OpenAIProvider) StreamMessage(ctx context.Context, req ChatRequest) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		messages := make([]openaiMessage, 0, len(req.Messages)+1)
		if req.SystemPrompt != "" {
			messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
		}
		for _, msg := range req.Messages {
			messages = append(messages, openaiMessage{Role: msg.Role, Content: msg.Content})
		}

		reqBody := openaiRequest{
			Model:       req.Model,
			Messages:    messages,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			Stream:      true,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.apiEndpoint+"/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			errCh <- err
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("openai api error: %d - %s", resp.StatusCode, string(body))
			return
		}

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- err
				}
				return
			}

			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}

			data := bytes.TrimPrefix(line, []byte("data: "))
			if bytes.Equal(data, []byte("[DONE]")) {
				return
			}

			var chunk openaiStreamChunk
			if err := json.Unmarshal(data, &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				select {
				case contentCh <- chunk.Choices[0].Delta.Content:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return contentCh, errCh
}
