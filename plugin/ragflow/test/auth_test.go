package ragflow_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/usememos/memos/plugin/ragflow"
)

// ==================== 测试辅助：Mock RAGFlow Server ====================

func mockRAGFlowServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/user/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAuthError(w, 102, "请求体解析失败")
			return
		}

		email := req["email"]
		nickname := req["nickname"]
		password := req["password"]

		if email == "" || nickname == "" || password == "" {
			writeAuthError(w, 102, "邮箱、昵称和密码不能为空")
			return
		}

		if email == "existing@knowtree.local" {
			writeAuthError(w, 102, "Email: existing@knowtree.local has already registered!")
			return
		}

		if email == "invalid-email" {
			writeAuthError(w, 102, "Invalid email address: invalid-email!")
			return
		}

		if email == "disabled@knowtree.local" {
			writeAuthError(w, 102, "User registration is disabled!")
			return
		}

		w.Header().Set("Authorization", "mock-auth-token-for-"+email)
		w.Header().Set("Access-Control-Expose-Headers", "Authorization")
		writeAuthSuccess(w, map[string]any{
			"id":       "user-id-12345",
			"email":    email,
			"nickname": nickname,
		}, nickname+", welcome aboard!")
	})

	mux.HandleFunc("/v1/user/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAuthError(w, 102, "请求体解析失败")
			return
		}

		email := req["email"]
		password := req["password"]

		if email == "" || password == "" {
			writeAuthError(w, 102, "邮箱和密码不能为空")
			return
		}

		if email == "notfound@knowtree.local" {
			writeAuthError(w, 102, "Email: notfound@knowtree.local is not registered!")
			return
		}

		if email == "wrongpwd@knowtree.local" {
			writeAuthError(w, 102, "Email and password do not match!")
			return
		}

		if email == "inactive@knowtree.local" {
			writeAuthError(w, 102, "This account has been disabled, please contact the administrator!")
			return
		}

		if email == "decrypt-fail@knowtree.local" {
			writeAuthError(w, 102, "Fail to crypt password")
			return
		}

		w.Header().Set("Authorization", "mock-auth-token-for-"+email)
		w.Header().Set("Access-Control-Expose-Headers", "Authorization")
		writeAuthSuccess(w, map[string]any{
			"id":    "user-id-67890",
			"email": email,
		}, "Welcome back!")
	})

	mux.HandleFunc("/v1/system/new_token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if authHeader == "invalid-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		writeAuthSuccess(w, map[string]any{
			"token":       "ragflow-mock-api-key-xxxxx",
			"tenant_id":   "tenant-12345",
			"beta":        "beta-token-xxxxx",
			"create_time": 1707264000,
		}, "success")
	})

	return httptest.NewServer(mux)
}

func writeAuthSuccess(w http.ResponseWriter, data any, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":    0,
		"data":    data,
		"message": message,
	})
}

func writeAuthError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"data":    nil,
		"message": message,
	})
}

// newTestAuthClient 创建连接到 Mock 服务器的 AuthClient
func newTestAuthClient(t *testing.T, serverURL string) *ragflow.AuthClient {
	t.Helper()
	client, err := ragflow.NewAuthClientForTesting(serverURL, []byte(testPublicKeyPEM))
	if err != nil {
		t.Fatalf("创建测试 AuthClient 失败: %v", err)
	}
	return client
}

// ==================== NewAuthClient 测试 ====================

func TestNewAuthClient_MissingBaseURL(t *testing.T) {
	cfg := &ragflow.Config{}
	_, err := ragflow.NewAuthClient(cfg)
	if err == nil {
		t.Fatal("缺少 BaseURL 应该返回错误")
	}
}

func TestNewAuthClient_MissingPublicKey(t *testing.T) {
	cfg := &ragflow.Config{
		BaseURL:       "http://localhost:9380",
		PublicKeyPath: "/nonexistent/path/public.pem",
	}
	_, err := ragflow.NewAuthClient(cfg)
	if err == nil {
		t.Fatal("缺少公钥应该返回错误")
	}
}

func TestNewAuthClient_WithEnvPublicKey(t *testing.T) {
	t.Setenv("RAGFLOW_PUBLIC_KEY", testPublicKeyPEM)

	cfg := &ragflow.Config{
		BaseURL: "http://localhost:9380",
	}
	client, err := ragflow.NewAuthClient(cfg)
	if err != nil {
		t.Fatalf("创建 AuthClient 失败: %v", err)
	}
	if client == nil {
		t.Fatal("AuthClient 为空")
	}
}

// ==================== Register 测试 ====================

func TestRegister_Success(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	result, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "42@knowtree.local",
		Nickname: "TestUser",
		Password: "securePassword123",
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	if result.AuthToken == "" {
		t.Error("Authorization Token 为空")
	}
	if result.AuthToken != "mock-auth-token-for-42@knowtree.local" {
		t.Errorf("Authorization Token 不正确: %s", result.AuthToken)
	}
	if result.UserID != "user-id-12345" {
		t.Errorf("UserID 不正确: %s", result.UserID)
	}
}

