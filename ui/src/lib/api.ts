import type { Definition } from "../generated/config";

export interface ConfigSummary {
  name: string;
  hash: string;
  updated_at: number;
  created_at: number;
}

export interface ConfigPayload {
  name: string;
  doc: Definition;
  toml: string;
  hash: string;
  updated_at: number;
  created_at: number;
  sip_secret_state: Record<string, boolean>;
}

export interface SIPStatusEvent {
  connection_id: string;
  state: string;
  code?: string;
  message?: string;
  local_port?: number;
  at: string;
}

export interface RuntimeSIPStatus {
  instance_id: string;
  last_seen: string;
  connections: Array<{ current: SIPStatusEvent; events: SIPStatusEvent[] }>;
}

export interface SIPStatusSnapshot {
  instances: RuntimeSIPStatus[];
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText}: ${text}`);
  }
  if (res.status === 204) return undefined as unknown as T;
  return res.json() as Promise<T>;
}

export const api = {
  list: () => req<ConfigSummary[]>("/api/configs"),
  get: (name: string) =>
    req<ConfigPayload>(`/api/configs/${encodeURIComponent(name)}`),
  save: (
    name: string,
    doc: Definition,
    toml: string,
    sipSecrets: Record<string, string | null> = {},
    draft = false,
  ) =>
    req<ConfigPayload>(`/api/configs/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ doc, toml, sip_secrets: sipSecrets, draft }),
    }),
  duplicate: (from: string, to: string) =>
    req<ConfigPayload>(`/api/configs/${encodeURIComponent(from)}/duplicate`, {
      method: "POST",
      body: JSON.stringify({ name: to }),
    }),
  remove: (name: string) =>
    req<void>(`/api/configs/${encodeURIComponent(name)}`, { method: "DELETE" }),
  sipStatus: (name: string) =>
    req<SIPStatusSnapshot>(
      `/api/configs/${encodeURIComponent(name)}/sip-status`,
    ),
  previewGenericJSON: (payload: {
    url: string;
    method?: string;
    body?: string;
    headers?: Record<string, string>;
  }) =>
    req<{
      status: number;
      contentType: string;
      body: string;
      truncated: boolean;
    }>("/api/genericjson/preview", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
