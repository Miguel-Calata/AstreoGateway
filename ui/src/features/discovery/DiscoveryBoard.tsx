import { RefreshCw, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { useDiscovery, useProviders, useStale, useRefreshProvider } from "@/lib/queries";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { EmptyState, PageHeader } from "@/components/PageHeader";
import { formatRelative } from "@/lib/format";

function hasFetchedAt(iso?: string): boolean {
  if (!iso) return false;
  const t = new Date(iso).getTime();
  return Number.isFinite(t) && t > 0;
}

export function DiscoveryBoard() {
  const discovery = useDiscovery();
  const providers = useProviders();
  const stale = useStale();
  const refresh = useRefreshProvider();
  const providerList = providers.data ?? [];

  const refreshAll = async () => {
    for (const p of providerList) {
      try { await refresh.mutateAsync(p.id); }
      catch { /* keep going */ }
    }
    toast.success("Refreshed all providers");
  };

  const refreshOne = async (id: string) => {
    try { await refresh.mutateAsync(id); toast.success("Dispatched refresh"); }
    catch (e) { toast.error(e instanceof ApiError ? e.message : "Refresh failed"); }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Discovery"
        description="Model sets advertised by each provider. Stale targets fall out of rotation automatically."
        actions={
          <Button variant="outline" onClick={refreshAll} disabled={refresh.isPending || providerList.length === 0}>
            <RefreshCw className={`size-4 ${refresh.isPending ? "animate-spin" : ""}`} /> Refresh all
          </Button>
        }
      />

      {stale.length > 0 && (
        <div className="rounded-lg border border-warning/40 bg-warning/10 p-4">
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-warning">
            <AlertTriangle className="size-4" /> {stale.length} stale target{stale.length > 1 ? "s" : ""}
          </div>
          <ul className="space-y-1 text-xs text-muted-foreground">
            {stale.map((s, i) => (
              <li key={i} className="font-mono">
                <span className="text-foreground">{s.alias_name}</span> → {s.provider_id}:{s.model_name}
              </li>
            ))}
          </ul>
        </div>
      )}

      {providerList.length === 0 ? (
        <EmptyState title="Nothing to discover" hint="Add providers first to list their models." />
      ) : discovery.isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2"><div className="h-44 animate-pulse rounded-lg border border-border bg-card" /><div className="h-44 animate-pulse rounded-lg border border-border bg-card" /></div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {providerList.map((p) => {
            const snap = discovery.data?.[p.id];
            const models = snap?.models ?? [];
            return (
              <div key={p.id} className="flex flex-col rounded-lg border border-border bg-card">
                <div className="flex items-center justify-between border-b border-border px-4 py-3">
                  <div>
                    <div className="font-medium">{p.name}</div>
                    <div className="font-mono text-xs text-muted-foreground">{p.id}</div>
                  </div>
                  <Button variant="ghost" size="icon" onClick={() => refreshOne(p.id)} disabled={refresh.isPending && refresh.variables === p.id} title="Refresh">
                    {refresh.isPending && refresh.variables === p.id ? <Spinner /> : <RefreshCw className="size-4" />}
                  </Button>
                </div>
                <div className="flex items-center gap-2 px-4 py-2 text-xs text-muted-foreground">
                  <Badge variant={snap?.error ? "destructive" : snap ? "success" : "secondary"}>
                    {snap?.error ? "error" : snap ? `${models.length} models` : "no data"}
                  </Badge>
                  {hasFetchedAt(snap?.fetched_at) && <span>fetched {formatRelative(snap!.fetched_at)}</span>}
                </div>
                {snap?.error && <div className="px-4 pb-2 text-xs text-destructive">{snap.error}</div>}
                <div className="max-h-56 overflow-y-auto scrollbar-thin px-4 py-2">
                  {models.length > 0 ? (
                    <ul className="space-y-0.5">
                      {models.map((m) => (
                        <li key={m.model_id} className="font-mono text-xs text-muted-foreground">{m.model_id}</li>
                      ))}
                    </ul>
                  ) : (
                    <p className="py-3 text-center text-xs text-muted-foreground">No models cached.</p>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}