import { timingSafeEqual } from "./hash";

export interface Env {
  DB: D1Database;
  CONFIG_BEARER_TOKEN: string;
  // Static assets binding (the built React app in ./dist). Calling
  // env.ASSETS.fetch(req) serves a file if one exists.
  ASSETS: Fetcher;
}

// /config (consumed by the Go binary). Plain bearer token. Cloudflare
// Access is not used here because the Go phones are headless clients.
export function checkBearer(req: Request, env: Env): boolean {
  const expected = env.CONFIG_BEARER_TOKEN;
  if (!expected) return false;
  const header = req.headers.get("Authorization") ?? "";
  if (!header.startsWith("Bearer ")) return false;
  const got = header.slice("Bearer ".length).trim();
  return timingSafeEqual(got, expected);
}

// /api/* (consumed by the React editor). Cloudflare Access is configured
// in the Cloudflare dashboard to gate the hostname — requests that
// reach the Worker have already been authenticated. We also verify the
// Cf-Access-Jwt-Assertion header is present as a defense-in-depth check
// so direct hits to the Worker URL (e.g. via worker.dev) without the
// Access policy still fail closed.
export function checkAccess(req: Request): boolean {
  // In local `wrangler dev`, the assertion isn't set — allow there only.
  const url = new URL(req.url);
  if (url.hostname === "localhost" || url.hostname === "127.0.0.1") return true;
  return req.headers.get("Cf-Access-Jwt-Assertion") !== null;
}
