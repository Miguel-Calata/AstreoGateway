// API types mirroring internal/model/model.go and admin endpoints.

export interface Provider {
  id: string;
  name: string;
  slug: string;
  protocol: "openai" | "anthropic" | "gemini";
  base_url: string;
  enabled: boolean;
  headers: Record<string, string>;
}

export interface ApiKey {
  id: string;
  provider_id: string;
  label: string;
  key_value?: string;
  priority: number;
  enabled: boolean;
}

export type RoutingMode = "random" | "round_robin" | "priority" | "failover";

export interface AliasTarget {
  provider_id: string;
  model_name: string;
  position: number;
}

export interface Alias {
  id: string;
  name: string;
  routing: RoutingMode;
  enabled: boolean;
  targets: AliasTarget[];
}

export interface GatewayKey {
  id: string;
  label: string;
  prefix: string;
  enabled: boolean;
  token?: string; // only present right after create
}

export interface AdminUser {
  id: string;
  username: string;
}

export interface BootstrapStatus {
  needed: boolean;
}

export interface DiscoveryModel {
  provider_id: string;
  model_id: string;
  owned_by?: string;
}

export interface ProviderSnapshot {
  models: DiscoveryModel[];
  fetched_at: string;
  error?: string;
  count: number;
}

export type DiscoveryMap = Record<string, ProviderSnapshot>;

export interface StaleTarget {
  alias_id: string;
  alias_name: string;
  provider_id: string;
  model_name: string;
}

export interface Healthz {
  status: string;
  uptime_seconds: number;
}

export interface RequestAttempt {
  provider_slug: string;
  model_name: string;
  key_id: string;
  status: number;
  fail_class: string;
  duration_ms: number;
}

export interface RequestLog {
  id: string;
  request_id: string;
  ts: number;
  gateway_key_id: string;
  method: string;
  path: string;
  directive: string;
  resolved_provider_slug: string;
  resolved_model: string;
  alias_name: string;
  status: number;
  attempts: number;
  duration_ms: number;
  tokens_prompt: number;
  tokens_completion: number;
  stream: boolean;
  error_class: string;
  client_ip: string;
  attempts_detail?: RequestAttempt[];
}

export interface RequestLogsList {
  items: RequestLog[];
  total: number;
  oldest_ts: number;
  capacity: number;
  size: number;
  truncated: boolean;
}

export interface ProviderStat {
  slug: string;
  requests: number;
  tokens: number;
  errors: number;
}

export interface GatewayKeyStat {
  id: string;
  requests: number;
  tokens: number;
}

export interface StatusClassStat {
  class: string;
  count: number;
}

export interface TimeBucket {
  ts: number;
  requests_ok: number;
  requests_client_err: number;
  requests_server_err: number;
  tokens: number;
  tokens_prompt: number;
  tokens_completion: number;
}

export interface RequestLogsStats {
  window: string;
  from: number;
  to: number;
  oldest_ts: number;
  truncated: boolean;
  total_requests: number;
  total_tokens: number;
  total_tokens_prompt: number;
  total_tokens_completion: number;
  error_rate: number;
  p95_duration_ms: number;
  by_provider: ProviderStat[];
  by_gateway_key: GatewayKeyStat[];
  by_status_class: StatusClassStat[];
  ts_buckets: TimeBucket[];
}

export type StatsWindow = "1h" | "24h" | "7d";

export interface RequestLogsQuery {
  from?: number;
  to?: number;
  gateway_key_id?: string;
  provider_slug?: string;
  status_class?: string;
  directive?: string;
  limit?: number;
  offset?: number;
  order?: string;
}

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

const JSON_HEADERS: HeadersInit = { "Content-Type": "application/json" };

async function parse(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    // Backend sometimes emits a raw JSON string via http.Error.
    return { error: text.replace(/^"|"$/g, "") };
  }
}

function extractError(body: unknown, status: number): string {
  if (body && typeof body === "object" && "error" in body) {
    const v = (body as { error: unknown }).error;
    if (typeof v === "string") return v;
  }
  if (typeof body === "string" && body.length) return body;
  return `Request failed (${status})`;
}

export async function apiFetch<T>(
  path: string,
  opts: { method?: string; body?: unknown; signal?: AbortSignal } = {},
): Promise<T> {
  const init: RequestInit = {
    method: opts.method ?? "GET",
    credentials: "same-origin",
    signal: opts.signal,
  };
  if (opts.body !== undefined) {
    init.headers = JSON_HEADERS;
    init.body = JSON.stringify(opts.body);
  }
  const res = await fetch(path, init);
  const body = await parse(res);
  if (!res.ok) {
    const msg = extractError(body, res.status);
    throw new ApiError(res.status, msg);
  }
  return body as T;
}

