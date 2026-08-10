import { definitionSchema } from "../../src/generated/schemas";
import type { Definition } from "../../src/generated/config";
import { validateSIPDefinition } from "../../src/lib/sip-validation";
import { renderToml } from "../../src/lib/toml-render";
import type { Env } from "../lib/auth";
import { notifyConfigChanged } from "../lib/config-broker";
import {
  ConfigChangeError,
  getConfigChange,
  listConfigChanges,
  rollbackConfigChange,
} from "../lib/config-changes";
import { sha256Hex } from "../lib/hash";
import { getConfig, listConfigs, upsertConfigStatement } from "../lib/db";
import {
  deleteSIPSecretStatement,
  decryptSIPPassword,
  CREDENTIAL_KEYS,
  credentialState,
  encryptCredentialBundle,
  getCredentialBundle,
  encryptSIPPassword,
  listSIPSecrets,
  putSIPSecretStatement,
  putCredentialBundleStatement,
  staleSIPSecrets,
} from "../lib/secrets";
import { accessIdentity } from "../lib/oauth";
import { badRequest, conflict, json, notFound } from "../lib/responses";

const NAME_RE = /^[a-zA-Z0-9_-]{1,64}$/;

export async function handleApi(
  req: Request,
  env: Env,
  pathname: string,
  ctx: ExecutionContext,
): Promise<Response> {
  // pathname is "/api/configs" or "/api/configs/<name>[/...]"
  const rest = pathname.replace(/^\/api\/configs/, "");

  if (rest === "" || rest === "/") {
    if (req.method !== "GET") return badRequest("method not allowed");
    return json(await listConfigs(env));
  }

  const m = rest.match(/^\/([^/]+)(\/.*)?$/);
  if (!m) return notFound();
  const name = decodeURIComponent(m[1]);
  if (!NAME_RE.test(name)) return badRequest("invalid name");
  const sub = m[2] ?? "";

  if (sub === "/duplicate") {
    if (req.method !== "POST") return badRequest("method not allowed");
    return duplicate(req, env, name);
  }
  if (sub === "/sip-status" && req.method === "GET") {
    return statusSnapshot(env, name);
  }
  if (sub === "/sip-status/ws" && req.headers.get("Upgrade") === "websocket") {
    return statusSocket(req, env, name);
  }
  if (sub === "/credentials") {
    if (req.method === "GET") return getCredentials(env, name);
    if (req.method === "PATCH") return patchCredentials(req, env, name, ctx);
    return badRequest("method not allowed");
  }
  if (sub === "/registration") {
    if (req.method === "GET") return getRegistration(env, name);
    return badRequest("method not allowed");
  }
  if (sub === "/registration/rotate") {
    if (req.method === "POST") return rotateRegistration(env, name);
    return badRequest("method not allowed");
  }
  if (sub === "/history") {
    if (req.method !== "GET") return badRequest("method not allowed");
    if (!(await getConfig(env, name))) return notFound();
    return json(await listConfigChanges(env, name));
  }
  const history = sub.match(/^\/history\/([0-9a-f-]{36})(\/rollback)?$/i);
  if (history) {
    if (history[2] === "/rollback") {
      if (req.method !== "POST") return badRequest("method not allowed");
      return rollback(req, env, ctx, name, history[1]);
    }
    if (req.method !== "GET") return badRequest("method not allowed");
    const change = await getConfigChange(env, name, history[1]);
    return change ? json(change) : notFound("config change not found");
  }

  if (sub !== "") return notFound();

  switch (req.method) {
    case "GET":
      return getOne(env, name);
    case "PUT":
      return putOne(req, env, name, ctx);
    case "DELETE":
      return deleteOne(env, name, ctx);
    default:
      return badRequest("method not allowed");
  }
}

function broker(env: Env) {
  return env.CONFIG_BROKER.get(env.CONFIG_BROKER.idFromName("global"));
}

function statusSnapshot(env: Env, name: string): Promise<Response> {
  return broker(env).fetch(
    `https://broker/status?name=${encodeURIComponent(name)}`,
  );
}

function statusSocket(req: Request, env: Env, name: string): Promise<Response> {
  const url = new URL(req.url);
  url.hostname = "broker";
  url.pathname = "/status/subscribe";
  url.search = `?name=${encodeURIComponent(name)}`;
  return broker(env).fetch(new Request(url, req));
}

