import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { WebStandardStreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/webStandardStreamableHttp.js";
import * as z from "zod/v4";
import type { Env } from "../lib/auth.ts";
import {
  applyConfigPatch,
  ConfigChangeError,
  getConfigChange,
  listConfigChanges,
  type ConfigPatchOperation,
  validateConfigPatch,
} from "../lib/config-changes.ts";
import { getConfig, listConfigs } from "../lib/db.ts";
import {
  authenticateOAuthBearer,
  mcpUnauthorized,
  type OAuthActor,
} from "../lib/oauth.ts";

const NAME_RE = /^[a-zA-Z0-9_-]{1,64}$/;
const HASH_RE = /^[a-f0-9]{64}$/;
const patchSchema = z.array(
  z.object({
    op: z.enum(["add", "remove", "replace", "test"]),
    path: z.string(),
    value: z.unknown().optional(),
  }),
);

export async function handleMCP(
  req: Request,
  env: Env,
  ctx: ExecutionContext,
): Promise<Response> {
  if (req.method === "OPTIONS") {
    return new Response(null, {
      status: 204,
      headers: corsHeaders(),
    });
  }
  const actor = await authenticateOAuthBearer(req, env);
  if (!actor) return withCORS(mcpUnauthorized(req));
  const server = createServer(env, ctx, actor);
  const transport = new WebStandardStreamableHTTPServerTransport({
    sessionIdGenerator: undefined,
    enableJsonResponse: true,
  });
  await server.connect(transport);
  const response = await transport.handleRequest(req, {
    authInfo: {
      token: "redacted",
      clientId: actor.clientId,
      scopes: actor.scopes,
      expiresAt: Math.floor(actor.expiresAt / 1000),
      resource: new URL(`${new URL(req.url).origin}/mcp`),
      extra: { grantId: actor.grantId, accessIdentity: actor.accessIdentity },
    },
  });
  return withCORS(response);
}

function createServer(env: Env, ctx: ExecutionContext, actor: OAuthActor) {
  const server = new McpServer({
    name: "benis-phone-config",
    version: "1.0.0",
  });

  server.registerTool(
    "list_configs",
    {
      title: "List IVR configs",
      description:
        "List every live IVR config available to this operator. Returns names, hashes, and timestamps but never secrets.",
      annotations: { readOnlyHint: true },
    },
    async () => runTool(actor, "config:read", () => listConfigs(env)),
  );

  server.registerTool(
    "get_config",
    {
      title: "Read an IVR config",
      description:
        "Read the current non-secret config document and canonical TOML. Save the returned hash and use it as base_hash for validation and apply calls.",
      inputSchema: { name: z.string().regex(NAME_RE) },
      annotations: { readOnlyHint: true },
    },
    async ({ name }) =>
      runTool(actor, "config:read", async () => {
        const row = await getConfig(env, name);
        if (!row) throw new ConfigChangeError("not_found", "config not found");
        return {
          name: row.name,
          doc: JSON.parse(row.doc),
          toml: row.toml,
          hash: row.hash,
          created_at: row.created_at,
          updated_at: row.updated_at,
        };
      }),
  );

  server.registerTool(
    "validate_config_patch",
    {
      title: "Validate an IVR config patch",
      description:
        "Apply a restricted RFC 6902 JSON Patch in memory, validate the complete config, render canonical TOML, and return the resulting diff. This never writes. Call it before apply_config_patch.",
      inputSchema: {
        name: z.string().regex(NAME_RE),
        base_hash: z.string().regex(HASH_RE),
        patch: patchSchema,
      },
      annotations: { readOnlyHint: true },
    },
    async ({ name, base_hash, patch }) =>
      runTool(actor, "config:write", () =>
        validateConfigPatch(
          env,
          name,
          base_hash,
          patch as ConfigPatchOperation[],
        ),
      ),
  );

  server.registerTool(
    "apply_config_patch",
    {
      title: "Apply an IVR config patch",
      description:
        "Revalidate and atomically save a previously reviewed config patch. The base hash must still match; otherwise read the config again and prepare a new patch.",
      inputSchema: {
        name: z.string().regex(NAME_RE),
        base_hash: z.string().regex(HASH_RE),
        patch: patchSchema,
      },
      annotations: { destructiveHint: true, idempotentHint: false },
    },
    async ({ name, base_hash, patch }) =>
      runTool(actor, "config:write", () =>
        applyConfigPatch(
          env,
          ctx,
          name,
          base_hash,
          patch as ConfigPatchOperation[],
          {
            kind: "mcp",
            id: actor.grantId,
            label: `${actor.clientName} (${actor.accessIdentity})`,
          },
        ),
      ),
  );

  server.registerTool(
    "get_config_status",
    {
      title: "Read live SIP status",
      description:
        "Read the latest runtime and SIP connection status for a config. This contains operational state only, never runtime credentials.",
      inputSchema: { name: z.string().regex(NAME_RE) },
      annotations: { readOnlyHint: true },
    },
    async ({ name }) =>
      runTool(actor, "status:read", async () => {
        if (!(await getConfig(env, name)))
          throw new ConfigChangeError("not_found", "config not found");
        const broker = env.CONFIG_BROKER.get(
          env.CONFIG_BROKER.idFromName("global"),
        );
        const response = await broker.fetch(
          `https://broker/status?name=${encodeURIComponent(name)}`,
        );
        if (!response.ok)
          throw new Error(`status broker returned ${response.status}`);
        return response.json();
      }),
  );

  server.registerTool(
    "list_config_history",
    {
      title: "List config change history",
      description:
        "List audited MCP changes and rollbacks for a config, newest first.",
      inputSchema: {
        name: z.string().regex(NAME_RE),
        limit: z.number().int().min(1).max(100).optional(),
      },
      annotations: { readOnlyHint: true },
    },
    async ({ name, limit }) =>
      runTool(actor, "history:read", () =>
        listConfigChanges(env, name, limit ?? 50),
      ),
  );

  server.registerTool(
    "get_config_change",
    {
      title: "Read a config change",
      description:
        "Read one audited config change including its non-secret before and after documents.",
      inputSchema: {
        name: z.string().regex(NAME_RE),
        change_id: z.string().uuid(),
      },
      annotations: { readOnlyHint: true },
    },
    async ({ name, change_id }) =>
      runTool(actor, "history:read", async () => {
        const change = await getConfigChange(env, name, change_id);
        if (!change)
          throw new ConfigChangeError("not_found", "config change not found");
        return change;
      }),
  );

  return server;
}

async function runTool(
  actor: OAuthActor,
  scope: string,
  operation: () => unknown | Promise<unknown>,
) {
  if (!actor.scopes.includes(scope)) {
    return toolResult({ error: `missing OAuth scope ${scope}` }, true);
  }
  try {
    return toolResult(await operation());
  } catch (caught) {
    if (caught instanceof ConfigChangeError) {
      return toolResult(
        { error: caught.code, message: caught.message, errors: caught.errors },
        true,
      );
    }
    return toolResult(
      {
        error: "internal_error",
        message: caught instanceof Error ? caught.message : String(caught),
      },
      true,
    );
  }
}

function toolResult(value: unknown, isError = false) {
  return {
    content: [{ type: "text" as const, text: JSON.stringify(value, null, 2) }],
    structuredContent: { result: value },
    ...(isError ? { isError: true } : {}),
  };
}

function corsHeaders(): HeadersInit {
  return {
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, DELETE, OPTIONS",
    "Access-Control-Allow-Headers":
      "Authorization, Content-Type, MCP-Protocol-Version, MCP-Session-Id, Last-Event-ID",
    "Access-Control-Expose-Headers":
      "MCP-Protocol-Version, MCP-Session-Id, WWW-Authenticate",
  };
}

function withCORS(response: Response): Response {
  const headers = new Headers(response.headers);
  for (const [key, value] of Object.entries(corsHeaders())) {
    headers.set(key, String(value));
  }
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}
