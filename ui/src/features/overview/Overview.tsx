import { Link } from "react-router-dom";
import { Server, Shuffle, KeyRound, Telescope, AlertTriangle } from "lucide-react";
import { useAliases, useGatewayKeys, useProviders, useStale } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/PageHeader";
import { protocolVariant } from "@/lib/format";

export function Overview() {
  const providers = useProviders();
  const aliases = useAliases();
  const keys = useGatewayKeys();
  const stale = useStale();

  return (
    <div className="space-y-6">
      <PageHeader title="Overview" description="Gateway at a glance." />

      {stale.length > 0 && (
        <Link to="/discovery" className="flex items-center gap-2 rounded-lg border border-warning/40 bg-warning/10 px-4 py-3 text-sm">
          <AlertTriangle className="size-4 text-warning" />
          <span>{stale.length} stale target{stale.length > 1 ? "s" : ""} need attention</span>
        </Link>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard to="/providers" icon={Server} label="Providers" value={providers.data.length} sub="upstream gateways" />
        <StatCard to="/aliases" icon={Shuffle} label="Aliases" value={aliases.data.length} sub="routed names" />
        <StatCard to="/gateway-keys" icon={KeyRound} label="Gateway keys" value={keys.data.length} sub="client tokens" />
        <StatCard to="/discovery" icon={Telescope} label="Stale" value={stale.length} sub="out-of-rotation" warn={stale.length > 0} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle>Providers</CardTitle></CardHeader>
          <CardContent>
            {providers.data.length === 0 ? (
              <p className="text-sm text-muted-foreground">No providers yet. <Link className="text-primary underline" to="/providers">Add one</Link>.</p>
            ) : (
              <ul className="space-y-2 text-sm">
                {providers.data.map((p) => (
                  <li key={p.id} className="flex items-center justify-between">
                    <Link to={`/providers/${p.id}`} className="font-medium hover:text-primary">{p.name}</Link>
                    <div className="flex items-center gap-2">
                      <Badge variant={protocolVariant(p.protocol)}>{p.protocol}</Badge>
                      <Badge variant={p.enabled ? "success" : "secondary"}>{p.enabled ? "on" : "off"}</Badge>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>Aliases</CardTitle></CardHeader>
          <CardContent>
            {aliases.data.length === 0 ? (
              <p className="text-sm text-muted-foreground">No aliases yet. <Link className="text-primary underline" to="/aliases">Create one</Link>.</p>
            ) : (
              <ul className="space-y-2 text-sm">
                {aliases.data.map((a) => (
                  <li key={a.id} className="flex items-center justify-between">
                    <span className="font-mono">{a.name}</span>
                    <div className="flex items-center gap-2">
                      <Badge variant="secondary">{a.routing}</Badge>
                      <span className="text-xs text-muted-foreground">{a.targets?.length ?? 0} targets</span>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function StatCard({ to, icon: Icon, label, value, sub, warn }: {to: string; icon: typeof Server; label: string; value: number; sub: string; warn?: boolean}) {
  return (
    <Link to={to}>
      <Card className="transition-colors hover:border-primary/40">
        <CardContent className="flex items-center justify-between p-5">
          <div>
            <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
            <div className={`mt-1 text-2xl font-semibold ${warn && value > 0 ? "text-warning" : ""}`}>{value}</div>
            <div className="text-xs text-muted-foreground">{sub}</div>
          </div>
          <Icon className="size-8 text-muted-foreground/40" />
        </CardContent>
      </Card>
    </Link>
  );
}