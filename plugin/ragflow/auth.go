package ragflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ==================== AuthClient 定义 ====================

// AuthClient RAGFlow 用户认证客户端
// 封装注册、登录、API Key 生成三个公开 API，为 Provisioner 提供认证能力
// 内部持有 Encryptor 实例，自动处理密码 RSA 加密
type AuthClient struct {
	config    *Config
	encryptor *Encryptor
	http      *http.Client
}

// NewAuthClient 创建认证客户端
// 参数: cfg - RAGFlow 配置（需包含 BaseURL 和公钥配置）
// 返回: AuthClient 实例或错误
func NewAuthClient(cfg *Config) (*AuthClient, error) {
	if err := cfg.ValidateForProvisioning(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 加载公钥并创建加密器
	publicKeyPEM, err := cfg.LoadPublicKey()
	if err != nil {
		return nil, fmt.Errorf("加载公钥失败: %w", err)
	}

	encryptor, err := NewEncryptor(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &AuthClient{
		config:    cfg,
		encryptor: encryptor,
		http: &http.Client{
			Timeout: timeout,
			// 不自动跟随重定向，以便提取 Response Header
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// ==================== 用户注册 ====================

// Register 注册新的 RAGFlow 用户
// API: POST /v1/user/register
// 参数: ctx - 上下文, req - 注册请求（密码为明文，内部自动 RSA 加密）
// 返回: 认证结果（包含 UserID 和 Authorization Token）或错误
//
// 错误处理:
//   - 用户已存在: 返回 AuthError{Kind: AuthErrorUserExists}
//   - 邮箱无效: 返回 AuthError{Kind: AuthErrorInvalidEmail}
//   - 注册被禁用: 返回 AuthError{Kind: AuthErrorRegistrationDisabled}
func (c *AuthClient) Register(ctx context.Context, req *RegisterRequest) (*AuthResult, error) {
	if req.Email == "" || req.Password == "" || req.Nickname == "" {
		return nil, fmt.Errorf("邮箱、密码和昵称不能为空")
	}

	// RSA 加密密码
	encryptedPassword, err := c.encryptor.EncryptPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("加密密码失败: %w", err)
	}

	payload := map[string]string{
		"email":    req.Email,
		"nickname": req.Nickname,
		"password": encryptedPassword,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/v1/user/register", payload)
	if err != nil {
		return nil, fmt.Errorf("注册请求失败: %w", err)
	}
	defer resp.Body.Close()

	return c.parseAuthResponse(resp, "注册")
}

// ==================== 用户登录 ====================

// Login 登录 RAGFlow 用户
// API: POST /v1/user/login
// 参数: ctx - 上下文, req - 登录请求（密码为明文，内部自动 RSA 加密）
// 返回: 认证结果（包含 UserID 和 Authorization Token）或错误
//
// 错误处理:
//   - 用户不存在: 返回 AuthError{Kind: AuthErrorUserNotFound}
//   - 密码错误: 返回 AuthError{Kind: AuthErrorInvalidCredentials}
//   - 账户已停用: 返回 AuthError{Kind: AuthErrorUserInactive}
func (c *AuthClient) Login(ctx context.Context, req *LoginRequest) (*AuthResult, error) {
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("邮箱和密码不能为空")
	}

	// RSA 加密密码
	encryptedPassword, err := c.encryptor.EncryptPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("加密密码失败: %w", err)
	}

	payload := map[string]string{
		"email":    req.Email,
		"password": encryptedPassword,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/v1/user/login", payload)
	if err != nil {
		return nil, fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	return c.parseAuthResponse(resp, "登录")
}

// ==================== API Key 生成 ====================

// GenerateAPIKey 生成独立的用户级 API Key
// API: POST /v1/system/new_token
// 参数: ctx - 上下文, authToken - 登录/注册获取的 Authorization Token
// 返回: API Key 字符串（格式: ragflow-xxxxx）或错误
//
// 注意: authToken 是从 Register/Login 返回的 AuthResult.AuthToken 中获取
func (c *AuthClient) GenerateAPIKey(ctx context.Context, authToken string) (string, error) {
	if authToken == "" {
		return "", fmt.Errorf("Authorization Token 不能为空")
	}

	url := c.config.BaseURL + "/v1/system/new_token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 使用登录获取的 Authorization Token 进行认证
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("生成 API Key 请求失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("生成 API Key 失败 (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result APIResponse[NewTokenData]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return "", &AuthError{
			Code:    result.Code,
			Message: fmt.Sprintf("生成 API Key 失败: %s", result.Message),
			Kind:    AuthErrorUnknown,
		}
	}

	if result.Data.Token == "" {
		return "", fmt.Errorf("生成 API Key 成功但返回的 token 为空")
	}

	return result.Data.Token, nil
}

// ==================== 内部方法 ====================

// doRequest 发送 HTTP 请求（不带 Authorization Header，用于公开 API）
func (c *AuthClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
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

	req.Header.Set("Content-Type", "application/json")

	return c.http.Do(req)
}

// parseAuthResponse 解析注册/登录响应
// 从 Response Body 提取 UserID，从 Response Header 提取 Authorization Token
func (c *AuthClient) parseAuthResponse(resp *http.Response, operation string) (*AuthResult, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取%s响应失败: %w", operation, err)
	}

	// 解析响应体
	var result APIResponse[map[string]any]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析%s响应失败: %w (body: %s)", operation, err, string(bodyBytes))
	}

	// 检查业务错误
	if result.Code != 0 {
		return nil, classifyAuthError(result.Code, result.Message)
	}

	// 提取 Authorization Token（从 Response Header）
	authToken := resp.Header.Get("Authorization")
	if authToken == "" {
		return nil, fmt.Errorf("%s成功但未返回 Authorization Header", operation)
	}

	// 提取 UserID（从 Response Body data 字段）
	userID := ""
	if result.Data != nil {
		if id, ok := result.Data["id"]; ok {
			userID = fmt.Sprintf("%v", id)
		}
	}

	return &AuthResult{
		UserID:    userID,
		AuthToken: authToken,
	}, nil
}

// classifyAuthError 根据 RAGFlow 错误消息分类错误类型
func classifyAuthError(code int, message string) *AuthError {
	msg := strings.ToLower(message)

	kind := AuthErrorUnknown
	switch {
	case strings.Contains(msg, "already registered") || strings.Contains(msg, "has already"):
		kind = AuthErrorUserExists
	case strings.Contains(msg, "invalid email"):
		kind = AuthErrorInvalidEmail
	case strings.Contains(msg, "do not match") || strings.Contains(msg, "not match"):
		kind = AuthErrorInvalidCredentials
	case strings.Contains(msg, "not registered") || strings.Contains(msg, "not found"):
		kind = AuthErrorUserNotFound
	case strings.Contains(msg, "account") && strings.Contains(msg, "disabled"),
		strings.Contains(msg, "inactive"):
		kind = AuthErrorUserInactive
	case strings.Contains(msg, "registration") && strings.Contains(msg, "disabled"):
		kind = AuthErrorRegistrationDisabled
	case strings.Contains(msg, "crypt") || strings.Contains(msg, "decrypt"):
		kind = AuthErrorDecryptFailed
	}

	return &AuthError{
		Code:    code,
		Message: message,
		Kind:    kind,
	}
}

// defaultTimeout 默认请求超时
const defaultTimeout = 30 * time.Second
