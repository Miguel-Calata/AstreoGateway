import { NavLink } from "react-router-dom";
import {
  LayoutGrid, Server, Shuffle, KeyRound, Telescope, SpaceIcon, ScrollText,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useHealthz, useStale } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";

const NAV = [
  { to: "/", label: "Overview", icon: LayoutGrid, end: true },
  { to: "/providers", label: "Providers", icon: Server },
  { to: "/aliases", label: "Aliases", icon: Shuffle },
  { to: "/gateway-keys", label: "Gateway Keys", icon: KeyRound },
  { to: "/discovery", label: "Discovery", icon: Telescope },
  { to: "/logs", label: "Logs", icon: ScrollText },
];

export function Sidebar() {
  const healthz = useHealthz();
  const stale = useStale();

  return (
    <aside className="flex h-screen w-60 shrink-0 flex-col border-r border-border bg-card/40">
      <div className="flex items-center gap-2 px-5 py-5">
        <div className="grid size-8 place-items-center rounded-md bg-primary/15 text-primary">
          <SpaceIcon className="size-4" />
        </div>
        <div className="leading-tight">
          <div className="text-sm font-semibold tracking-tight">AstreoGateway</div>
          <div className="text-[10px] tracking-widest uppercase text-muted-foreground">Admin Console</div>
        </div>
      </div>

      <nav className="flex-1 space-y-1 px-3 py-2">
        {NAV.map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            end={n.end}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                isActive
                  ? "bg-accent text-foreground"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
              )
            }
          >
            <n.icon className="size-4" />
            <span className="flex-1">{n.label}</span>
            {n.to === "/discovery" && stale.length > 0 && (
              <Badge variant="warning" className="px-1.5 py-0">{stale.length}</Badge>
            )}
          </NavLink>
        ))}
      </nav>

      <div className="flex items-center gap-2 px-5 py-4 text-xs text-muted-foreground">
        <div
          className={cn(
            "size-2 rounded-full",
            healthz.data?.status === "ok" ? "bg-success" : "bg-destructive",
          )}
        />
        <span>{healthz.data?.status === "ok" ? "Gateway live" : "Gateway down"}</span>
      </div>
    </aside>
  );
}