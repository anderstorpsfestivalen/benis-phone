import type { Env } from "./auth.ts";
import { checkAccess } from "./auth.ts";
import { sha256Hex } from "./hash.ts";
import { badRequest, json, notFound, unauthorized } from "./responses.ts";

const SCOPES = [
  "config:read",
  "config:write",
  "status:read",
  "history:read",
] as const;
const DEFAULT_SCOPE = SCOPES.join(" ");
const AUTHORIZE_TTL_MS = 10 * 60 * 1000;
const CODE_TTL_MS = 5 * 60 * 1000;
const ACCESS_TTL_MS = 60 * 60 * 1000;
const REFRESH_TTL_MS = 30 * 24 * 60 * 60 * 1000;
const MAX_CLIENTS_PER_HOUR = 100;
const CSRF_COOKIE = "benis_mcp_oauth_csrf";

interface OAuthClientRow {
  client_id: string;
  client_name: string;
  redirect_uris: string;
  created_at: number;
}

interface AuthorizationRequestRow {
  request_id: string;
  client_id: string;
  redirect_uri: string;
  state: string | null;
  code_challenge: string;
  resource: string;
  scope: string;
  csrf_hash: string;
  expires_at: number;
  status: "pending" | "approved" | "denied" | "expired";
}

interface TokenRow {
  token_hash: string;
  grant_id: string;
  kind: "access" | "refresh";
  expires_at: number;
  token_revoked_at: number | null;
  client_id: string;
  client_name: string;
  access_identity: string;
  scope: string;
  grant_revoked_at: number | null;
}

export interface OAuthActor {
  grantId: string;
  clientId: string;
  clientName: string;
  accessIdentity: string;
  scopes: string[];
  expiresAt: number;
}

export function isOAuthPath(pathname: string): boolean {
  return (
    pathname === "/.well-known/oauth-protected-resource" ||
    pathname === "/.well-known/oauth-protected-resource/mcp" ||
    pathname === "/.well-known/oauth-authorization-server" ||
    pathname === "/.well-known/oauth-authorization-server/mcp" ||
    pathname.startsWith("/oauth/")
  );
}

export async function handleOAuth(
  req: Request,
  env: Env,
  pathname: string,
): Promise<Response> {
  const origin = new URL(req.url).origin;
  if (
    pathname === "/.well-known/oauth-protected-resource" ||
    pathname === "/.well-known/oauth-protected-resource/mcp"
  ) {
    return json({
      resource: `${origin}/mcp`,
      authorization_servers: [origin],
      scopes_supported: [...SCOPES],
      bearer_methods_supported: ["header"],
      resource_name: "ATP IVR live configuration",
    });
  }
  if (
    pathname === "/.well-known/oauth-authorization-server" ||
    pathname === "/.well-known/oauth-authorization-server/mcp"
  ) {
    return json({
      issuer: origin,
      authorization_endpoint: `${origin}/oauth/authorize`,
      token_endpoint: `${origin}/oauth/token`,
      registration_endpoint: `${origin}/oauth/register`,
      revocation_endpoint: `${origin}/oauth/revoke`,
      response_types_supported: ["code"],
      grant_types_supported: ["authorization_code", "refresh_token"],
      code_challenge_methods_supported: ["S256"],
      token_endpoint_auth_methods_supported: ["none"],
      scopes_supported: [...SCOPES],
    });
  }
  if (pathname === "/oauth/register") {
    if (req.method !== "POST") return methodNotAllowed();
    return registerClient(req, env);
  }
  if (pathname === "/oauth/authorize") {
    if (!checkAccess(req)) return unauthorized();
    if (req.method === "GET") return authorizationPage(req, env);
    if (req.method === "POST") return decideAuthorization(req, env);
    return methodNotAllowed();
  }
  if (pathname === "/oauth/token") {
    if (req.method !== "POST") return methodNotAllowed();
    return exchangeToken(req, env);
  }
  if (pathname === "/oauth/revoke") {
    if (req.method !== "POST") return methodNotAllowed();
    return revokeToken(req, env);
  }
  return notFound();
}

