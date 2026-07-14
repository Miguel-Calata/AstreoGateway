import type { ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { useHealthz } from "@/lib/queries";
import { useEffect } from "react";

declare global {
  // eslint-disable-next-line no-var
  var __healthzUptime: number | undefined;
}

const TITLES: Record<string, string> = {
  "/": "Overview",
  "/providers": "Providers",
  "/aliases": "Aliases",
  "/gateway-keys": "Gateway Keys",
  "/discovery": "Discovery",
  "/logs": "Logs",
  "/logs/requests": "Request logs",
};

export function Layout({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const healthz = useHealthz();

  useEffect(() => {
    if (healthz.data?.uptime_seconds != null) {
      globalThis.__healthzUptime = healthz.data.uptime_seconds;
    }
  }, [healthz.data]);

  const title =
    TITLES[pathname] ??
    (pathname.startsWith("/providers/")
      ? "Provider"
      : pathname.startsWith("/logs")
        ? "Logs"
        : "AstreoGateway");

  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar title={title} />
        <main className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="mx-auto max-w-6xl px-6 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}