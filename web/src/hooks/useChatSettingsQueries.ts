// ==================== 聊天设置管理 React Query Hooks ====================
// 用于管理用户的 Dataset 选择和对话选项（引用、推理开关）

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { chatSettingsServiceClient } from "@/connect";
import type { ChatSettings, Dataset } from "@/types/proto/api/v1/chat_settings_service_pb";

// ==================== Query Keys ====================

export const chatSettingsKeys = {
  all: ["chatSettings"] as const,
  settings: () => [...chatSettingsKeys.all, "settings"] as const,
  datasets: () => [...chatSettingsKeys.all, "datasets"] as const,
};

// ==================== Queries ====================

/**
 * 获取用户的聊天设置
 * 包含：选中的 Dataset、引用开关、推理开关
 */
export const useChatSettings = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: chatSettingsKeys.settings(),
    queryFn: async (): Promise<ChatSettings> => {
      const response = await chatSettingsServiceClient.getChatSettings({});
      return response;
    },
    staleTime: 30 * 1000, // 30 秒
    enabled: options?.enabled ?? true,
  });
};

/**
 * 获取用户可用的 Dataset 列表
 */
export const useAvailableDatasets = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: chatSettingsKeys.datasets(),
    queryFn: async (): Promise<Dataset[]> => {
      const response = await chatSettingsServiceClient.listDatasets({});
      return response.datasets || [];
    },
    staleTime: 60 * 1000, // 1 分钟
    enabled: options?.enabled ?? true,
  });
};

// ==================== Mutations ====================

/**
 * 更新聊天设置
 * 支持部分更新：只传需要更新的字段
 */
export const useUpdateChatSettings = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (updates: {
      datasetIds?: string[];
      quoteEnabled?: boolean;
      reasoningEnabled?: boolean;
    }): Promise<ChatSettings> => {
      const response = await chatSettingsServiceClient.updateChatSettings({
        datasetIds: updates.datasetIds || [],
        quoteEnabled: updates.quoteEnabled,
        reasoningEnabled: updates.reasoningEnabled,
      });
      return response;
    },
    onSuccess: (data) => {
      // 更新缓存
      queryClient.setQueryData(chatSettingsKeys.settings(), data);
    },
  });
};

// ==================== Convenience Hooks ====================

/**
 * 更新 Dataset 选择的便捷 hook
 */
export const useUpdateDatasetSelection = () => {
  const mutation = useUpdateChatSettings();

  return {
    ...mutation,
    mutate: (datasetIds: string[]) => mutation.mutate({ datasetIds }),
    mutateAsync: (datasetIds: string[]) => mutation.mutateAsync({ datasetIds }),
  };
};

/**
 * 切换引用开关的便捷 hook
 */
export const useToggleQuote = () => {
  const mutation = useUpdateChatSettings();

  return {
    ...mutation,
    mutate: (enabled: boolean) => mutation.mutate({ quoteEnabled: enabled }),
    mutateAsync: (enabled: boolean) => mutation.mutateAsync({ quoteEnabled: enabled }),
  };
};

/**
 * 切换推理开关的便捷 hook
 */
export const useToggleReasoning = () => {
  const mutation = useUpdateChatSettings();

  return {
    ...mutation,
    mutate: (enabled: boolean) => mutation.mutate({ reasoningEnabled: enabled }),
    mutateAsync: (enabled: boolean) => mutation.mutateAsync({ reasoningEnabled: enabled }),
  };
};

// ==================== Helper Functions ====================

/**
 * 获取 Dataset 的显示名称
 */
export function getDatasetDisplayName(dataset: Dataset): string {
  return dataset.name || dataset.id;
}

/**
 * 格式化文档数量
 */
export function formatDocumentCount(count: number): string {
  if (count === 0) return "暂无文档";
  if (count === 1) return "1 个文档";
  return `${count} 个文档`;
}
