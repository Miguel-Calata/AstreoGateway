import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import {
  DndContext, type DragEndEvent, KeyboardSensor, PointerSensor, closestCenter, useSensor, useSensors,
} from "@dnd-kit/core";
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical, Plus, Trash2, X } from "lucide-react";
import {
  useAliases, useCreateAlias, useUpdateAlias, useDeleteAlias, useDiscovery, useProviders, ROUTING_MODES,
} from "@/lib/queries";
import { ApiError, type Alias, type AliasTarget, type RoutingMode } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,
} from "@/components/ui/sheet";
import { shortId } from "@/lib/format";

const EMPTY_ALIAS: Alias = { id: "", name: "", routing: "failover", enabled: true, targets: [] };

export function AliasesList() {
  const { data, isLoading } = useAliases();
  const del = useDeleteAlias();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Alias | undefined>();

  const openNew = () => { setEditing(undefined); setEditorOpen(true); };
  const openEdit = (a: Alias) => { setEditing(a); setEditorOpen(true); };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Aliases"
        description="Friendly model names routing to one or more provider:model targets."
        actions={<Button onClick={openNew}><Plus className="size-4" /> New alias</Button>}
      />
      {isLoading ? (
        <Spinner className="size-6" />
      ) : data.length === 0 ? (
        <EmptyState title="No aliases yet" hint="Create an alias like `gpt-4` that routes across multiple providers." action={<Button onClick={openNew}><Plus className="size-4" /> New alias</Button>} />
      ) : (
        <div className="rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead className="border-b border-border text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Routing</th>
                <th className="px-3 py-2 text-left">Targets</th>
                <th className="px-3 py-2 text-left">Enabled</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {data.map((a) => (
                <tr key={a.id} className="border-b border-border/60">
                  <td className="px-3 py-2"><span className="font-mono font-medium">{a.name}</span></td>
                  <td className="px-3 py-2"><Badge variant="secondary">{a.routing}</Badge></td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{a.targets?.length ?? 0}</td>
                  <td className="px-3 py-2"><Badge variant={a.enabled ? "success" : "secondary"}>{a.enabled ? "on" : "off"}</Badge></td>
                  <td className="px-3 py-2 text-right">
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon" onClick={() => openEdit(a)} title="Edit"><Plus className="size-4 rotate-45" /></Button>
                      <ConfirmDialog
                        trigger={(open) => <Button variant="ghost" size="icon" onClick={open} title="Delete"><Trash2 className="size-4" /></Button>}
                        title="Delete alias?"
                        description={`Removing the alias ${a.name} stops routing on that name.`}
                        confirmLabel="Delete"
                        onConfirm={async () => {
                          try { await del.mutateAsync(a.id); toast.success("Alias deleted"); }
                          catch (e) { toast.error(e instanceof ApiError ? e.message : "Delete failed"); }
                        }}
                      />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <AliasEditor open={editorOpen} onOpenChange={setEditorOpen} alias={editing} />
    </div>
  );
}

function AliasEditor({ open, onOpenChange, alias }: { open: boolean; onOpenChange: (v: boolean) => void; alias?: Alias }) {
  const isEdit = !!alias;
  const [form, setForm] = useState<Alias>(EMPTY_ALIAS);
  const create = useCreateAlias();
  const update = useUpdateAlias();
  const providers = useProviders();
  const discovery = useDiscovery();
  const navigate = useNavigate();

  useEffect(() => {
    setForm(alias ? { ...alias, targets: alias.targets?.map((t) => ({ ...t })) ?? [] } : EMPTY_ALIAS);
  }, [alias, open]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const set = (patch: Partial<Alias>) => setForm((f) => ({ ...f, ...patch }));

  const addTarget = () => {
    const firstProvider = providers.data[0]?.id ?? "";
    set({ targets: [...(form.targets ?? []), { provider_id: firstProvider, model_name: "", position: (form.targets ?? []).length }] });
  };
  const updateTarget = (i: number, patch: Partial<AliasTarget>) => {
    set({ targets: (form.targets ?? []).map((t, idx) => (idx === i ? { ...t, ...patch } : t)) });
  };
  const removeTarget = (i: number) => {
    set({ targets: (form.targets ?? []).filter((_, idx) => idx !== i).map((t, idx) => ({ ...t, position: idx })) });
  };

  const onDragEnd = (e: DragEndEvent) => {
    const { active, over } = e;
    if (!over || active.id === over.id) return;
    setForm((f) => {
      const arr = f.targets ?? [];
      const oldI = arr.findIndex((_, idx) => String(idx) === active.id);
      const newI = arr.findIndex((_, idx) => String(idx) === over.id);
      if (oldI < 0 || newI < 0) return f;
      const moved = arrayMove(arr, oldI, newI).map((t, idx) => ({ ...t, position: idx }));
      return { ...f, targets: moved };
    });
  };

  const hasDupes = (form.targets ?? []).some((t, _i, arr) => arr.filter((x) => x.provider_id === t.provider_id && x.model_name === t.model_name).length > 1);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (hasDupes) { toast.error("Duplicate (provider, model) targets not allowed."); return; }
    const targets = (form.targets ?? []).map((t, i) => ({ ...t, position: i }));
    const body = { ...form, targets };
    try {
      if (isEdit) await update.mutateAsync({ id: form.id, body });
      else await create.mutateAsync(body);
      toast.success(isEdit ? "Alias updated" : "Alias created");
      onOpenChange(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Save failed");
    }
  };

  const busy = create.isPending || update.isPending;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>{isEdit ? "Edit alias" : "New alias"}</SheetTitle>
          <SheetDescription>Order targets by drag. Position drives <code>priority</code> / <code>failover</code>.</SheetDescription>
        </SheetHeader>
        <form onSubmit={onSubmit} className="flex flex-1 flex-col overflow-hidden">
          <div className="flex-1 space-y-5 overflow-y-auto scrollbar-thin p-6">
            <div className="space-y-2">
              <Label htmlFor="alias-name">Name</Label>
              <Input id="alias-name" value={form.name} onChange={(e) => set({ name: e.target.value })} required placeholder="gpt-4o" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label>Routing</Label>
                <Select value={form.routing} onValueChange={(v) => set({ routing: v as RoutingMode })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {ROUTING_MODES.map((m) => <SelectItem key={m} value={m}>{m}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Enabled</Label>
                <div className="flex h-9 items-center gap-2">
                  <Switch checked={form.enabled} onCheckedChange={(c) => set({ enabled: c })} />
                  <span className="text-sm text-muted-foreground">{form.enabled ? "Yes" : "No"}</span>
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>Targets</Label>
                <Button type="button" variant="outline" size="sm" onClick={addTarget} disabled={providers.data.length === 0}>
                  <Plus className="size-4" /> Add
                </Button>
              </div>
              {providers.data.length === 0 ? (
                <p className="rounded-md border border-dashed border-border px-3 py-6 text-center text-xs text-muted-foreground">
                  No providers. <button type="button" className="text-primary underline" onClick={() => navigate("/providers")}>Add one</button>.
                </p>
              ) : (
                <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
                  <SortableContext items={(form.targets ?? []).map((_, i) => String(i))} strategy={verticalListSortingStrategy}>
                    <div className="space-y-2">
                      {(form.targets ?? []).map((t, i) => (
                        <SortableTarget
                          key={i}
                          id={String(i)}
                          target={t}
                          providers={providers.data.map((p) => ({ id: p.id, name: p.name }))}
                          models={(discovery.data?.[t.provider_id]?.models ?? []).map((m) => m.model_id)}
                          onChange={(patch) => updateTarget(i, patch)}
                          onRemove={() => removeTarget(i)}
                        />
                      ))}
                    </div>
                  </SortableContext>
                </DndContext>
              )}
              {hasDupes && <p className="text-xs text-destructive">Duplicate (provider, model) combo in this alias.</p>}
            </div>
          </div>
          <SheetFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>Cancel</Button>
            <Button type="submit" disabled={busy}>{busy ? <Spinner /> : isEdit ? "Save" : "Create"}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

function SortableTarget({
  id, target, providers, models, onChange, onRemove,
}: {
  id: string;
  target: AliasTarget;
  providers: { id: string; name: string }[];
  models: string[];
  onChange: (patch: Partial<AliasTarget>) => void;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.6 : 1,
  };

  return (
    <div ref={setNodeRef} style={style} className="flex items-center gap-2 rounded-md border border-border bg-background/40 p-2">
      <button type="button" {...attributes} {...listeners} className="cursor-grab text-muted-foreground hover:text-foreground" title="Drag to reorder">
        <GripVertical className="size-4" />
      </button>
      <div className="grid flex-1 grid-cols-2 gap-2">
        <Select value={target.provider_id} onValueChange={(v) => onChange({ provider_id: v, model_name: "" })}>
          <SelectTrigger className="text-xs"><SelectValue /></SelectTrigger>
          <SelectContent>
            {providers.map((p) => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}
          </SelectContent>
        </Select>
        <Select value={target.model_name} onValueChange={(v) => onChange({ model_name: v })}>
          <SelectTrigger className="text-xs"><SelectValue placeholder="select model" /></SelectTrigger>
          <SelectContent>
            {models.length === 0
              ? <SelectItem value="__none" disabled>No models discovered — refresh provider</SelectItem>
              : models.map((m) => <SelectItem key={m} value={m}>{m}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      <Button type="button" variant="ghost" size="icon" onClick={onRemove} title="Remove"><X className="size-4" /></Button>
    </div>
  );
}