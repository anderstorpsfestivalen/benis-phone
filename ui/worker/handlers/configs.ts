import { definitionSchema } from "../../src/generated/schemas";
import type { Env } from "../lib/auth";
import { sha256Hex } from "../lib/hash";
import {
  deleteConfig,
  getConfig,
  listConfigs,
  upsertConfig,
} from "../lib/db";
import { badRequest, json, notFound } from "../lib/responses";

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
  });
}

async function putOne(req: Request, env: Env, name: string, ctx: ExecutionContext): Promise<Response> {
  let body: { doc: unknown; toml: string };
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

  const now = Date.now();
  const existing = await getConfig(env, name);
  const row = {
    name,
    doc: JSON.stringify(body.doc),
    toml: body.toml,
    hash: await sha256Hex(body.toml),
    created_at: existing?.created_at ?? now,
    updated_at: now,
  };
  await upsertConfig(env, row);
  // Tell the broker so subscribed Go binaries pull the new config. Fire
  // and forget — failures here don't roll the save back; the binary will
  // catch up on next reconnect or SIGUSR1.
  notifyBroker(env, ctx, row.name, row.hash);
  return json({
    name: row.name,
    doc: body.doc,
    toml: row.toml,
    hash: row.hash,
    created_at: row.created_at,
    updated_at: row.updated_at,
  });
}

async function deleteOne(env: Env, name: string, ctx: ExecutionContext): Promise<Response> {
  const ok = await deleteConfig(env, name);
  if (!ok) return notFound();
  // Notify subscribers — they'll find /config returns 404 on next pull
  // and can decide how to handle it (most likely: stay on current).
  notifyBroker(env, ctx, name, "");
  return new Response(null, { status: 204 });
}

async function duplicate(req: Request, env: Env, from: string): Promise<Response> {
  let body: { name: string };
  try {
    body = (await req.json()) as { name: string };
  } catch {
    return badRequest("invalid json");
  }
  if (!NAME_RE.test(body.name)) return badRequest("invalid target name");
  const src = await getConfig(env, from);
  if (!src) return notFound("source not found");
  if (await getConfig(env, body.name)) return badRequest("target already exists");
  const now = Date.now();
  const row = {
    name: body.name,
    doc: src.doc,
    toml: src.toml,
    hash: src.hash,
    created_at: now,
    updated_at: now,
  };
  await upsertConfig(env, row);
  return json({
    name: row.name,
    doc: JSON.parse(row.doc),
    toml: row.toml,
    hash: row.hash,
    created_at: row.created_at,
    updated_at: row.updated_at,
  });
}

function notifyBroker(env: Env, ctx: ExecutionContext, name: string, hash: string) {
  const id = env.CONFIG_BROKER.idFromName("global");
  const stub = env.CONFIG_BROKER.get(id);
  const url = `https://broker/notify?name=${encodeURIComponent(name)}&hash=${encodeURIComponent(hash)}`;
  ctx.waitUntil(stub.fetch(url, { method: "POST" }));
}
