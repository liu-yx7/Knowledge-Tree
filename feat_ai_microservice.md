# Feature: RAGFlow AI Microservice Integration

## Overview

This document outlines the step-by-step plan to integrate RAGFlow as an AI microservice for the Knowledge-Tree (Memos) project. RAGFlow provides RAG (Retrieval-Augmented Generation) capabilities including document parsing, vector embeddings, semantic search, and conversational AI with citations.

**Architecture Pattern:** All traffic routes through the Go backend, which acts as a gRPC-to-HTTP adapter for RAGFlow's REST API.

---

## Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [Architecture Design](#architecture-design)
3. [Phase 1: Cleanup & Preparation](#phase-1-cleanup--preparation)
4. [Phase 2: RAGFlow Client Plugin](#phase-2-ragflow-client-plugin)
5. [Phase 3: Protocol Buffer Definitions](#phase-3-protocol-buffer-definitions)
6. [Phase 4: Backend Service Implementation](#phase-4-backend-service-implementation)
7. [Phase 5: Background Sync Runner](#phase-5-background-sync-runner)
8. [Phase 6: Frontend Integration](#phase-6-frontend-integration)
9. [Phase 7: Configuration & Admin](#phase-7-configuration--admin)
10. [Phase 8: Testing & Deployment](#phase-8-testing--deployment)

---

## Current State Analysis

### Existing AI Implementation (To Be Replaced)

The project currently has a partial AI implementation that should be **removed or refactored**:

| Component              | Path                       | Status                                          |
| ---------------------- | -------------------------- | ----------------------------------------------- |
| LLM Manager            | `plugin/llm/manager.go`    | **Remove** - Replace with RAGFlow client        |
| LLM Provider Interface | `plugin/llm/provider.go`   | **Remove** - RAGFlow handles LLM                |
| OpenAI Provider        | `plugin/llm/openai/`       | **Remove** - RAGFlow handles LLM                |
| DeepSeek Provider      | `plugin/llm/deepseek/`     | **Remove** - RAGFlow handles LLM                |
| AI Conversation Store  | `store/ai_conversation.go` | **Keep** - Used for local conversation metadata |
| AI Message Store       | `store/ai_message.go`      | **Keep** - Used for local message cache         |
| AI Service (Python)    | `ai-service/src/`          | **Remove** - Replaced by RAGFlow                |

### Why Replace with RAGFlow?

| Current Approach                           | RAGFlow Approach                                  |
| ------------------------------------------ | ------------------------------------------------- |
| Direct LLM calls (no context)              | RAG with document retrieval                       |
| No document indexing                       | Automatic document parsing & chunking             |
| No semantic search                         | Vector embeddings & semantic search               |
| Multiple provider integrations to maintain | Single RAGFlow API, RAGFlow manages LLM providers |
| No citation/grounding                      | Responses grounded with source citations          |

---

## Architecture Design

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              System Architecture                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────┐         ┌─────────────────────────────────────────────┐  │
│  │   Frontend   │         │              Go Backend (Memos)             │  │
│  │   (React)    │         │  ┌─────────────────────────────────────┐    │  │
│  │              │ Connect │  │         API Layer (gRPC/Connect)    │    │  │
│  │  • Search UI │ ──RPC── │  │  • RAGFlowService (new)             │    │  │
│  │  • Chat UI   │         │  │  • MemoService (existing)           │    │  │
│  │  • Settings  │         │  │  • UserService (existing)           │    │  │
│  └──────────────┘         │  └──────────────┬──────────────────────┘    │  │
│                           │                 │                           │  │
│                           │  ┌──────────────▼──────────────────────┐    │  │
│                           │  │      RAGFlow Client Plugin          │    │  │
│                           │  │  plugin/ragflow/                    │    │  │
│                           │  │  • client.go (HTTP client)          │    │  │
│                           │  │  • dataset.go (knowledge base ops)  │    │  │
│                           │  │  • document.go (document ops)       │    │  │
│                           │  │  • chat.go (conversation ops)       │    │  │
│                           │  │  • search.go (retrieval ops)        │    │  │
│                           │  └──────────────┬──────────────────────┘    │  │
│                           │                 │ HTTP/REST                 │  │
│                           └─────────────────┼───────────────────────────┘  │
│                                             │                              │
│                           ┌─────────────────▼───────────────────────────┐  │
│                           │            RAGFlow Service                  │  │
│                           │  ┌─────────────────────────────────────┐    │  │
│                           │  │  • Document Parser (PDF, Word, etc) │    │  │
│                           │  │  • Chunking & Embedding             │    │  │
│                           │  │  • Vector Store (Elasticsearch)     │    │  │
│                           │  │  • LLM Integration                  │    │  │
│                           │  │  • RAG Pipeline                     │    │  │
│                           │  └─────────────────────────────────────┘    │  │
│                           └─────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
1. Semantic Search:
   Browser → Go Backend (auth + ACL) → RAGFlow Search API → Results
                    ↓
            Filter by memo permissions
                    ↓
            Return to browser

2. RAG Chat:
   Browser → Go Backend (auth) → RAGFlow Chat API → Streaming Response
                    ↓
            Save conversation locally
                    ↓
            Stream to browser

3. Document Indexing (Background):
   Memo Created/Updated → Background Runner → RAGFlow Upload API
```

### Data Model

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Data Flow                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Local Database (SQLite/MySQL/PostgreSQL)                                   │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │
│  │      memo       │    │ ai_conversation │    │   ragflow_sync_status   │  │
│  ├─────────────────┤    ├─────────────────┤    ├─────────────────────────┤  │
│  │ id              │◄───│ user_id         │    │ memo_id (FK)            │  │
│  │ content         │    │ id              │    │ ragflow_document_id     │  │
│  │ creator_id      │    │ title           │    │ last_synced_at          │  │
│  │ ...             │    │ ragflow_conv_id │    │ sync_status             │  │
│  └─────────────────┘    │ ...             │    │ content_hash            │  │
│                         └─────────────────┘    └─────────────────────────┘  │
│                                                                             │
│  RAGFlow (External)                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │
│  │    Dataset      │    │    Document     │    │     Conversation        │  │
│  │  (Knowledge     │◄───│  (Indexed       │    │   (Chat Session)        │  │
│  │    Base)        │    │    Memo)        │    │                         │  │
│  └─────────────────┘    └─────────────────┘    └─────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Cleanup & Preparation

### Step 1.1: Remove Existing LLM Plugin

Remove the direct LLM provider implementations that will be replaced by RAGFlow:

```bash
# Files to remove
rm -rf plugin/llm/openai/
rm -rf plugin/llm/deepseek/
rm plugin/llm/provider.go
rm plugin/llm/manager.go

# Directory to remove
rm -rf ai-service/
```

### Step 1.2: Update Driver Interface

**File: `store/driver.go`**

Add RAGFlow sync status methods to the Driver interface:

```go
// Add to Driver interface:

// RAGFlowSyncStatus model related methods.
CreateRAGFlowSyncStatus(ctx context.Context, create *RAGFlowSyncStatus) (*RAGFlowSyncStatus, error)
GetRAGFlowSyncStatus(ctx context.Context, memoID int32) (*RAGFlowSyncStatus, error)
UpdateRAGFlowSyncStatus(ctx context.Context, update *UpdateRAGFlowSyncStatus) error
DeleteRAGFlowSyncStatus(ctx context.Context, memoID int32) error
ListPendingSyncMemos(ctx context.Context, limit int) ([]*RAGFlowSyncStatus, error)
```

### Step 1.3: Create Sync Status Store Model

**File: `store/ragflow_sync.go`** (NEW)

```go
package store

import "context"

// SyncStatus represents the sync state of a memo with RAGFlow.
type SyncStatus string

const (
	SyncStatusPending SyncStatus = "pending"
	SyncStatusSynced  SyncStatus = "synced"
	SyncStatusFailed  SyncStatus = "failed"
	SyncStatusDeleted SyncStatus = "deleted"
)

// RAGFlowSyncStatus tracks the sync state of memos with RAGFlow.
type RAGFlowSyncStatus struct {
	MemoID            int32
	RAGFlowDocumentID string
	LastSyncedAt      int64
	SyncStatus        SyncStatus
	ContentHash       string // MD5 hash of memo content for change detection
	ErrorMessage      string
}

// UpdateRAGFlowSyncStatus specifies fields to update.
type UpdateRAGFlowSyncStatus struct {
	MemoID            int32
	RAGFlowDocumentID *string
	LastSyncedAt      *int64
	SyncStatus        *SyncStatus
	ContentHash       *string
	ErrorMessage      *string
}

// Store methods

func (s *Store) CreateRAGFlowSyncStatus(ctx context.Context, create *RAGFlowSyncStatus) (*RAGFlowSyncStatus, error) {
	return s.driver.CreateRAGFlowSyncStatus(ctx, create)
}

func (s *Store) GetRAGFlowSyncStatus(ctx context.Context, memoID int32) (*RAGFlowSyncStatus, error) {
	return s.driver.GetRAGFlowSyncStatus(ctx, memoID)
}

func (s *Store) UpdateRAGFlowSyncStatus(ctx context.Context, update *UpdateRAGFlowSyncStatus) error {
	return s.driver.UpdateRAGFlowSyncStatus(ctx, update)
}

func (s *Store) DeleteRAGFlowSyncStatus(ctx context.Context, memoID int32) error {
	return s.driver.DeleteRAGFlowSyncStatus(ctx, memoID)
}

func (s *Store) ListPendingSyncMemos(ctx context.Context, limit int) ([]*RAGFlowSyncStatus, error) {
	return s.driver.ListPendingSyncMemos(ctx, limit)
}
```

### Step 1.4: Create Database Migrations

**File: `store/migration/sqlite/prod/0.XX/01__add_ragflow_sync.sql`**

```sql
-- RAGFlow sync status tracking
CREATE TABLE ragflow_sync_status (
    memo_id INTEGER PRIMARY KEY,
    ragflow_document_id TEXT NOT NULL DEFAULT '',
    last_synced_at BIGINT NOT NULL DEFAULT 0,
    sync_status TEXT NOT NULL DEFAULT 'pending' CHECK (sync_status IN ('pending', 'synced', 'failed', 'deleted')),
    content_hash TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (memo_id) REFERENCES memo(id) ON DELETE CASCADE
);

CREATE INDEX idx_ragflow_sync_status ON ragflow_sync_status(sync_status);
CREATE INDEX idx_ragflow_sync_last_synced ON ragflow_sync_status(last_synced_at);

-- Add ragflow_conversation_id to ai_conversation table
ALTER TABLE ai_conversation ADD COLUMN ragflow_conversation_id TEXT NOT NULL DEFAULT '';
```

**File: `store/migration/mysql/prod/0.XX/01__add_ragflow_sync.sql`**

```sql
-- RAGFlow sync status tracking
CREATE TABLE ragflow_sync_status (
    memo_id INT PRIMARY KEY,
    ragflow_document_id VARCHAR(256) NOT NULL DEFAULT '',
    last_synced_at BIGINT NOT NULL DEFAULT 0,
    sync_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL,
    FOREIGN KEY (memo_id) REFERENCES memo(id) ON DELETE CASCADE,
    INDEX idx_ragflow_sync_status (sync_status),
    INDEX idx_ragflow_sync_last_synced (last_synced_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Add ragflow_conversation_id to ai_conversation table
ALTER TABLE ai_conversation ADD COLUMN ragflow_conversation_id VARCHAR(256) NOT NULL DEFAULT '';
```

**File: `store/migration/postgres/prod/0.XX/01__add_ragflow_sync.sql`**

```sql
-- RAGFlow sync status tracking
CREATE TABLE ragflow_sync_status (
    memo_id INT PRIMARY KEY REFERENCES memo(id) ON DELETE CASCADE,
    ragflow_document_id VARCHAR(256) NOT NULL DEFAULT '',
    last_synced_at BIGINT NOT NULL DEFAULT 0,
    sync_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_ragflow_sync_status ON ragflow_sync_status(sync_status);
CREATE INDEX idx_ragflow_sync_last_synced ON ragflow_sync_status(last_synced_at);

-- Add ragflow_conversation_id to ai_conversation table
ALTER TABLE ai_conversation ADD COLUMN ragflow_conversation_id VARCHAR(256) NOT NULL DEFAULT '';
```

### Step 1.5: Update AI Conversation Model

**File: `store/ai_conversation.go`** (UPDATE)

```go
// Add to AIConversation struct:
type AIConversation struct {
	// ...existing fields...
	RAGFlowConversationID string // NEW: RAGFlow conversation/session ID
}

// Add to UpdateAIConversation struct:
type UpdateAIConversation struct {
	// ...existing fields...
	RAGFlowConversationID *string // NEW
}
```

---

## Phase 2: RAGFlow Client Plugin

### Step 2.1: Create Plugin Directory Structure

```
plugin/ragflow/
├── client.go           # HTTP client with auth, retry, error handling
├── config.go           # Configuration management
├── dataset.go          # Dataset (knowledge base) operations
├── document.go         # Document upload/delete operations
├── chat.go             # Conversation and chat operations
├── search.go           # Semantic search operations
├── types.go            # Request/response types
└── README.md           # Plugin documentation
```

### Step 2.2: Define Types

**File: `plugin/ragflow/types.go`**

```go
package ragflow

import "time"

// Dataset represents a RAGFlow knowledge base.
type Dataset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ChunkMethod string    `json:"chunk_method"`
	CreatedAt   time.Time `json:"create_time"`
	UpdatedAt   time.Time `json:"update_time"`
}

// Document represents a document in a RAGFlow dataset.
type Document struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DatasetID string    `json:"dataset_id"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"` // "pending", "parsing", "done", "failed"
	CreatedAt time.Time `json:"create_time"`
}

// Conversation represents a RAGFlow chat conversation.
type Conversation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"create_time"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "user", "assistant"
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	DatasetIDs     []string `json:"dataset_ids,omitempty"`
	Question       string `json:"question"`
	Stream         bool   `json:"stream"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	Answer     string      `json:"answer"`
	References []Reference `json:"references,omitempty"`
}

// Reference represents a source citation.
type Reference struct {
	DocumentID   string  `json:"document_id"`
	DocumentName string  `json:"document_name"`
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
}

// SearchRequest represents a semantic search request.
type SearchRequest struct {
	DatasetIDs []string `json:"dataset_ids"`
	Question   string   `json:"question"`
	TopK       int      `json:"top_k"`
}

// SearchResult represents a search result.
type SearchResult struct {
	DocumentID   string            `json:"document_id"`
	DocumentName string            `json:"document_name"`
	Content      string            `json:"content"`
	Score        float64           `json:"score"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// APIError represents a RAGFlow API error.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}
```

### Step 2.3: Create Configuration

**File: `plugin/ragflow/config.go`**

```go
package ragflow

import (
	"fmt"
	"time"
)

// Config holds RAGFlow client configuration.
type Config struct {
	// Enabled indicates whether RAGFlow integration is enabled.
	Enabled bool `json:"enabled"`

	// Endpoint is the RAGFlow API base URL.
	Endpoint string `json:"endpoint"`

	// APIKey is the RAGFlow API key for authentication.
	APIKey string `json:"api_key"`

	// DatasetID is the default dataset (knowledge base) ID.
	DatasetID string `json:"dataset_id"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout"`

	// RetryAttempts is the number of retry attempts for failed requests.
	RetryAttempts int `json:"retry_attempts"`

	// AutoIndex enables automatic indexing of new/updated memos.
	AutoIndex bool `json:"auto_index"`

	// SyncInterval is the interval for background sync (e.g., "5m").
	SyncInterval time.Duration `json:"sync_interval"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:       false,
		Endpoint:      "http://localhost:9380",
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		AutoIndex:     true,
		SyncInterval:  5 * time.Minute,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Endpoint == "" {
		return fmt.Errorf("ragflow endpoint is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("ragflow api_key is required")
	}
	return nil
}
```

### Step 2.4: Create HTTP Client

**File: `plugin/ragflow/client.go`**

```go
package ragflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the RAGFlow HTTP client.
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient creates a new RAGFlow client.
func NewClient(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// IsEnabled returns whether RAGFlow is enabled.
func (c *Client) IsEnabled() bool {
	return c.config.Enabled
}

// GetConfig returns the client configuration.
func (c *Client) GetConfig() *Config {
	return c.config
}

// doRequest performs an HTTP request with authentication and error handling.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := c.config.Endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	// Retry logic
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break
		}
		if attempt < c.config.RetryAttempts {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.config.RetryAttempts, lastErr)
	}

	return resp, nil
}

// parseResponse parses the response body into the target struct.
func (c *Client) parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return &apiErr
	}

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// HealthCheck checks if RAGFlow service is available.
func (c *Client) HealthCheck(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}
	return nil
}
```

### Step 2.5: Implement Dataset Operations

**File: `plugin/ragflow/dataset.go`**

```go
package ragflow

import (
	"context"
	"fmt"
	"net/http"
)

// CreateDataset creates a new dataset (knowledge base).
func (c *Client) CreateDataset(ctx context.Context, name, description string) (*Dataset, error) {
	body := map[string]interface{}{
		"name":         name,
		"description":  description,
		"chunk_method": "naive", // or "qa", "manual", etc.
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/datasets", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Dataset `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// GetDataset retrieves a dataset by ID.
func (c *Client) GetDataset(ctx context.Context, datasetID string) (*Dataset, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s", datasetID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Dataset `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// DeleteDataset deletes a dataset.
func (c *Client) DeleteDataset(ctx context.Context, datasetID string) error {
	path := fmt.Sprintf("/api/v1/datasets/%s", datasetID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.parseResponse(resp, nil)
}
```

### Step 2.6: Implement Document Operations

**File: `plugin/ragflow/document.go`**

```go
package ragflow

import (
	"context"
	"fmt"
	"net/http"
)

// UploadDocument uploads a document to a dataset.
func (c *Client) UploadDocument(ctx context.Context, datasetID, name, content string, metadata map[string]string) (*Document, error) {
	body := map[string]interface{}{
		"name":     name,
		"content":  content,
		"metadata": metadata,
	}

	path := fmt.Sprintf("/api/v1/datasets/%s/documents", datasetID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Document `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// UpdateDocument updates an existing document.
func (c *Client) UpdateDocument(ctx context.Context, datasetID, documentID, name, content string, metadata map[string]string) (*Document, error) {
	body := map[string]interface{}{
		"name":     name,
		"content":  content,
		"metadata": metadata,
	}

	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s", datasetID, documentID)
	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Document `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// DeleteDocument deletes a document from a dataset.
func (c *Client) DeleteDocument(ctx context.Context, datasetID, documentID string) error {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s", datasetID, documentID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.parseResponse(resp, nil)
}

// GetDocumentStatus checks the parsing status of a document.
func (c *Client) GetDocumentStatus(ctx context.Context, datasetID, documentID string) (*Document, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s", datasetID, documentID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Document `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}
```

### Step 2.7: Implement Search Operations

**File: `plugin/ragflow/search.go`**

```go
package ragflow

import (
	"context"
	"net/http"
)

// Search performs semantic search across datasets.
func (c *Client) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	if req.TopK == 0 {
		req.TopK = 10
	}
	if len(req.DatasetIDs) == 0 && c.config.DatasetID != "" {
		req.DatasetIDs = []string{c.config.DatasetID}
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/retrieval", req)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []SearchResult `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}
```

### Step 2.8: Implement Chat Operations

**File: `plugin/ragflow/chat.go`**

```go
package ragflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateConversation creates a new chat conversation.
func (c *Client) CreateConversation(ctx context.Context, name string, datasetIDs []string) (*Conversation, error) {
	if len(datasetIDs) == 0 && c.config.DatasetID != "" {
		datasetIDs = []string{c.config.DatasetID}
	}

	body := map[string]interface{}{
		"name":        name,
		"dataset_ids": datasetIDs,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/conversations", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data Conversation `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// DeleteConversation deletes a conversation.
func (c *Client) DeleteConversation(ctx context.Context, conversationID string) error {
	path := fmt.Sprintf("/api/v1/conversations/%s", conversationID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return c.parseResponse(resp, nil)
}

// Chat sends a message and gets a response.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	if len(req.DatasetIDs) == 0 && c.config.DatasetID != "" {
		req.DatasetIDs = []string{c.config.DatasetID}
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/chat", req)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data ChatResponse `json:"data"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ChatStream sends a message and streams the response.
func (c *Client) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	req.Stream = true
	if len(req.DatasetIDs) == 0 && c.config.DatasetID != "" {
		req.DatasetIDs = []string{c.config.DatasetID}
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/chat", req)
	if err != nil {
		return nil, err
	}

	chunks := make(chan StreamChunk)
	go func() {
		defer close(chunks)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}

			// Parse SSE data
			if len(line) > 6 && line[:6] == "data: " {
				var chunk StreamChunk
				if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
					continue
				}
				select {
				case chunks <- chunk:
				case <-ctx.Done():
					return
				}
				if chunk.Done {
					return
				}
			}
		}
	}()

	return chunks, nil
}
```

### Step 2.9: Create Plugin README

**File: `plugin/ragflow/README.md`**

```markdown
# RAGFlow Plugin

This plugin provides integration with RAGFlow, an open-source RAG (Retrieval-Augmented Generation) engine.

## Features

- Document indexing and parsing
- Semantic search across memos
- RAG-powered chat with citations
- Automatic background sync

## Configuration

| Variable                | Description                | Default                 |
| ----------------------- | -------------------------- | ----------------------- |
| `RAGFLOW_ENABLED`       | Enable RAGFlow integration | `false`                 |
| `RAGFLOW_ENDPOINT`      | RAGFlow API endpoint       | `http://localhost:9380` |
| `RAGFLOW_API_KEY`       | RAGFlow API key            | (required)              |
| `RAGFLOW_DATASET_ID`    | Default dataset ID         | (required)              |
| `RAGFLOW_AUTO_INDEX`    | Auto-index new memos       | `true`                  |
| `RAGFLOW_SYNC_INTERVAL` | Background sync interval   | `5m`                    |

## Usage

See the main documentation for API usage and frontend integration.
```

---

## Phase 3: Protocol Buffer Definitions

### Step 3.1: Create RAGFlow Service Proto

**File: `proto/api/v1/ragflow_service.proto`**

```protobuf
syntax = "proto3";

package memos.api.v1;

import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

option go_package = "gen/api/v1";

// RAGFlowService provides RAG-powered AI capabilities.
service RAGFlowService {
  // SemanticSearch performs semantic search across indexed memos.
  rpc SemanticSearch(SemanticSearchRequest) returns (SemanticSearchResponse) {
    option (google.api.http) = {
      post: "/api/v1/ragflow/search"
      body: "*"
    };
  }

  // CreateChat creates a new RAG chat conversation.
  rpc CreateChat(CreateChatRequest) returns (Chat) {
    option (google.api.http) = {
      post: "/api/v1/ragflow/chats"
      body: "*"
    };
  }

  // ListChats lists all chat conversations for the current user.
  rpc ListChats(ListChatsRequest) returns (ListChatsResponse) {
    option (google.api.http) = {get: "/api/v1/ragflow/chats"};
  }

  // GetChat retrieves a chat conversation with messages.
  rpc GetChat(GetChatRequest) returns (Chat) {
    option (google.api.http) = {get: "/api/v1/ragflow/chats/{chat_id}"};
  }

  // DeleteChat deletes a chat conversation.
  rpc DeleteChat(DeleteChatRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {delete: "/api/v1/ragflow/chats/{chat_id}"};
  }

  // SendChatMessage sends a message and gets AI response.
  rpc SendChatMessage(SendChatMessageRequest) returns (SendChatMessageResponse) {
    option (google.api.http) = {
      post: "/api/v1/ragflow/chats/{chat_id}/messages"
      body: "*"
    };
  }

  // StreamChatMessage sends a message and streams AI response.
  rpc StreamChatMessage(StreamChatMessageRequest) returns (stream StreamChatMessageResponse) {
    option (google.api.http) = {
      post: "/api/v1/ragflow/chats/{chat_id}/messages:stream"
      body: "*"
    };
  }

  // GetRAGFlowStatus returns the RAGFlow integration status and config.
  rpc GetRAGFlowStatus(GetRAGFlowStatusRequest) returns (GetRAGFlowStatusResponse) {
    option (google.api.http) = {get: "/api/v1/ragflow/status"};
  }

  // TriggerSync manually triggers memo sync to RAGFlow.
  rpc TriggerSync(TriggerSyncRequest) returns (TriggerSyncResponse) {
    option (google.api.http) = {
      post: "/api/v1/ragflow/sync"
      body: "*"
    };
  }
}

// SemanticSearchRequest is the request for semantic search.
message SemanticSearchRequest {
  // The search query.
  string query = 1 [(google.api.field_behavior) = REQUIRED];

  // Maximum number of results to return.
  int32 top_k = 2;

  // Minimum similarity score (0.0 - 1.0).
  float similarity_threshold = 3;

  // Filter by tags (optional).
  repeated string tags = 4;
}

// SemanticSearchResponse is the response for semantic search.
message SemanticSearchResponse {
  repeated SearchResult results = 1;
}

// SearchResult represents a single search result.
message SearchResult {
  // The memo resource name (memos/{id}).
  string memo_name = 1;

  // Relevant content snippet.
  string content_snippet = 2;

  // Similarity score (0.0 - 1.0).
  float similarity_score = 3;

  // Highlighted matches.
  repeated string highlights = 4;
}

// Chat represents a RAG chat conversation.
message Chat {
  // Unique identifier.
  string id = 1 [(google.api.field_behavior) = OUTPUT_ONLY];

  // Chat title.
  string title = 2;

  // Creation timestamp.
  google.protobuf.Timestamp create_time = 3 [(google.api.field_behavior) = OUTPUT_ONLY];

  // Last update timestamp.
  google.protobuf.Timestamp update_time = 4 [(google.api.field_behavior) = OUTPUT_ONLY];

  // Messages in this chat (populated on GetChat).
  repeated ChatMessage messages = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// ChatMessage represents a message in a chat.
message ChatMessage {
  // Unique identifier.
  string id = 1 [(google.api.field_behavior) = OUTPUT_ONLY];

  // Message role (user or assistant).
  ChatMessageRole role = 2;

  // Message content.
  string content = 3;

  // Creation timestamp.
  google.protobuf.Timestamp create_time = 4 [(google.api.field_behavior) = OUTPUT_ONLY];

  // Source references (for assistant messages).
  repeated Reference references = 5;
}

enum ChatMessageRole {
  CHAT_MESSAGE_ROLE_UNSPECIFIED = 0;
  USER = 1;
  ASSISTANT = 2;
}

// Reference represents a source citation.
message Reference {
  // The memo resource name.
  string memo_name = 1;

  // Relevant content from the memo.
  string content = 2;

  // Relevance score.
  float score = 3;
}

// Request/Response messages

message CreateChatRequest {
  string title = 1;
}

message ListChatsRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message ListChatsResponse {
  repeated Chat chats = 1;
  string next_page_token = 2;
}

message GetChatRequest {
  string chat_id = 1 [(google.api.field_behavior) = REQUIRED];
}

message DeleteChatRequest {
  string chat_id = 1 [(google.api.field_behavior) = REQUIRED];
}

message SendChatMessageRequest {
  string chat_id = 1 [(google.api.field_behavior) = REQUIRED];
  string content = 2 [(google.api.field_behavior) = REQUIRED];
}

message SendChatMessageResponse {
  ChatMessage user_message = 1;
  ChatMessage assistant_message = 2;
}

message StreamChatMessageRequest {
  string chat_id = 1 [(google.api.field_behavior) = REQUIRED];
  string content = 2 [(google.api.field_behavior) = REQUIRED];
}

message StreamChatMessageResponse {
  // Streaming content chunk.
  string chunk = 1;

  // Whether this is the final chunk.
  bool done = 2;

  // The complete assistant message (sent with final chunk).
  ChatMessage assistant_message = 3;
}

message GetRAGFlowStatusRequest {}

message GetRAGFlowStatusResponse {
  // Whether RAGFlow is enabled.
  bool enabled = 1;

  // Whether RAGFlow service is healthy.
  bool healthy = 2;

  // Number of indexed memos.
  int32 indexed_count = 3;

  // Number of pending memos.
  int32 pending_count = 4;

  // Last sync timestamp.
  google.protobuf.Timestamp last_sync_time = 5;
}

message TriggerSyncRequest {
  // If true, re-index all memos. Otherwise, only sync pending.
  bool full_sync = 1;
}

message TriggerSyncResponse {
  // Number of memos queued for sync.
  int32 queued_count = 1;
}
```

### Step 3.2: Regenerate Protobuf Code

```bash
cd proto && buf generate
```

---

## Phase 4: Backend Service Implementation

### Step 4.1: Create RAGFlow Service

**File: `server/router/api/v1/ragflow_service.go`**

```go
package v1

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/ragflow"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

// SemanticSearch performs semantic search across indexed memos.
func (s *APIV1Service) SemanticSearch(ctx context.Context, req *v1pb.SemanticSearchRequest) (*v1pb.SemanticSearchResponse, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !s.RAGFlowClient.IsEnabled() {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow is not enabled")
	}

	// Call RAGFlow search
	topK := int(req.TopK)
	if topK == 0 {
		topK = 10
	}

	results, err := s.RAGFlowClient.Search(ctx, &ragflow.SearchRequest{
		Question: req.Query,
		TopK:     topK,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Map RAGFlow results to memos and filter by user access
	var protoResults []*v1pb.SearchResult
	for _, r := range results {
		// Get memo by RAGFlow document ID
		syncStatus, err := s.Store.GetRAGFlowSyncStatusByDocumentID(ctx, r.DocumentID)
		if err != nil {
			continue // Skip if not found
		}

		// Get memo and check access
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{ID: &syncStatus.MemoID})
		if err != nil || memo == nil {
			continue
		}

		// Check user has access to this memo
		if !s.canUserAccessMemo(ctx, user, memo) {
			continue
		}

		// Apply similarity threshold
		if req.SimilarityThreshold > 0 && float32(r.Score) < req.SimilarityThreshold {
			continue
		}

		protoResults = append(protoResults, &v1pb.SearchResult{
			MemoName:        fmt.Sprintf("memos/%d", memo.ID),
			ContentSnippet:  r.Content,
			SimilarityScore: float32(r.Score),
		})
	}

	return &v1pb.SemanticSearchResponse{Results: protoResults}, nil
}

// CreateChat creates a new RAG chat conversation.
func (s *APIV1Service) CreateChat(ctx context.Context, req *v1pb.CreateChatRequest) (*v1pb.Chat, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !s.RAGFlowClient.IsEnabled() {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow is not enabled")
	}

	// Create conversation in RAGFlow
	ragflowConv, err := s.RAGFlowClient.CreateConversation(ctx, req.Title, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create RAGFlow conversation: %v", err)
	}

	// Create local conversation record
	title := req.Title
	if title == "" {
		title = "New Chat"
	}

	conversation := &store.AIConversation{
		UID:                   util.GenerateUID(),
		UserID:                user.ID,
		Title:                 title,
		RAGFlowConversationID: ragflowConv.ID,
	}

	created, err := s.Store.CreateAIConversation(ctx, conversation)
	if err != nil {
		// Try to clean up RAGFlow conversation
		_ = s.RAGFlowClient.DeleteConversation(ctx, ragflowConv.ID)
		return nil, status.Errorf(codes.Internal, "failed to create conversation: %v", err)
	}

	return s.convertConversationToChat(created), nil
}

// SendChatMessage sends a message and gets AI response.
func (s *APIV1Service) SendChatMessage(ctx context.Context, req *v1pb.SendChatMessageRequest) (*v1pb.SendChatMessageResponse, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !s.RAGFlowClient.IsEnabled() {
		return nil, status.Errorf(codes.FailedPrecondition, "RAGFlow is not enabled")
	}

	// Get conversation and verify ownership
	conversation, err := s.getConversationByUID(ctx, req.ChatId, user.ID)
	if err != nil {
		return nil, err
	}

	// Save user message locally
	userMsg := &store.AIMessage{
		UID:            util.GenerateUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleUser,
		Content:        req.Content,
		CreatedTs:      time.Now().Unix(),
	}
	if _, err := s.Store.CreateAIMessage(ctx, userMsg); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save user message: %v", err)
	}

	// Call RAGFlow chat
	chatResp, err := s.RAGFlowClient.Chat(ctx, &ragflow.ChatRequest{
		ConversationID: conversation.RAGFlowConversationID,
		Question:       req.Content,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "chat request failed: %v", err)
	}

	// Save assistant message locally
	assistantMsg := &store.AIMessage{
		UID:            util.GenerateUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleAssistant,
		Content:        chatResp.Answer,
		CreatedTs:      time.Now().Unix(),
	}
	if _, err := s.Store.CreateAIMessage(ctx, assistantMsg); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save assistant message: %v", err)
	}

	// Convert references to proto
	var references []*v1pb.Reference
	for _, ref := range chatResp.References {
		references = append(references, &v1pb.Reference{
			MemoName: ref.DocumentName, // Map back to memo name
			Content:  ref.Content,
			Score:    float32(ref.Score),
		})
	}

	return &v1pb.SendChatMessageResponse{
		UserMessage: &v1pb.ChatMessage{
			Id:         userMsg.UID,
			Role:       v1pb.ChatMessageRole_USER,
			Content:    userMsg.Content,
			CreateTime: timestamppb.New(time.Unix(userMsg.CreatedTs, 0)),
		},
		AssistantMessage: &v1pb.ChatMessage{
			Id:         assistantMsg.UID,
			Role:       v1pb.ChatMessageRole_ASSISTANT,
			Content:    assistantMsg.Content,
			CreateTime: timestamppb.New(time.Unix(assistantMsg.CreatedTs, 0)),
			References: references,
		},
	}, nil
}

// StreamChatMessage sends a message and streams AI response.
func (s *APIV1Service) StreamChatMessage(req *v1pb.StreamChatMessageRequest, stream v1pb.RAGFlowService_StreamChatMessageServer) error {
	ctx := stream.Context()

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !s.RAGFlowClient.IsEnabled() {
		return status.Errorf(codes.FailedPrecondition, "RAGFlow is not enabled")
	}

	// Get conversation and verify ownership
	conversation, err := s.getConversationByUID(ctx, req.ChatId, user.ID)
	if err != nil {
		return err
	}

	// Save user message
	userMsg := &store.AIMessage{
		UID:            util.GenerateUID(),
		ConversationID: conversation.ID,
		Role:           store.AIMessageRoleUser,
		Content:        req.Content,
		CreatedTs:      time.Now().Unix(),
	}
	if _, err := s.Store.CreateAIMessage(ctx, userMsg); err != nil {
		return status.Errorf(codes.Internal, "failed to save user message: %v", err)
	}

	// Stream from RAGFlow
	chunks, err := s.RAGFlowClient.ChatStream(ctx, &ragflow.ChatRequest{
		ConversationID: conversation.RAGFlowConversationID,
		Question:       req.Content,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "stream request failed: %v", err)
	}

	var fullContent string
	for chunk := range chunks {
		fullContent += chunk.Content

		resp := &v1pb.StreamChatMessageResponse{
			Chunk: chunk.Content,
			Done:  chunk.Done,
		}

		if chunk.Done {
			// Save complete assistant message
			assistantMsg := &store.AIMessage{
				UID:            util.GenerateUID(),
				ConversationID: conversation.ID,
				Role:           store.AIMessageRoleAssistant,
				Content:        fullContent,
				CreatedTs:      time.Now().Unix(),
			}
			if _, err := s.Store.CreateAIMessage(ctx, assistantMsg); err != nil {
				// Log error but don't fail the stream
			}

			resp.AssistantMessage = &v1pb.ChatMessage{
				Id:         assistantMsg.UID,
				Role:       v1pb.ChatMessageRole_ASSISTANT,
				Content:    fullContent,
				CreateTime: timestamppb.New(time.Unix(assistantMsg.CreatedTs, 0)),
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

// GetRAGFlowStatus returns the RAGFlow integration status.
func (s *APIV1Service) GetRAGFlowStatus(ctx context.Context, req *v1pb.GetRAGFlowStatusRequest) (*v1pb.GetRAGFlowStatusResponse, error) {
	enabled := s.RAGFlowClient.IsEnabled()

	resp := &v1pb.GetRAGFlowStatusResponse{
		Enabled: enabled,
	}

	if !enabled {
		return resp, nil
	}

	// Check health
	if err := s.RAGFlowClient.HealthCheck(ctx); err != nil {
		resp.Healthy = false
	} else {
		resp.Healthy = true
	}

	// Get sync stats
	// TODO: Implement these store methods
	// resp.IndexedCount = ...
	// resp.PendingCount = ...
	// resp.LastSyncTime = ...

	return resp, nil
}

// Helper methods

func (s *APIV1Service) getConversationByUID(ctx context.Context, uid string, userID int32) (*store.AIConversation, error) {
	conversations, err := s.Store.ListAIConversations(ctx, &store.FindAIConversation{
		UID:    &uid,
		UserID: &userID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if len(conversations) == 0 {
		return nil, status.Errorf(codes.NotFound, "conversation not found")
	}
	return conversations[0], nil
}

func (s *APIV1Service) convertConversationToChat(c *store.AIConversation) *v1pb.Chat {
	return &v1pb.Chat{
		Id:         c.UID,
		Title:      c.Title,
		CreateTime: timestamppb.New(time.Unix(c.CreatedTs, 0)),
		UpdateTime: timestamppb.New(time.Unix(c.UpdatedTs, 0)),
	}
}

func (s *APIV1Service) canUserAccessMemo(ctx context.Context, user *store.User, memo *store.Memo) bool {
	// User owns the memo
	if memo.CreatorID == user.ID {
		return true
	}
	// Memo is public
	if memo.Visibility == store.Public {
		return true
	}
	// TODO: Add more access control logic (e.g., shared memos)
	return false
}

func contentHash(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}
```

### Step 4.2: Register RAGFlow Service

**File: `server/router/api/v1/v1.go`** (UPDATE)

Add to `APIV1Service` struct:

```go
type APIV1Service struct {
	// ...existing fields...
	RAGFlowClient *ragflow.Client
}
```

Add to service registration:

```go
// In RegisterGateway function:
if err := v1pb.RegisterRAGFlowServiceHandlerServer(ctx, gwMux, s); err != nil {
	return err
}
```

### Step 4.3: Update Server Initialization

**File: `server/server.go`** (UPDATE)

Add RAGFlow client initialization:

```go
// In NewServer or server setup:
ragflowConfig := &ragflow.Config{
	Enabled:   profile.RAGFlowEnabled,
	Endpoint:  profile.RAGFlowEndpoint,
	APIKey:    profile.RAGFlowAPIKey,
	DatasetID: profile.RAGFlowDatasetID,
	// ...
}

ragflowClient, err := ragflow.NewClient(ragflowConfig)
if err != nil {
	// Handle error or log warning
}

// Pass to APIV1Service
apiV1Service.RAGFlowClient = ragflowClient
```

---

## Phase 5: Background Sync Runner

### Step 5.1: Create Sync Runner

**File: `server/runner/ragflowsync/runner.go`**

```go
package ragflowsync

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// Runner syncs memos to RAGFlow in the background.
type Runner struct {
	Store         *store.Store
	RAGFlowClient *ragflow.Client
	Interval      time.Duration
}

// NewRunner creates a new sync runner.
func NewRunner(s *store.Store, client *ragflow.Client, interval time.Duration) *Runner {
	return &Runner{
		Store:         s,
		RAGFlowClient: client,
		Interval:      interval,
	}
}

// Run starts the background sync loop.
func (r *Runner) Run(ctx context.Context) {
	if !r.RAGFlowClient.IsEnabled() {
		return
	}

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	// Initial sync
	r.syncPendingMemos(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.syncPendingMemos(ctx)
		}
	}
}

// syncPendingMemos syncs all pending memos to RAGFlow.
func (r *Runner) syncPendingMemos(ctx context.Context) {
	// Get memos that need syncing
	pendingList, err := r.Store.ListPendingSyncMemos(ctx, 100)
	if err != nil {
		// Log error
		return
	}

	for _, syncStatus := range pendingList {
		if err := r.syncMemo(ctx, syncStatus); err != nil {
			// Mark as failed
			r.Store.UpdateRAGFlowSyncStatus(ctx, &store.UpdateRAGFlowSyncStatus{
				MemoID:       syncStatus.MemoID,
				SyncStatus:   ptr(store.SyncStatusFailed),
				ErrorMessage: ptr(err.Error()),
			})
		}
	}
}

// syncMemo syncs a single memo to RAGFlow.
func (r *Runner) syncMemo(ctx context.Context, syncStatus *store.RAGFlowSyncStatus) error {
	// Get memo
	memos, err := r.Store.ListMemos(ctx, &store.FindMemo{ID: &syncStatus.MemoID})
	if err != nil || len(memos) == 0 {
		return fmt.Errorf("memo not found: %d", syncStatus.MemoID)
	}
	memo := memos[0]

	// Check if content changed
	newHash := contentHash(memo.Content)
	if syncStatus.RAGFlowDocumentID != "" && syncStatus.ContentHash == newHash {
		// No change, mark as synced
		return r.Store.UpdateRAGFlowSyncStatus(ctx, &store.UpdateRAGFlowSyncStatus{
			MemoID:       syncStatus.MemoID,
			SyncStatus:   ptr(store.SyncStatusSynced),
			LastSyncedAt: ptr(time.Now().Unix()),
		})
	}

	// Prepare metadata
	metadata := map[string]string{
		"memo_id":    fmt.Sprintf("%d", memo.ID),
		"creator_id": fmt.Sprintf("%d", memo.CreatorID),
		"visibility": string(memo.Visibility),
	}

	var documentID string
	config := r.RAGFlowClient.GetConfig()

	if syncStatus.RAGFlowDocumentID == "" {
		// Create new document
		doc, err := r.RAGFlowClient.UploadDocument(ctx, config.DatasetID,
			fmt.Sprintf("memo_%d", memo.ID),
			memo.Content,
			metadata,
		)
		if err != nil {
			return fmt.Errorf("failed to upload document: %w", err)
		}
		documentID = doc.ID
	} else {
		// Update existing document
		doc, err := r.RAGFlowClient.UpdateDocument(ctx, config.DatasetID,
			syncStatus.RAGFlowDocumentID,
			fmt.Sprintf("memo_%d", memo.ID),
			memo.Content,
			metadata,
		)
		if err != nil {
			return fmt.Errorf("failed to update document: %w", err)
		}
		documentID = doc.ID
	}

	// Update sync status
	return r.Store.UpdateRAGFlowSyncStatus(ctx, &store.UpdateRAGFlowSyncStatus{
		MemoID:            syncStatus.MemoID,
		RAGFlowDocumentID: &documentID,
		SyncStatus:        ptr(store.SyncStatusSynced),
		ContentHash:       &newHash,
		LastSyncedAt:      ptr(time.Now().Unix()),
		ErrorMessage:      ptr(""),
	})
}

// QueueMemoForSync queues a memo for background sync.
func (r *Runner) QueueMemoForSync(ctx context.Context, memoID int32) error {
	// Check if sync status exists
	existing, _ := r.Store.GetRAGFlowSyncStatus(ctx, memoID)
	if existing != nil {
		// Update to pending
		return r.Store.UpdateRAGFlowSyncStatus(ctx, &store.UpdateRAGFlowSyncStatus{
			MemoID:     memoID,
			SyncStatus: ptr(store.SyncStatusPending),
		})
	}

	// Create new sync status
	_, err := r.Store.CreateRAGFlowSyncStatus(ctx, &store.RAGFlowSyncStatus{
		MemoID:     memoID,
		SyncStatus: store.SyncStatusPending,
	})
	return err
}

// DeleteMemoFromRAGFlow removes a memo from RAGFlow.
func (r *Runner) DeleteMemoFromRAGFlow(ctx context.Context, memoID int32) error {
	syncStatus, err := r.Store.GetRAGFlowSyncStatus(ctx, memoID)
	if err != nil || syncStatus.RAGFlowDocumentID == "" {
		return nil // Nothing to delete
	}

	config := r.RAGFlowClient.GetConfig()
	if err := r.RAGFlowClient.DeleteDocument(ctx, config.DatasetID, syncStatus.RAGFlowDocumentID); err != nil {
		return err
	}

	return r.Store.DeleteRAGFlowSyncStatus(ctx, memoID)
}

func contentHash(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

func ptr[T any](v T) *T {
	return &v
}
```

### Step 5.2: Hook into Memo Lifecycle

**File: `server/router/api/v1/memo_service.go`** (UPDATE)

Add hooks for RAGFlow sync:

```go
// In CreateMemo:
func (s *APIV1Service) CreateMemo(ctx context.Context, req *v1pb.CreateMemoRequest) (*v1pb.Memo, error) {
	// ...existing code...

	// Queue for RAGFlow sync
	if s.RAGFlowClient != nil && s.RAGFlowClient.IsEnabled() {
		go s.RAGFlowSyncRunner.QueueMemoForSync(context.Background(), memo.ID)
	}

	return convertedMemo, nil
}

// In UpdateMemo:
func (s *APIV1Service) UpdateMemo(ctx context.Context, req *v1pb.UpdateMemoRequest) (*v1pb.Memo, error) {
	// ...existing code...

	// Queue for RAGFlow re-sync if content changed
	if s.RAGFlowClient != nil && s.RAGFlowClient.IsEnabled() && contentChanged {
		go s.RAGFlowSyncRunner.QueueMemoForSync(context.Background(), memo.ID)
	}

	return convertedMemo, nil
}

// In DeleteMemo:
func (s *APIV1Service) DeleteMemo(ctx context.Context, req *v1pb.DeleteMemoRequest) (*emptypb.Empty, error) {
	// ...existing code...

	// Delete from RAGFlow
	if s.RAGFlowClient != nil && s.RAGFlowClient.IsEnabled() {
		go s.RAGFlowSyncRunner.DeleteMemoFromRAGFlow(context.Background(), memoID)
	}

	return &emptypb.Empty{}, nil
}
```

---

## Phase 6: Frontend Integration

### Step 6.1: Create React Query Hooks

**File: `web/src/hooks/useRAGFlowQueries.ts`**

```typescript
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ragflowServiceClient } from "@/lib/connect";
import type {
  Chat,
  SearchResult,
  GetRAGFlowStatusResponse,
} from "@/types/proto/api/v1/ragflow_service_pb";

export const ragflowKeys = {
  all: ["ragflow"] as const,
  status: () => [...ragflowKeys.all, "status"] as const,
  search: (query: string) => [...ragflowKeys.all, "search", query] as const,
  chats: () => [...ragflowKeys.all, "chats"] as const,
  chat: (id: string) => [...ragflowKeys.all, "chat", id] as const,
};

// Status
export function useRAGFlowStatus() {
  return useQuery({
    queryKey: ragflowKeys.status(),
    queryFn: async () => {
      const response = await ragflowServiceClient.getRAGFlowStatus({});
      return response;
    },
    staleTime: 1000 * 60 * 5, // 5 minutes
  });
}

// Semantic Search
export function useSemanticSearch(
  query: string,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: ragflowKeys.search(query),
    queryFn: async () => {
      const response = await ragflowServiceClient.semanticSearch({
        query,
        topK: 10,
      });
      return response.results;
    },
    enabled: (options?.enabled ?? true) && query.length > 0,
    staleTime: 1000 * 60, // 1 minute
  });
}

// Chats
export function useRAGFlowChats() {
  return useQuery({
    queryKey: ragflowKeys.chats(),
    queryFn: async () => {
      const response = await ragflowServiceClient.listChats({});
      return response.chats;
    },
  });
}

export function useRAGFlowChat(
  chatId: string,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: ragflowKeys.chat(chatId),
    queryFn: async () => {
      const response = await ragflowServiceClient.getChat({ chatId });
      return response;
    },
    enabled: (options?.enabled ?? true) && !!chatId,
  });
}

export function useCreateRAGFlowChat() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (title?: string) => {
      const response = await ragflowServiceClient.createChat({ title });
      return response;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ragflowKeys.chats() });
    },
  });
}

export function useDeleteRAGFlowChat() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (chatId: string) => {
      await ragflowServiceClient.deleteChat({ chatId });
      return chatId;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ragflowKeys.chats() });
    },
  });
}

export function useSendRAGFlowMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      chatId,
      content,
    }: {
      chatId: string;
      content: string;
    }) => {
      const response = await ragflowServiceClient.sendChatMessage({
        chatId,
        content,
      });
      return response;
    },
    onSuccess: (_, { chatId }) => {
      queryClient.invalidateQueries({ queryKey: ragflowKeys.chat(chatId) });
    },
  });
}

export function useTriggerRAGFlowSync() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (fullSync = false) => {
      const response = await ragflowServiceClient.triggerSync({ fullSync });
      return response;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ragflowKeys.status() });
    },
  });
}
```

### Step 6.2: Update Connect Client

**File: `web/src/lib/connect.ts`** (UPDATE)

```typescript
import { RAGFlowService } from "@/types/proto/api/v1/ragflow_service_pb";