export async function authenticateOAuthBearer(
  req: Request,
  env: Env,
): Promise<OAuthActor | null> {
  const authorization = req.headers.get("Authorization") ?? "";
  const match = authorization.match(/^Bearer ([A-Za-z0-9_-]{32,256})$/);
  if (!match) return null;
  const tokenHash = await sha256Hex(match[1]);
  const row = await env.DB.prepare(
    `SELECT t.token_hash, t.grant_id, t.kind, t.expires_at,
            t.revoked_at AS token_revoked_at,
            g.client_id, g.access_identity, g.scope,
            g.revoked_at AS grant_revoked_at,
            c.client_name
     FROM oauth_tokens t
     JOIN oauth_grants g ON g.grant_id = t.grant_id
     JOIN oauth_clients c ON c.client_id = g.client_id
     WHERE t.token_hash = ? AND t.kind = 'access'`,
  )
    .bind(tokenHash)
    .first<TokenRow>();
  const now = Date.now();
  if (
    !row ||
    row.token_revoked_at !== null ||
    row.grant_revoked_at !== null ||
    row.expires_at <= now
  )
    return null;
  await env.DB.prepare(
    "UPDATE oauth_grants SET last_used_at = ? WHERE grant_id = ?",
  )
    .bind(now, row.grant_id)
    .run();
  return {
    grantId: row.grant_id,
    clientId: row.client_id,
    clientName: row.client_name,
    accessIdentity: row.access_identity,
    scopes: row.scope.split(/\s+/).filter(Boolean),
    expiresAt: row.expires_at,
  };
}

export function mcpUnauthorized(req: Request): Response {
  const origin = new URL(req.url).origin;
  const response = json({ error: "unauthorized" }, 401);
  response.headers.set(
    "WWW-Authenticate",
    `Bearer resource_metadata="${origin}/.well-known/oauth-protected-resource/mcp", scope="${DEFAULT_SCOPE}"`,
  );
  return response;
}

export async function handleAgentsApi(
  req: Request,
  env: Env,
  pathname: string,
): Promise<Response> {
  if (pathname === "/api/agents" || pathname === "/api/agents/") {
    if (req.method !== "GET") return methodNotAllowed();
    const result = await env.DB.prepare(
      `SELECT g.grant_id, g.client_id, c.client_name, g.access_identity,
              g.scope, g.created_at, g.last_used_at, g.revoked_at
       FROM oauth_grants g JOIN oauth_clients c ON c.client_id = g.client_id
       ORDER BY g.created_at DESC`,
    ).all<{
      grant_id: string;
      client_id: string;
      client_name: string;
      access_identity: string;
      scope: string;
      created_at: number;
      last_used_at: number | null;
      revoked_at: number | null;
    }>();
    return json(
      (result.results ?? []).map((item) => ({
        ...item,
        scopes: item.scope.split(/\s+/).filter(Boolean),
        scope: undefined,
      })),
    );
  }
  const revoke = pathname.match(/^\/api\/agents\/([0-9a-f-]{36})\/revoke$/i);
  if (revoke) {
    if (req.method !== "POST") return methodNotAllowed();
    const now = Date.now();
    const result = await env.DB.prepare(
      "UPDATE oauth_grants SET revoked_at = ? WHERE grant_id = ? AND revoked_at IS NULL",
    )
      .bind(now, revoke[1])
      .run();
    if ((result.meta?.changes ?? 0) === 0)
      return notFound("active agent grant not found");
    await env.DB.prepare(
      "UPDATE oauth_tokens SET revoked_at = ? WHERE grant_id = ? AND revoked_at IS NULL",
    )
      .bind(now, revoke[1])
      .run();
    return json({ ok: true, revoked_at: now });
  }
  return notFound();
}

export function accessIdentity(req: Request): string {
  return (
    req.headers.get("Cf-Access-Authenticated-User-Email") ??
    "cloudflare-access-user"
  ).slice(0, 255);
}

