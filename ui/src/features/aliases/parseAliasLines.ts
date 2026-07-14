import type { Alias, AliasTarget, DiscoveryMap, Provider, RoutingMode } from "@/lib/api";

export const ROUTING_SET = new Set<RoutingMode>(["random", "round_robin", "priority", "failover"]);

export interface ParsedTargetRef {
  slug: string;
  model_name: string;
}

export interface ParsedAliasLine {
  line: number;
  raw: string;
  name: string;
  routing: RoutingMode;
  targets: ParsedTargetRef[];
  parseError?: string;
}

export type ResolveStatus = "ok" | "warn" | "skip" | "error";

export interface ResolvedAliasRow {
  line: number;
  raw: string;
  status: ResolveStatus;
  name?: string;
  routing?: RoutingMode;
  targets?: AliasTarget[];
  targetLabels?: string[];
  messages: string[];
  warnings: string[];
  needsOverride: boolean;
}

export function isRoutingMode(s: string): s is RoutingMode {
  return ROUTING_SET.has(s as RoutingMode);
}

export function parseTargetToken(token: string): ParsedTargetRef | null {
  const i = token.indexOf(":");
  if (i <= 0 || i === token.length - 1) return null;
  return { slug: token.slice(0, i), model_name: token.slice(i + 1) };
}

export function parseAliasText(text: string, defaultRouting: RoutingMode = "failover"): ParsedAliasLine[] {
  const lines = text.split(/\r?\n/);
  const out: ParsedAliasLine[] = [];

  for (let idx = 0; idx < lines.length; idx++) {
    const raw = lines[idx];
    const trimmed = raw.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    const parts = trimmed.split(/\s+/).filter(Boolean);
    if (parts.length === 0) continue;

    const name = parts[0];
    let routing = defaultRouting;
    let targetStart = 1;

    if (parts.length > 1 && isRoutingMode(parts[1])) {
      routing = parts[1];
      targetStart = 2;
    }

    const targets: ParsedTargetRef[] = [];
    let parseError: string | undefined;

    if (name.includes(":")) {
      parseError = "alias name must not contain ':'";
    }

    for (const tok of parts.slice(targetStart)) {
      const t = parseTargetToken(tok);
      if (!t) {
        parseError = parseError ?? `invalid target "${tok}" (expected slug:model)`;
        continue;
      }
      targets.push(t);
    }

    out.push({ line: idx + 1, raw: trimmed, name, routing, targets, parseError });
  }

  return out;
}

export function resolveProviderId(
  ref: string,
  providers: Pick<Provider, "id" | "slug" | "name">[],
): string | undefined {
  const bySlug = providers.find((p) => p.slug === ref);
  if (bySlug) return bySlug.id;
  const byId = providers.find((p) => p.id === ref);
  if (byId) return byId.id;
  return undefined;
}

export function resolveAliasLines(
  text: string,
  opts: {
    defaultRouting?: RoutingMode;
    providers: Pick<Provider, "id" | "slug" | "name">[];
    existingNames: Set<string> | string[];
    discovery?: DiscoveryMap;
    allowUnknownModels?: boolean;
  },
): ResolvedAliasRow[] {
  const defaultRouting = opts.defaultRouting ?? "failover";
  const existing = opts.existingNames instanceof Set
    ? opts.existingNames
    : new Set(opts.existingNames);
  const seenInBatch = new Set<string>();
  const parsed = parseAliasText(text, defaultRouting);

  return parsed.map((row) => {
    const messages: string[] = [];
    const warnings: string[] = [];

    if (row.parseError) {
      return {
        line: row.line,
        raw: row.raw,
        status: "error" as const,
        messages: [row.parseError],
        warnings,
        needsOverride: false,
      };
    }

    if (!row.name) {
      return {
        line: row.line,
        raw: row.raw,
        status: "error" as const,
        messages: ["name is required"],
        warnings,
        needsOverride: false,
      };
    }

    if (existing.has(row.name)) {
      return {
        line: row.line,
        raw: row.raw,
        status: "skip" as const,
        name: row.name,
        routing: row.routing,
        messages: [`alias "${row.name}" already exists`],
        warnings,
        needsOverride: false,
      };
    }

    if (seenInBatch.has(row.name)) {
      return {
        line: row.line,
        raw: row.raw,
        status: "error" as const,
        name: row.name,
        messages: [`duplicate name "${row.name}" in paste`],
        warnings,
        needsOverride: false,
      };
    }
    seenInBatch.add(row.name);

    const targets: AliasTarget[] = [];
    const targetLabels: string[] = [];
    const pairKeys = new Set<string>();

    const allowUnknown = opts.allowUnknownModels ?? false;
    let needsOverride = false;

    for (let i = 0; i < row.targets.length; i++) {
      const t = row.targets[i];
      const providerId = resolveProviderId(t.slug, opts.providers);
      if (!providerId) {
        messages.push(`unknown provider "${t.slug}"`);
        continue;
      }
      const key = `${providerId}\0${t.model_name}`;
      if (pairKeys.has(key)) {
        messages.push(`duplicate target ${t.slug}:${t.model_name}`);
        continue;
      }
      pairKeys.add(key);

      const models = opts.discovery?.[providerId]?.models ?? [];
      if (models.length > 0 && !models.some((m) => m.model_id === t.model_name)) {
        warnings.push(`${t.slug}:${t.model_name} not in discovery`);
        if (!allowUnknown) {
          needsOverride = true;
        }
      }

      targets.push({
        provider_id: providerId,
        model_name: t.model_name,
        position: targets.length,
      });
      targetLabels.push(`${t.slug}:${t.model_name}`);
    }

    if (messages.length > 0) {
      return {
        line: row.line,
        raw: row.raw,
        status: "error" as const,
        name: row.name,
        routing: row.routing,
        messages,
        warnings,
        needsOverride,
      };
    }

    if (targets.length === 0) {
      warnings.push("no targets (alias will 503 until targets are added)");
    }

    const status: ResolveStatus = needsOverride ? "warn" : "ok";

    return {
      line: row.line,
      raw: row.raw,
      status,
      name: row.name,
      routing: row.routing,
      targets,
      targetLabels,
      messages: [],
      warnings,
      needsOverride,
    };
  });
}

export function rowToAlias(row: ResolvedAliasRow): Alias | null {
  if ((row.status !== "ok" && row.status !== "warn") || !row.name || !row.routing) return null;
  return {
    id: "",
    name: row.name,
    routing: row.routing,
    enabled: true,
    targets: row.targets ?? [],
  };
}