export const ragflowServiceClient = createPromiseClient(
  RAGFlowService,
  transport
);
```

### Step 6.3: Create Search Component

**File: `web/src/components/SemanticSearch.tsx`**

```typescript
import { useState } from "react";
import { Search, Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useSemanticSearch } from "@/hooks/useRAGFlowQueries";
import { useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";

interface Props {
  className?: string;
}

const SemanticSearch = ({ className }: Props) => {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const navigate = useNavigate();

  const { data: results, isLoading } = useSemanticSearch(debouncedQuery, {
    enabled: debouncedQuery.length >= 2,
  });

  // Debounce input
  const handleInputChange = (value: string) => {
    setQuery(value);
    const timeoutId = setTimeout(() => setDebouncedQuery(value), 300);
    return () => clearTimeout(timeoutId);
  };

  const handleResultClick = (memoName: string) => {
    const memoId = memoName.replace("memos/", "");
    navigate(`/memos/${memoId}`);
    setQuery("");
    setDebouncedQuery("");
  };

  return (
    <div className={cn("relative", className)}>
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => handleInputChange(e.target.value)}
          placeholder="Search your knowledge..."
          className="pl-10"
        />
        {isLoading && (
          <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 animate-spin" />
        )}
      </div>

      {results && results.length > 0 && (
        <div className="absolute top-full left-0 right-0 mt-2 bg-background border rounded-lg shadow-lg z-50 max-h-96 overflow-auto">
          {results.map((result, index) => (
            <button
              key={index}
              onClick={() => handleResultClick(result.memoName)}
              className="w-full px-4 py-3 text-left hover:bg-accent border-b last:border-b-0"
            >
              <p className="text-sm line-clamp-2">{result.contentSnippet}</p>
              <p className="text-xs text-muted-foreground mt-1">
                Score: {(result.similarityScore * 100).toFixed(1)}%
              </p>
            </button>
          ))}
        </div>
      )}
    </div>
  );
};

