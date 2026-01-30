# Changelog

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
