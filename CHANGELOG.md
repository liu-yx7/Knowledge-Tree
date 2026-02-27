# Changelog

## [2026-02-26] Bug 修复：ListAvailableModels 暴露了 RAGFlow 未注册的模型

### 问题

用户选择 `qwen3.5-plus` 后触发 Assistant 创建失败：

```
WARN 创建 Chat Assistant 失败（非阻塞） userID=12 llmID=qwen3.5-plus@Tongyi-Qianwen
     error="RAGFlow 错误 (code=102): `model_name` qwen3.5-plus@Tongyi-Qianwen doesn't exist"
```

### 根因

`qwen3.5-plus` 存在于 DashScope API 返回列表（`isChatModel` 前缀 `qwen3` 匹配），
但**不在 `ragflow/conf/llm_factories.json` 的 `Tongyi-Qianwen` 工厂注册表中**。

`EnsureLLMConfig` 调用 `SetLLMAPIKey("Tongyi-Qianwen", key)` 时，RAGFlow 只为
`llm_factories.json` 中静态注册的模型写入 `TenantLLM` 表记录，未注册的模型永远
不会出现在 `TenantLLM` 中，后续 `CreateChatAssistant` 用该 model_id 必然失败。

### 修复

在 `ListChatModels` 加第二道过滤——**RAGFlow 注册表白名单**：

```
DashScope API → isChatModel → isRAGFlowRegistered → 前端展示
```

`ragflowRegisteredChatModels`：以 `map[string]struct{}` 硬编码 `llm_factories.json`
中 `Tongyi-Qianwen.llm` 下所有 chat 类模型名称（42 个），O(1) 查找，零运行时开销。

同步在 `SetUserLLMPreference` 用白名单替换 `DashScope.ModelExists` 动态校验，
确保服务端与前端过滤逻辑完全一致，杜绝旁路绕过。

### 修改文件

- `plugin/dashscope/client.go`：新增 `ragflowRegisteredChatModels` 白名单集合、
  `isRAGFlowRegistered`（内部）、`IsRAGFlowRegistered`（导出）；
  `ListChatModels` 加第二道白名单过滤
- `server/router/api/v1/llm_service.go`：`SetUserLLMPreference` 的模型校验从
  `DashScopeClient.ModelExists` 改为 `dashscope.IsRAGFlowRegistered`；import dashscope 包
- `plugin/dashscope/client_test.go`：修复遗留的 `ModelsResponse`（旧 API 格式）
  引用错误；补充 `TestIsRAGFlowRegistered` 白名单测试；
  `TestClient_ListChatModels` 新增对 `qwen3.5-plus` 被正确过滤的断言

### 验证

```
go test ./plugin/dashscope/... ./server/router/api/v1/... ./plugin/ragflow/...
# ok  plugin/dashscope       (所有 9 个测试通过，含新增白名单测试)
# ok  server/router/api/v1   (cached)
# ok  plugin/ragflow         (cached)
```

### 维护说明

当 `ragflow/conf/llm_factories.json` 的 `Tongyi-Qianwen.llm` 新增 chat 模型时，
**同步** 在 `plugin/dashscope/client.go` 的 `ragflowRegisteredChatModels` 追加对应名称。

## [2026-02-25] Bug 修复：Chat Assistant 创建失败（model_name doesn't exist）

### 问题

用户完成 LLM 偏好设置后触发 AI Chat，Assistant 创建失败：

```
WARN 创建 Chat Assistant 失败（非阻塞） userID=12 llmID=qwen-plus@Tongyi-Qianwen
     error="RAGFlow 错误 (code=102): `model_name` qwen-plus@Tongyi-Qianwen doesn't exist"
```

### 根因（三个独立缺陷叠加）

1. **`ensureAssistant` 在 TenantLLM 未配置时直接尝试创建 Assistant**
   RAGFlow 校验 `model_name` 时会查 `TenantLLM` 表（该 tenant 下已配置的模型列表）。
   用户刚注册 RAGFlow，`EnsureLLMConfig` 从未被调用，`TenantLLM` 表无记录 → 报错。

2. **`ensureAssistant` 忽略用户的 `preferred_llm_id`，始终用 `DefaultLLMID`**
   用户已在 `15:11:43` 通过 `SetUserLLMPreference` 设置了 `qwen3.5-plus@system`，
   但 `ensureAssistant` 仍读 `p.config.DefaultLLMID`（`qwen-plus@Tongyi-Qianwen`），
   偏好设置形同虚设。

3. **`SetUserLLMPreference` 设置偏好后没有补偿创建 Assistant**
   Assistant 尚未创建时，`SetUserLLMPreference` 只更新 `preferred_llm_id`，
   不触发 `ensureAssistant`，导致下次 `EnsureUserResources` 才尝试创建，
   但此时已错过 LLM 偏好写入前的窗口。

### 修复

**`plugin/ragflow/provisioner.go` — `ensureAssistant` 方法**：
- 创建 Assistant 前先调用 `EnsureLLMConfig`（幂等），确保 RAGFlow `TenantLLM` 表
  中已有该 tenant 的 LLM 配置记录
- LLM ID 选取优先级改为：`preferred_llm_id` > `DefaultLLMID`（而非直接用 DefaultLLMID）