func TestRegister_UserExists(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "existing@knowtree.local",
		Nickname: "TestUser",
		Password: "securePassword123",
	})
	if err == nil {
		t.Fatal("已注册用户应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorUserExists {
		t.Errorf("错误类型应该是 AuthErrorUserExists, got %d", authErr.Kind)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "invalid-email",
		Nickname: "TestUser",
		Password: "securePassword123",
	})
	if err == nil {
		t.Fatal("无效邮箱应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorInvalidEmail {
		t.Errorf("错误类型应该是 AuthErrorInvalidEmail, got %d", authErr.Kind)
	}
}

func TestRegister_RegistrationDisabled(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "disabled@knowtree.local",
		Nickname: "TestUser",
		Password: "securePassword123",
	})
	if err == nil {
		t.Fatal("注册禁用应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorRegistrationDisabled {
		t.Errorf("错误类型应该是 AuthErrorRegistrationDisabled, got %d", authErr.Kind)
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	tests := []struct {
		name string
		req  *ragflow.RegisterRequest
	}{
		{"空邮箱", &ragflow.RegisterRequest{Email: "", Nickname: "Nick", Password: "pass"}},
		{"空昵称", &ragflow.RegisterRequest{Email: "a@b.c", Nickname: "", Password: "pass"}},
		{"空密码", &ragflow.RegisterRequest{Email: "a@b.c", Nickname: "Nick", Password: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Register(ctx, tt.req)
			if err == nil {
				t.Error("空字段应该返回错误")
			}
		})
	}
}

// ==================== Login 测试 ====================

func TestLogin_Success(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	result, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "42@knowtree.local",
		Password: "securePassword123",
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	if result.AuthToken == "" {
		t.Error("Authorization Token 为空")
	}
	if result.AuthToken != "mock-auth-token-for-42@knowtree.local" {
		t.Errorf("Authorization Token 不正确: %s", result.AuthToken)
	}
	if result.UserID != "user-id-67890" {
		t.Errorf("UserID 不正确: %s", result.UserID)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "notfound@knowtree.local",
		Password: "somePassword",
	})
	if err == nil {
		t.Fatal("不存在的用户应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorUserNotFound {
		t.Errorf("错误类型应该是 AuthErrorUserNotFound, got %d", authErr.Kind)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "wrongpwd@knowtree.local",
		Password: "wrongPassword",
	})
	if err == nil {
		t.Fatal("错误密码应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorInvalidCredentials {
		t.Errorf("错误类型应该是 AuthErrorInvalidCredentials, got %d", authErr.Kind)
	}
}

func TestLogin_InactiveUser(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "inactive@knowtree.local",
		Password: "somePassword",
	})
	if err == nil {
		t.Fatal("停用账户应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorUserInactive {
		t.Errorf("错误类型应该是 AuthErrorUserInactive, got %d", authErr.Kind)
	}
}

func TestLogin_DecryptFailed(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "decrypt-fail@knowtree.local",
		Password: "somePassword",
	})
	if err == nil {
		t.Fatal("解密失败应该返回错误")
	}

	authErr, ok := err.(*ragflow.AuthError)
	if !ok {
		t.Fatalf("错误类型应该是 *AuthError, got %T", err)
	}
	if authErr.Kind != ragflow.AuthErrorDecryptFailed {
		t.Errorf("错误类型应该是 AuthErrorDecryptFailed, got %d", authErr.Kind)
	}
}

func TestLogin_EmptyFields(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	tests := []struct {
		name string
		req  *ragflow.LoginRequest
	}{
		{"空邮箱", &ragflow.LoginRequest{Email: "", Password: "pass"}},
		{"空密码", &ragflow.LoginRequest{Email: "a@b.c", Password: ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Login(ctx, tt.req)
			if err == nil {
				t.Error("空字段应该返回错误")
			}
		})
	}
}

// ==================== GenerateAPIKey 测试 ====================

func TestGenerateAPIKey_Success(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	apiKey, err := client.GenerateAPIKey(ctx, "valid-auth-token")
	if err != nil {
		t.Fatalf("生成 API Key 失败: %v", err)
	}

	if apiKey != "ragflow-mock-api-key-xxxxx" {
		t.Errorf("API Key 不正确: %s", apiKey)
	}
}

func TestGenerateAPIKey_EmptyToken(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.GenerateAPIKey(ctx, "")
	if err == nil {
		t.Fatal("空 Token 应该返回错误")
	}
}

func TestGenerateAPIKey_InvalidToken(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	_, err := client.GenerateAPIKey(ctx, "invalid-token")
	if err == nil {
		t.Fatal("无效 Token 应该返回错误")
	}
}