export default SemanticSearch;
```

### Step 6.4: Update AI Chat Page

**File: `web/src/pages/KnowtreeAI.tsx`** (UPDATE)

Refactor to use RAGFlow hooks instead of direct LLM hooks:

```typescript
// Replace imports
import {
  useRAGFlowChats,
  useRAGFlowChat,
  useCreateRAGFlowChat,
  useDeleteRAGFlowChat,
  useSendRAGFlowMessage,
  useRAGFlowStatus,
} from "@/hooks/useRAGFlowQueries";

// Update component to use RAGFlow hooks
const KnowtreeAI = () => {
  const { data: status } = useRAGFlowStatus();
  const { data: chats = [], isLoading } = useRAGFlowChats();
  // ... rest of implementation using RAGFlow hooks
};
```

---

## Phase 7: Configuration & Admin

### Step 7.1: Add Profile Configuration

**File: `internal/profile/profile.go`** (UPDATE)

```go
type Profile struct {
	// ...existing fields...

	// RAGFlow configuration
	RAGFlowEnabled      bool          `json:"ragflow_enabled"`
	RAGFlowEndpoint     string        `json:"ragflow_endpoint"`
	RAGFlowAPIKey       string        `json:"ragflow_api_key"`
	RAGFlowDatasetID    string        `json:"ragflow_dataset_id"`
	RAGFlowAutoIndex    bool          `json:"ragflow_auto_index"`
	RAGFlowSyncInterval time.Duration `json:"ragflow_sync_interval"`
}

