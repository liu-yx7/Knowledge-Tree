// Package ragflow 提供 RAGFlow HTTP API 客户端
// 本模块是 RAGFlow 服务的薄封装层，不实现任何 RAG 逻辑
// 所有文档处理、嵌入、检索能力均由 RAGFlow 服务提供
//
// 依赖文档：docs/ragflow/RAGFlow_Complete_Documentation.md
package ragflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// ==================== Client 定义 ====================

// Client RAGFlow HTTP API 客户端
// 职责边界：仅封装 HTTP 调用，不实现任何算法逻辑
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient 创建 RAGFlow 客户端
func NewClient(cfg *Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// ==================== 基础 HTTP 方法 ====================

// request 发送 HTTP 请求
func (c *Client) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// decodeResponse 解码 API 响应
func decodeResponse[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result APIResponse[T]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("RAGFlow 错误: %s", result.Message)
	}

	return &result.Data, nil
}

// ==================== 数据集管理 ====================

// CreateDataset 创建数据集
// RAGFlow API: POST /api/v1/datasets
func (c *Client) CreateDataset(ctx context.Context, req *CreateDatasetRequest) (*Dataset, error) {
	payload := map[string]any{
		"name":         req.Name,
		"description":  req.Description,
		"chunk_method": req.ChunkMethod,
	}
	if req.ParserConfig != nil {
		payload["parser_config"] = req.ParserConfig
	}

	resp, err := c.request(ctx, http.MethodPost, "/api/v1/datasets", payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[Dataset](resp)
}

// ListDatasets 列出数据集
// RAGFlow API: GET /api/v1/datasets
func (c *Client) ListDatasets(ctx context.Context, opts *ListOptions) ([]Dataset, error) {
	path := fmt.Sprintf("/api/v1/datasets?page=%d&page_size=%d", opts.Page, opts.PageSize)
	if opts.Name != "" {
		path += "&name=" + opts.Name
	}

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	result, err := decodeResponse[[]Dataset](resp)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

// DeleteDataset 删除数据集
// RAGFlow API: DELETE /api/v1/datasets
func (c *Client) DeleteDataset(ctx context.Context, id string) error {
	payload := map[string]any{
		"ids": []string{id},
	}

	resp, err := c.request(ctx, http.MethodDelete, "/api/v1/datasets", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除数据集失败: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ==================== 文档管理 ====================

// UploadDocument 上传文档到数据集
// RAGFlow API: POST /api/v1/datasets/{dataset_id}/documents
// 注意：必须使用 multipart/form-data 格式，表单字段名为 "file"
// 返回值：RAGFlow 返回的是数组（支持批量上传），这里只取第一个
func (c *Client) UploadDocument(ctx context.Context, datasetID string, doc *Document) (*DocumentInfo, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents", datasetID)
	url := c.config.BaseURL + path

	// 构建 multipart/form-data 请求体
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 创建 file 字段
	part, err := writer.CreateFormFile("file", doc.Name)
	if err != nil {
		return nil, fmt.Errorf("创建表单文件字段失败: %w", err)
	}

	// 写入文件内容
	if _, err := part.Write(doc.Content); err != nil {
		return nil, fmt.Errorf("写入文件内容失败: %w", err)
	}

	// 关闭 writer 以完成 multipart 消息
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// RAGFlow 返回数组格式，需要解析为 []DocumentInfo
	docs, err := decodeResponse[[]DocumentInfo](resp)
	if err != nil {
		return nil, err
	}

	if docs == nil || len(*docs) == 0 {
		return nil, fmt.Errorf("上传成功但未返回文档信息")
	}

	return &(*docs)[0], nil
}

// ListDocuments 列出数据集中的文档
// RAGFlow API: GET /api/v1/datasets/{dataset_id}/documents
func (c *Client) ListDocuments(ctx context.Context, datasetID string, opts *ListOptions) ([]DocumentInfo, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents?page=%d&page_size=%d",
		datasetID, opts.Page, opts.PageSize)
	if opts.Keywords != "" {
		path += "&keywords=" + opts.Keywords
	}

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	result, err := decodeResponse[[]DocumentInfo](resp)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

// ParseDocuments 解析文档（触发分块和嵌入）
// RAGFlow API: POST /api/v1/datasets/{dataset_id}/chunks
// 注意：这是异步操作
func (c *Client) ParseDocuments(ctx context.Context, datasetID string, docIDs []string) error {
	path := fmt.Sprintf("/api/v1/datasets/%s/chunks", datasetID)
	payload := map[string]any{
		"document_ids": docIDs,
	}

	resp, err := c.request(ctx, http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("解析文档失败: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteDocuments 删除文档
// RAGFlow API: DELETE /api/v1/datasets/{dataset_id}/documents
func (c *Client) DeleteDocuments(ctx context.Context, datasetID string, docIDs []string) error {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents", datasetID)
	payload := map[string]any{
		"ids": docIDs,
	}

	resp, err := c.request(ctx, http.MethodDelete, path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除文档失败: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDocument 获取单个文档详情
// RAGFlow API: GET /api/v1/datasets/{dataset_id}/documents/{document_id}
func (c *Client) GetDocument(ctx context.Context, datasetID, documentID string) (*DocumentInfo, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s", datasetID, documentID)

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	return decodeResponse[DocumentInfo](resp)
}

// UpdateDocument 更新文档元数据
// RAGFlow API: PUT /api/v1/datasets/{dataset_id}/documents/{document_id}
// 用于更新文档的元数据，如 visibility 标签
func (c *Client) UpdateDocument(ctx context.Context, datasetID, documentID string, update *UpdateDocumentRequest) (*DocumentInfo, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s", datasetID, documentID)

	payload := map[string]any{}
	if update.Name != "" {
		payload["name"] = update.Name
	}
	if update.Metadata != nil {
		payload["meta_fields"] = update.Metadata
	}
	if update.ChunkMethod != "" {
		payload["chunk_method"] = update.ChunkMethod
	}
	if update.ParserConfig != nil {
		payload["parser_config"] = update.ParserConfig
	}

	resp, err := c.request(ctx, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[DocumentInfo](resp)
}

// GetDocumentChunks 获取文档的分块列表
// RAGFlow API: GET /api/v1/datasets/{dataset_id}/documents/{document_id}/chunks
func (c *Client) GetDocumentChunks(ctx context.Context, datasetID, documentID string, opts *ListOptions) ([]Chunk, error) {
	path := fmt.Sprintf("/api/v1/datasets/%s/documents/%s/chunks?page=%d&page_size=%d",
		datasetID, documentID, opts.Page, opts.PageSize)

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	result, err := decodeResponse[[]Chunk](resp)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

// ==================== 检索 ====================

// Retrieve 检索相关内容
// RAGFlow API: POST /api/v1/retrieval
// 这是 RAGFlow 的核心能力，包含向量检索、关键词检索、重排序
func (c *Client) Retrieve(ctx context.Context, req *RetrievalRequest) (*RetrievalResult, error) {
	payload := map[string]any{
		"dataset_ids":               req.DatasetIDs,
		"question":                  req.Question,
		"top_k":                     req.TopK,
		"similarity_threshold":      req.SimilarityThreshold,
		"keyword_similarity_weight": req.KeywordWeight,
	}
	if len(req.DocumentIDs) > 0 {
		payload["document_ids"] = req.DocumentIDs
	}

	resp, err := c.request(ctx, http.MethodPost, "/api/v1/retrieval", payload)
	if err != nil {
		return nil, err
	}

	return decodeResponse[RetrievalResult](resp)
}

// ==================== 健康检查 ====================

// HealthCheck 检查 RAGFlow 服务是否可用
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.ListDatasets(ctx, &ListOptions{Page: 1, PageSize: 1})
	return err
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// GetConfig 获取客户端配置
func (c *Client) GetConfig() *Config {
	return c.config
}
