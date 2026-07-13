import { useNavigate } from "react-router-dom";
import { LogOut } from "lucide-react";
import { useLogout, useSession } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import { formatUptime } from "@/lib/format";

export function Topbar({ title }: { title: string }) {
  const session = useSession();
  const logout = useLogout();
  const navigate = useNavigate();

  const onLogout = async () => {
    await logout.mutateAsync();
    navigate("/");
    window.location.reload();
  };

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-border bg-background/80 px-6 backdrop-blur">
      <div className="text-sm font-medium tracking-tight">{title}</div>
      <div className="flex items-center gap-4 text-sm text-muted-foreground">
        <span className="hidden font-mono text-xs sm:inline">
          up {formatUptime(globalThis.__healthzUptime ?? 0)}
        </span>
        <span className="text-foreground">{session.data?.username ?? "—"}</span>
        <Button variant="ghost" size="icon" onClick={onLogout} title="Sign out">
          <LogOut className="size-4" />
        </Button>
      </div>
    </header>
  );
}