import { createContext, type ReactNode, useCallback, useContext, useEffect, useState } from "react";
import useCurrentUser from "@/hooks/useCurrentUser";
import { useNotebooks } from "@/hooks/useNotebookQueries";

const STORAGE_KEY = "notebook-selected-id";

interface NotebookContextValue {
  /** Currently selected notebook ID, undefined means loading/not-yet-resolved. */
  selectedNotebookId: number | undefined;
  /** Switch to a different notebook. Pass undefined to reset to default. */
  selectNotebook: (id: number | undefined) => void;
  /** The dataset_id of the currently selected notebook (for AI Chat). */
  selectedDatasetId: string | undefined;
}

const NotebookContext = createContext<NotebookContextValue | null>(null);

export function NotebookProvider({ children }: { children: ReactNode }) {
  const currentUser = useCurrentUser();
  const { data: notebooks } = useNotebooks({ enabled: !!currentUser });
  const [selectedNotebookId, setSelectedNotebookId] = useState<number | undefined>(undefined);

  // Restore persisted selection or fall back to default notebook.
  useEffect(() => {
    if (!notebooks || notebooks.length === 0) return;

    const persisted = localStorage.getItem(STORAGE_KEY);
    if (persisted) {
      const id = Number(persisted);
      // Validate the persisted notebook still exists.
      if (notebooks.some((nb) => nb.id === id)) {
        setSelectedNotebookId(id);
        return;
      }
    }

    // Fall back to the default notebook.
    const defaultNb = notebooks.find((nb) => nb.isDefault) ?? notebooks[0];
    setSelectedNotebookId(defaultNb.id);
  }, [notebooks]);

  const selectNotebook = useCallback(
    (id: number | undefined) => {
      if (id !== undefined) {
        localStorage.setItem(STORAGE_KEY, String(id));
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
      setSelectedNotebookId(id);
    },
    [],
  );

  const selectedDatasetId =
    notebooks?.find((nb) => nb.id === selectedNotebookId)?.datasetId ?? undefined;

  return (
    <NotebookContext.Provider value={{ selectedNotebookId, selectNotebook, selectedDatasetId }}>
      {children}
    </NotebookContext.Provider>
  );
}

export function useNotebookContext() {
  const context = useContext(NotebookContext);
  if (!context) throw new Error("useNotebookContext must be used within NotebookProvider");
  return context;
}
