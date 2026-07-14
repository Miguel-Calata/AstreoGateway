import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Activity,
  AlertTriangle,
  Clock3,
  Coins,
  List,
  TrendingUp,
} from "lucide-react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useGatewayKeys, useRequestLogsStats } from "@/lib/queries";
import type { StatsWindow } from "@/lib/api";
import { formatDurationMs, formatNumber, formatPercent, shortId } from "@/lib/format";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const WINDOWS: { value: StatsWindow; label: string }[] = [
  { value: "1h", label: "Last 1h" },
  { value: "24h", label: "Last 24h" },
  { value: "7d", label: "Last 7d" },
];

export function LogsDashboard() {
  const [window, setWindow] = useState<StatsWindow>("24h");
  const stats = useRequestLogsStats(window);
  const keys = useGatewayKeys();
  const data = stats.data;

  const keyLabel = useMemo(() => {
    const map = new Map<string, string>();
    for (const k of keys.data) {
      map.set(k.id, k.label || k.prefix || shortId(k.id));
    }
    return (id: string) => map.get(id) ?? (id === "(none)" ? "—" : shortId(id));
  }, [keys.data]);

  const volumeData = useMemo(() => {
    if (!data?.ts_buckets) return [];
    return data.ts_buckets.map((b) => ({
      ts: b.ts,
      label: formatBucketLabel(b.ts, window),
      ok: b.requests_ok,
      client: b.requests_client_err,
      server: b.requests_server_err,
      tokens: b.tokens,
      prompt: b.tokens_prompt,
      completion: b.tokens_completion,
    }));
  }, [data, window]);

  const totalProviders = data?.by_provider?.reduce((s, p) => s + p.requests, 0) || 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Logs"
        description="Live usage from the in-memory request ring buffer."
        actions={
          <div className="flex items-center gap-2">
            <Select value={window} onValueChange={(v) => setWindow(v as StatsWindow)}>
              <SelectTrigger className="w-[130px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {WINDOWS.map((w) => (
                  <SelectItem key={w.value} value={w.value}>{w.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant="outline" asChild>
              <Link to="/logs/requests"><List className="size-4" /> Requests</Link>
            </Button>
          </div>
        }
      />

      {data?.truncated && (
        <div className="flex items-center gap-2 rounded-lg border border-warning/40 bg-warning/10 px-4 py-3 text-sm">
          <AlertTriangle className="size-4 text-warning" />
          <span>
            Buffer only covers from {data.oldest_ts ? new Date(data.oldest_ts).toLocaleString() : "now"}.
            Older data for this window is not available.
          </span>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <KpiCard
          icon={Activity}
          label="Requests"
          value={stats.isLoading ? null : formatNumber(data?.total_requests ?? 0)}
          sub={windowLabel(window)}
        />
        <KpiCard
          icon={Coins}
          label="Tokens"
          value={stats.isLoading ? null : formatNumber(data?.total_tokens ?? 0)}
          sub={`${formatNumber(data?.total_tokens_prompt ?? 0)} prompt · ${formatNumber(data?.total_tokens_completion ?? 0)} completion`}
        />
        <KpiCard
          icon={TrendingUp}
          label="Error rate"
          value={stats.isLoading ? null : formatPercent(data?.error_rate ?? 0)}
          sub="4xx + 5xx over total"
          warn={(data?.error_rate ?? 0) > 0.05}
        />
        <KpiCard
          icon={Clock3}
          label="p95 latency"
          value={stats.isLoading ? null : formatDurationMs(data?.p95_duration_ms ?? 0)}
          sub="end-to-end gateway time"
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Request volume</CardTitle>
            <Badge variant="secondary">{window}</Badge>
          </CardHeader>
          <CardContent className="h-64 pt-2">
            {stats.isLoading ? (
              <Skeleton className="h-full w-full" />
            ) : volumeData.length === 0 ? (
              <EmptyChart hint="No requests in this window yet." />
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={volumeData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="okFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="hsl(var(--success))" stopOpacity={0.35} />
                      <stop offset="100%" stopColor="hsl(var(--success))" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="clientFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="hsl(var(--warning))" stopOpacity={0.35} />
                      <stop offset="100%" stopColor="hsl(var(--warning))" stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="serverFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="hsl(var(--destructive))" stopOpacity={0.35} />
                      <stop offset="100%" stopColor="hsl(var(--destructive))" stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                  <XAxis dataKey="label" tick={{ fontSize: 11 }} className="text-muted-foreground" />
                  <YAxis allowDecimals={false} tick={{ fontSize: 11 }} width={36} />
                  <Tooltip contentStyle={tooltipStyle} />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Area type="monotone" dataKey="ok" name="OK" stackId="1" stroke="hsl(var(--success))" fill="url(#okFill)" />
                  <Area type="monotone" dataKey="client" name="4xx" stackId="1" stroke="hsl(var(--warning))" fill="url(#clientFill)" />
                  <Area type="monotone" dataKey="server" name="5xx" stackId="1" stroke="hsl(var(--destructive))" fill="url(#serverFill)" />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>Token usage</CardTitle>
            <Badge variant="secondary">{window}</Badge>
          </CardHeader>
          <CardContent className="h-64 pt-2">
            {stats.isLoading ? (
              <Skeleton className="h-full w-full" />
            ) : volumeData.length === 0 ? (
              <EmptyChart hint="No token data yet. Usage appears after chat/embeddings traffic." />
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={volumeData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                  <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                  <YAxis tick={{ fontSize: 11 }} width={40} tickFormatter={(v) => formatNumber(Number(v))} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(v) => formatNumber(Number(v ?? 0))} />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Bar dataKey="prompt" name="Prompt" stackId="t" fill="hsl(var(--primary))" radius={[0, 0, 0, 0]} />
                  <Bar dataKey="completion" name="Completion" stackId="t" fill="hsl(200 80% 55%)" radius={[2, 2, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Top providers</CardTitle></CardHeader>
          <CardContent>
            {stats.isLoading ? (
              <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
            ) : !data?.by_provider?.length ? (
              <p className="text-sm text-muted-foreground">No provider traffic yet.</p>
            ) : (
              <table className="w-full text-sm">
                <thead className="text-xs uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="pb-2 text-left font-medium">Provider</th>
                    <th className="pb-2 text-right font-medium">Reqs</th>
                    <th className="pb-2 text-right font-medium">Tokens</th>
                    <th className="pb-2 text-right font-medium">Errors</th>
                    <th className="pb-2 text-right font-medium">Share</th>
                  </tr>
                </thead>
                <tbody>
                  {data.by_provider.slice(0, 8).map((p) => {
                    const share = totalProviders ? p.requests / totalProviders : 0;
                    return (
                      <tr key={p.slug} className="border-t border-border/60">
                        <td className="py-2 font-mono text-xs">{p.slug}</td>
                        <td className="py-2 text-right tabular-nums">{formatNumber(p.requests)}</td>
                        <td className="py-2 text-right tabular-nums text-muted-foreground">{formatNumber(p.tokens)}</td>
                        <td className="py-2 text-right tabular-nums">
                          {p.errors > 0 ? (
                            <span className="text-destructive">{p.errors}</span>
                          ) : (
                            <span className="text-muted-foreground">0</span>
                          )}
                        </td>
                        <td className="py-2 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted">
                              <div className="h-full bg-primary" style={{ width: `${Math.round(share * 100)}%` }} />
                            </div>
                            <span className="w-10 text-right text-xs tabular-nums text-muted-foreground">
                              {formatPercent(share)}
                            </span>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader><CardTitle>Top gateway keys</CardTitle></CardHeader>
          <CardContent>
            {stats.isLoading ? (
              <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-8 w-full" />)}</div>
            ) : !data?.by_gateway_key?.length ? (
              <p className="text-sm text-muted-foreground">No key traffic yet.</p>
            ) : (
              <table className="w-full text-sm">
                <thead className="text-xs uppercase tracking-wide text-muted-foreground">
                  <tr>
                    <th className="pb-2 text-left font-medium">Key</th>
                    <th className="pb-2 text-right font-medium">Reqs</th>
                    <th className="pb-2 text-right font-medium">Tokens</th>
                  </tr>
                </thead>
                <tbody>
                  {data.by_gateway_key.slice(0, 8).map((k) => (
                    <tr key={k.id} className="border-t border-border/60">
                      <td className="py-2">
                        <div className="font-medium">{keyLabel(k.id)}</div>
                        <div className="font-mono text-[10px] text-muted-foreground">{shortId(k.id, 10)}</div>
                      </td>
                      <td className="py-2 text-right tabular-nums">{formatNumber(k.requests)}</td>
                      <td className="py-2 text-right tabular-nums text-muted-foreground">{formatNumber(k.tokens)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function KpiCard({
  icon: Icon,
  label,
  value,
  sub,
  warn,
}: {
  icon: typeof Activity;
  label: string;
  value: string | null;
  sub: string;
  warn?: boolean;
}) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between p-5">
        <div className="min-w-0">
          <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
          {value == null ? (
            <Skeleton className="mt-2 h-8 w-20" />
          ) : (
            <div className={`mt-1 truncate text-2xl font-semibold tabular-nums ${warn ? "text-warning" : ""}`}>
              {value}
            </div>
          )}
          <div className="mt-0.5 truncate text-xs text-muted-foreground">{sub}</div>
        </div>
        <Icon className="size-8 shrink-0 text-muted-foreground/40" />
      </CardContent>
    </Card>
  );
}

function EmptyChart({ hint }: { hint: string }) {
  return (
    <div className="grid h-full place-items-center text-sm text-muted-foreground">{hint}</div>
  );
}

function windowLabel(w: StatsWindow): string {
  switch (w) {
    case "1h": return "last hour";
    case "7d": return "last 7 days";
    default: return "last 24 hours";
  }
}

function formatBucketLabel(ts: number, window: StatsWindow): string {
  const d = new Date(ts);
  if (window === "7d") {
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
  }
  if (window === "1h") {
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

const tooltipStyle: React.CSSProperties = {
  background: "hsl(var(--popover))",
  border: "1px solid hsl(var(--border))",
  borderRadius: 8,
  fontSize: 12,
};
