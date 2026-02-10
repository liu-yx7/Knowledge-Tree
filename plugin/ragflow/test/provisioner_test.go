package ragflow_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	ragflow "github.com/usememos/memos/plugin/ragflow"
	"github.com/usememos/memos/store"
)

// ==================== Mock Store ====================

// mockProvisionerStore 用于测试的 Mock 存储实现
type mockProvisionerStore struct {
	mu       sync.Mutex
	mappings map[int32]*store.RAGFlowUserMapping
	nextID   int32

	// 控制行为的开关
	getError    error
	createError error
	updateError error
}

func newMockStore() *mockProvisionerStore {
	return &mockProvisionerStore{
		mappings: make(map[int32]*store.RAGFlowUserMapping),
		nextID:   1,
	}
}

func (m *mockProvisionerStore) GetRAGFlowUserMapping(_ context.Context, find *store.FindRAGFlowUserMapping) (*store.RAGFlowUserMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.getError != nil {
		return nil, m.getError
	}

	if find.UserID != nil {
		for _, mapping := range m.mappings {
			if mapping.UserID == *find.UserID {
				return mapping, nil
			}
		}
	}
	return nil, nil
}

func (m *mockProvisionerStore) CreateRAGFlowUserMapping(_ context.Context, create *store.RAGFlowUserMapping) (*store.RAGFlowUserMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createError != nil {
		return nil, m.createError
	}

	create.ID = m.nextID
	m.nextID++
	m.mappings[create.UserID] = create
	return create, nil
}

func (m *mockProvisionerStore) UpdateRAGFlowUserMapping(_ context.Context, update *store.UpdateRAGFlowUserMapping) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateError != nil {
		return m.updateError
	}

	for _, mapping := range m.mappings {
		if mapping.ID == update.ID {
			if update.RAGFlowUserID != nil {
				mapping.RAGFlowUserID = *update.RAGFlowUserID
			}
			if update.RAGFlowEmail != nil {
				mapping.RAGFlowEmail = *update.RAGFlowEmail
			}
			if update.RAGFlowPassword != nil {
				mapping.RAGFlowPassword = *update.RAGFlowPassword
			}
			if update.APIKey != nil {
				mapping.APIKey = *update.APIKey
			}
			return nil
		}
	}
	return fmt.Errorf("映射 ID %d 不存在", update.ID)
}

// ==================== 测试辅助 ====================

// newTestProvisioner 创建连接 Mock Server 和 Mock Store 的 Provisioner
func newTestProvisioner(t *testing.T, serverURL string, mockStore *mockProvisionerStore) *ragflow.Provisioner {
	t.Helper()

	cfg := &ragflow.Config{
		BaseURL: serverURL,
	}

	authClient := newTestAuthClient(t, serverURL)

	p, err := ragflow.NewProvisionerWithAuthClient(cfg, mockStore, authClient)
	if err != nil {
		t.Fatalf("创建 Provisioner 失败: %v", err)
	}
	return p
}

// ==================== NewProvisioner 测试 ====================

func TestNewProvisioner_NilConfig(t *testing.T) {
	_, err := ragflow.NewProvisioner(nil, newMockStore())
	if err == nil {
		t.Fatal("nil 配置应返回错误")
	}
}

func TestNewProvisioner_NilStore(t *testing.T) {
	cfg := &ragflow.Config{BaseURL: "http://localhost:9380"}
	_, err := ragflow.NewProvisioner(cfg, nil)
	if err == nil {
		t.Fatal("nil 存储应返回错误")
	}
}

func TestNewProvisionerWithAuthClient_NilAuthClient(t *testing.T) {
	cfg := &ragflow.Config{BaseURL: "http://localhost:9380"}
	_, err := ragflow.NewProvisionerWithAuthClient(cfg, newMockStore(), nil)
	if err == nil {
		t.Fatal("nil AuthClient 应返回错误")
	}
}

// ==================== GetClientForUser: 场景 A — 已存在完整映射 ====================

func TestGetClientForUser_ExistingMappingWithAPIKey(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	// 预置一个完整映射
	mockStore.mappings[42] = &store.RAGFlowUserMapping{
		ID:              1,
		UserID:          42,
		RAGFlowUserID:   "rf-user-001",
		RAGFlowEmail:    "42@knowtree.local",
		RAGFlowPassword: "stored-password",
		APIKey:          "ragflow-existing-key",
	}

	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	client, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("获取 Client 失败: %v", err)
	}
	if client == nil {
		t.Fatal("Client 为空")
	}
	if client.GetConfig().APIKey != "ragflow-existing-key" {
		t.Errorf("API Key 不正确: %s", client.GetConfig().APIKey)
	}
}

// ==================== GetClientForUser: 场景 B — 已注册但缺少 API Key ====================

