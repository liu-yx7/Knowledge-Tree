# Changelog

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
