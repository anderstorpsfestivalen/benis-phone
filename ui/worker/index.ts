import { checkAccess, type Env } from "./lib/auth";
import { handleApi } from "./handlers/configs";
import { handleBridge } from "./handlers/bridge";
import { handleRegistrationsApi } from "./handlers/registrations";
import { handleFiles } from "./handlers/files";
import { handlePreview } from "./handlers/preview";
import { notFound, unauthorized } from "./lib/responses";
import { handleMCP } from "./handlers/mcp";
import { handleAgentsApi, handleOAuth, isOAuthPath } from "./lib/oauth";

export { ConfigBroker } from "./durable/configBroker";

export default {
  async fetch(
    req: Request,
    env: Env,
    ctx: ExecutionContext,
  ): Promise<Response> {
    const url = new URL(req.url);
    try {
      // Credentialless bridge enrollment and Ed25519-authenticated runtime
      // endpoints live outside Cloudflare Access for headless phones.
      if (url.pathname.startsWith("/bridge/")) {
        return await handleBridge(req, env, url.pathname);
      }

      // Remote MCP clients authenticate with the Worker's OAuth server.  The
      // protocol and token endpoints must bypass Cloudflare Access; only the
      // browser-facing /oauth/authorize page is Access protected.
      if (isOAuthPath(url.pathname)) {
        return await handleOAuth(req, env, url.pathname);
      }
      if (url.pathname === "/mcp") {
        return await handleMCP(req, env, ctx);
      }

      // Editor CRUD. Requires Cloudflare Access (Cf-Access-Jwt-Assertion).
      if (url.pathname.startsWith("/api/")) {
        if (!checkAccess(req)) return unauthorized();
        if (
          url.pathname.startsWith("/api/registrations") ||
          url.pathname.startsWith("/api/bridges")
        ) {
          return await handleRegistrationsApi(req, env, url.pathname);
        }
        if (url.pathname.startsWith("/api/agents")) {
          return await handleAgentsApi(req, env, url.pathname);
        }
        if (url.pathname.startsWith("/api/configs")) {
          return await handleApi(req, env, url.pathname, ctx);
        }
        if (url.pathname.startsWith("/api/files")) {
          return await handleFiles(req, env, url.pathname);
        }
        if (url.pathname === "/api/genericjson/preview") {
          return await handlePreview(req);
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