async function getOne(env: Env, name: string): Promise<Response> {
  const row = await getConfig(env, name);
  if (!row) return notFound();
  return json({
    name: row.name,
    doc: JSON.parse(row.doc),
    toml: row.toml,
    hash: row.hash,
    created_at: row.created_at,
    updated_at: row.updated_at,
    registration_id: row.registration_id,
    sip_secret_state: Object.fromEntries(
      (await listSIPSecrets(env, name)).map((secret) => [
        secret.connection_id,
        true,
      ]),
    ),
  });
}

async function putOne(
  req: Request,
  env: Env,
  name: string,
  ctx: ExecutionContext,
): Promise<Response> {
  let body: {
    doc: unknown;
    toml: string;
    sip_secrets?: Record<string, string | null>;
    draft?: boolean;
    expected_hash?: string;
  };
  try {
    body = (await req.json()) as { doc: unknown; toml: string };
  } catch {
    return badRequest("invalid json");
  }
  if (typeof body.toml !== "string" || !body.toml.trim()) {
    return badRequest("toml is required");
  }
  const parsed = definitionSchema.safeParse(body.doc);
  if (!parsed.success) {
    return badRequest(`doc validation failed: ${parsed.error.message}`);
  }
  const candidate = body.doc as Partial<Definition>;
  if (
    !candidate.sip ||
    !Array.isArray(candidate.sip.connection) ||
    !Array.isArray(candidate.fn) ||
    !candidate.general ||
    !Array.isArray(candidate.queue)
  ) {
    return badRequest(
      "doc must contain general, sip.connection, fn, and queue",
    );
  }
  const doc = candidate as Definition;
  const existing = await getConfig(env, name);
  if (existing && body.expected_hash !== existing.hash) {
    return conflict(
      `config changed since it was loaded; current hash is ${existing.hash}`,
    );
  }
  const newDraft = body.draft === true && existing === null;
  const configErrors = validateSIPDefinition(doc).filter(
    (error) => !(newDraft && error === "Add at least one SIP connection."),
  );
  if (configErrors.length) return badRequest(configErrors.join(" "));
  if (body.toml !== renderToml(doc)) {
    return badRequest("toml does not match the submitted config document");
  }
  const connectionIDs = new Set(
    (doc.sip?.connection ?? []).map((connection) => connection.id),
  );
  if (
    connectionIDs.size !== (doc.sip?.connection ?? []).length ||
    connectionIDs.has("")
  ) {
    return badRequest("every SIP connection requires a unique id");
  }
  for (const [connectionID, password] of Object.entries(
    body.sip_secrets ?? {},
  )) {
    if (!connectionIDs.has(connectionID))
      return badRequest(`unknown SIP connection id ${connectionID}`);
    if (
      password !== null &&
      (typeof password !== "string" || password.length === 0)
    ) {
      return badRequest(
        `SIP password for ${connectionID} must be non-empty or null`,
      );
    }
  }

  const now = Date.now();
  const existingSecrets = await listSIPSecrets(env, name);
  const explicitChanges = Object.entries(body.sip_secrets ?? {});
  const removedSecrets = staleSIPSecrets(existingSecrets, connectionIDs);
  const finalSecretIDs = new Set(
    existingSecrets.map((secret) => secret.connection_id),
  );
  for (const removed of removedSecrets)
    finalSecretIDs.delete(removed.connection_id);
  for (const [connectionID, password] of explicitChanges) {
    if (password === null) finalSecretIDs.delete(connectionID);
    else finalSecretIDs.add(connectionID);
  }
  for (const connection of doc.sip.connection) {
    if (!newDraft && !finalSecretIDs.has(connection.id)) {
      return badRequest(
        `${connection.name || connection.id}: a SIP password is required`,
      );
    }
  }
  const secretsChanged =
    explicitChanges.length > 0 || removedSecrets.length > 0;
  const secretRevision =
    (existing?.secret_revision ?? 0) + (secretsChanged ? 1 : 0);
  const row = {
    name,
    doc: JSON.stringify(body.doc),
    toml: body.toml,
    hash: await sha256Hex(`${body.toml}\nsecret-revision:${secretRevision}`),
    secret_revision: secretRevision,
    registration_id: existing?.registration_id ?? crypto.randomUUID(),
    created_at: existing?.created_at ?? now,
    updated_at: now,
  };
  const statements: D1PreparedStatement[] = [upsertConfigStatement(env, row)];
  for (const secret of removedSecrets) {
    statements.push(deleteSIPSecretStatement(env, name, secret.connection_id));
  }
  for (const [connectionID, password] of explicitChanges) {
    if (password === null) {
      statements.push(deleteSIPSecretStatement(env, name, connectionID));
      continue;
    }
    const encrypted = await encryptSIPPassword(
      env.SIP_SECRET_ENCRYPTION_KEY,
      name,
      connectionID,
      password,
    );
    statements.push(
      putSIPSecretStatement(env, {
        config_name: name,
        connection_id: connectionID,
        ...encrypted,
        updated_at: now,
      }),
    );
  }
  await env.DB.batch(statements);
  // Tell the broker so subscribed Go binaries pull the new config. Fire
  // and forget — failures here don't roll the save back; the binary will
  // catch up on next reconnect or SIGUSR1.
  notifyConfigChanged(env, ctx, row.name, row.hash);
  return json({
    name: row.name,
    doc: body.doc,
    toml: row.toml,
    hash: row.hash,
    created_at: row.created_at,
    updated_at: row.updated_at,
    registration_id: row.registration_id,
    sip_secret_state: Object.fromEntries(
      [...connectionIDs].map((id) => [
        id,
        body.sip_secrets?.[id] === null
          ? false
          : body.sip_secrets?.[id] !== undefined ||
            existingSecrets.some((secret) => secret.connection_id === id),
      ]),
    ),
  });
}

