import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Plus, Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  useApiKeys, useCreateApiKey, useDeleteApiKey, useUpdateApiKey, useProviders,
} from "@/lib/queries";
import { ApiError, type ApiKey } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { PageHeader } from "@/components/PageHeader";
import { ProviderEditor } from "@/features/providers/ProviderEditor";
import { shortId } from "@/lib/format";

export function ProviderDetail() {
  const { id = "" } = useParams();
  const providers = useProviders();
  const provider = providers.data.find((p) => p.id === id);
  const keys = useApiKeys(id, !!provider);
  const del = useDeleteApiKey();
  const upd = useUpdateApiKey();

  const [editorOpen, setEditorOpen] = useState(false);
  const [keyDialog, setKeyDialog] = useState(false);

  return (
    <div className="space-y-6">
      <div>
        <Link to="/providers" className="inline-flex items-center text-xs text-muted-foreground hover:text-foreground">
          <ArrowLeft className="mr-1 size-3.5" /> Providers
        </Link>
      </div>
      <PageHeader
        title={provider?.name ?? "Provider"}
        description={provider ? <span className="font-mono text-xs">{provider.id}</span> : undefined}
        actions={
          <>
            <Button variant="outline" onClick={() => setEditorOpen(true)}><Pencil className="size-4" /> Edit</Button>
            <Button onClick={() => setKeyDialog(true)}><Plus className="size-4" /> New key</Button>
          </>
        }
      />

      <div className="grid grid-cols-2 gap-4 text-sm">
        <DetailRow label="Protocol" value={<Badge variant={provider?.protocol === "anthropic" ? "warning" : "secondary"}>{provider?.protocol}</Badge>} />
        <DetailRow label="Enabled" value={provider?.enabled ? "Yes" : "No"} />
        <DetailRow label="Base URL" value={<span className="font-mono text-xs">{provider?.base_url}</span>} />
        <DetailRow label="Headers" value={provider && Object.keys(provider.headers ?? {}).length > 0 ? `${Object.keys(provider.headers).length} custom` : "none"} />
      </div>

      <div className="space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">API Keys</h2>
        {keys.isLoading ? (
          <Skeleton className="h-24 w-full" />
        ) : keys.data.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border py-10 text-center text-sm text-muted-foreground">
            No API keys. Add one to route requests to this provider.
          </div>
        ) : (
          <div className="rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="border-b border-border text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 text-left">Label</th>
                  <th className="px-3 py-2 text-left">ID</th>
                  <th className="px-3 py-2 text-left">Priority</th>
                  <th className="px-3 py-2 text-left">Enabled</th>
                  <th className="px-3 py-2 text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {keys.data.map((k) => (
                  <tr key={k.id} className="border-b border-border/60">
                    <td className="px-3 py-2">{k.label || <span className="text-muted-foreground">—</span>}</td>
                    <td className="px-3 py-2 font-mono text-xs text-muted-foreground">{shortId(k.id)}</td>
                    <td className="px-3 py-2 font-mono">{k.priority}</td>
                    <td className="px-3 py-2">
                      <Switch
                        checked={k.enabled}
                        disabled={upd.isPending}
                        onCheckedChange={() => upd.mutateAsync({ id: k.id, body: { ...k, enabled: !k.enabled } })}
                      />
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Button
                        variant="ghost" size="icon" title="Delete"
                        onClick={async () => {
                          try { await del.mutateAsync(k.id); toast.success("Key deleted"); }
                          catch (e) { toast.error(e instanceof ApiError ? e.message : "Delete failed"); }
                        }}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ProviderEditor open={editorOpen} onOpenChange={setEditorOpen} provider={provider} />
      <ApiKeyDialog open={keyDialog} onOpenChange={setKeyDialog} providerId={id} />
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between rounded-md border border-border bg-card/40 px-3 py-2">
      <span className="text-xs uppercase tracking-wide text-muted-foreground">{label}</span>
      <span>{value}</span>
    </div>
  );
}

function ApiKeyDialog({
  open, onOpenChange, providerId,
}: { open: boolean; onOpenChange: (v: boolean) => void; providerId: string }) {
  const [label, setLabel] = useState("");
  const [value, setValue] = useState("");
  const [priority, setPriority] = useState(0);
  const create = useCreateApiKey(providerId);

  const onOpenChangeReset = (v: boolean) => {
    if (v) { setLabel(""); setValue(""); setPriority(0); }
    onOpenChange(v);
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await create.mutateAsync({ id: "", provider_id: providerId, label, key_value: value, priority, enabled: true });
      toast.success("API key added");
      onOpenChangeReset(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Add failed");
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChangeReset}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New API key</DialogTitle>
          <DialogDescription>Stored in the gateway. List &amp; detail views redact the value after creation.</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="label">Label</Label>
            <Input id="label" value={label} onChange={(e) => setLabel(e.target.value)} placeholder="prod / pool-A" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="value">Key value</Label>
            <Input id="value" type="password" value={value} onChange={(e) => setValue(e.target.value)} required placeholder="sk-..." />
          </div>
          <div className="space-y-2">
            <Label htmlFor="priority">Priority</Label>
            <Input id="priority" type="number" value={priority} onChange={(e) => setPriority(Number(e.target.value))} />
            <p className="text-xs text-muted-foreground">Lower number = higher priority. Used by <code>priority</code> / <code>failover</code> routing.</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChangeReset(false)}>Cancel</Button>
            <Button type="submit" disabled={create.isPending}>{create.isPending ? <Spinner /> : "Add key"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}