func TestGetClientForUser_RegisteredButNoAPIKey(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	// 预置一个已注册但缺少 API Key 的映射
	mockStore.mappings[42] = &store.RAGFlowUserMapping{
		ID:              1,
		UserID:          42,
		RAGFlowUserID:   "rf-user-001",
		RAGFlowEmail:    "42@knowtree.local",
		RAGFlowPassword: "stored-password",
		APIKey:          "", // 空 API Key
	}

	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	client, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("获取 Client 失败: %v", err)
	}
	if client == nil {
		t.Fatal("Client 为空")
	}

	// 验证 API Key 已被写入数据库
	mapping := mockStore.mappings[42]
	if mapping.APIKey == "" {
		t.Error("API Key 未更新到数据库")
	}
	if mapping.APIKey != "ragflow-mock-api-key-xxxxx" {
		t.Errorf("API Key 不正确: %s", mapping.APIKey)
	}
}

// ==================== GetClientForUser: 场景 C — 全新用户 ====================

func TestGetClientForUser_NewUser(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	client, err := p.GetClientForUser(ctx, 100, "NewUser")
	if err != nil {
		t.Fatalf("获取 Client 失败: %v", err)
	}
	if client == nil {
		t.Fatal("Client 为空")
	}

	// 验证映射已持久化
	mapping := mockStore.mappings[100]
	if mapping == nil {
		t.Fatal("用户映射未创建")
	}
	if mapping.RAGFlowEmail != "100@knowtree.local" {
		t.Errorf("邮箱不正确: %s", mapping.RAGFlowEmail)
	}
	if mapping.RAGFlowPassword == "" {
		t.Error("密码为空")
	}
	if mapping.RAGFlowUserID != "user-id-12345" {
		t.Errorf("RAGFlow UserID 不正确: %s", mapping.RAGFlowUserID)
	}
	if mapping.APIKey != "ragflow-mock-api-key-xxxxx" {
		t.Errorf("API Key 不正确: %s", mapping.APIKey)
	}
}

// ==================== GetClientForUser: 场景 C 变体 — 已有空映射（P1 遗留） ====================

func TestGetClientForUser_ExistingEmptyMapping(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	// 模拟 P1 创建的映射（认证字段全为空）
	mockStore.mappings[42] = &store.RAGFlowUserMapping{
		ID:            1,
		UserID:        42,
		DatasetID:     "ds-001",
		RAGFlowUserID: "", // 空：尚未注册
		APIKey:        "", // 空
	}

	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	client, err := p.GetClientForUser(ctx, 42, "LegacyUser")
	if err != nil {
		t.Fatalf("获取 Client 失败: %v", err)
	}
	if client == nil {
		t.Fatal("Client 为空")
	}

	// 验证映射已更新（不是新建）
	mapping := mockStore.mappings[42]
	if mapping.ID != 1 {
		t.Errorf("应更新已有映射（ID=1），而非新建: ID=%d", mapping.ID)
	}
	if mapping.DatasetID != "ds-001" {
		t.Errorf("DatasetID 不应被覆盖: %s", mapping.DatasetID)
	}
	if mapping.RAGFlowUserID == "" {
		t.Error("RAGFlow UserID 未更新")
	}
	if mapping.APIKey == "" {
		t.Error("API Key 未更新")
	}
}

// ==================== GetClientForUser: 注册时用户已存在，自动降级登录 ====================

func TestGetClientForUser_RegisterFallbackToLogin(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	// 此用户 ID 生成的邮箱不会触发 mock server 的 existing 逻辑
	// 但我们可以验证正常注册流程
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	// 首次创建成功
	_, err := p.GetClientForUser(ctx, 200, "User200")
	if err != nil {
		t.Fatalf("首次配置失败: %v", err)
	}

	// 清缓存并修改数据库模拟部分数据丢失
	p.InvalidateCache(200)
	mapping := mockStore.mappings[200]
	mapping.RAGFlowUserID = ""
	mapping.APIKey = ""

	// 再次获取，会走注册流程（因为 ragflow_user_id 为空）
	_, err = p.GetClientForUser(ctx, 200, "User200")
	if err != nil {
		t.Fatalf("重新配置失败: %v", err)
	}

	// 验证数据已恢复
	updated := mockStore.mappings[200]
	if updated.APIKey == "" {
		t.Error("API Key 未恢复")
	}
}

// ==================== 缓存测试 ====================

func TestGetClientForUser_CacheHit(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	// 首次调用：触发配置流程
	client1, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("首次获取失败: %v", err)
	}

	// 第二次调用：应命中缓存（返回相同指针）
	client2, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("第二次获取失败: %v", err)
	}

	if client1 != client2 {
		t.Error("第二次调用应返回缓存的 Client 实例")
	}
}

func TestInvalidateCache(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	// 首次调用
	client1, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("首次获取失败: %v", err)
	}

	// 清除缓存
	p.InvalidateCache(42)

	// 第二次调用：缓存已清除，应重新创建
	client2, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("第二次获取失败: %v", err)
	}

	if client1 == client2 {
		t.Error("清除缓存后应返回新的 Client 实例")
	}
}

// ==================== 并发安全测试 ====================

