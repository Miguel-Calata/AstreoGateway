import { useMemo, useState } from "react";
import { toast } from "sonner";
import { FileUp } from "lucide-react";
import { useAliases, useCreateAlias, useDiscovery, useProviders, ROUTING_MODES } from "@/lib/queries";
import { ApiError, type RoutingMode } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle,
} from "@/components/ui/sheet";
import { resolveAliasLines, rowToAlias, type ResolvedAliasRow } from "./parseAliasLines";

const PLACEHOLDER = `# name [routing] slug:model …
coding failover openai:gpt-4o anthropic:claude-sonnet-4
fast random openrouter:meta-llama/Llama-3.3-70B
cheap openai:gpt-4o-mini`;

export function ImportAliasesSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const [text, setText] = useState("");
  const [defaultRouting, setDefaultRouting] = useState<RoutingMode>("failover");
  const [busy, setBusy] = useState(false);

  const providers = useProviders();
  const aliases = useAliases();
  const discovery = useDiscovery();
  const create = useCreateAlias();

  const rows = useMemo(() => {
    if (!text.trim()) return [] as ResolvedAliasRow[];
    return resolveAliasLines(text, {
      defaultRouting,
      providers: providers.data,
      existingNames: (aliases.data ?? []).map((a) => a.name),
      discovery: discovery.data,
    });
  }, [text, defaultRouting, providers.data, aliases.data, discovery.data]);

  const okCount = rows.filter((r) => r.status === "ok").length;
  const skipCount = rows.filter((r) => r.status === "skip").length;
  const errCount = rows.filter((r) => r.status === "error").length;

  const onApply = async () => {
    const toCreate = rows.map(rowToAlias).filter((a): a is NonNullable<typeof a> => a != null);
    if (toCreate.length === 0) {
      toast.error("Nothing to create");
      return;
    }
    setBusy(true);
    let created = 0;
    let failed = 0;
    try {
      for (const body of toCreate) {
        try {
          await create.mutateAsync(body);
          created++;
        } catch {
          failed++;
        }
      }
      if (created > 0 && failed === 0) {
        toast.success(`Created ${created} alias${created === 1 ? "" : "es"}`);
        setText("");
        onOpenChange(false);
      } else if (created > 0) {
        toast.warning(`Created ${created}, failed ${failed}`);
      } else {
        toast.error(`Create failed (${failed})`);
      }
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : "Import failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!busy) onOpenChange(v); }}>
      <SheetContent side="right" className="max-w-2xl">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <FileUp className="size-4" /> Import aliases
          </SheetTitle>
          <SheetDescription>
            One alias per line: <code className="text-xs">name [routing] slug:model …</code>.
            Existing names are skipped. Comments start with <code className="text-xs">#</code>.
          </SheetDescription>
        </SheetHeader>

        <div className="flex flex-1 flex-col gap-4 overflow-hidden p-6">
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="alias-import">Lines</Label>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">Default routing</span>
                <Select value={defaultRouting} onValueChange={(v) => setDefaultRouting(v as RoutingMode)}>
                  <SelectTrigger className="h-8 w-[140px] text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {ROUTING_MODES.map((m) => <SelectItem key={m} value={m}>{m}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Textarea
              id="alias-import"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={PLACEHOLDER}
              className="min-h-[160px] font-mono text-xs"
              disabled={busy}
            />
          </div>

          {rows.length > 0 && (
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <Badge variant="success">{okCount} ready</Badge>
              {skipCount > 0 && <Badge variant="secondary">{skipCount} skip</Badge>}
              {errCount > 0 && <Badge variant="destructive">{errCount} error</Badge>}
            </div>
          )}

          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border scrollbar-thin">
            {rows.length === 0 ? (
              <p className="px-3 py-8 text-center text-xs text-muted-foreground">
                Paste lines to preview.
              </p>
            ) : (
              <table className="w-full text-xs">
                <thead className="sticky top-0 border-b border-border bg-card text-left uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="px-2 py-1.5 w-10">#</th>
                    <th className="px-2 py-1.5">Status</th>
                    <th className="px-2 py-1.5">Alias</th>
                    <th className="px-2 py-1.5">Targets</th>
                    <th className="px-2 py-1.5">Notes</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.line} className="border-b border-border/60 align-top">
                      <td className="px-2 py-1.5 font-mono text-muted-foreground">{r.line}</td>
                      <td className="px-2 py-1.5">
                        <Badge
                          variant={
                            r.status === "ok" ? "success"
                              : r.status === "skip" ? "secondary"
                                : "destructive"
                          }
                        >
                          {r.status}
                        </Badge>
                      </td>
                      <td className="px-2 py-1.5 font-mono">
                        {r.name ?? "—"}
                        {r.routing && r.status !== "error" && (
                          <span className="ml-1 text-muted-foreground">({r.routing})</span>
                        )}
                      </td>
                      <td className="px-2 py-1.5 font-mono text-muted-foreground">
                        {(r.targetLabels ?? []).join(", ") || "—"}
                      </td>
                      <td className="px-2 py-1.5 text-muted-foreground">
                        {[...r.messages, ...r.warnings.map((w) => `⚠ ${w}`)].join("; ") || "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        <SheetFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button type="button" onClick={onApply} disabled={busy || okCount === 0}>
            {busy ? <Spinner /> : `Create ${okCount || ""}`.trim()}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
