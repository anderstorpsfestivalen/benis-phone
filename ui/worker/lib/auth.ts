export interface Env {
  DB: D1Database;
  // Base64-encoded 32-byte AES-GCM key for all write-only credentials.
  // SIP ciphertext keeps its existing AAD and remains readable.
  SIP_SECRET_ENCRYPTION_KEY: string;
  // Static assets binding (the built React app in ./dist). Calling
  // env.ASSETS.fetch(req) serves a file if one exists.
  ASSETS: Fetcher;
  // ConfigBroker DO namespace — used by the editor save path to notify
  // subscribers and by /bridge/ws to upgrade incoming subscriptions.
  CONFIG_BROKER: DurableObjectNamespace;
  // R2 bucket holding audio assets. Read/written by /api/files/* (the
  // editor's Files tab) and read by the Go binary at startup via the S3
  // API (see core/filesync/).
  BUCKET: R2Bucket;
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
