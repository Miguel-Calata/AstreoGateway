import { useState } from "react";
import { Link } from "react-router-dom";
import { Plus, Pencil, Trash2 } from "lucide-react";
import { useProviders, useDeleteProvider } from "@/lib/queries";
import { type Provider } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/table";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { protocolVariant } from "@/lib/format";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { ProviderEditor } from "./ProviderEditor";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { useUpdateProvider } from "@/lib/queries";

export function ProvidersList() {
  const { data, isLoading } = useProviders();
  const del = useDeleteProvider();
  const upd = useUpdateProvider();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Provider | undefined>();

  const openNew = () => { setEditing(undefined); setEditorOpen(true); };
  const openEdit = (p: Provider) => { setEditing(p); setEditorOpen(true); };

  const onDelete = async (id: string) => {
    try {
      await del.mutateAsync(id);
      toast.success("Provider deleted");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Delete failed");
    }
  };

  const toggleEnabled = async (p: Provider) => {
    try {
      await upd.mutateAsync({ id: p.id, body: { ...p, enabled: !p.enabled } });
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Update failed");
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Providers"
        description="Upstream LLM gateways routed by the brain of the gateway."
        actions={<Button onClick={openNew}><Plus className="size-4" /> New provider</Button>}
      />
      {isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : data.length === 0 ? (
        <EmptyState title="No providers yet" hint="Add your first LLM gateway — OpenAI, Anthropic, OpenRouter, local Ollama…" action={<Button onClick={openNew}><Plus className="size-4" /> New provider</Button>} />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <THead>
              <Tr>
                <Th>Name</Th>
                <Th>Protocol</Th>
                <Th>Base URL</Th>
                <Th>Enabled</Th>
                <Th className="text-right">Actions</Th>
              </Tr>
            </THead>
            <TBody>
              {data.map((p) => (
                <Tr key={p.id}>
                  <Td>
                    <Link to={`/providers/${p.id}`} className="font-medium hover:text-primary">{p.name}</Link>
                    <div className="font-mono text-xs text-muted-foreground">{p.slug || p.id}</div>
                  </Td>
                  <Td><Badge variant={protocolVariant(p.protocol)}>{p.protocol}</Badge></Td>
                  <Td><span className="font-mono text-xs text-muted-foreground">{p.base_url}</span></Td>
                  <Td>
                    <Switch checked={p.enabled} onCheckedChange={() => toggleEnabled(p)} disabled={upd.isPending} />
                  </Td>
                  <Td className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon" onClick={() => openEdit(p)} title="Edit"><Pencil className="size-4" /></Button>
                      <ConfirmDialog
                        trigger={(open) => <Button variant="ghost" size="icon" onClick={open} title="Delete"><Trash2 className="size-4" /></Button>}
                        title="Delete provider?"
                        description={`Removing ${p.name} also discards its API keys. Routes targeting it may fail.`}
                        confirmLabel="Delete"
                        onConfirm={() => onDelete(p.id)}
                      />
                    </div>
                  </Td>
                </Tr>
              ))}
            </TBody>
          </Table>
        </div>
      )}
      <ProviderEditor open={editorOpen} onOpenChange={setEditorOpen} provider={editing} />
    </div>
  );
}