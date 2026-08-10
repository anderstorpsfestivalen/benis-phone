import type { Env } from "../lib/auth";
import { getConfig } from "../lib/db";
import { badRequest, notFound, text } from "../lib/responses";
import { json } from "../lib/responses";
import { decryptedSIPPasswords } from "../lib/secrets";

// GET /config?name=X            → text/plain TOML body
// GET /config?name=X&hash=1     → text/plain 64-char hex sha256
export async function handleConfig(req: Request, env: Env): Promise<Response> {
  if (req.method !== "GET") return badRequest("method not allowed");
  const url = new URL(req.url);
  const name = url.searchParams.get("name");
  if (!name) return badRequest("name is required");
  const row = await getConfig(env, name);
  if (!row) return notFound();
  if (url.searchParams.get("hash") === "1") {
    return text(row.hash);
  }
  return text(row.toml);
}

// GET /config/runtime?name=X returns the complete runtime bundle. This route
// is bearer-only; the editor API never receives decrypted passwords.
export async function handleRuntimeConfig(
  req: Request,
  env: Env,
): Promise<Response> {
  if (req.method !== "GET") return badRequest("method not allowed");
  const name = new URL(req.url).searchParams.get("name");
  if (!name) return badRequest("name is required");
  const row = await getConfig(env, name);
  if (!row) return notFound();
  let registeredConnectionIDs = new Set<string>();
  try {
    const doc = JSON.parse(row.doc) as {
      sip?: { connection?: Array<{ id?: string; registration?: string }> };
    };
    registeredConnectionIDs = new Set(
      (doc.sip?.connection ?? [])
        .filter(
          (connection) =>
            connection.registration === "registered" && !!connection.id,
        )
        .map((connection) => connection.id as string),
    );
  } catch {
    // Invalid stored JSON is handled by the TOML decoder in the runtime. Do
    // not release any credentials when its connection ownership is unknown.
  }
  return json({
    revision: row.hash,
    toml: row.toml,
    sip_passwords: await decryptedSIPPasswords(
      env,
      name,
      registeredConnectionIDs,
    ),
  });
}

// GET /config/ws?name=X — WebSocket upgrade. Resolves the singleton
// ConfigBroker DO and forwards the upgrade request unchanged so the DO
// lands the socket at its /subscribe route.
export async function handleConfigWS(
  req: Request,
  env: Env,
): Promise<Response> {
  if (req.headers.get("Upgrade") !== "websocket") {
    return badRequest("expected websocket upgrade");
  }
  const url = new URL(req.url);
  const name = url.searchParams.get("name");
  if (!name) return badRequest("name is required");

  const id = env.CONFIG_BROKER.idFromName("global");
  const stub = env.CONFIG_BROKER.get(id);
  // Forward to /subscribe — the DO reads the upgrade header from the
  // request and returns a 101 with the paired client WebSocket.
  const brokerURL = new URL(req.url);
  brokerURL.pathname = "/subscribe";
  return stub.fetch(new Request(brokerURL, req));
}
