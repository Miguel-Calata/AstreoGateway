import { useEffect, useState } from "react";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { useCreateProvider, useUpdateProvider } from "@/lib/queries";
import { type Provider } from "@/lib/api";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";

const EMPTY: Provider = { id: "", name: "", protocol: "openai", base_url: "", enabled: true, headers: {} };

export function ProviderEditor({
  open, onOpenChange, provider,
}: { open: boolean; onOpenChange: (v: boolean) => void; provider?: Provider }) {
  const isEdit = !!provider;
  const [form, setForm] = useState<Provider>(EMPTY);
  const [headerKey, setHeaderKey] = useState("");
  const [headerVal, setHeaderVal] = useState("");
  const create = useCreateProvider();
  const update = useUpdateProvider();

  useEffect(() => {
    setForm(provider ? { ...provider, headers: { ...provider.headers } } : EMPTY);
    setHeaderKey("");
    setHeaderVal("");
  }, [provider, open]);

  const set = (patch: Partial<Provider>) => setForm((f) => ({ ...f, ...patch }));

  const addHeader = () => {
    if (!headerKey.trim()) return;
    set({ headers: { ...form.headers, [headerKey.trim()]: headerVal } });
    setHeaderKey("");
    setHeaderVal("");
  };
  const removeHeader = (k: string) => {
    const next = { ...form.headers };
    delete next[k];
    set({ headers: next });
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (isEdit) await update.mutateAsync({ id: form.id, body: form });
      else await create.mutateAsync(form);
      toast.success(isEdit ? "Provider updated" : "Provider created");
      onOpenChange(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Save failed");
    }
  };

  const busy = create.isPending || update.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit provider" : "New provider"}</DialogTitle>
          <DialogDescription>Configure an upstream LLM gateway.</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={form.name} onChange={(e) => set({ name: e.target.value })} required placeholder="mistral" />
            <p className="text-xs text-muted-foreground">
              Public model IDs use this as prefix: <code>{(form.name || "name").trim() || "name"}:model</code>. No colons.
            </p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="protocol">Protocol</Label>
              <Select value={form.protocol} onValueChange={(v) => set({ protocol: v as Provider["protocol"] })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai">openai</SelectItem>
                  <SelectItem value="anthropic">anthropic</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="enabled">Enabled</Label>
              <div className="flex h-9 items-center gap-2">
                <Switch checked={form.enabled} onCheckedChange={(c) => set({ enabled: c })} />
                <span className="text-sm text-muted-foreground">{form.enabled ? "Yes" : "No"}</span>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="base_url">Base URL</Label>
            <Input id="base_url" value={form.base_url} onChange={(e) => set({ base_url: e.target.value })} placeholder="https://api.openai.com/v1" required />
            <p className="text-xs text-muted-foreground">May include or omit the <code>/v1</code> suffix; paths are joined robustly.</p>
          </div>
          <div className="space-y-2">
            <Label>Extra headers</Label>
            <div className="flex gap-2">
              <Input value={headerKey} onChange={(e) => setHeaderKey(e.target.value)} placeholder="header name" />
              <Input value={headerVal} onChange={(e) => setHeaderVal(e.target.value)} placeholder="value" />
              <Button type="button" variant="outline" size="sm" onClick={addHeader}>Add</Button>
            </div>
            <div className="space-y-1">
              {Object.entries(form.headers ?? {}).map(([k, v]) => (
                <div key={k} className="flex items-center justify-between rounded-md border border-border bg-background/40 px-3 py-1.5 text-xs">
                  <span className="font-mono">{k}: <span className="text-muted-foreground">{v}</span></span>
                  <Button type="button" variant="ghost" size="sm" onClick={() => removeHeader(k)}>Remove</Button>
                </div>
              ))}
            </div>
          </div>
          <DialogFooter className="mt-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? <Spinner /> : isEdit ? "Save" : "Create"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}