// ==================== 完整认证链路测试 ====================

func TestFullAuthFlow_RegisterAndGenerateAPIKey(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	authResult, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "100@knowtree.local",
		Nickname: "Memos User 100",
		Password: "randomSecurePassword32chars12345",
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if authResult.AuthToken == "" {
		t.Fatal("注册后 AuthToken 为空")
	}
	if authResult.UserID == "" {
		t.Fatal("注册后 UserID 为空")
	}

	apiKey, err := client.GenerateAPIKey(ctx, authResult.AuthToken)
	if err != nil {
		t.Fatalf("生成 API Key 失败: %v", err)
	}
	if apiKey == "" {
		t.Fatal("API Key 为空")
	}

	t.Logf("完整认证链路成功: UserID=%s, APIKey=%s", authResult.UserID, apiKey)
}

func TestFullAuthFlow_LoginAndGenerateAPIKey(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	authResult, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "existing-user@knowtree.local",
		Password: "existingPassword123",
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if authResult.AuthToken == "" {
		t.Fatal("登录后 AuthToken 为空")
	}

	apiKey, err := client.GenerateAPIKey(ctx, authResult.AuthToken)
	if err != nil {
		t.Fatalf("生成 API Key 失败: %v", err)
	}
	if apiKey == "" {
		t.Fatal("API Key 为空")
	}

	t.Logf("完整认证链路成功: UserID=%s, APIKey=%s", authResult.UserID, apiKey)
}

func TestFullAuthFlow_RegisterThenLogin(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	client := newTestAuthClient(t, server.URL)
	ctx := context.Background()

	email := "200@knowtree.local"
	password := "secureRandomPassword32charslong!"

	_, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    email,
		Nickname: "User 200",
		Password: password,
	})
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, err = client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "existing@knowtree.local",
		Nickname: "Duplicate",
		Password: password,
	})

	authErr, ok := err.(*ragflow.AuthError)
	if !ok || authErr.Kind != ragflow.AuthErrorUserExists {
		t.Fatalf("重复注册应该返回 AuthErrorUserExists, got: %v", err)
	}

	loginResult, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		t.Fatalf("降级登录失败: %v", err)
	}

	apiKey, err := client.GenerateAPIKey(ctx, loginResult.AuthToken)
	if err != nil {
		t.Fatalf("生成 API Key 失败: %v", err)
	}
	if apiKey == "" {
		t.Fatal("API Key 为空")
	}
}

// ==================== ClassifyAuthError 测试 ====================

func TestClassifyAuthError(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected ragflow.AuthErrorKind
	}{
		{"用户已注册", "Email: test@x.com has already registered!", ragflow.AuthErrorUserExists},
		{"邮箱无效", "Invalid email address: bad!", ragflow.AuthErrorInvalidEmail},
		{"密码不匹配", "Email and password do not match!", ragflow.AuthErrorInvalidCredentials},
		{"注册禁用", "User registration is disabled!", ragflow.AuthErrorRegistrationDisabled},
		{"用户未注册", "Email: test@x.com is not registered!", ragflow.AuthErrorUserNotFound},
		{"账户停用", "This account has been inactive", ragflow.AuthErrorUserInactive},
		{"解密失败", "Fail to crypt password", ragflow.AuthErrorDecryptFailed},
			{"未知错误", "Some unknown error occurred", ragflow.AuthErrorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ragflow.ClassifyAuthError(102, tt.message)
			if err.Kind != tt.expected {
				t.Errorf("错误分类不正确: got %d, want %d (message: %s)", err.Kind, tt.expected, tt.message)
			}
		})
	}
}

// ==================== 服务不可用测试 ====================

func TestRegister_ServerUnavailable(t *testing.T) {
	client := newTestAuthClient(t, "http://127.0.0.1:1")
	ctx := context.Background()

	_, err := client.Register(ctx, &ragflow.RegisterRequest{
		Email:    "42@knowtree.local",
		Nickname: "TestUser",
		Password: "securePassword123",
	})
	if err == nil {
		t.Fatal("服务不可用应该返回错误")
	}
}

func TestLogin_ServerUnavailable(t *testing.T) {
	client := newTestAuthClient(t, "http://127.0.0.1:1")
	ctx := context.Background()

	_, err := client.Login(ctx, &ragflow.LoginRequest{
		Email:    "42@knowtree.local",
		Password: "securePassword123",
	})
	if err == nil {
		t.Fatal("服务不可用应该返回错误")
	}
}

func TestGenerateAPIKey_ServerUnavailable(t *testing.T) {
	client := newTestAuthClient(t, "http://127.0.0.1:1")
	ctx := context.Background()

	_, err := client.GenerateAPIKey(ctx, "some-token")
	if err == nil {
		t.Fatal("服务不可用应该返回错误")
	}
}
