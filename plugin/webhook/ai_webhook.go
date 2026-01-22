// Package webhook provides AI service webhook functionality.
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
)

// AIWebhookConfig holds configuration for AI service webhooks.
type AIWebhookConfig struct {
	// BaseURL is the base URL of the AI service (e.g., "http://ai-service:8000")
	BaseURL string
	// Secret is the shared secret for signing webhook payloads
	Secret string
	// Timeout for webhook requests
	Timeout time.Duration
}

// DefaultAIWebhookConfig returns the default configuration from environment variables.
func DefaultAIWebhookConfig() *AIWebhookConfig {
	baseURL := os.Getenv("MEMOS_AI_WEBHOOK_URL")
	secret := os.Getenv("MEMOS_AI_WEBHOOK_SECRET")

	if baseURL == "" {
		return nil // AI webhooks not configured
	}

	return &AIWebhookConfig{
		BaseURL: baseURL,
		Secret:  secret,
		Timeout: 30 * time.Second,
	}
}

// AIWebhookClient handles sending webhooks to the AI service.
type AIWebhookClient struct {
	config *AIWebhookConfig
	client *http.Client
}

// NewAIWebhookClient creates a new AI webhook client.
func NewAIWebhookClient(config *AIWebhookConfig) *AIWebhookClient {
	if config == nil {
		return nil
	}

	return &AIWebhookClient{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// MemoWebhookPayload represents the payload for memo webhooks.
type MemoWebhookPayload struct {
	Action string    `json:"action"` // "create", "update", "delete"
	Memo   *MemoData `json:"memo"`
}

// MemoData represents memo data sent in webhooks.
type MemoData struct {
	UID       string `json:"uid"`
	CreatorID int32  `json:"creator_id"`
	Content   string `json:"content"`
}

// AttachmentWebhookPayload represents the payload for attachment webhooks.
type AttachmentWebhookPayload struct {
	Action     string          `json:"action"` // "create", "delete"
	Attachment *AttachmentData `json:"attachment"`
}

// AttachmentData represents attachment data sent in webhooks.
type AttachmentData struct {
	UID           string `json:"uid"`
	CreatorID     int32  `json:"creator_id"`
	ExtractedText string `json:"extracted_text,omitempty"` // Pre-extracted text content
	MimeType      string `json:"mime_type,omitempty"`
	Filename      string `json:"filename,omitempty"`
}

// signPayload generates an HMAC-SHA256 signature for the payload.
func (c *AIWebhookClient) signPayload(payload []byte) string {
	if c.config.Secret == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(c.config.Secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// post sends a POST request to the specified endpoint with the given payload.
func (c *AIWebhookClient) post(endpoint string, payload any) error {
	if c == nil || c.config == nil {
		return nil // AI webhooks not configured, silently skip
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal webhook payload")
	}

	url := c.config.BaseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrap(err, "failed to create webhook request")
	}

	req.Header.Set("Content-Type", "application/json")

	// Add signature header if secret is configured
	if signature := c.signPayload(body); signature != "" {
		req.Header.Set("X-Webhook-Signature", signature)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to send webhook to %s", url)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read webhook response")
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return errors.Errorf("webhook failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// postAsync sends a webhook asynchronously (fire-and-forget).
func (c *AIWebhookClient) postAsync(endpoint string, payload any) {
	if c == nil {
		return
	}

	go func() {
		if err := c.post(endpoint, payload); err != nil {
			slog.Warn("Failed to send AI webhook",
				slog.String("endpoint", endpoint),
				slog.Any("error", err))
		}
	}()
}

// NotifyMemoCreated notifies the AI service that a memo was created.
func (c *AIWebhookClient) NotifyMemoCreated(memoUID string, creatorID int32, content string) {
	c.postAsync("/webhooks/memo", &MemoWebhookPayload{
		Action: "create",
		Memo: &MemoData{
			UID:       memoUID,
			CreatorID: creatorID,
			Content:   content,
		},
	})
}

// NotifyMemoUpdated notifies the AI service that a memo was updated.
func (c *AIWebhookClient) NotifyMemoUpdated(memoUID string, creatorID int32, content string) {
	c.postAsync("/webhooks/memo", &MemoWebhookPayload{
		Action: "update",
		Memo: &MemoData{
			UID:       memoUID,
			CreatorID: creatorID,
			Content:   content,
		},
	})
}

// NotifyMemoDeleted notifies the AI service that a memo was deleted.
func (c *AIWebhookClient) NotifyMemoDeleted(memoUID string) {
	c.postAsync("/webhooks/memo", &MemoWebhookPayload{
		Action: "delete",
		Memo: &MemoData{
			UID: memoUID,
		},
	})
}

// NotifyAttachmentCreated notifies the AI service that an attachment was created.
// The extractedText should contain pre-extracted text content for the attachment.
func (c *AIWebhookClient) NotifyAttachmentCreated(attachmentUID string, creatorID int32, extractedText, mimeType, filename string) {
	c.postAsync("/webhooks/attachment", &AttachmentWebhookPayload{
		Action: "create",
		Attachment: &AttachmentData{
			UID:           attachmentUID,
			CreatorID:     creatorID,
			ExtractedText: extractedText,
			MimeType:      mimeType,
			Filename:      filename,
		},
	})
}

// NotifyAttachmentDeleted notifies the AI service that an attachment was deleted.
func (c *AIWebhookClient) NotifyAttachmentDeleted(attachmentUID string) {
	c.postAsync("/webhooks/attachment", &AttachmentWebhookPayload{
		Action: "delete",
		Attachment: &AttachmentData{
			UID: attachmentUID,
		},
	})
}

// Global AI webhook client instance
var globalAIWebhookClient *AIWebhookClient

// InitAIWebhooks initializes the global AI webhook client.
// Should be called during server startup.
func InitAIWebhooks() {
	config := DefaultAIWebhookConfig()
	if config != nil {
		globalAIWebhookClient = NewAIWebhookClient(config)
		slog.Info("AI webhooks initialized",
			slog.String("baseURL", config.BaseURL))
	}
}

// GetAIWebhookClient returns the global AI webhook client.
// Returns nil if AI webhooks are not configured.
func GetAIWebhookClient() *AIWebhookClient {
	return globalAIWebhookClient
}

// Convenience functions using the global client

// AINotifyMemoCreated is a convenience function to notify memo creation.
func AINotifyMemoCreated(memoUID string, creatorID int32, content string) {
	if client := GetAIWebhookClient(); client != nil {
		client.NotifyMemoCreated(memoUID, creatorID, content)
	}
}

// AINotifyMemoUpdated is a convenience function to notify memo update.
func AINotifyMemoUpdated(memoUID string, creatorID int32, content string) {
	if client := GetAIWebhookClient(); client != nil {
		client.NotifyMemoUpdated(memoUID, creatorID, content)
	}
}

// AINotifyMemoDeleted is a convenience function to notify memo deletion.
func AINotifyMemoDeleted(memoUID string) {
	if client := GetAIWebhookClient(); client != nil {
		client.NotifyMemoDeleted(memoUID)
	}
}

// AINotifyAttachmentCreated is a convenience function to notify attachment creation.
func AINotifyAttachmentCreated(attachmentUID string, creatorID int32, extractedText, mimeType, filename string) {
	if client := GetAIWebhookClient(); client != nil {
		client.NotifyAttachmentCreated(attachmentUID, creatorID, extractedText, mimeType, filename)
	}
}

// AINotifyAttachmentDeleted is a convenience function to notify attachment deletion.
func AINotifyAttachmentDeleted(attachmentUID string) {
	if client := GetAIWebhookClient(); client != nil {
		client.NotifyAttachmentDeleted(attachmentUID)
	}
}