async function registerClient(req: Request, env: Env): Promise<Response> {
  let input: Record<string, unknown>;
  try {
    input = (await req.json()) as Record<string, unknown>;
  } catch {
    return oauthError("invalid_client_metadata", "invalid JSON", 400);
  }
  const redirectUris = input.redirect_uris;
  if (
    !Array.isArray(redirectUris) ||
    redirectUris.length < 1 ||
    redirectUris.length > 10 ||
    !redirectUris.every(
      (value) => typeof value === "string" && validRedirectURI(value),
    )
  ) {
    return oauthError(
      "invalid_redirect_uri",
      "redirect_uris must contain valid HTTPS or loopback callback URLs",
      400,
    );
  }
  if (
    input.token_endpoint_auth_method !== undefined &&
    input.token_endpoint_auth_method !== "none"
  ) {
    return oauthError(
      "invalid_client_metadata",
      "only public PKCE clients are supported",
      400,
    );
  }
  const now = Date.now();
  const recent = await env.DB.prepare(
    "SELECT count(*) AS count FROM oauth_clients WHERE created_at >= ?",
  )
    .bind(now - 60 * 60 * 1000)
    .first<{ count: number }>();
  if ((recent?.count ?? 0) >= MAX_CLIENTS_PER_HOUR)
    return oauthError(
      "temporarily_unavailable",
      "registration limit reached",
      429,
    );
  const clientID = randomToken("mcp_client");
  const clientName =
    typeof input.client_name === "string" && input.client_name.trim()
      ? input.client_name.trim().slice(0, 128)
      : "MCP client";
  await env.DB.prepare(
    `INSERT INTO oauth_clients (client_id, client_name, redirect_uris, created_at)
     VALUES (?, ?, ?, ?)`,
  )
    .bind(clientID, clientName, JSON.stringify(redirectUris), now)
    .run();
  return json(
    {
      client_id: clientID,
      client_id_issued_at: Math.floor(now / 1000),
      client_name: clientName,
      redirect_uris: redirectUris,
      token_endpoint_auth_method: "none",
      grant_types: ["authorization_code", "refresh_token"],
      response_types: ["code"],
    },
    201,
  );
}

