import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Alias, type ApiKey, type DiscoveryMap, type Provider, type RoutingMode } from "./api";

const q = {
  bootstrap: ["bootstrap"] as const,
  session: ["session"] as const,
  providers: ["providers"] as const,
  provider: (id: string) => ["providers", id] as const,
  apiKeys: (pid: string) => ["api-keys", pid] as const,
  aliases: ["aliases"] as const,
  alias: (id: string) => ["aliases", id] as const,
  gatewayKeys: ["gateway-keys"] as const,
  discovery: ["discovery"] as const,
  stale: ["stale"] as const,
  healthz: ["healthz"] as const,
};

// --- Auth ---
export const useBootstrap = () => useQuery({ queryKey: q.bootstrap, queryFn: api.getBootstrap }).data;
export const useBootstrapQuery = () => useQuery({ queryKey: q.bootstrap, queryFn: api.getBootstrap });
export const useSession = () => useQuery({ queryKey: q.session, queryFn: api.session, retry: false });
export const useLogout = () => {
  const qc = useQueryClient();
  return useMutation({ mutationFn: api.logout, onSuccess: () => qc.clear() });
};
export const useLogin = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.login,
    onSuccess: (user) => {
      qc.setQueryData(q.session, user);
    },
  });
};
export const useBootstrapMutation = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.bootstrap,
    onSuccess: (user) => {
      qc.setQueryData(q.bootstrap, { needed: false });
      qc.setQueryData(q.session, user);
    },
  });
};

// --- Providers ---
// initialDataUpdatedAt: 0 → treat as stale immediately so mount still fetches
// (plain initialData + staleTime would skip the network call).
export const useProviders = () =>
  useQuery({
    queryKey: q.providers,
    queryFn: () => api.listProviders().then((d) => d ?? []),
    initialData: [],
    initialDataUpdatedAt: 0,
  });
export const useCreateProvider = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: Provider) => api.createProvider(b),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.providers });
      qc.invalidateQueries({ queryKey: q.discovery });
    },
  });
};
export const useUpdateProvider = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Provider }) => api.updateProvider(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.providers });
      qc.invalidateQueries({ queryKey: q.discovery });
    },
  });
};
export const useDeleteProvider = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteProvider(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.providers });
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      qc.invalidateQueries({ queryKey: q.discovery });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};

// --- API Keys ---
export const useApiKeys = (pid: string, enabled = true) =>
  useQuery({
    queryKey: q.apiKeys(pid),
    queryFn: () => api.listApiKeys(pid).then((d) => d ?? []),
    enabled,
    initialData: [],
    initialDataUpdatedAt: 0,
  });
export const useCreateApiKey = (pid: string) => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: ApiKey) => api.createApiKey(pid, b),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.apiKeys(pid) });
      qc.invalidateQueries({ queryKey: q.discovery });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};
export const useUpdateApiKey = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: ApiKey }) => api.updateApiKey(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      qc.invalidateQueries({ queryKey: q.discovery });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};
export const useDeleteApiKey = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteApiKey(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-keys"] });
      qc.invalidateQueries({ queryKey: q.discovery });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};

// --- Aliases ---
export const useAliases = () =>
  useQuery({
    queryKey: q.aliases,
    queryFn: () => api.listAliases().then((d) => d ?? []),
    initialData: [],
    initialDataUpdatedAt: 0,
  });
export const useCreateAlias = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: Alias) => api.createAlias(b),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.aliases });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};
export const useUpdateAlias = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Alias }) => api.updateAlias(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.aliases });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};
export const useDeleteAlias = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteAlias(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.aliases });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};

// --- Gateway keys ---
export const useGatewayKeys = () =>
  useQuery({
    queryKey: q.gatewayKeys,
    queryFn: () => api.listGatewayKeys().then((d) => d ?? []),
    initialData: [],
    initialDataUpdatedAt: 0,
  });
export const useCreateGatewayKey = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (b: { label?: string }) => api.createGatewayKey(b),
    onSuccess: () => qc.invalidateQueries({ queryKey: q.gatewayKeys }),
  });
};
export const useDeleteGatewayKey = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteGatewayKey(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: q.gatewayKeys }),
  });
};

// --- Discovery ---
export const useDiscovery = () =>
  useQuery({
    queryKey: q.discovery,
    queryFn: async () => {
      const d = await api.discoveryModels();
      const out: DiscoveryMap = {};
      for (const [k, v] of Object.entries(d ?? {})) {
        out[k] = { ...v, models: v.models ?? [] };
      }
      return out;
    },
  });
export const useStale = (pollMs = 60_000) => {
  const r = useQuery({
    queryKey: q.stale,
    queryFn: () => api.discoveryStale().then((d) => d ?? []),
    refetchInterval: pollMs,
    initialData: [],
    initialDataUpdatedAt: 0,
  });
  return r.data ?? [];
};
export const useRefreshProvider = () => {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.refreshProvider(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: q.discovery });
      qc.invalidateQueries({ queryKey: q.stale });
    },
  });
};

// --- Health ---
export const useHealthz = () =>
  useQuery({ queryKey: q.healthz, queryFn: api.healthz, refetchInterval: 30_000, retry: 0 });

export const ROUTING_MODES: RoutingMode[] = ["random", "round_robin", "priority", "failover"];