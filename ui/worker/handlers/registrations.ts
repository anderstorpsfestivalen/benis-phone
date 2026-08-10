import type { Env } from "../lib/auth.ts";
import { badRequest, json, notFound } from "../lib/responses.ts";

interface PendingEnrollment {
  request_id: string;
  config_name: string;
  public_key: string;
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

export async function handleRegistrationsApi(
  req: Request,
  env: Env,
  pathname: string,
): Promise<Response> {
  if (pathname === "/api/registrations" || pathname === "/api/registrations/") {
    if (req.method !== "GET") return badRequest("method not allowed");
    return list(env);
  }
  const decision = pathname.match(
    /^\/api\/registrations\/([0-9a-f-]{36})\/(approve|deny)$/i,
  );
  if (decision) {
    if (req.method !== "POST") return badRequest("method not allowed");
    return decide(req, env, decision[1], decision[2] === "approve");
  }
  const revoke = pathname.match(/^\/api\/bridges\/([0-9a-f-]{36})\/revoke$/i);
  if (revoke) {
    if (req.method !== "POST") return badRequest("method not allowed");
    return revokeBridge(env, revoke[1]);
  }
  return notFound();
}

async function list(env: Env): Promise<Response> {
  const now = Date.now();
  await env.DB.prepare(
    "UPDATE bridge_enrollments SET status = 'expired' WHERE status = 'pending' AND expires_at <= ?",
  )
    .bind(now)
    .run();
  const enrollments = await env.DB.prepare(
    `SELECT request_id, config_name, fingerprint, hostname, platform, version,
            created_at, expires_at, status, decided_at, bridge_id
     FROM bridge_enrollments ORDER BY created_at DESC`,
  ).all<Omit<PendingEnrollment, "public_key">>();
  const bridges = await env.DB.prepare(
    `SELECT bridge_id, config_name, fingerprint, approved_at, last_seen, revoked_at
     FROM bridges ORDER BY approved_at DESC`,
  ).all<{
    bridge_id: string;
    config_name: string;
    fingerprint: string;
    approved_at: number;
    last_seen: number | null;
    revoked_at: number | null;
  }>();
  return json({
    enrollments: enrollments.results ?? [],
    bridges: (bridges.results ?? []).map((bridge) => ({
      ...bridge,
      online:
        bridge.revoked_at === null &&
        bridge.last_seen !== null &&
        bridge.last_seen >= now - 90_000,
    })),
  });
}

async function decide(
  req: Request,
  env: Env,
  requestId: string,
  approve: boolean,
): Promise<Response> {
  const enrollment = await env.DB.prepare(
    "SELECT * FROM bridge_enrollments WHERE request_id = ?",
  )
    .bind(requestId)
    .first<PendingEnrollment>();
  if (!enrollment) return notFound();
  const now = Date.now();
  if (enrollment.status === "pending" && enrollment.expires_at <= now) {
    await env.DB.prepare(
      "UPDATE bridge_enrollments SET status = 'expired' WHERE request_id = ?",
    )
      .bind(requestId)
      .run();
    return badRequest("enrollment has expired");
  }
  if (enrollment.status !== "pending") {
    return badRequest(`enrollment is already ${enrollment.status}`);
  }
  if (!approve) {
    await env.DB.prepare(
      "UPDATE bridge_enrollments SET status = 'denied', decided_at = ? WHERE request_id = ? AND status = 'pending'",
    )
      .bind(now, requestId)
      .run();
    return json({ request_id: requestId, status: "denied" });
  }
  let body: { fingerprint?: string };
  try {
    body = (await req.json()) as { fingerprint?: string };
  } catch {
    return badRequest("invalid json");
  }
  if (body.fingerprint !== enrollment.fingerprint) {
    return badRequest("fingerprint confirmation does not match");
  }
  const bridgeId = crypto.randomUUID();
  await env.DB.batch([
    env.DB.prepare(
      `INSERT INTO bridges
       (bridge_id, config_name, public_key, fingerprint, approved_at)
       VALUES (?, ?, ?, ?, ?)`,
    ).bind(
      bridgeId,
      enrollment.config_name,
      enrollment.public_key,
      enrollment.fingerprint,
      now,
    ),
    env.DB.prepare(
      `UPDATE bridge_enrollments
       SET status = 'approved', decided_at = ?, bridge_id = ?
       WHERE request_id = ? AND status = 'pending'`,
    ).bind(now, bridgeId, requestId),
  ]);
  return json({
    request_id: requestId,
    status: "approved",
    bridge_id: bridgeId,
  });
}

async function revokeBridge(env: Env, bridgeId: string): Promise<Response> {
  const now = Date.now();
  const result = await env.DB.prepare(
    "UPDATE bridges SET revoked_at = ? WHERE bridge_id = ? AND revoked_at IS NULL",
  )
    .bind(now, bridgeId)
    .run();
  if ((result.meta?.changes ?? 0) !== 1)
    return notFound("active bridge not found");
  const broker = env.CONFIG_BROKER.get(env.CONFIG_BROKER.idFromName("global"));
  await broker.fetch(
    `https://broker/revoke?bridge_id=${encodeURIComponent(bridgeId)}`,
    { method: "POST" },
  );
  return json({ bridge_id: bridgeId, revoked_at: now });
}
