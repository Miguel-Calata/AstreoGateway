import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  LayoutDashboard,
  Radio,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import {
  useClearRequestLogs,
  useGatewayKeys,
  useProviders,
  useRequestLogs,
} from "@/lib/queries";
import { ApiError, type RequestLog } from "@/lib/api";
import {
  formatDurationMs,
  formatNumber,
  formatRelativeMs,
  shortId,
  statusVariant,
} from "@/lib/format";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { RequestLogSheet } from "./RequestLogSheet";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const STATUS_OPTS = [
  { value: "all", label: "All status" },
  { value: "ok", label: "OK (2xx/3xx)" },
  { value: "client_err", label: "Client (4xx)" },
  { value: "server_err", label: "Server (5xx)" },
];

export function LogsList() {
  const [provider, setProvider] = useState("all");
  const [gatewayKey, setGatewayKey] = useState("all");
  const [statusClass, setStatusClass] = useState("all");
  const [directive, setDirective] = useState("");
  const [directiveDebounced, setDirectiveDebounced] = useState("");
  const [live, setLive] = useState(true);
  const [selected, setSelected] = useState<RequestLog | null>(null);

  const providers = useProviders();
  const keys = useGatewayKeys();
  const clear = useClearRequestLogs();

  useEffect(() => {
    const t = setTimeout(() => setDirectiveDebounced(directive.trim()), 300);
    return () => clearTimeout(t);
  }, [directive]);

  const query = useMemo(
    () => ({
      limit: 100,
      order: "ts_desc",
      provider_slug: provider === "all" ? undefined : provider,
      gateway_key_id: gatewayKey === "all" ? undefined : gatewayKey,
      status_class: statusClass === "all" ? undefined : statusClass,
      directive: directiveDebounced || undefined,
    }),
    [provider, gatewayKey, statusClass, directiveDebounced],
  );

  const logs = useRequestLogs(query, { refetchInterval: live ? 3000 : false });
  const items = logs.data?.items ?? [];
  const total = logs.data?.total ?? 0;
  const size = logs.data?.size ?? 0;
  const capacity = logs.data?.capacity ?? 0;

  const keyLabel = useMemo(() => {
    const map = new Map<string, string>();
    for (const k of keys.data) {
      map.set(k.id, k.label || k.prefix || shortId(k.id));
    }
    return (id: string) => {
      if (!id) return "—";
      return map.get(id) ?? shortId(id);
    };
  }, [keys.data]);

  const providerSlugs = useMemo(() => {
    const set = new Set(providers.data.map((p) => p.slug).filter(Boolean));
    for (const row of items) {
      if (row.resolved_provider_slug) set.add(row.resolved_provider_slug);
    }
    return Array.from(set).sort();
  }, [providers.data, items]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Request logs"
        description={
          capacity
            ? `Ring buffer ${formatNumber(size)} / ${formatNumber(capacity)} · ${formatNumber(total)} match filters`
            : "In-memory request stream"
        }
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/logs"><LayoutDashboard className="size-4" /> Dashboard</Link>
            </Button>
            <ConfirmDialog
              trigger={(open) => (
                <Button variant="outline" onClick={open} disabled={size === 0}>
                  <Trash2 className="size-4" /> Clear
                </Button>
              )}
              title="Clear request log buffer?"
              description="This empties the in-memory ring buffer. Historical data cannot be recovered."
              confirmLabel="Clear buffer"
              onConfirm={async () => {
                try {
                  await clear.mutateAsync();
                  setSelected(null);
                  toast.success("Request log cleared");
                } catch (e) {
                  toast.error(e instanceof ApiError ? e.message : "Clear failed");
                }
              }}
            />
          </div>
        }
      />

      <div className="flex flex-col gap-3 rounded-lg border border-border bg-card/40 p-3 lg:flex-row lg:items-end">
        <div className="grid flex-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <FilterField label="Provider">
            <Select value={provider} onValueChange={setProvider}>
              <SelectTrigger><SelectValue placeholder="All providers" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All providers</SelectItem>
                {providerSlugs.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FilterField>
          <FilterField label="Gateway key">
            <Select value={gatewayKey} onValueChange={setGatewayKey}>
              <SelectTrigger><SelectValue placeholder="All keys" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All keys</SelectItem>
                {keys.data.map((k) => (
                  <SelectItem key={k.id} value={k.id}>
                    {k.label || k.prefix || shortId(k.id)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FilterField>
          <FilterField label="Status">
            <Select value={statusClass} onValueChange={setStatusClass}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {STATUS_OPTS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FilterField>
          <FilterField label="Directive">
            <div className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-8"
                value={directive}
                onChange={(e) => setDirective(e.target.value)}
                placeholder="alias or slug:model"
              />
            </div>
          </FilterField>
        </div>
        <div className="flex items-center gap-3 lg:pb-0.5">
          <div className="flex items-center gap-2 rounded-md border border-border px-3 py-1.5">
            <Radio className={`size-3.5 ${live ? "text-success" : "text-muted-foreground"}`} />
            <Label htmlFor="live-tail" className="text-xs text-muted-foreground">Live tail</Label>
            <Switch id="live-tail" checked={live} onCheckedChange={setLive} />
          </div>
          <Button
            variant="outline"
            size="icon"
            onClick={() => logs.refetch()}
            disabled={logs.isFetching}
            title="Refresh"
          >
            <RefreshCw className={`size-4 ${logs.isFetching ? "animate-spin" : ""}`} />
          </Button>
        </div>
      </div>

      {logs.isLoading ? (
        <div className="flex items-center justify-center py-16"><Spinner className="size-6" /></div>
      ) : items.length === 0 ? (
        <EmptyState
          title="No requests"
          hint="Traffic to /v1/* will appear here. Toggle live tail to auto-refresh."
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-border">
          <div className="overflow-x-auto scrollbar-thin">
            <table className="w-full min-w-[900px] text-sm">
              <thead className="border-b border-border bg-card/60 text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 text-left font-medium">Time</th>
                  <th className="px-3 py-2 text-left font-medium">Status</th>
                  <th className="px-3 py-2 text-left font-medium">Directive</th>
                  <th className="px-3 py-2 text-left font-medium">Resolved</th>
                  <th className="px-3 py-2 text-right font-medium">Duration</th>
                  <th className="px-3 py-2 text-right font-medium">Tokens</th>
                  <th className="px-3 py-2 text-left font-medium">Key</th>
                  <th className="px-3 py-2 text-left font-medium">Flags</th>
                </tr>
              </thead>
              <tbody>
                {items.map((row) => {
                  const tokens = row.tokens_prompt + row.tokens_completion;
                  const active = selected?.id === row.id;
                  return (
                    <tr
                      key={row.id}
                      onClick={() => setSelected(row)}
                      className={`cursor-pointer border-b border-border/50 transition-colors hover:bg-accent/40 ${
                        active ? "bg-accent/60" : ""
                      }`}
                    >
                      <td className="px-3 py-2 whitespace-nowrap text-xs text-muted-foreground" title={new Date(row.ts).toLocaleString()}>
                        {formatRelativeMs(row.ts)}
                      </td>
                      <td className="px-3 py-2">
                        <Badge variant={statusVariant(row.status)}>{row.status || "—"}</Badge>
                      </td>
                      <td className="px-3 py-2">
                        <div className="max-w-[220px] truncate font-mono text-xs" title={row.directive}>
                          {row.directive || <span className="text-muted-foreground">{row.path}</span>}
                        </div>
                      </td>
                      <td className="px-3 py-2">
                        <div className="max-w-[180px] truncate font-mono text-xs text-muted-foreground" title={`${row.resolved_provider_slug}:${row.resolved_model}`}>
                          {row.resolved_provider_slug
                            ? `${row.resolved_provider_slug}:${row.resolved_model}`
                            : "—"}
                        </div>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums text-xs">
                        {formatDurationMs(row.duration_ms)}
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums text-xs text-muted-foreground">
                        {tokens ? formatNumber(tokens) : "—"}
                      </td>
                      <td className="px-3 py-2 text-xs text-muted-foreground">
                        {keyLabel(row.gateway_key_id)}
                      </td>
                      <td className="px-3 py-2">
                        <div className="flex flex-wrap items-center gap-1">
                          {row.stream && <Badge variant="secondary">stream</Badge>}
                          {row.alias_name && <Badge variant="outline">{row.alias_name}</Badge>}
                          {row.error_class && <Badge variant="warning">{row.error_class}</Badge>}
                          {row.attempts > 1 && <Badge variant="secondary">{row.attempts}×</Badge>}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {total > items.length && (
            <div className="border-t border-border px-3 py-2 text-xs text-muted-foreground">
              Showing {items.length} of {total} matching requests
            </div>
          )}
        </div>
      )}

      <RequestLogSheet
        log={selected}
        open={!!selected}
        onOpenChange={(v) => { if (!v) setSelected(null); }}
        keyLabel={keyLabel}
      />
    </div>
  );
}

function FilterField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      {children}
    </div>
  );
}
