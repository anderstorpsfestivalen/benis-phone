import type { Env } from "./auth";

export const BRIDGE_SIGNATURE_VERSION = "v1";
export const BRIDGE_CLOCK_SKEW_SECONDS = 5 * 60;

export interface AuthenticatedBridge {
  bridgeId: string;
  configName: string;
  fingerprint: string;
}

interface BridgeAuthRow {
  bridge_id: string;
  config_name: string;
  public_key: string;
  fingerprint: string;
  revoked_at: number | null;
}

export function canonicalBridgeRequest(
  method: string,
  escapedPath: string,
  bodyHash: string,
  timestamp: string,
  nonce: string,
): string {
  return [
    BRIDGE_SIGNATURE_VERSION,
    method.toUpperCase(),
    escapedPath,
    bodyHash.toLowerCase(),
    timestamp,
    nonce,
  ].join("\n");
}

export async function authenticateBridge(
  req: Request,
  env: Env,
  body = new Uint8Array(),
  nowSeconds = Math.floor(Date.now() / 1000),
): Promise<AuthenticatedBridge | null> {
  const bridgeId = req.headers.get("X-Bridge-ID") ?? "";
  const timestamp = req.headers.get("X-Bridge-Timestamp") ?? "";
  const nonce = req.headers.get("X-Bridge-Nonce") ?? "";
  const signature = req.headers.get("X-Bridge-Signature") ?? "";
  if (
    !/^[0-9a-f-]{36}$/i.test(bridgeId) ||
    !/^\d{1,16}$/.test(timestamp) ||
    !/^[A-Za-z0-9_-]{16,128}$/.test(nonce) ||
    !signature
  ) {
    return null;
  }
  const signedAt = Number(timestamp);
  if (
    !Number.isSafeInteger(signedAt) ||
    Math.abs(nowSeconds - signedAt) > BRIDGE_CLOCK_SKEW_SECONDS
  ) {
    return null;
  }
  const row = await env.DB.prepare(
    `SELECT bridge_id, config_name, public_key, fingerprint, revoked_at
     FROM bridges WHERE bridge_id = ?`,
  )
    .bind(bridgeId)
    .first<BridgeAuthRow>();
  if (!row || row.revoked_at !== null) return null;

  let publicKey: CryptoKey;
  try {
    publicKey = await crypto.subtle.importKey(
      "raw",
      base64ToBytes(row.public_key),
      { name: "Ed25519" },
      false,
      ["verify"],
    );
  } catch {
    return null;
  }
  const bodyHash = await sha256HexBytes(body);
  const canonical = canonicalBridgeRequest(
    req.method,
    new URL(req.url).pathname,
    bodyHash,
    timestamp,
    nonce,
  );
  let valid = false;
  try {
    valid = await crypto.subtle.verify(
      { name: "Ed25519" },
      publicKey,
      base64ToBytes(signature),
      new TextEncoder().encode(canonical),
    );
  } catch {
    return null;
  }
  if (!valid) return null;

  await env.DB.prepare("DELETE FROM bridge_nonces WHERE expires_at < ?")
    .bind(nowSeconds)
    .run();
  const inserted = await env.DB.prepare(
    "INSERT OR IGNORE INTO bridge_nonces (bridge_id, nonce, expires_at) VALUES (?, ?, ?)",
  )
    .bind(bridgeId, nonce, nowSeconds + BRIDGE_CLOCK_SKEW_SECONDS)
    .run();
  if ((inserted.meta?.changes ?? 0) !== 1) return null;
  return {
    bridgeId: row.bridge_id,
    configName: row.config_name,
    fingerprint: row.fingerprint,
  };
}

export async function sha256HexBytes(bytes: Uint8Array): Promise<string> {
  const result = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(result)]
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

export async function fingerprintForPublicKey(
  publicKey: Uint8Array,
): Promise<string> {
  const hex = await sha256HexBytes(publicKey);
  return hex.match(/.{1,2}/g)?.join(":") ?? hex;
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

export function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export async function enrollmentCanonicalBody(value: {
  request_id: string;
  registration_id: string;
  public_key: string;
  hostname: string;
  platform: string;
  version: string;
  timestamp: string;
}): Promise<string> {
  return [
    "benis-phone-enrollment-v1",
    value.request_id,
    value.registration_id,
    value.public_key,
    value.hostname,
    value.platform,
    value.version,
    value.timestamp,
  ].join("\n");
}
