import { Copy, CheckCheck } from "lucide-react";
import { useState } from "react";
import type { RequestLog } from "@/lib/api";
import {
  formatDurationMs,
  formatNumber,
  formatRelativeMs,
  formatTsMs,
  shortId,
  statusVariant,
} from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

export function RequestLogSheet({
  log,
  open,
  onOpenChange,
  keyLabel,
}: {
  log: RequestLog | null;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  keyLabel: (id: string) => string;
}) {
  const [copied, setCopied] = useState(false);

  if (!log) return null;

  const tokens = log.tokens_prompt + log.tokens_completion;
  const attempts = log.attempts_detail ?? [];

  const copyId = () => {
    navigator.clipboard.writeText(log.request_id || log.id);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="overflow-y-auto scrollbar-thin">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2 pr-8">
            <Badge variant={statusVariant(log.status)}>{log.status || "—"}</Badge>
            <span className="truncate font-mono text-sm">{log.directive || log.path}</span>
          </SheetTitle>
          <SheetDescription>
            {formatTsMs(log.ts)} · {formatRelativeMs(log.ts)}
          </SheetDescription>
        </SheetHeader>

        <div className="space-y-6 p-6">
          <section className="grid grid-cols-2 gap-3 text-sm">
            <Field label="Duration" value={formatDurationMs(log.duration_ms)} />
            <Field label="Attempts" value={String(log.attempts || attempts.length || 1)} />
            <Field label="Tokens" value={tokens ? formatNumber(tokens) : "—"} />
            <Field
              label="Prompt / completion"
              value={
                tokens
                  ? `${formatNumber(log.tokens_prompt)} / ${formatNumber(log.tokens_completion)}`
                  : "—"
              }
            />
            <Field label="Stream" value={log.stream ? "yes" : "no"} />
            <Field label="Error class" value={log.error_class || "—"} />
            <Field label="Method" value={log.method} />
            <Field label="Path" value={log.path} mono />
            <Field label="Provider" value={log.resolved_provider_slug || "—"} mono />
            <Field label="Model" value={log.resolved_model || "—"} mono />
            <Field label="Alias" value={log.alias_name || "—"} mono />
            <Field label="Client IP" value={log.client_ip || "—"} mono />
            <Field label="Gateway key" value={keyLabel(log.gateway_key_id)} />
            <div className="col-span-2 space-y-1">
              <div className="text-xs uppercase tracking-wide text-muted-foreground">Request ID</div>
              <div className="flex items-center gap-2">
                <code className="flex-1 truncate rounded-md border border-border bg-background/50 px-2 py-1 font-mono text-xs">
                  {log.request_id || log.id}
                </code>
                <Button type="button" size="icon" variant="outline" onClick={copyId} title="Copy">
                  {copied ? <CheckCheck className="size-4 text-success" /> : <Copy className="size-4" />}
                </Button>
              </div>
            </div>
          </section>

          <section className="space-y-3">
            <div className="text-sm font-medium">Attempts</div>
            {attempts.length === 0 ? (
              <div className="rounded-md border border-dashed border-border px-3 py-6 text-center text-sm text-muted-foreground">
                No attempt detail recorded for this request.
              </div>
            ) : (
              <ol className="space-y-2">
                {attempts.map((a, i) => (
                  <li
                    key={`${a.provider_slug}-${a.key_id}-${i}`}
                    className="rounded-md border border-border bg-background/40 px-3 py-2"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-muted-foreground">#{i + 1}</span>
                        <Badge variant={statusVariant(a.status)}>{a.status || "—"}</Badge>
                        {a.fail_class && (
                          <Badge variant="warning">{a.fail_class}</Badge>
                        )}
                      </div>
                      <span className="text-xs tabular-nums text-muted-foreground">
                        {formatDurationMs(a.duration_ms)}
                      </span>
                    </div>
                    <div className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
                      <span className="text-muted-foreground">Provider</span>
                      <span className="font-mono text-right">{a.provider_slug || "—"}</span>
                      <span className="text-muted-foreground">Model</span>
                      <span className="truncate font-mono text-right">{a.model_name || "—"}</span>
                      <span className="text-muted-foreground">API key</span>
                      <span className="font-mono text-right">{a.key_id ? shortId(a.key_id, 10) : "—"}</span>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </section>
        </div>
      </SheetContent>
    </Sheet>
  );
}

function Field({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="space-y-1">
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className={`truncate text-sm ${mono ? "font-mono text-xs" : ""}`}>{value}</div>
    </div>
  );
}