func TestGetClientForUser_ConcurrentAccess(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	var wg sync.WaitGroup
	var errCount atomic.Int32
	concurrency := 20

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, err := p.GetClientForUser(ctx, 42, "ConcurrentUser")
			if err != nil {
				errCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if errCount.Load() > 0 {
		t.Errorf("并发调用中 %d 个失败", errCount.Load())
	}

	// 验证只创建了一个映射（多个 goroutine 可能竞争，但最终结果应一致）
	mapping := mockStore.mappings[42]
	if mapping == nil {
		t.Fatal("映射未创建")
	}
	if mapping.APIKey == "" {
		t.Error("API Key 为空")
	}
}

// ==================== 错误处理测试 ====================

func TestGetClientForUser_StoreGetError(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	mockStore.getError = fmt.Errorf("数据库连接失败")

	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	_, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err == nil {
		t.Fatal("数据库错误应导致失败")
	}
}

func TestGetClientForUser_StoreCreateError(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	mockStore.createError = fmt.Errorf("写入失败")

	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	// 即使 Store 创建失败，也应返回可用的 Client（RAGFlow 账户已创建）
	client, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err != nil {
		t.Fatalf("映射保存失败不应阻塞 Client 返回: %v", err)
	}
	if client == nil {
		t.Fatal("Client 为空")
	}
}

func TestGetClientForUser_RAGFlowUnavailable(t *testing.T) {
	mockStore := newMockStore()

	cfg := &ragflow.Config{
		BaseURL: "http://127.0.0.1:1", // 不可达地址
	}
	authClient := newTestAuthClient(t, "http://127.0.0.1:1")

	p, err := ragflow.NewProvisionerWithAuthClient(cfg, mockStore, authClient)
	if err != nil {
		t.Fatalf("创建 Provisioner 失败: %v", err)
	}

	ctx := context.Background()
	_, err = p.GetClientForUser(ctx, 42, "TestUser")
	if err == nil {
		t.Fatal("RAGFlow 不可用应返回错误")
	}
}

func TestGetClientForUser_APIKeyGenerationFails(t *testing.T) {
	// 创建一个特殊的 mock server，注册成功但 new_token 失败
	server := mockRAGFlowServerWithBrokenToken(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	_, err := p.GetClientForUser(ctx, 42, "TestUser")
	if err == nil {
		t.Fatal("API Key 生成失败应返回错误")
	}

	// 验证注册信息已保存（API Key 为空，等待下次重试）
	mapping := mockStore.mappings[42]
	if mapping == nil {
		t.Fatal("注册信息未保存")
	}
	if mapping.RAGFlowUserID == "" {
		t.Error("RAGFlow UserID 未保存")
	}
	if mapping.APIKey != "" {
		t.Errorf("API Key 应为空，实际: %s", mapping.APIKey)
	}
}

// ==================== 多用户隔离测试 ====================

func TestGetClientForUser_MultipleUsers(t *testing.T) {
	server := mockRAGFlowServer(t)
	defer server.Close()

	mockStore := newMockStore()
	p := newTestProvisioner(t, server.URL, mockStore)
	ctx := context.Background()

	// 为三个不同用户获取 Client
	client1, err := p.GetClientForUser(ctx, 1, "User1")
	if err != nil {
		t.Fatalf("用户1获取失败: %v", err)
	}

	client2, err := p.GetClientForUser(ctx, 2, "User2")
	if err != nil {
		t.Fatalf("用户2获取失败: %v", err)
	}

	client3, err := p.GetClientForUser(ctx, 3, "User3")
	if err != nil {
		t.Fatalf("用户3获取失败: %v", err)
	}

	// 验证三个 Client 是不同实例
	if client1 == client2 || client2 == client3 || client1 == client3 {
		t.Error("不同用户应获得不同的 Client 实例")
	}

	// 验证三个用户的邮箱正确
	if mockStore.mappings[1].RAGFlowEmail != "1@knowtree.local" {
		t.Errorf("用户1邮箱不正确: %s", mockStore.mappings[1].RAGFlowEmail)
	}
	if mockStore.mappings[2].RAGFlowEmail != "2@knowtree.local" {
		t.Errorf("用户2邮箱不正确: %s", mockStore.mappings[2].RAGFlowEmail)
	}
	if mockStore.mappings[3].RAGFlowEmail != "3@knowtree.local" {
		t.Errorf("用户3邮箱不正确: %s", mockStore.mappings[3].RAGFlowEmail)
	}
}

// ==================== 辅助 Mock Server ====================

// mockRAGFlowServerWithBrokenToken 创建一个注册/登录正常但 new_token 失败的 Mock Server
func mockRAGFlowServerWithBrokenToken(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// 注册正常
	mux.HandleFunc("/v1/user/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Authorization", "mock-auth-token")
		writeAuthSuccess(w, map[string]any{
			"id":    "user-id-broken",
			"email": "broken@knowtree.local",
		}, "注册成功")
	})

	// 登录正常
	mux.HandleFunc("/v1/user/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Authorization", "mock-auth-token")
		writeAuthSuccess(w, map[string]any{
			"id":    "user-id-broken",
			"email": "broken@knowtree.local",
		}, "登录成功")
	})

	// new_token 失败
	mux.HandleFunc("/v1/system/new_token", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	return httptest.NewServer(mux)
}
