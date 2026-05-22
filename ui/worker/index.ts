import { checkAccess, checkBearer, type Env } from "./lib/auth";
import { handleApi } from "./handlers/configs";
import { handleFiles } from "./handlers/files";
import { handleConfig, handleConfigWS } from "./handlers/serve";
import { notFound, unauthorized } from "./lib/responses";

export { ConfigBroker } from "./durable/configBroker";

export default {
  async fetch(req: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url = new URL(req.url);
    try {
      // Bearer-token WebSocket endpoint (Go binary). Long-lived push
      // channel for config-changed events; replaces the old hash-polling
      // loop. Outside the Access perimeter because phones are headless.
      if (url.pathname === "/config/ws") {
        if (!checkBearer(req, env)) return unauthorized();
        return await handleConfigWS(req, env);
      }

      // Bearer-token endpoint (Go binary). Outside the Access perimeter on
      // purpose — phones are headless clients.
      if (url.pathname === "/config") {
        if (!checkBearer(req, env)) return unauthorized();
        return await handleConfig(req, env);
      }

      // Editor CRUD. Requires Cloudflare Access (Cf-Access-Jwt-Assertion).
      if (url.pathname.startsWith("/api/")) {
        if (!checkAccess(req)) return unauthorized();
        if (url.pathname.startsWith("/api/configs")) {
          return await handleApi(req, env, url.pathname, ctx);
        }
        if (url.pathname.startsWith("/api/files")) {
          return await handleFiles(req, env, url.pathname);
        }
        return notFound();
      }

      // Static assets (React build). Asset routing already runs ahead of
      // this Worker for real files in dist/; we only land here on a miss
      // — which for our SPA means a client-side route like /editor/X.
      // Rewrite to the SPA shell so react-router can take over.
      return env.ASSETS.fetch(new Request(new URL("/index.html", url), req));
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      return new Response(JSON.stringify({ error: msg }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      });
    }
  },
} satisfies ExportedHandler<Env>;
