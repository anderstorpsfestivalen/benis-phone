import type { Env } from "../lib/auth.ts";
import {
  authenticateBridge,
  base64ToBytes,
  enrollmentCanonicalBody,
  fingerprintForPublicKey,
} from "../lib/bridge-auth.ts";
import { getConfig } from "../lib/db.ts";
import { badRequest, json, notFound, unauthorized } from "../lib/responses.ts";
import { decryptedSIPPasswords, getCredentialBundle } from "../lib/secrets.ts";

const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const ENROLLMENT_TTL_MS = 24 * 60 * 60 * 1000;
const MAX_PENDING_PER_CONFIG = 5;

interface EnrollmentBody {
  registration_id: string;
  request_id: string;
  public_key: string;
  hostname: string;
  platform: string;
  version: string;
  timestamp: string;
  signature: string;
}

interface EnrollmentRow {
  request_id: string;
  status: "pending" | "approved" | "denied" | "expired";
  bridge_id: string | null;
  expires_at: number;
}

export async function handleBridge(
  req: Request,
  env: Env,
  pathname: string,
): Promise<Response> {
  if (pathname === "/bridge/enroll") {
    if (req.method !== "POST") return badRequest("method not allowed");
    return enroll(req, env);
  }
  const poll = pathname.match(/^\/bridge\/enroll\/([0-9a-f-]{36})$/i);
  if (poll) {
    if (req.method !== "GET") return badRequest("method not allowed");
    return enrollmentStatus(env, poll[1]);
  }
  if (pathname === "/bridge/runtime") {
    if (req.method !== "GET") return badRequest("method not allowed");
    return runtime(req, env);
  }
  if (pathname === "/bridge/hash") {
    if (req.method !== "GET") return badRequest("method not allowed");
    return runtimeHash(req, env);
  }
  if (pathname === "/bridge/ws") {
    if (req.headers.get("Upgrade") !== "websocket") {
      return badRequest("expected websocket upgrade");
    }
    return runtimeSocket(req, env);
  }
  return notFound();
}

