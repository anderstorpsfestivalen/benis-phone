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
  registeredConnectionIDs: ReadonlySet<string>,
): SIPSecretRow[] {
  return rows.filter((row) => !registeredConnectionIDs.has(row.connection_id));
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
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
