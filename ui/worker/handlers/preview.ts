import { badRequest, json } from "../lib/responses";

// /api/genericjson/preview proxies an HTTP fetch on behalf of the editor
// so authors can iterate on a GenericJSON node's URL/headers/body without
// hitting CORS or saving the config + ringing the IVR. The proxy is
// intentionally minimal — it just relays the response so the user can see
// the JSON shape they're templating against. The Go side does the actual
// template + jq rendering at call time; reproducing it here would require
// porting Go text/template + gojq, which would inevitably drift.
//
// Caveats baked in:
//   - 10s timeout (matches the runtime default in core/functions/genericjson.go)
//   - 1 MiB response cap (much smaller than the runtime 8 MiB — this is just
//     for previewing structure)
//   - Only http(s) URLs; refuses file://, data:, javascript:, etc.
//   - Sits behind Cloudflare Access (handled in index.ts), so the proxy is
//     not abusable by anonymous callers.

const TIMEOUT_MS = 10_000;
const MAX_BODY = 1 << 20; // 1 MiB

type PreviewRequest = {
  url?: string;
  method?: string;
  body?: string;
  headers?: Record<string, string>;
};

export async function handlePreview(req: Request): Promise<Response> {
  if (req.method !== "POST") return badRequest("method not allowed");

  let payload: PreviewRequest;
  try {
    payload = (await req.json()) as PreviewRequest;
  } catch {
    return badRequest("invalid JSON request body");
  }

  const url = (payload.url ?? "").trim();
  if (!url) return badRequest("missing url");

  let target: URL;
  try {
    target = new URL(url);
  } catch {
    return badRequest("invalid url");
  }
  if (target.protocol !== "http:" && target.protocol !== "https:") {
    return badRequest("only http(s) URLs are allowed");
  }

  const method = (payload.method ?? "GET").trim().toUpperCase() || "GET";
  const headers = new Headers();
  for (const [k, v] of Object.entries(payload.headers ?? {})) {
    if (k && typeof v === "string") headers.set(k, v);
  }
  if (!headers.has("Accept")) headers.set("Accept", "application/json");
  let body: string | undefined;
  if (payload.body && method !== "GET" && method !== "HEAD") {
    body = payload.body;
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
  }

  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS);
  let resp: Response;
  try {
    resp = await fetch(target.toString(), {
      method,
      headers,
      body,
      signal: ctrl.signal,
      redirect: "follow",
    });
  } catch (err) {
    clearTimeout(timer);
    const msg = err instanceof Error ? err.message : String(err);
    return json({ error: `fetch: ${msg}` }, 502);
  }
  clearTimeout(timer);

  // Read up to MAX_BODY bytes; anything beyond is truncated so the
  // editor doesn't choke on a huge payload during preview.
  const reader = resp.body?.getReader();
  let received = new Uint8Array(0);
  let truncated = false;
  if (reader) {
    while (received.length < MAX_BODY) {
      const { value, done } = await reader.read();
      if (done) break;
      if (!value) continue;
      const need = MAX_BODY - received.length;
      const slice = value.length > need ? value.subarray(0, need) : value;
      const merged = new Uint8Array(received.length + slice.length);
      merged.set(received, 0);
      merged.set(slice, received.length);
      received = merged;
      if (value.length > need) {
        truncated = true;
        break;
      }
    }
    // Drain remaining if any so the upstream connection closes cleanly.
    try {
      await reader.cancel();
    } catch {
      /* ignore */
    }
  }

  const text = new TextDecoder().decode(received);
  return json({
    status: resp.status,
    contentType: resp.headers.get("content-type") ?? "",
    body: text,
    truncated,
  });
}
