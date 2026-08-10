import type { Env } from "./auth";

export interface EncryptedSecret {
  version: number;
  iv: string;
  ciphertext: string;
}

export interface SIPSecretRow extends EncryptedSecret {
  config_name: string;
  connection_id: string;
  updated_at: number;
}

export function filterSIPSecrets(
  rows: SIPSecretRow[],
  allowedConnectionIDs: ReadonlySet<string>,
): SIPSecretRow[] {
  return rows.filter((row) => allowedConnectionIDs.has(row.connection_id));
}

export function staleSIPSecrets(
  rows: SIPSecretRow[],
  configuredConnectionIDs: ReadonlySet<string>,
): SIPSecretRow[] {
  return rows.filter((row) => !configuredConnectionIDs.has(row.connection_id));
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

export const CREDENTIAL_KEYS = [
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

export type CredentialKey = (typeof CREDENTIAL_KEYS)[number];
export type CredentialBundle = Record<CredentialKey, string>;

export interface CredentialBundleRow extends EncryptedSecret {
  config_name: string;
  updated_at: number;
}

export function emptyCredentialBundle(): CredentialBundle {
  return Object.fromEntries(
    CREDENTIAL_KEYS.map((key) => [key, ""]),
  ) as CredentialBundle;
}

function credentialAAD(configName: string): Uint8Array {
  return new TextEncoder().encode(`benis-pbx:credentials:v1:${configName}`);
}

export async function encryptCredentialBundle(
  encodedKey: string,
  configName: string,
  bundle: CredentialBundle,
): Promise<EncryptedSecret> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv, additionalData: credentialAAD(configName) },
    await encryptionKey(encodedKey),
    new TextEncoder().encode(JSON.stringify(bundle)),
  );
  return {
    version: 1,
    iv: bytesToBase64(iv),
    ciphertext: bytesToBase64(new Uint8Array(ciphertext)),
  };
}

export async function decryptCredentialBundle(
  encodedKey: string,
  row: CredentialBundleRow,
): Promise<CredentialBundle> {
  if (row.version !== 1)
    throw new Error(`unsupported credential bundle version ${row.version}`);
  const cleartext = await crypto.subtle.decrypt(
    {
      name: "AES-GCM",
      iv: base64ToBytes(row.iv),
      additionalData: credentialAAD(row.config_name),
    },
    await encryptionKey(encodedKey),
    base64ToBytes(row.ciphertext),
  );
  const decoded = JSON.parse(new TextDecoder().decode(cleartext)) as Record<
    string,
    unknown
  >;
  const bundle = emptyCredentialBundle();
  for (const key of CREDENTIAL_KEYS) {
    if (typeof decoded[key] === "string") bundle[key] = decoded[key];
  }
  return bundle;
}

export async function getCredentialBundle(
  env: Env,
  configName: string,
): Promise<CredentialBundle> {
  const row = await env.DB.prepare(
    "SELECT config_name, version, iv, ciphertext, updated_at FROM credential_bundles WHERE config_name = ?",
  )
    .bind(configName)
    .first<CredentialBundleRow>();
  return row
    ? decryptCredentialBundle(env.SIP_SECRET_ENCRYPTION_KEY, row)
    : emptyCredentialBundle();
}

export function credentialState(
  bundle: CredentialBundle,
): Record<CredentialKey, boolean> {
  return Object.fromEntries(
    CREDENTIAL_KEYS.map((key) => [key, bundle[key] !== ""]),
  ) as Record<CredentialKey, boolean>;
}

export function putCredentialBundleStatement(
  env: Env,
  row: CredentialBundleRow,
): D1PreparedStatement {
  return env.DB.prepare(
    `INSERT INTO credential_bundles (config_name, version, iv, ciphertext, updated_at)
     VALUES (?, ?, ?, ?, ?)
     ON CONFLICT(config_name) DO UPDATE SET
       version = excluded.version, iv = excluded.iv,
       ciphertext = excluded.ciphertext, updated_at = excluded.updated_at`,
  ).bind(row.config_name, row.version, row.iv, row.ciphertext, row.updated_at);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  return Uint8Array.from(binary, (c) => c.charCodeAt(0));
}