// GetRAGFlowConfig returns RAGFlow configuration from profile.
func (p *Profile) GetRAGFlowConfig() *ragflow.Config {
	return &ragflow.Config{
		Enabled:       p.RAGFlowEnabled,
		Endpoint:      p.RAGFlowEndpoint,
		APIKey:        p.RAGFlowAPIKey,
		DatasetID:     p.RAGFlowDatasetID,
		AutoIndex:     p.RAGFlowAutoIndex,
		SyncInterval:  p.RAGFlowSyncInterval,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}
}
```

### Step 7.2: Environment Variables

Add to configuration documentation:

| Variable                      | Default                 | Description                |
| ----------------------------- | ----------------------- | -------------------------- |
| `MEMOS_RAGFLOW_ENABLED`       | `false`                 | Enable RAGFlow integration |
| `MEMOS_RAGFLOW_ENDPOINT`      | `http://localhost:9380` | RAGFlow API endpoint       |
| `MEMOS_RAGFLOW_API_KEY`       | ``                      | RAGFlow API key            |
| `MEMOS_RAGFLOW_DATASET_ID`    | ``                      | Default dataset ID         |
| `MEMOS_RAGFLOW_AUTO_INDEX`    | `true`                  | Auto-index new memos       |
| `MEMOS_RAGFLOW_SYNC_INTERVAL` | `5m`                    | Background sync interval   |

