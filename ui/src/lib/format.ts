export function formatUptime(seconds: number): string {
  if (!seconds || seconds < 0) return "0s";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function formatRelative(iso: string): string {
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t) || t <= 0) return "—";
  const diff = Date.now() - t;
  const abs = Math.abs(diff);
  const fmt = (v: number, u: string) => `${diff >= 0 ? "" : "in "}${v}${u}${diff >= 0 ? " ago" : ""}`;
  if (abs < 60_000) return "just now";
  if (abs < 3_600_000) return fmt(Math.round(abs / 60_000), "m");
  if (abs < 86_400_000) return fmt(Math.round(abs / 3_600_000), "h");
  return fmt(Math.round(abs / 86_400_000), "d");
}

export function shortId(id: string, n = 6): string {
  if (!id) return "—";
  return id.length <= n + 2 ? id : `${id.slice(0, n)}…`;
}