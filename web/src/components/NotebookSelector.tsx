import { BookOpenIcon, CheckIcon, PlusIcon } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useNotebookContext } from "@/contexts/NotebookContext";
import { useNotebooks, useCreateNotebook } from "@/hooks/useNotebookQueries";
import { cn } from "@/lib/utils";
import { useTranslate } from "@/utils/i18n";

interface Props {
  collapsed?: boolean;
}

const NotebookSelector = ({ collapsed }: Props) => {
  const t = useTranslate();
  const { data: notebooks = [] } = useNotebooks();
  const { selectedNotebookId, selectNotebook } = useNotebookContext();
  const createNotebook = useCreateNotebook();

  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [newTitle, setNewTitle] = useState("");

  const selectedNotebook = notebooks.find((nb) => nb.id === selectedNotebookId);
  const displayLabel = selectedNotebook
    ? `${selectedNotebook.icon || "📓"} ${selectedNotebook.title}`
    : t("common.notebooks");

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    const notebook = await createNotebook.mutateAsync({ title: newTitle.trim(), icon: "📓" });
    selectNotebook(notebook.id);
    setNewTitle("");
    setShowCreateDialog(false);
  };

  const trigger = (
    <DropdownMenuTrigger asChild>
      <button
        className={cn(
          "px-2 py-2 rounded-2xl border flex flex-row items-center text-lg text-sidebar-foreground transition-colors",
          collapsed ? "" : "w-full px-4",
          "border-transparent hover:bg-sidebar-accent hover:text-sidebar-accent-foreground hover:border-sidebar-accent-border opacity-80",
        )}
      >
        {collapsed ? (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <div>
                  <BookOpenIcon className="w-6 h-auto shrink-0" />
                </div>
              </TooltipTrigger>
              <TooltipContent side="right">
                <p>{displayLabel}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        ) : (
          <BookOpenIcon className="w-6 h-auto shrink-0" />
        )}
        {!collapsed && <span className="ml-3 truncate">{displayLabel}</span>}
      </button>
    </DropdownMenuTrigger>
  );

  return (
    <>
      <DropdownMenu>
        {trigger}
        <DropdownMenuContent align="start" className="w-56">
          {notebooks.map((nb) => (
            <DropdownMenuItem
              key={nb.id}
              onClick={() => selectNotebook(nb.id)}
              className="flex items-center justify-between"
            >
              <span>
                {nb.icon || "📓"} {nb.title}
              </span>
              {nb.id === selectedNotebookId && <CheckIcon className="w-4 h-4 text-primary" />}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setShowCreateDialog(true)}>
            <PlusIcon className="w-4 h-4 mr-2" />
            {t("common.new-notebook")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent className="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>{t("common.new-notebook")}</DialogTitle>
            <DialogDescription>{""}</DialogDescription>
          </DialogHeader>
          <Input
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
            placeholder={t("common.name")}
            autoFocus
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreate();
            }}
          />
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={!newTitle.trim() || createNotebook.isPending}>
              {t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};

export default NotebookSelector;
