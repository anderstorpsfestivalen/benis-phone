import { checkAccess, checkBearer, type Env } from "./lib/auth";
import { handleApi } from "./handlers/configs";
import { handleConfig } from "./handlers/serve";
import { notFound, unauthorized } from "./lib/responses";

export default {
  async fetch(req: Request, env: Env): Promise<Response> {
    const url = new URL(req.url);
    try {
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
          return await handleApi(req, env, url.pathname);
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
