// ==================== LLM 模型管理 React Query Hooks ====================
// 用于获取可用模型列表和管理用户的模型偏好设置

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { llmServiceClient } from "@/connect";
import type { LLMModel, UserLLMPreference } from "@/types/proto/api/v1/llm_service_pb";

// ==================== Query Keys ====================

export const llmKeys = {
  all: ["llm"] as const,
  models: () => [...llmKeys.all, "models"] as const,
  preference: () => [...llmKeys.all, "preference"] as const,
};

// ==================== Queries ====================

/**
 * 获取可用的 LLM 模型列表
 * 从 DashScope API 动态获取，带缓存（后端 5 分钟 TTL）
 */
export const useAvailableModels = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: llmKeys.models(),
    queryFn: async (): Promise<LLMModel[]> => {
      const response = await llmServiceClient.listAvailableModels({});
      return response.models || [];
    },
    staleTime: 5 * 60 * 1000, // 5 分钟，与后端缓存一致
    enabled: options?.enabled ?? true,
  });
};

/**
 * 获取用户的 LLM 偏好设置
 */
export const useUserLLMPreference = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: llmKeys.preference(),
    queryFn: async (): Promise<UserLLMPreference> => {
      const response = await llmServiceClient.getUserLLMPreference({});
      return response;
    },
    staleTime: 30 * 1000, // 30 秒
    enabled: options?.enabled ?? true,
  });
};

// ==================== Mutations ====================

/**
 * 设置用户的 LLM 偏好
 * 会同步更新 RAGFlow Assistant 的 LLM 配置
 */
export const useSetUserLLMPreference = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (modelId: string): Promise<UserLLMPreference> => {
      const response = await llmServiceClient.setUserLLMPreference({ modelId });
      return response;
    },
    onSuccess: (data) => {
      // 更新缓存
      queryClient.setQueryData(llmKeys.preference(), data);
    },
  });
};

// ==================== Helper Functions ====================

/**
 * 解析模型 ID，提取模型名称和提供商
 * 格式：{model_name}@{provider}
 */
export function parseModelId(modelId: string): { modelName: string; provider: string } {
  if (!modelId) return { modelName: "", provider: "" };
  const parts = modelId.split("@");
  if (parts.length >= 2) {
    return { modelName: parts[0], provider: parts.slice(1).join("@") };
  }
  return { modelName: modelId, provider: "" };
}

/**
 * 获取模型的显示名称
 * 优先使用 displayName，其次是 modelName
 */
export function getModelDisplayName(model: LLMModel): string {
  return model.displayName || model.modelName || model.modelId;
}

/**
 * 格式化提供商名称为友好文本
 */
export function formatProviderName(provider: string): string {
  const providerMap: Record<string, string> = {
    "Tongyi-Qianwen": "通义千问",
    "DeepSeek": "DeepSeek",
    "OpenAI": "OpenAI",
  };
  return providerMap[provider] || provider;
}