export const api = {
  // admin/api...
  getBootstrap: () => apiFetch<BootstrapStatus>("/admin/api/bootstrap"),
  bootstrap: (body: { username: string; password: string }) =>
    apiFetch<AdminUser>("/admin/api/bootstrap", { method: "POST", body }),
  login: (body: { username: string; password: string }) =>
    apiFetch<AdminUser>("/admin/api/login", { method: "POST", body }),
  logout: () => apiFetch<{ status: string }>("/admin/api/logout", { method: "POST" }),
  session: () => apiFetch<AdminUser>("/admin/api/session"),

  listProviders: () => apiFetch<Provider[] | null>("/admin/api/providers"),
  createProvider: (b: Provider) => apiFetch<Provider>("/admin/api/providers", { method: "POST", body: b }),
  updateProvider: (id: string, b: Provider) => apiFetch<Provider>(`/admin/api/providers/${id}`, { method: "PUT", body: b }),
  deleteProvider: (id: string) => apiFetch<void>(`/admin/api/providers/${id}`, { method: "DELETE" }),

  listApiKeys: (pid: string) => apiFetch<ApiKey[] | null>(`/admin/api/providers/${pid}/api-keys`),
  createApiKey: (pid: string, b: ApiKey) => apiFetch<ApiKey>(`/admin/api/providers/${pid}/api-keys`, { method: "POST", body: b }),
  updateApiKey: (id: string, b: ApiKey) => apiFetch<ApiKey>(`/admin/api/api-keys/${id}`, { method: "PUT", body: b }),
  deleteApiKey: (id: string) => apiFetch<void>(`/admin/api/api-keys/${id}`, { method: "DELETE" }),

  listAliases: () => apiFetch<Alias[] | null>("/admin/api/aliases"),
  createAlias: (b: Alias) => apiFetch<Alias>("/admin/api/aliases", { method: "POST", body: b }),
  updateAlias: (id: string, b: Alias) => apiFetch<Alias>(`/admin/api/aliases/${id}`, { method: "PUT", body: b }),
  deleteAlias: (id: string) => apiFetch<void>(`/admin/api/aliases/${id}`, { method: "DELETE" }),

  listGatewayKeys: () => apiFetch<GatewayKey[] | null>("/admin/api/gateway-keys"),
  createGatewayKey: (b: { label?: string }) => apiFetch<GatewayKey>("/admin/api/gateway-keys", { method: "POST", body: b }),
  deleteGatewayKey: (id: string) => apiFetch<void>(`/admin/api/gateway-keys/${id}`, { method: "DELETE" }),

  discoveryModels: () => apiFetch<DiscoveryMap>("/admin/api/discovery/models"),
  discoveryStale: () => apiFetch<StaleTarget[] | null>("/admin/api/discovery/stale"),
  refreshProvider: (id: string) =>
    apiFetch<{ status: string; provider: string }>(`/admin/api/discovery/refresh?provider=${encodeURIComponent(id)}`, { method: "POST" }),

  listRequestLogs: (q: RequestLogsQuery = {}) => {
    const params = new URLSearchParams();
    if (q.from) params.set("from", String(q.from));
    if (q.to) params.set("to", String(q.to));
    if (q.gateway_key_id) params.set("gateway_key_id", q.gateway_key_id);
    if (q.provider_slug) params.set("provider_slug", q.provider_slug);
    if (q.status_class) params.set("status_class", q.status_class);
    if (q.directive) params.set("directive", q.directive);
    if (q.limit) params.set("limit", String(q.limit));
    if (q.offset) params.set("offset", String(q.offset));
    if (q.order) params.set("order", q.order);
    const qs = params.toString();
    return apiFetch<RequestLogsList>(`/admin/api/request-logs${qs ? `?${qs}` : ""}`);
  },
  requestLogsStats: (window: StatsWindow = "24h", groupBy?: string) => {
    const params = new URLSearchParams({ window });
    if (groupBy) params.set("group_by", groupBy);
    return apiFetch<RequestLogsStats>(`/admin/api/request-logs/stats?${params}`);
  },
  clearRequestLogs: () => apiFetch<void>("/admin/api/request-logs", { method: "DELETE" }),

  healthz: () => apiFetch<Healthz>("/healthz"),
};