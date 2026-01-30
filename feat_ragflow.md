# RAGFlow 集成实施计划

> 状态: ✅ 已完成
> 执行时间: 2026-01-28

## 📋 项目概述

将现有的自实现 AI 逻辑替换为 RAGFlow 集成，采用插件直连模式（方案 B），在 Go plugin 层实现 RAGFlow 客户端。

## 🧩 架构变更

```
变更前:
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│ AI Service  │────▶│ LLM Plugin  │────▶│ OpenAI/Deep  │
│ (Go)        │     │ (Go)        │     │ Seek API     │
└─────────────┘     └─────────────┘     └──────────────┘

变更后:
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│ AI Service  │────▶│ RAGFlow      │────▶│ RAGFlow      │
│ (Go)        │     │ Plugin (Go)  │     │ Server API   │
└─────────────┘     └──────────────┘     └──────────────┘
```

## ✅ 执行清单

### Phase 1: 代码清理 ✅

| 任务                   | 状态 | 说明                  |
| ---------------------- | ---- | --------------------- |
| 删除 `ai-service/src/` | ✅   | 已删除 Python AI 代码 |

### Phase 2: RAGFlow Plugin 实现 ✅

| 文件                             | 状态 | 功能                          |
| -------------------------------- | ---- | ----------------------------- |
| `plugin/ragflow/ragflow.go`      | ✅   | HTTP 客户端主实现             |
| `plugin/ragflow/config.go`       | ✅   | 配置管理与验证                |
| `plugin/ragflow/types.go`        | ✅   | 数据类型定义                  |
| `plugin/ragflow/conversation.go` | ✅   | 聊天助手与会话管理            |
| `plugin/ragflow/ragflow_test.go` | ✅   | 单元测试（15 个测试全部通过） |

### Phase 3: 服务集成 ✅

| 文件                                 | 状态 | 修改内容                         |
| ------------------------------------ | ---- | -------------------------------- |
| `server/router/api/v1/v1.go`         | ✅   | 替换 LLMManager 为 RAGFlowClient |
| `server/router/api/v1/ai_service.go` | ✅   | 更新 AI 服务调用 RAGFlow API     |
| `server/server.go`                   | ✅   | 初始化 RAGFlow 客户端            |
| `internal/profile/profile.go`        | ✅   | 添加 RAGFlow 配置字段            |

### Phase 4: 部署配置 ✅

| 文件                   | 状态 | 修改内容              |
| ---------------------- | ---- | --------------------- |
| `scripts/compose.yaml` | ✅   | 添加 RAGFlow 服务配置 |
| `.env.example`         | ✅   | 添加环境变量示例      |

### Phase 5: 验证 ✅

| 检查项                    | 状态       |
| ------------------------- | ---------- |
| `go build ./...` 编译通过 | ✅         |
| RAGFlow 插件测试通过      | ✅ (15/15) |
| 无 Python 代码残留        | ✅         |

---

## 📁 文件变更汇总

### 新增文件

```
plugin/ragflow/
├── ragflow.go          # RAGFlow HTTP 客户端
├── config.go           # 配置管理
├── types.go            # 数据类型定义
├── conversation.go     # 会话管理
└── ragflow_test.go     # 单元测试

.env.example            # 环境变量示例
```

### 修改文件

```
server/server.go                      # 初始化 RAGFlow 客户端
server/router/api/v1/v1.go            # 替换 LLMManager
server/router/api/v1/ai_service.go    # RAGFlow API 调用
internal/profile/profile.go           # 添加配置字段
scripts/compose.yaml                  # Docker 服务配置
```

### 删除文件

```
ai-service/src/                       # Python AI 代码（已删除）
```

---

## 🔧 配置说明

### 环境变量

```bash
# RAGFlow 服务地址
MEMOS_RAGFLOW_BASE_URL=http://localhost:9380

# RAGFlow API 密钥
MEMOS_RAGFLOW_API_KEY=your-api-key

# RAGFlow 助手 ID
MEMOS_RAGFLOW_ASSISTANT_ID=your-assistant-id
```

### RAGFlow 准备工作

1. 部署 RAGFlow 服务（参考 https://github.com/infiniflow/ragflow）
2. 在 RAGFlow 控制台创建知识库
3. 创建 Chat Assistant 并获取 ID
4. 获取 API Key
5. 配置环境变量

---

## 🏗️ RAGFlow Plugin 架构

```
plugin/ragflow/
├── ragflow.go          # Client 结构体与核心方法
│   ├── NewClient()     # 创建客户端
│   ├── request()       # HTTP 请求封装
│   ├── CreateDataset() # 数据集管理
│   ├── UploadDocument()# 文档上传
│   ├── Retrieve()      # 知识检索
│   └── HealthCheck()   # 健康检查
│
├── conversation.go     # 聊天功能
│   ├── CreateChatAssistant()  # 创建助手
│   ├── CreateSession()        # 创建会话
│   ├── Chat()                 # 非流式聊天
│   └── ChatStream()           # 流式聊天
│
├── config.go           # 配置
│   ├── Config{}        # 配置结构
│   ├── Validate()      # 验证
│   └── ChunkMethod     # 分块方法枚举
│
└── types.go            # 类型定义
    ├── Dataset         # 数据集
    ├── Document        # 文档
    ├── Chunk           # 检索块
    └── ChatResponse    # 聊天响应
```

---

## ⚠️ 注意事项

1. **RAGFlow 服务依赖**：RAGFlow 需要 Elasticsearch、MinIO 等服务支持
2. **会话映射**：当前使用 conversation UID 作为 RAGFlow session ID
3. **流式响应**：ChatStream 方法支持 SSE 流式响应
4. **Proto 接口**：保持与前端的接口兼容，无需修改前端代码

---

## 📚 参考文档

- [RAGFlow 完整文档](docs/ragflow/RAGFlow_Complete_Documentation.md)
- [RAGFlow GitHub](https://github.com/infiniflow/ragflow)
- [RAGFlow HTTP API](docs/ragflow/RAGFlow_Complete_Documentation.md#5-api-参考-references)
