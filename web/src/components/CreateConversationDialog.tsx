import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import type { Provider } from "@/types/proto/api/v1/ai_service";
import { useTranslate } from "@/utils/i18n";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (name: string, provider: string, model: string, systemPrompt?: string) => void;
  providers: Provider[];
}

const CreateConversationDialog = ({ open, onOpenChange, onConfirm, providers }: Props) => {
  const t = useTranslate();
  const [name, setName] = useState("");
  const [provider, setProvider] = useState("");
  const [model, setModel] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");

  const selectedProvider = providers.find((p) => p.name === provider);
  const availableModels = selectedProvider?.availableModels || [];

  const handleConfirm = () => {
    if (!name.trim() || !provider || !model) return;
    onConfirm(name.trim(), provider, model, systemPrompt.trim() || undefined);
    // Reset form
    setName("");
    setProvider("");
    setModel("");
    setSystemPrompt("");
  };

  const handleProviderChange = (value: string) => {
    setProvider(value);
    setModel(""); // Reset model when provider changes
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{t("ai.create-conversation")}</DialogTitle>
          <DialogDescription>{t("ai.create-conversation-description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="name">{t("ai.conversation-name")}</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("ai.conversation-name-placeholder")}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="provider">{t("ai.provider")}</Label>
            <Select value={provider} onValueChange={handleProviderChange}>
              <SelectTrigger id="provider">
                <SelectValue placeholder={t("ai.select-provider")} />
              </SelectTrigger>
              <SelectContent>
                {providers.map((p) => (
                  <SelectItem key={p.name} value={p.name}>
                    {p.displayName}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="model">{t("ai.model")}</Label>
            <Select value={model} onValueChange={setModel} disabled={!provider}>
              <SelectTrigger id="model">
                <SelectValue placeholder={t("ai.select-model")} />
              </SelectTrigger>
              <SelectContent>
                {availableModels.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="systemPrompt">{t("ai.system-prompt")} ({t("common.optional")})</Label>
            <Textarea
              id="systemPrompt"
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              placeholder={t("ai.system-prompt-placeholder")}
              rows={4}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={handleConfirm} disabled={!name.trim() || !provider || !model}>
            {t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default CreateConversationDialog;