### Step 7.3: Admin Settings UI (Optional)

Create admin settings page for RAGFlow configuration:

- Enable/disable toggle
- Endpoint and API key fields
- Test connection button
- Manual sync trigger
- Index statistics display

---

## Phase 8: Testing & Deployment

### Step 8.1: Backend Unit Tests

**File: `plugin/ragflow/client_test.go`**

```go
package ragflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/retrieval", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": [{"document_id": "doc1", "content": "test", "score": 0.95}]}`))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		Enabled:  true,
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	results, err := client.Search(context.Background(), &SearchRequest{
		Question: "test query",
		TopK:     5,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "doc1", results[0].DocumentID)
}

func TestClient_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		Enabled:  true,
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	err = client.HealthCheck(context.Background())
	assert.NoError(t, err)
}
```

### Step 8.2: Integration Tests

**File: `server/router/api/v1/test/ragflow_service_test.go`**

```go
package test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

func TestSemanticSearch_Integration(t *testing.T) {
	if os.Getenv("RAGFLOW_TEST_ENDPOINT") == "" {
		t.Skip("RAGFlow not available for integration tests")
	}

	ctx := context.Background()
	s := getTestingService(ctx, t)

	resp, err := s.SemanticSearch(ctx, &v1pb.SemanticSearchRequest{
		Query: "test query",
		TopK:  5,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}
```

### Step 8.3: Docker Compose for Development

**File: `scripts/compose-dev.yaml`** (NEW or UPDATE)

```yaml
version: "3.8"

services:
  memos:
    build: .
    ports:
      - "8081:8081"
    environment:
      - MEMOS_RAGFLOW_ENABLED=true
      - MEMOS_RAGFLOW_ENDPOINT=http://ragflow:9380
      - MEMOS_RAGFLOW_API_KEY=${RAGFLOW_API_KEY}
      - MEMOS_RAGFLOW_DATASET_ID=${RAGFLOW_DATASET_ID}
    depends_on:
      - ragflow

  ragflow:
    image: infiniflow/ragflow:latest
    ports:
      - "9380:9380"
    volumes:
      - ragflow_data:/ragflow/data
    environment:
      - RAGFLOW_API_KEY=${RAGFLOW_API_KEY}
    depends_on:
      - elasticsearch
      - mysql

  elasticsearch:
    image: elasticsearch:8.11.1
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    volumes:
      - es_data:/usr/share/elasticsearch/data

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=ragflow
      - MYSQL_DATABASE=ragflow
    volumes:
      - mysql_data:/var/lib/mysql

volumes:
  ragflow_data:
  es_data:
  mysql_data:
```

### Step 8.4: Deployment Checklist

- [ ] RAGFlow service deployed and accessible
- [ ] Elasticsearch configured with sufficient resources
- [ ] API key generated and secured
- [ ] Dataset created in RAGFlow
- [ ] Environment variables configured
- [ ] Initial full sync completed
- [ ] Health check endpoint verified
- [ ] Monitoring/alerting configured

---

## Summary

### Files to Create

| Path                                      | Description                 |
| ----------------------------------------- | --------------------------- |
| `plugin/ragflow/client.go`                | HTTP client for RAGFlow API |
| `plugin/ragflow/config.go`                | Configuration management    |
| `plugin/ragflow/types.go`                 | Request/response types      |
| `plugin/ragflow/dataset.go`               | Dataset operations          |
| `plugin/ragflow/document.go`              | Document operations         |
| `plugin/ragflow/search.go`                | Search operations           |
| `plugin/ragflow/chat.go`                  | Chat operations             |
| `plugin/ragflow/README.md`                | Plugin documentation        |
| `store/ragflow_sync.go`                   | Sync status model           |
| `proto/api/v1/ragflow_service.proto`      | Protocol buffer definitions |
| `server/router/api/v1/ragflow_service.go` | gRPC service implementation |
| `server/runner/ragflowsync/runner.go`     | Background sync runner      |
| `web/src/hooks/useRAGFlowQueries.ts`      | React Query hooks           |
| `web/src/components/SemanticSearch.tsx`   | Search UI component         |

### Files to Modify

| Path                                   | Changes                               |
| -------------------------------------- | ------------------------------------- |
| `store/driver.go`                      | Add RAGFlow sync methods to interface |
| `store/ai_conversation.go`             | Add RAGFlowConversationID field       |
| `store/db/*/ragflow_sync.go`           | Implement sync status for each driver |
| `server/router/api/v1/v1.go`           | Register RAGFlow service              |
| `server/router/api/v1/memo_service.go` | Add sync hooks                        |
| `server/server.go`                     | Initialize RAGFlow client and runner  |
| `internal/profile/profile.go`          | Add RAGFlow config fields             |
| `web/src/lib/connect.ts`               | Add RAGFlow service client            |
| `web/src/pages/KnowtreeAI.tsx`         | Use RAGFlow hooks                     |

### Files to Remove

| Path                     | Reason              |
| ------------------------ | ------------------- |
| `plugin/llm/openai/`     | Replaced by RAGFlow |
| `plugin/llm/deepseek/`   | Replaced by RAGFlow |
| `plugin/llm/provider.go` | Replaced by RAGFlow |
| `plugin/llm/manager.go`  | Replaced by RAGFlow |
| `ai-service/`            | Replaced by RAGFlow |

---

## Implementation Timeline

| Phase                      | Duration | Dependencies |
| -------------------------- | -------- | ------------ |
| Phase 1: Cleanup           | 1 day    | None         |
| Phase 2: RAGFlow Plugin    | 2-3 days | Phase 1      |
| Phase 3: Proto Definitions | 0.5 day  | Phase 1      |
| Phase 4: Backend Service   | 2-3 days | Phase 2, 3   |
| Phase 5: Background Sync   | 1-2 days | Phase 4      |
| Phase 6: Frontend          | 2-3 days | Phase 4      |
| Phase 7: Configuration     | 1 day    | Phase 4      |
| Phase 8: Testing           | 2 days   | All phases   |

**Total Estimated Time: 2-3 weeks**
