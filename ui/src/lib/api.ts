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
  registration_id: string;
}

export const credentialKeys = [
  "r2_access_key",
  "r2_secret_key",
  "r2_account_id",
  "r2_bucket",
  "polly_key",
  "polly_secret",
  "elevenlabs_api_key",
  "backend_username",
  "backend_password",
  "trafikverket_key",
  "media_server_url",
  "http_username",
  "http_password",
] as const;

export type CredentialKey = (typeof credentialKeys)[number];
export type CredentialState = Record<CredentialKey, boolean>;

export interface EnrollmentRecord {
  request_id: string;
  config_name: string;
  fingerprint: string;
  hostname: string;
  platform: string;
  version: string;
  created_at: number;
  expires_at: number;
  status: "pending" | "approved" | "denied" | "expired";
  decided_at: number | null;
  bridge_id: string | null;
}

export interface BridgeRecord {
  bridge_id: string;
  config_name: string;
  fingerprint: string;
  approved_at: number;
  last_seen: number | null;
  revoked_at: number | null;
  online: boolean;
}

export interface RegistrationsPayload {
  enrollments: EnrollmentRecord[];
  bridges: BridgeRecord[];
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
  credentials: (name: string) =>
    req<{ state: CredentialState }>(
      `/api/configs/${encodeURIComponent(name)}/credentials`,
    ),
  patchCredentials: (
    name: string,
    patch: Partial<Record<CredentialKey, string | null>>,
  ) =>
    req<{ state: CredentialState; hash: string; updated_at: number }>(
      `/api/configs/${encodeURIComponent(name)}/credentials`,
      { method: "PATCH", body: JSON.stringify({ patch }) },
    ),
  rotateRegistration: (name: string) =>
    req<{ registration_id: string }>(
      `/api/configs/${encodeURIComponent(name)}/registration/rotate`,
      { method: "POST" },
    ),
  registrations: () => req<RegistrationsPayload>("/api/registrations"),
  approveRegistration: (requestID: string, fingerprint: string) =>
    req<{ bridge_id: string }>(
      `/api/registrations/${encodeURIComponent(requestID)}/approve`,
      { method: "POST", body: JSON.stringify({ fingerprint }) },
    ),
  denyRegistration: (requestID: string) =>
    req<void>(`/api/registrations/${encodeURIComponent(requestID)}/deny`, {
      method: "POST",
      body: "{}",
    }),
  revokeBridge: (bridgeID: string) =>
    req<void>(`/api/bridges/${encodeURIComponent(bridgeID)}/revoke`, {
      method: "POST",
      body: "{}",
    }),
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