async function deleteOne(
  env: Env,
  name: string,
  ctx: ExecutionContext,
): Promise<Response> {
  const exists = await getConfig(env, name);
  if (!exists) return notFound();
  await env.DB.batch([
    deleteSIPSecretStatement(env, name),
    env.DB.prepare("DELETE FROM configs WHERE name = ?").bind(name),
  ]);
  // Notify subscribers — they'll find /bridge/runtime returns 404 on next pull
  // and can decide how to handle it (most likely: stay on current).
  notifyConfigChanged(env, ctx, name, "");
  return new Response(null, { status: 204 });
}

async function duplicate(
  req: Request,
  env: Env,
  from: string,
): Promise<Response> {
  let body: { name: string };
  try {
    body = (await req.json()) as { name: string };
  } catch {
    return badRequest("invalid json");
  }
  if (!NAME_RE.test(body.name)) return badRequest("invalid target name");
  const src = await getConfig(env, from);
  if (!src) return notFound("source not found");
  if (await getConfig(env, body.name))
    return badRequest("target already exists");
  const now = Date.now();
  let sourceConnectionIDs = new Set<string>();
  try {
    const sourceDoc = JSON.parse(src.doc) as Definition;
    sourceConnectionIDs = new Set(
      (sourceDoc.sip?.connection ?? []).map((connection) => connection.id),
    );
  } catch {
    // Preserve the config duplicate but fail closed for credential copying.
  }
  const sourceSecrets = (await listSIPSecrets(env, from)).filter((secret) =>
    sourceConnectionIDs.has(secret.connection_id),
  );
  const secretRevision = sourceSecrets.length > 0 ? 1 : 0;
  const row = {
    name: body.name,
    doc: src.doc,
    toml: src.toml,
    hash: await sha256Hex(`${src.toml}\nsecret-revision:${secretRevision}`),
    secret_revision: secretRevision,
    registration_id: crypto.randomUUID(),
    created_at: now,
    updated_at: now,
  };
  const statements: D1PreparedStatement[] = [upsertConfigStatement(env, row)];
  for (const sourceSecret of sourceSecrets) {
    const password = await decryptSIPPassword(
      env.SIP_SECRET_ENCRYPTION_KEY,
      sourceSecret,
    );
    const encrypted = await encryptSIPPassword(
      env.SIP_SECRET_ENCRYPTION_KEY,
      body.name,
      sourceSecret.connection_id,
      password,
    );
    statements.push(
      putSIPSecretStatement(env, {
        config_name: body.name,
        connection_id: sourceSecret.connection_id,
        ...encrypted,
        updated_at: now,
      }),
    );
  }
  await env.DB.batch(statements);
  return json({
    name: row.name,
    doc: JSON.parse(row.doc),
    toml: row.toml,
    hash: row.hash,
    created_at: row.created_at,
    updated_at: row.updated_at,
    registration_id: row.registration_id,
    sip_secret_state: Object.fromEntries(
      sourceSecrets.map((secret) => [secret.connection_id, true]),
    ),
  });
}

