import { useEffect, useState } from "react";
import { Copy, Plus, Trash2, KeyRound, CheckCheck } from "lucide-react";
import { toast } from "sonner";
import { useGatewayKeys, useCreateGatewayKey, useDeleteGatewayKey } from "@/lib/queries";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";

export function GatewayKeysList() {
  const { data, isLoading } = useGatewayKeys();
  const del = useDeleteGatewayKey();
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Gateway Keys"
        description="Bearer tokens clients use to call /v1/*. Tokens are shown once at creation."
        actions={<Button onClick={() => setCreateOpen(true)}><Plus className="size-4" /> New key</Button>}
      />
      {isLoading ? (
        <div className="flex items-center justify-center py-16"><Spinner className="size-6" /></div>
      ) : data.length === 0 ? (
        <EmptyState title="No gateway keys" hint="Create one and use it as the Bearer token in your OpenAI client." action={<Button onClick={() => setCreateOpen(true)}><Plus className="size-4" /> New key</Button>} />
      ) : (
        <div className="rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <th className="px-3 py-2 text-left">Label</th>
                <th className="px-3 py-2 text-left">Prefix</th>
                <th className="px-3 py-2 text-left">Enabled</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {data.map((k) => (
                <tr key={k.id} className="border-b border-border/60">
                  <td className="px-3 py-2">
                    <div className="font-medium">{k.label || <span className="text-muted-foreground">—</span>}</div>
                    <div className="font-mono text-xs text-muted-foreground">{k.id}</div>
                  </td>
                  <td className="px-3 py-2"><span className="font-mono text-xs">{k.prefix}…</span></td>
                  <td className="px-3 py-2"><Badge variant={k.enabled ? "success" : "secondary"}>{k.enabled ? "active" : "off"}</Badge></td>
                  <td className="px-3 py-2 text-right">
                    <ConfirmDialog
                      trigger={(open) => <Button variant="ghost" size="icon" onClick={open} title="Delete"><Trash2 className="size-4" /></Button>}
                      title="Revoke gateway key?"
                      description="Clients using this key will immediately receive 401. Cannot be undone."
                      confirmLabel="Revoke"
                      onConfirm={async () => {
                        try { await del.mutateAsync(k.id); toast.success("Key revoked"); }
                        catch (e) { toast.error(e instanceof ApiError ? e.message : "Revoke failed"); }
                      }}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <CreateGatewayKeyDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  );
}

function CreateGatewayKeyDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const [label, setLabel] = useState("");
  const [token, setToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const create = useCreateGatewayKey();

  useEffect(() => {
    if (!open) { setLabel(""); setToken(null); setCopied(false); }
  }, [open]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await create.mutateAsync({ label: label || undefined });
      setToken(res.token ?? null);
      toast.success("Gateway key created");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Create failed");
    }
  };

  const copy = () => {
    if (!token) return;
    navigator.clipboard.writeText(token);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {token ? (
          <div className="space-y-4">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2"><KeyRound className="size-5 text-primary" /> Gateway key ready</DialogTitle>
              <DialogDescription>Copy it now. It will not be shown again.</DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2 rounded-md border border-warning/40 bg-warning/10 px-3 py-2">
              <code className="flex-1 truncate font-mono text-sm">{token}</code>
              <Button type="button" size="sm" variant="outline" onClick={copy}>
                {copied ? <CheckCheck className="size-4 text-success" /> : <Copy className="size-4" />}
              </Button>
            </div>
            <DialogFooter className="mt-4">
              <Button type="button" onClick={() => onOpenChange(false)}>Done</Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="space-y-4">
            <DialogHeader>
              <DialogTitle>New gateway key</DialogTitle>
              <DialogDescription>Used as <code>Authorization: Bearer …</code> on <code>/v1/*</code>.</DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <Label htmlFor="gk-label">Label</Label>
              <Input id="gk-label" value={label} onChange={(e) => setLabel(e.target.value)} placeholder="cursor / opencode / prod" autoFocus />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
              <Button type="submit" disabled={create.isPending}>{create.isPending ? <Spinner /> : "Generate"}</Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}