import type { Env } from "../lib/auth";
import { getConfig } from "../lib/db";
import { badRequest, notFound, text } from "../lib/responses";

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