**`server/router/api/v1/llm_service.go` — `SetUserLLMPreference` 方法**：
- 在用户偏好写入 DB 后，若 `AssistantID` 为空，**异步触发 `EnsureUserResources`**
  完成 Assistant 补偿创建（此时 `preferred_llm_id` 和 `LLMConfigured` 均已就绪）

### 修改文件

- `plugin/ragflow/provisioner.go` — `ensureAssistant` 方法重写
- `server/router/api/v1/llm_service.go` — `SetUserLLMPreference` 新增补偿创建逻辑

### 验证

- `go build ./...` → 零错误

## [2026-02-09] 删除废弃的 plugin/llm 插件

### 背景

`plugin/llm` 是早期 LLM 直连时代的产物（OpenAI/DeepSeek 直连客户端），P1 阶段已被 `plugin/ragflow/` 完整替代。
项目中零外部引用，P3 OpenAI 兼容客户端需要 RAGFlow 特有逻辑，无法复用。属于死代码，直接删除。

### 删除

- `plugin/llm/provider.go` - Provider 接口定义（LLM 抽象层）
- `plugin/llm/manager.go` - LLM Manager（Provider 注册/查找）
- `plugin/llm/deepseek/client.go` - DeepSeek 直连客户端
- `plugin/llm/openai/client.go` - OpenAI 直连客户端

### 验证

- `go build ./...` → 零错误

## [2026-02-09] P3 存储层统一改造：消除双模型共存

### 背景

深度代码审查发现项目中存在两套 AI 对话模型共存（`ai_conversation`/`ai_message` + `ragflow_conversation`/`ragflow_message`），
导致 Driver 接口膨胀（14 个方法做同一件事）、旧 `SendMessage` 从未真正工作（冒充 Session ID）。
决策：统一为 `ai_conversation`/`ai_message`，字段改造适配 P3 OpenAI 兼容 API 方案。

### 修改

- `store/ai_conversation.go` - 删除 `Model`/`Provider` 字段（P3 由 RAGFlow Chat Assistant 管理）
- `store/ai_message.go` - 删除 `TokenCount`/`AIMessageRoleSystem`，新增 `ReasoningContent`/`ReferencesJSON`/`TokenUsageJSON`
- `store/driver.go` - 删除 6 个 `RAGFlowConversation`/`RAGFlowMessage` 接口方法
- `store/db/sqlite/ai_conversation.go` - 适配新字段
- `store/db/sqlite/ai_message.go` - 适配新字段
- `store/db/mysql/ai_conversation.go` - 适配新字段
- `store/db/mysql/ai_message.go` - 适配新字段
- `store/db/postgres/ai_conversation.go` - 适配新字段
- `store/db/postgres/ai_message.go` - 适配新字段
- `store/migration/{sqlite,mysql,postgres}/0.27/01__add_ai_tables.sql` - P3 新表结构
- `store/migration/{sqlite,mysql,postgres}/LATEST.sql` - 更新 AI 表定义，删除 ragflow_* 表
- `proto/api/v1/ai_service.proto` - Conversation 删除 model/provider；Message 新增 reasoning_content/references_json
- `proto/gen/api/v1/*.go` - buf generate 重新生成
- `web/src/types/proto/api/v1/ai_service_pb.ts` - buf generate 重新生成
- `server/router/api/v1/ai_service.go` - SendMessage 改为占位实现，转换函数适配新字段
- `web/src/hooks/useAIQueries.ts` - 更新注释
- `bank_memory/implement_plan_ragflow_p3.md` - 添加执行记录，更新任务状态

### 删除

- `store/ragflow_conversation.go` - 被 `ai_conversation.go` 替代
- `store/ragflow_message.go` - 被 `ai_message.go` 替代
- `store/db/sqlite/ragflow_conversation.go` - 冗余驱动
- `store/db/sqlite/ragflow_message.go` - 冗余驱动
- `store/db/mysql/ragflow_conversation.go` - 冗余驱动
- `store/db/mysql/ragflow_message.go` - 冗余驱动
- `store/db/postgres/ragflow_conversation.go` - 冗余驱动
- `store/db/postgres/ragflow_message.go` - 冗余驱动

### 验证

- `go build ./...` → 零错误

## [2026-01-28] RAGFlow 集成

### 新增

- `plugin/ragflow/` - RAGFlow HTTP API 客户端插件
  - `ragflow.go` - 核心客户端实现（数据集、文档、检索 API）
  - `config.go` - 配置管理与验证
  - `types.go` - 数据类型定义
  - `conversation.go` - 聊天助手与会话管理
  - `ragflow_test.go` - 单元测试（15 个测试）
- `.env.example` - 环境变量配置示例

### 修改

- `server/server.go` - 替换 LLM Manager 为 RAGFlow Client 初始化
- `server/router/api/v1/v1.go` - 更新服务结构使用 RAGFlowClient
- `server/router/api/v1/ai_service.go` - AI 服务调用 RAGFlow API
- `internal/profile/profile.go` - 添加 RAGFlow 配置字段
- `scripts/compose.yaml` - 添加 RAGFlow 服务配置

### 删除

- `ai-service/src/` - 移除 Python AI 代码

### 技术说明

- 采用插件直连模式（方案 B），无编排层
- RAGFlow 作为唯一 RAG 引擎，替代原有 LLM 直连方式
- 保持 Proto 接口兼容，前端无需修改