async function enroll(req: Request, env: Env): Promise<Response> {
  let body: EnrollmentBody;
  try {
    body = (await req.json()) as EnrollmentBody;
  } catch {
    return badRequest("invalid json");
  }
  if (!UUID_RE.test(body.registration_id ?? ""))
    return badRequest("invalid registration_id");
  if (!UUID_RE.test(body.request_id ?? ""))
    return badRequest("invalid request_id");
  if (
    typeof body.hostname !== "string" ||
    body.hostname.length < 1 ||
    body.hostname.length > 255 ||
    typeof body.platform !== "string" ||
    body.platform.length < 1 ||
    body.platform.length > 128 ||
    typeof body.version !== "string" ||
    body.version.length < 1 ||
    body.version.length > 128 ||
    !/^\d{1,16}$/.test(body.timestamp ?? "")
  ) {
    return badRequest("invalid enrollment metadata");
  }
  const signedAt = Number(body.timestamp);
  const nowSeconds = Math.floor(Date.now() / 1000);
  if (!Number.isSafeInteger(signedAt) || Math.abs(nowSeconds - signedAt) > 300)
    return unauthorized();

  let rawPublicKey: Uint8Array;
  let rawSignature: Uint8Array;
  try {
    rawPublicKey = base64ToBytes(body.public_key);
    rawSignature = base64ToBytes(body.signature);
  } catch {
    return badRequest("invalid public key or signature encoding");
  }
  if (rawPublicKey.byteLength !== 32 || rawSignature.byteLength !== 64)
    return badRequest("invalid Ed25519 public key or signature");
  try {
    const key = await crypto.subtle.importKey(
      "raw",
      rawPublicKey,
      { name: "Ed25519" },
      false,
      ["verify"],
    );
    const valid = await crypto.subtle.verify(
      { name: "Ed25519" },
      key,
      rawSignature,
      new TextEncoder().encode(await enrollmentCanonicalBody(body)),
    );
    if (!valid) return unauthorized();
  } catch {
    return unauthorized();
  }

  const config = await env.DB.prepare(
    "SELECT name FROM configs WHERE registration_id = ?",
  )
    .bind(body.registration_id)
    .first<{ name: string }>();
  if (!config) return notFound("registration id not found");
  const now = Date.now();
  await env.DB.prepare(
    "UPDATE bridge_enrollments SET status = 'expired' WHERE config_name = ? AND status = 'pending' AND expires_at <= ?",
  )
    .bind(config.name, now)
    .run();

  const active = await env.DB.prepare(
    `SELECT bridge_id FROM bridges
     WHERE config_name = ? AND public_key = ? AND revoked_at IS NULL`,
  )
    .bind(config.name, body.public_key)
    .first<{ bridge_id: string }>();
  if (active) {
    return json({
      request_id: body.request_id,
      status: "approved",
      bridge_id: active.bridge_id,
    });
  }
  const existing = await env.DB.prepare(
    `SELECT request_id, status, bridge_id, expires_at
     FROM bridge_enrollments
     WHERE config_name = ? AND public_key = ?
     ORDER BY created_at DESC LIMIT 1`,
  )
    .bind(config.name, body.public_key)
    .first<EnrollmentRow>();
  if (existing) return json(enrollmentResponse(existing));

  const count = await env.DB.prepare(
    "SELECT count(*) AS count FROM bridge_enrollments WHERE config_name = ? AND status = 'pending'",
  )
    .bind(config.name)
    .first<{ count: number }>();
  if ((count?.count ?? 0) >= MAX_PENDING_PER_CONFIG) {
    return json({ error: "pending enrollment limit reached" }, 429);
  }
  const fingerprint = await fingerprintForPublicKey(rawPublicKey);
  const expiresAt = now + ENROLLMENT_TTL_MS;
  try {
    await env.DB.prepare(
      `INSERT INTO bridge_enrollments
       (request_id, config_name, public_key, fingerprint, hostname, platform,
        version, created_at, expires_at, status)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
    )
      .bind(
        body.request_id,
        config.name,
        body.public_key,
        fingerprint,
        body.hostname,
        body.platform,
        body.version,
        now,
        expiresAt,
      )
      .run();
  } catch {
    const raced = await env.DB.prepare(
      `SELECT request_id, status, bridge_id, expires_at
       FROM bridge_enrollments WHERE config_name = ? AND public_key = ?
       ORDER BY created_at DESC LIMIT 1`,
    )
      .bind(config.name, body.public_key)
      .first<EnrollmentRow>();
    if (raced) return json(enrollmentResponse(raced));
    const afterRaceCount = await env.DB.prepare(
      "SELECT count(*) AS count FROM bridge_enrollments WHERE config_name = ? AND status = 'pending'",
    )
      .bind(config.name)
      .first<{ count: number }>();
    if ((afterRaceCount?.count ?? 0) >= MAX_PENDING_PER_CONFIG) {
      return json({ error: "pending enrollment limit reached" }, 429);
    }
    throw new Error("could not create enrollment");
  }
  return json(
    enrollmentResponse({
      request_id: body.request_id,
      status: "pending",
      bridge_id: null,
      expires_at: expiresAt,
    }),
    202,
  );
}

async function enrollmentStatus(
  env: Env,
  requestId: string,
): Promise<Response> {
  const now = Date.now();
  await env.DB.prepare(
    "UPDATE bridge_enrollments SET status = 'expired' WHERE request_id = ? AND status = 'pending' AND expires_at <= ?",
  )
    .bind(requestId, now)
    .run();
  const row = await env.DB.prepare(
    "SELECT request_id, status, bridge_id, expires_at FROM bridge_enrollments WHERE request_id = ?",
  )
    .bind(requestId)
    .first<EnrollmentRow>();
  if (!row) return notFound();
  return json(enrollmentResponse(row), 200);
}

function enrollmentResponse(row: EnrollmentRow) {
  return {
    request_id: row.request_id,
    status: row.status,
    expires_at: row.expires_at,
    ...(row.status === "approved" && row.bridge_id
      ? { bridge_id: row.bridge_id }
      : {}),
  };
}

async function runtime(req: Request, env: Env): Promise<Response> {
  const bridge = await authenticateBridge(req, env);
  if (!bridge) return unauthorized();
  const row = await getConfig(env, bridge.configName);
  if (!row) return notFound();
  const connectionIDs = configuredConnections(row.doc);
  const response = json({
    revision: row.hash,
    toml: row.toml,
    sip_passwords: await decryptedSIPPasswords(
      env,
      bridge.configName,
      connectionIDs,
    ),
    credentials: runtimeCredentials(
      await getCredentialBundle(env, bridge.configName),
    ),
  });
  response.headers.set("Cache-Control", "no-store");
  return response;
}

async function runtimeHash(req: Request, env: Env): Promise<Response> {
  const bridge = await authenticateBridge(req, env);
  if (!bridge) return unauthorized();
  const row = await getConfig(env, bridge.configName);
  if (!row) return notFound();
  const response = json({ revision: row.hash });
  response.headers.set("Cache-Control", "no-store");
  return response;
}

async function runtimeSocket(req: Request, env: Env): Promise<Response> {
  const bridge = await authenticateBridge(req, env);
  if (!bridge) return unauthorized();
  await env.DB.prepare("UPDATE bridges SET last_seen = ? WHERE bridge_id = ?")
    .bind(Date.now(), bridge.bridgeId)
    .run();
  const brokerURL = new URL(req.url);
  brokerURL.hostname = "broker";
  brokerURL.pathname = "/subscribe";
  brokerURL.search = `?name=${encodeURIComponent(bridge.configName)}&bridge_id=${encodeURIComponent(bridge.bridgeId)}`;
  return env.CONFIG_BROKER.get(env.CONFIG_BROKER.idFromName("global")).fetch(
    new Request(brokerURL, req),
  );
}

function configuredConnections(docJSON: string): Set<string> {
  try {
    const doc = JSON.parse(docJSON) as {
      sip?: { connection?: Array<{ id?: string }> };
    };
    return new Set(
      (doc.sip?.connection ?? [])
        .filter((connection) => !!connection.id)
        .map((connection) => connection.id as string),
    );
  } catch {
    return new Set();
  }
}

function runtimeCredentials(
  bundle: Awaited<ReturnType<typeof getCredentialBundle>>,
) {
  return {
    r2: {
      access_key_id: bundle.r2_access_key,
      secret_access_key: bundle.r2_secret_key,
      account_id: bundle.r2_account_id,
      bucket: bundle.r2_bucket,
    },
    polly: { key: bundle.polly_key, secret: bundle.polly_secret },
    elevenlabs_api_key: bundle.elevenlabs_api_key,
    backend: {
      username: bundle.backend_username,
      password: bundle.backend_password,
    },
    trafikverket_key: bundle.trafikverket_key,
    media_server_url: bundle.media_server_url,
    http_server_auth: {
      username: bundle.http_username,
      password: bundle.http_password,
    },
  };
}