async function getCredentials(env: Env, name: string): Promise<Response> {
  if (!(await getConfig(env, name))) return notFound();
  return json({ state: credentialState(await getCredentialBundle(env, name)) });
}

async function patchCredentials(
  req: Request,
  env: Env,
  name: string,
  ctx: ExecutionContext,
): Promise<Response> {
  const config = await getConfig(env, name);
  if (!config) return notFound();
  let body: { patch?: Record<string, unknown>; expected_hash?: string };
  try {
    body = (await req.json()) as { patch?: Record<string, unknown> };
  } catch {
    return badRequest("invalid json");
  }
  if (body.expected_hash !== config.hash) {
    return conflict(
      `config changed since it was loaded; current hash is ${config.hash}`,
    );
  }
  if (!body.patch || typeof body.patch !== "object") {
    return badRequest("patch is required");
  }
  const allowed = new Set<string>(CREDENTIAL_KEYS);
  const changes = Object.entries(body.patch);
  if (changes.length === 0) return badRequest("credential patch is empty");
  for (const [key, value] of changes) {
    if (!allowed.has(key)) return badRequest(`unknown credential ${key}`);
    if (value !== null && (typeof value !== "string" || value.length === 0)) {
      return badRequest(`${key} must be a non-empty string or null`);
    }
  }
  const bundle = await getCredentialBundle(env, name);
  for (const [key, value] of changes) {
    bundle[key as keyof typeof bundle] =
      value === null ? "" : (value as string);
  }
  const encrypted = await encryptCredentialBundle(
    env.SIP_SECRET_ENCRYPTION_KEY,
    name,
    bundle,
  );
  const now = Date.now();
  const secretRevision = config.secret_revision + 1;
  const hash = await sha256Hex(
    `${config.toml}\nsecret-revision:${secretRevision}`,
  );
  await env.DB.batch([
    putCredentialBundleStatement(env, {
      config_name: name,
      ...encrypted,
      updated_at: now,
    }),
    env.DB.prepare(
      "UPDATE configs SET secret_revision = ?, hash = ?, updated_at = ? WHERE name = ?",
    ).bind(secretRevision, hash, now, name),
  ]);
  notifyConfigChanged(env, ctx, name, hash);
  return json({ state: credentialState(bundle), hash, updated_at: now });
}

async function rollback(
  req: Request,
  env: Env,
  ctx: ExecutionContext,
  name: string,
  changeID: string,
): Promise<Response> {
  let body: { expected_hash?: string };
  try {
    body = (await req.json()) as { expected_hash?: string };
  } catch {
    return badRequest("invalid json");
  }
  if (typeof body.expected_hash !== "string")
    return badRequest("expected_hash is required");
  try {
    return json(
      await rollbackConfigChange(env, ctx, name, body.expected_hash, changeID, {
        kind: "human",
        id: accessIdentity(req),
        label: accessIdentity(req),
      }),
    );
  } catch (caught) {
    if (caught instanceof ConfigChangeError) {
      if (caught.code === "not_found") return notFound(caught.message);
      if (caught.code === "conflict") return conflict(caught.message);
      return badRequest(caught.errors.join(" "));
    }
    throw caught;
  }
}

async function getRegistration(env: Env, name: string): Promise<Response> {
  const config = await getConfig(env, name);
  if (!config) return notFound();
  return json({ registration_id: config.registration_id });
}

async function rotateRegistration(env: Env, name: string): Promise<Response> {
  if (!(await getConfig(env, name))) return notFound();
  const registrationId = crypto.randomUUID();
  await env.DB.prepare(
    "UPDATE configs SET registration_id = ?, updated_at = ? WHERE name = ?",
  )
    .bind(registrationId, Date.now(), name)
    .run();
  return json({ registration_id: registrationId });
}