async function authorizationPage(req: Request, env: Env): Promise<Response> {
  const url = new URL(req.url);
  const responseType = url.searchParams.get("response_type");
  const clientID = url.searchParams.get("client_id") ?? "";
  const redirectURI = url.searchParams.get("redirect_uri") ?? "";
  const state = url.searchParams.get("state");
  const challenge = url.searchParams.get("code_challenge") ?? "";
  const challengeMethod = url.searchParams.get("code_challenge_method");
  const origin = url.origin;
  const resource = url.searchParams.get("resource") ?? `${origin}/mcp`;
  const scope = normalizeScope(url.searchParams.get("scope"));
  if (
    responseType !== "code" ||
    !clientID ||
    !redirectURI ||
    !/^[A-Za-z0-9_-]{43,128}$/.test(challenge) ||
    challengeMethod !== "S256" ||
    resource !== `${origin}/mcp` ||
    scope === null
  ) {
    return badRequest("invalid OAuth authorization request");
  }
  const client = await env.DB.prepare(
    "SELECT client_id, client_name, redirect_uris, created_at FROM oauth_clients WHERE client_id = ?",
  )
    .bind(clientID)
    .first<OAuthClientRow>();
  if (
    !client ||
    !(JSON.parse(client.redirect_uris) as string[]).includes(redirectURI)
  )
    return badRequest("unknown client or redirect URI");
  const requestID = crypto.randomUUID();
  const csrf = randomToken("csrf");
  const now = Date.now();
  await env.DB.prepare(
    `INSERT INTO oauth_authorization_requests
     (request_id, client_id, redirect_uri, state, code_challenge, resource,
      scope, csrf_hash, created_at, expires_at, status)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
  )
    .bind(
      requestID,
      clientID,
      redirectURI,
      state,
      challenge,
      resource,
      scope,
      await sha256Hex(csrf),
      now,
      now + AUTHORIZE_TTL_MS,
    )
    .run();
  const response = new Response(
    authorizationHTML(client.client_name, redirectURI, requestID, csrf, scope),
    {
      headers: {
        "Content-Type": "text/html; charset=utf-8",
        "Cache-Control": "no-store",
      },
    },
  );
  response.headers.append(
    "Set-Cookie",
    `${CSRF_COOKIE}=${csrf}; Path=/oauth/authorize; HttpOnly; SameSite=Strict; Max-Age=600${url.protocol === "https:" ? "; Secure" : ""}`,
  );
  return response;
}

async function decideAuthorization(req: Request, env: Env): Promise<Response> {
  const form = await req.formData();
  const requestID = String(form.get("request_id") ?? "");
  const csrf = String(form.get("csrf") ?? "");
  const decision = String(form.get("decision") ?? "");
  const cookie = cookieValue(req.headers.get("Cookie"), CSRF_COOKIE);
  const row = await env.DB.prepare(
    `SELECT request_id, client_id, redirect_uri, state, code_challenge,
            resource, scope, csrf_hash, expires_at, status
     FROM oauth_authorization_requests WHERE request_id = ?`,
  )
    .bind(requestID)
    .first<AuthorizationRequestRow>();
  const now = Date.now();
  if (
    !row ||
    row.status !== "pending" ||
    row.expires_at <= now ||
    !csrf ||
    !cookie ||
    cookie !== csrf ||
    (await sha256Hex(csrf)) !== row.csrf_hash
  ) {
    return badRequest("authorization request expired or invalid");
  }
  const redirect = new URL(row.redirect_uri);
  if (decision !== "approve") {
    await env.DB.prepare(
      "UPDATE oauth_authorization_requests SET status = 'denied', decided_at = ? WHERE request_id = ? AND status = 'pending'",
    )
      .bind(now, requestID)
      .run();
    redirect.searchParams.set("error", "access_denied");
    if (row.state) redirect.searchParams.set("state", row.state);
    return Response.redirect(redirect.toString(), 302);
  }
  const grantID = crypto.randomUUID();
  const code = randomToken("mcp_code");
  await env.DB.batch([
    env.DB.prepare(
      `INSERT INTO oauth_grants
       (grant_id, client_id, access_identity, scope, created_at)
       VALUES (?, ?, ?, ?, ?)`,
    ).bind(grantID, row.client_id, accessIdentity(req), row.scope, now),
    env.DB.prepare(
      `INSERT INTO oauth_codes
       (code_hash, grant_id, client_id, redirect_uri, code_challenge, resource, expires_at)
       VALUES (?, ?, ?, ?, ?, ?, ?)`,
    ).bind(
      await sha256Hex(code),
      grantID,
      row.client_id,
      row.redirect_uri,
      row.code_challenge,
      row.resource,
      now + CODE_TTL_MS,
    ),
    env.DB.prepare(
      `UPDATE oauth_authorization_requests
       SET status = 'approved', decided_at = ?
       WHERE request_id = ? AND status = 'pending'`,
    ).bind(now, requestID),
  ]);
  redirect.searchParams.set("code", code);
  if (row.state) redirect.searchParams.set("state", row.state);
  return Response.redirect(redirect.toString(), 302);
}

async function exchangeToken(req: Request, env: Env): Promise<Response> {
  const form = await req.formData();
  const grantType = String(form.get("grant_type") ?? "");
  if (grantType === "authorization_code") {
    return exchangeAuthorizationCode(form, env);
  }
  if (grantType === "refresh_token") return exchangeRefreshToken(form, env);
  return oauthError("unsupported_grant_type", "unsupported grant_type", 400);
}

async function exchangeAuthorizationCode(form: FormData, env: Env) {
  const code = String(form.get("code") ?? "");
  const clientID = String(form.get("client_id") ?? "");
  const redirectURI = String(form.get("redirect_uri") ?? "");
  const verifier = String(form.get("code_verifier") ?? "");
  if (
    !code ||
    !clientID ||
    !redirectURI ||
    !/^[A-Za-z0-9._~-]{43,128}$/.test(verifier)
  )
    return oauthError("invalid_request", "missing token request fields", 400);
  const codeHash = await sha256Hex(code);
  const row = await env.DB.prepare(
    `SELECT code_hash, grant_id, client_id, redirect_uri, code_challenge,
            resource, expires_at, used_at
     FROM oauth_codes WHERE code_hash = ?`,
  )
    .bind(codeHash)
    .first<{
      code_hash: string;
      grant_id: string;
      client_id: string;
      redirect_uri: string;
      code_challenge: string;
      resource: string;
      expires_at: number;
      used_at: number | null;
    }>();
  const now = Date.now();
  if (
    !row ||
    row.used_at !== null ||
    row.expires_at <= now ||
    row.client_id !== clientID ||
    row.redirect_uri !== redirectURI ||
    (await pkceChallenge(verifier)) !== row.code_challenge
  ) {
    return oauthError("invalid_grant", "authorization code is invalid", 400);
  }
  const consumed = await env.DB.prepare(
    "UPDATE oauth_codes SET used_at = ? WHERE code_hash = ? AND used_at IS NULL",
  )
    .bind(now, codeHash)
    .run();
  if ((consumed.meta?.changes ?? 0) !== 1)
    return oauthError(
      "invalid_grant",
      "authorization code was already used",
      400,
    );
  return issueTokens(env, row.grant_id, now);
}

async function exchangeRefreshToken(form: FormData, env: Env) {
  const refreshToken = String(form.get("refresh_token") ?? "");
  const clientID = String(form.get("client_id") ?? "");
  if (!refreshToken || !clientID)
    return oauthError(
      "invalid_request",
      "refresh_token and client_id are required",
      400,
    );
  const tokenHash = await sha256Hex(refreshToken);
  const row = await lookupToken(env, tokenHash);
  const now = Date.now();
  if (
    !row ||
    row.kind !== "refresh" ||
    row.client_id !== clientID ||
    row.expires_at <= now ||
    row.token_revoked_at !== null ||
    row.grant_revoked_at !== null
  ) {
    return oauthError("invalid_grant", "refresh token is invalid", 400);
  }
  const consumed = await env.DB.prepare(
    "UPDATE oauth_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL",
  )
    .bind(now, tokenHash)
    .run();
  if ((consumed.meta?.changes ?? 0) !== 1)
    return oauthError("invalid_grant", "refresh token was already used", 400);
  return issueTokens(env, row.grant_id, now);
}

async function issueTokens(env: Env, grantID: string, now: number) {
  const grant = await env.DB.prepare(
    "SELECT grant_id, scope, revoked_at FROM oauth_grants WHERE grant_id = ?",
  )
    .bind(grantID)
    .first<{ grant_id: string; scope: string; revoked_at: number | null }>();
  if (!grant || grant.revoked_at !== null)
    return oauthError("invalid_grant", "authorization grant is revoked", 400);
  const accessToken = randomToken("mcp_at");
  const refreshToken = randomToken("mcp_rt");
  await env.DB.batch([
    env.DB.prepare(
      `INSERT INTO oauth_tokens
       (token_hash, grant_id, kind, created_at, expires_at)
       VALUES (?, ?, 'access', ?, ?)`,
    ).bind(await sha256Hex(accessToken), grantID, now, now + ACCESS_TTL_MS),
    env.DB.prepare(
      `INSERT INTO oauth_tokens
       (token_hash, grant_id, kind, created_at, expires_at)
       VALUES (?, ?, 'refresh', ?, ?)`,
    ).bind(await sha256Hex(refreshToken), grantID, now, now + REFRESH_TTL_MS),
  ]);
  const response = json({
    access_token: accessToken,
    token_type: "Bearer",
    expires_in: Math.floor(ACCESS_TTL_MS / 1000),
    refresh_token: refreshToken,
    scope: grant.scope,
  });
  response.headers.set("Cache-Control", "no-store");
  response.headers.set("Pragma", "no-cache");
  return response;
}

async function revokeToken(req: Request, env: Env) {
  const form = await req.formData();
  const token = String(form.get("token") ?? "");
  if (!token) return json({});
  const row = await lookupToken(env, await sha256Hex(token));
  if (row) {
    const now = Date.now();
    await env.DB.batch([
      env.DB.prepare(
        "UPDATE oauth_grants SET revoked_at = ? WHERE grant_id = ? AND revoked_at IS NULL",
      ).bind(now, row.grant_id),
      env.DB.prepare(
        "UPDATE oauth_tokens SET revoked_at = ? WHERE grant_id = ? AND revoked_at IS NULL",
      ).bind(now, row.grant_id),
    ]);
  }
  return json({});
}

async function lookupToken(
  env: Env,
  tokenHash: string,
): Promise<TokenRow | null> {
  return (
    (await env.DB.prepare(
      `SELECT t.token_hash, t.grant_id, t.kind, t.expires_at,
              t.revoked_at AS token_revoked_at,
              g.client_id, g.access_identity, g.scope,
              g.revoked_at AS grant_revoked_at,
              c.client_name
       FROM oauth_tokens t
       JOIN oauth_grants g ON g.grant_id = t.grant_id
       JOIN oauth_clients c ON c.client_id = g.client_id
       WHERE t.token_hash = ?`,
    )
      .bind(tokenHash)
      .first<TokenRow>()) ?? null
  );
}

function normalizeScope(value: string | null): string | null {
  if (!value) return DEFAULT_SCOPE;
  const requested = [...new Set(value.split(/\s+/).filter(Boolean))];
  if (requested.some((scope) => !(SCOPES as readonly string[]).includes(scope)))
    return null;
  return requested.length ? requested.join(" ") : DEFAULT_SCOPE;
}

function validRedirectURI(value: string): boolean {
  try {
    const url = new URL(value);
    if (url.hash || url.username || url.password) return false;
    if (url.protocol === "https:") return true;
    if (url.protocol !== "http:") return false;
    return (
      url.hostname === "127.0.0.1" ||
      url.hostname === "[::1]" ||
      url.hostname === "localhost"
    );
  } catch {
    return false;
  }
}

async function pkceChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    new TextEncoder().encode(verifier),
  );
  return base64url(new Uint8Array(digest));
}

function randomToken(prefix: string): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  return `${prefix}_${base64url(bytes)}`;
}

function base64url(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function cookieValue(header: string | null, name: string): string | null {
  for (const part of (header ?? "").split(";")) {
    const [key, ...rest] = part.trim().split("=");
    if (key === name) return rest.join("=");
  }
  return null;
}

function authorizationHTML(
  clientName: string,
  redirectURI: string,
  requestID: string,
  csrf: string,
  scope: string,
): string {
  const scopes = scope
    .split(" ")
    .map((item) => `<li>${escapeHTML(scopeLabel(item))}</li>`)
    .join("");
  return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Authorize MCP client</title><style>
body{background:#111820;color:#f7fafc;font:15px system-ui;margin:0;min-height:100vh;display:grid;place-items:center}
main{width:min(560px,calc(100% - 32px));background:#202a33;border:1px solid #465563;border-radius:10px;padding:24px;box-sizing:border-box}
h1{font:20px ui-monospace,monospace;margin-top:0}p,li{color:#b8c7d1;line-height:1.5}code{word-break:break-all}.actions{display:flex;gap:10px;justify-content:flex-end;margin-top:24px}
button{border:1px solid #607788;border-radius:6px;padding:9px 16px;background:transparent;color:#fff;cursor:pointer}.approve{background:#607788}</style></head>
<body><main><h1>Authorize ${escapeHTML(clientName)}</h1>
<p>This MCP client will be able to work with all current and future IVR configs. It will never receive passwords, runtime credentials, files, registration IDs, or bridge administration access.</p>
<ul>${scopes}</ul><p>Callback: <code>${escapeHTML(redirectURI)}</code></p>
<form method="post" action="/oauth/authorize"><input type="hidden" name="request_id" value="${escapeHTML(requestID)}"><input type="hidden" name="csrf" value="${escapeHTML(csrf)}">
<div class="actions"><button name="decision" value="deny">Deny</button><button class="approve" name="decision" value="approve">Authorize</button></div></form>
</main></body></html>`;
}

function scopeLabel(scope: string): string {
  return (
    {
      "config:read": "Read non-secret configuration",
      "config:write": "Validate and apply configuration patches",
      "status:read": "Read live SIP status",
      "history:read": "Read configuration change history",
    }[scope] ?? scope
  );
}

function escapeHTML(value: string): string {
  return value.replace(
    /[&<>"']/g,
    (character) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[character] ?? character,
  );
}

function oauthError(error: string, description: string, status: number) {
  const response = json({ error, error_description: description }, status);
  response.headers.set("Cache-Control", "no-store");
  return response;
}

function methodNotAllowed() {
  return json({ error: "method_not_allowed" }, 405);
}