async function encryptionKey(encoded: string): Promise<CryptoKey> {
  if (!encoded) throw new Error("SIP_SECRET_ENCRYPTION_KEY is not configured");
  const raw = base64ToBytes(encoded);
  if (raw.byteLength !== 32) {
    throw new Error(
      "SIP_SECRET_ENCRYPTION_KEY must be a base64-encoded 32-byte key",
    );
  }
  return crypto.subtle.importKey("raw", raw, "AES-GCM", false, [
    "encrypt",
    "decrypt",
  ]);
}

function aad(configName: string, connectionID: string): Uint8Array {
  return new TextEncoder().encode(
    `benis-pbx:sip:v1:${configName}:${connectionID}`,
  );
}

export async function encryptSIPPassword(
  encodedKey: string,
  configName: string,
  connectionID: string,
  password: string,
): Promise<EncryptedSecret> {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv, additionalData: aad(configName, connectionID) },
    await encryptionKey(encodedKey),
    new TextEncoder().encode(password),
  );
  return {
    version: 1,
    iv: bytesToBase64(iv),
    ciphertext: bytesToBase64(new Uint8Array(ciphertext)),
  };
}

export async function decryptSIPPassword(
  encodedKey: string,
  row: Pick<
    SIPSecretRow,
    "config_name" | "connection_id" | "version" | "iv" | "ciphertext"
  >,
): Promise<string> {
  if (row.version !== 1)
    throw new Error(`unsupported SIP secret version ${row.version}`);
  const cleartext = await crypto.subtle.decrypt(
    {
      name: "AES-GCM",
      iv: base64ToBytes(row.iv),
      additionalData: aad(row.config_name, row.connection_id),
    },
    await encryptionKey(encodedKey),
    base64ToBytes(row.ciphertext),
  );
  return new TextDecoder().decode(cleartext);
}

export async function listSIPSecrets(
  env: Env,
  configName: string,
): Promise<SIPSecretRow[]> {
  const result = await env.DB.prepare(
    "SELECT config_name, connection_id, version, iv, ciphertext, updated_at FROM sip_secrets WHERE config_name = ?",
  )
    .bind(configName)
    .all<SIPSecretRow>();
  return result.results ?? [];
}

export async function decryptedSIPPasswords(
  env: Env,
  configName: string,
  allowedConnectionIDs?: ReadonlySet<string>,
): Promise<Record<string, string>> {
  const stored = await listSIPSecrets(env, configName);
  const rows =
    allowedConnectionIDs === undefined
      ? stored
      : filterSIPSecrets(stored, allowedConnectionIDs);
  const entries = await Promise.all(
    rows.map(
      async (row) =>
        [
          row.connection_id,
          await decryptSIPPassword(env.SIP_SECRET_ENCRYPTION_KEY, row),
        ] as const,
    ),
  );
  return Object.fromEntries(entries);
}

export function putSIPSecretStatement(
  env: Env,
  row: SIPSecretRow,
): D1PreparedStatement {
  return env.DB.prepare(
    `INSERT INTO sip_secrets (config_name, connection_id, version, iv, ciphertext, updated_at)
     VALUES (?, ?, ?, ?, ?, ?)
     ON CONFLICT(config_name, connection_id) DO UPDATE SET
       version = excluded.version, iv = excluded.iv,
       ciphertext = excluded.ciphertext, updated_at = excluded.updated_at`,
  ).bind(
    row.config_name,
    row.connection_id,
    row.version,
    row.iv,
    row.ciphertext,
    row.updated_at,
  );
}

export function deleteSIPSecretStatement(
  env: Env,
  configName: string,
  connectionID?: string,
): D1PreparedStatement {
  if (connectionID !== undefined) {
    return env.DB.prepare(
      "DELETE FROM sip_secrets WHERE config_name = ? AND connection_id = ?",
    ).bind(configName, connectionID);
  }
  return env.DB.prepare("DELETE FROM sip_secrets WHERE config_name = ?").bind(
    configName,
  );
}
