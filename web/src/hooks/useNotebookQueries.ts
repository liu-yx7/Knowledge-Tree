import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { notebookServiceClient } from "@/connect";
import type { Notebook } from "@/types/proto/api/v1/notebook_service_pb";

// ==================== Query Keys ====================

export const notebookKeys = {
  all: ["notebooks"] as const,
  list: () => [...notebookKeys.all, "list"] as const,
  detail: (id: number) => [...notebookKeys.all, "detail", id] as const,
};

// ==================== Queries ====================

/**
 * List all notebooks for the current user.
 * Auto-creates a default notebook if none exist (handled server-side).
 */
export const useNotebooks = (options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: notebookKeys.list(),
    queryFn: async (): Promise<Notebook[]> => {
      const response = await notebookServiceClient.listNotebooks({});
      return response.notebooks;
    },
    staleTime: 5 * 60 * 1000, // 5 minutes — notebooks change infrequently
    enabled: options?.enabled ?? true,
  });
};

/**
 * Get a specific notebook by ID.
 */
export const useNotebook = (id: number, options?: { enabled?: boolean }) => {
  return useQuery({
    queryKey: notebookKeys.detail(id),
    queryFn: async (): Promise<Notebook> => {
      return await notebookServiceClient.getNotebook({ id });
    },
    enabled: (options?.enabled ?? true) && id > 0,
  });
};

// ==================== Mutations ====================

/**
 * Create a new notebook.
 */
export const useCreateNotebook = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ title, icon }: { title: string; icon?: string }): Promise<Notebook> => {
      return await notebookServiceClient.createNotebook({ title, icon: icon ?? "" });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notebookKeys.list() });
    },
  });
};

/**
 * Update an existing notebook.
 */
export const useUpdateNotebook = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, title, icon }: { id: number; title?: string; icon?: string }): Promise<Notebook> => {
      const paths: string[] = [];
      const notebook: Partial<Notebook> & { id: number } = { id };

      if (title !== undefined) {
        paths.push("title");
        notebook.title = title;
      }
      if (icon !== undefined) {
        paths.push("icon");
        notebook.icon = icon;
      }

      return await notebookServiceClient.updateNotebook({
        notebook: notebook as Notebook,
        updateMask: { paths },
      });
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: notebookKeys.list() });
      queryClient.invalidateQueries({ queryKey: notebookKeys.detail(data.id) });
    },
  });
};

/**
 * Delete a notebook. Memos are moved to the default notebook server-side.
 */
export const useDeleteNotebook = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: number): Promise<void> => {
      await notebookServiceClient.deleteNotebook({ id });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: notebookKeys.list() });
    },
  });
};
