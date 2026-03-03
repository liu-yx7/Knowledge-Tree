// filepath: /Users/yuxuanli/Desktop/Project/Knowtree/Knowledge-Tree/web/src/hooks/useRAGFlowQueries.ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ragflowServiceClient } from "@/connect";
import type {
  ContentSyncState,
  SearchResult,
  SyncStats,
  SyncStatus,
} from "@/types/proto/api/v1/ragflow_service_pb";

// ==================== Query Keys ====================

export const ragflowKeys = {
  all: ["ragflow"] as const,
  syncStatus: () => [...ragflowKeys.all, "sync-status"] as const,
  syncStats: () => [...ragflowKeys.all, "sync-stats"] as const,
  syncStates: (filters?: { statusFilter?: string; contentTypeFilter?: string }) =>
    [...ragflowKeys.all, "sync-states", filters] as const,
  search: (query: string) => [...ragflowKeys.all, "search", query] as const,
};

// ==================== Types ====================

export interface Reference {
  memoUid: string;
  memoName: string;
  title: string;
  contentSnippet: string;
  similarityScore: number;
  createTime?: Date;
}

// ==================== Queries ====================

/**
 * 获取 RAGFlow 同步服务状态（需要管理员权限）
 */
export const useSyncStatus = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: ragflowKeys.syncStatus(),
    queryFn: async (): Promise<SyncStatus> => {
      const response = await ragflowServiceClient.getSyncStatus({});
      return response;
    },
    staleTime: 30 * 1000, // 30 秒
    enabled: options?.enabled ?? true,
  });
};

/**
 * 获取同步统计信息（需要管理员权限）
 */
export const useSyncStats = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: ragflowKeys.syncStats(),
    queryFn: async (): Promise<SyncStats> => {
      const response = await ragflowServiceClient.getSyncStats({});
      return response;
    },
    staleTime: 30 * 1000, // 30 秒
    enabled: options?.enabled ?? true,
  });
};

/**
 * 列出内容同步状态
 */
export const useContentSyncStates = (
  filters?: {
    statusFilter?: string;
    contentTypeFilter?: string;
    pageSize?: number;
    pageToken?: string;
  },
  options?: { enabled?: boolean }
) => {
  return useQuery({
    queryKey: ragflowKeys.syncStates(filters),
    queryFn: async () => {
      const response = await ragflowServiceClient.listContentSyncStates({
        statusFilter: filters?.statusFilter || "",
        contentTypeFilter: filters?.contentTypeFilter || "",
        pageSize: filters?.pageSize || 20,
        pageToken: filters?.pageToken || "",
      });
      return {
        syncStates: response.syncStates as ContentSyncState[],
        nextPageToken: response.nextPageToken,
      };
    },
    enabled: options?.enabled ?? true,
  });
};

// ==================== Mutations ====================

/**
 * 手动触发同步（需要管理员权限）
 */
export const useTriggerSync = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (): Promise<void> => {
      await ragflowServiceClient.triggerSync({});
    },
    onSuccess: () => {
      // 刷新同步状态和统计信息
      queryClient.invalidateQueries({ queryKey: ragflowKeys.syncStatus() });
      queryClient.invalidateQueries({ queryKey: ragflowKeys.syncStats() });
      queryClient.invalidateQueries({ queryKey: ragflowKeys.syncStates() });
    },
  });
};

/**
 * 语义检索
 */
export const useSemanticSearch = () => {
  return useMutation({
    mutationFn: async ({
      query,
      topK = 6,
      similarityThreshold = 0.3,
    }: {
      query: string;
      topK?: number;
      similarityThreshold?: number;
    }): Promise<Reference[]> => {
      const response = await ragflowServiceClient.semanticSearch({
        query,
        topK,
        similarityThreshold,
      });

      // 转换为 Reference 类型
      return (response.results || []).map((result: SearchResult) => ({
        memoUid: extractMemoUid(result.memoName),
        memoName: result.memoName,
        title: result.title || "未命名笔记",
        contentSnippet: result.contentSnippet,
        similarityScore: result.similarityScore,
        createTime: result.createTime ? new Date(result.createTime.seconds.toString()) : undefined,
      }));
    },
  });
};

// ==================== Helper Functions ====================

/**
 * 从 memo_name (memos/{uid}) 提取 UID
 */
function extractMemoUid(memoName: string): string {
  if (!memoName) return "";
  const parts = memoName.split("/");
  return parts.length >= 2 ? parts[1] : memoName;
}

/**
 * 格式化同步状态为可读文本
 */
export function formatSyncStatus(status: string): string {
  const statusMap: Record<string, string> = {
    pending: "待同步",
    synced: "已同步",
    failed: "同步失败",
    skipped: "已跳过",
  };
  return statusMap[status] || status;
}

/**
 * 获取同步状态的颜色类名
 */
export function getSyncStatusColor(status: string): string {
  const colorMap: Record<string, string> = {
    pending: "text-yellow-600 bg-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30",
    synced: "text-green-600 bg-green-100 dark:text-green-400 dark:bg-green-900/30",
    failed: "text-red-600 bg-red-100 dark:text-red-400 dark:bg-red-900/30",
    skipped: "text-gray-600 bg-gray-100 dark:text-gray-400 dark:bg-gray-800",
  };
  return colorMap[status] || "text-gray-600 bg-gray-100";
}

/**
 * 格式化内容类型为可读文本
 */
export function formatContentType(contentType: string): string {
  const typeMap: Record<string, string> = {
    memo: "笔记",
    attachment: "附件",
  };
  return typeMap[contentType] || contentType;
}
