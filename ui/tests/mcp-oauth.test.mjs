import assert from "node:assert/strict";
import test from "node:test";

import { handleMCP } from "../worker/handlers/mcp.ts";
import { handleOAuth } from "../worker/lib/oauth.ts";

test("OAuth discovery advertises the path-specific MCP resource and PKCE", async () => {
  const env = { DB: new OAuthDB() };
  const resource = await handleOAuth(
    new Request("https://ivr.test/.well-known/oauth-protected-resource/mcp"),
    env,
    "/.well-known/oauth-protected-resource/mcp",
  );
  assert.equal(resource.status, 200);
  assert.deepEqual(await resource.json(), {
    resource: "https://ivr.test/mcp",
    authorization_servers: ["https://ivr.test"],
    scopes_supported: [
      "config:read",
      "config:write",
      "status:read",
      "history:read",
    ],
    bearer_methods_supported: ["header"],
    resource_name: "ATP IVR live configuration",
  });

  const metadata = await handleOAuth(
    new Request("https://ivr.test/.well-known/oauth-authorization-server"),
    env,
    "/.well-known/oauth-authorization-server",
  );
  const body = await metadata.json();
  assert.equal(body.authorization_endpoint, "https://ivr.test/oauth/authorize");
  assert.deepEqual(body.code_challenge_methods_supported, ["S256"]);
  assert.deepEqual(body.token_endpoint_auth_methods_supported, ["none"]);
});

test("dynamic registration accepts loopback public clients and rejects non-PKCE secrets", async () => {
  const db = new OAuthDB();
  const accepted = await handleOAuth(
    jsonRequest("/oauth/register", {
      client_name: "Codex",
      redirect_uris: ["http://127.0.0.1:49152/callback"],
      token_endpoint_auth_method: "none",
    }),
    { DB: db },
    "/oauth/register",
  );
  assert.equal(accepted.status, 201);
  const client = await accepted.json();
  assert.match(client.client_id, /^mcp_client_/);
  assert.equal(db.clients.length, 1);

  const rejected = await handleOAuth(
    jsonRequest("/oauth/register", {
      redirect_uris: ["http://attacker.example/callback"],
      token_endpoint_auth_method: "client_secret_basic",
    }),
    { DB: db },
    "/oauth/register",
  );
  assert.equal(rejected.status, 400);
});

test("MCP endpoint challenges unauthenticated clients with protected-resource metadata", async () => {
  const response = await handleMCP(
    new Request("https://ivr.test/mcp", {
      method: "POST",
      headers: {
        Accept: "application/json, text/event-stream",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: 1,
        method: "initialize",
        params: {
          protocolVersion: "2025-06-18",
          capabilities: {},
          clientInfo: { name: "test", version: "1" },
        },
      }),
    }),
    { DB: new OAuthDB() },
    { waitUntil() {} },
  );
  assert.equal(response.status, 401);
  assert.match(
    response.headers.get("WWW-Authenticate") ?? "",
    /oauth-protected-resource\/mcp/,
  );
});

test("authenticated MCP tool calls return object-shaped structured content", async () => {
  const db = new OAuthDB();
  db.tokenRow = {
    token_hash: "hash",
    grant_id: "123e4567-e89b-42d3-a456-426614174000",
    kind: "access",
    expires_at: Date.now() + 60_000,
    token_revoked_at: null,
    client_id: "mcp_client_test",
    client_name: "Codex",
    access_identity: "operator@example.test",
    scope: "config:read config:write status:read history:read",
    grant_revoked_at: null,
  };
  db.configs.push({
    name: "festival",
    hash: "a".repeat(64),
    created_at: 1,
    updated_at: 2,
  });
  const response = await handleMCP(
    new Request("https://ivr.test/mcp", {
      method: "POST",
      headers: {
        Authorization: `Bearer mcp_at_${"a".repeat(43)}`,
        Accept: "application/json, text/event-stream",
        "Content-Type": "application/json",
        "MCP-Protocol-Version": "2025-06-18",
      },
      body: JSON.stringify({
        jsonrpc: "2.0",
        id: 2,
        method: "tools/call",
        params: { name: "list_configs", arguments: {} },
      }),
    }),
    { DB: db },
    { waitUntil() {} },
  );
  assert.equal(response.status, 200);
  const body = await response.json();
  assert.deepEqual(body.result.structuredContent, { result: db.configs });
  assert.equal(body.result.isError, undefined);
});

function jsonRequest(path, body) {
  return new Request(`https://ivr.test${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

class OAuthDB {
  clients = [];
  configs = [];
  tokenRow = null;

  prepare(sql) {
    return new OAuthStatement(this, sql.replace(/\s+/g, " ").trim());
  }
}

class OAuthStatement {
  constructor(db, sql) {
    this.db = db;
    this.sql = sql;
    this.values = [];
  }

  bind(...values) {
    this.values = values;
    return this;
  }

  async first() {
    if (this.sql.includes("count(*) AS count FROM oauth_clients"))
      return { count: this.db.clients.length };
    if (this.sql.includes("FROM oauth_tokens t")) return this.db.tokenRow;
    throw new Error(`unexpected first SQL: ${this.sql}`);
  }

  async run() {
    if (this.sql.startsWith("INSERT INTO oauth_clients")) {
      this.db.clients.push({
        client_id: this.values[0],
        client_name: this.values[1],
        redirect_uris: this.values[2],
        created_at: this.values[3],
      });
      return { meta: { changes: 1 } };
    }
    if (this.sql.startsWith("UPDATE oauth_grants SET last_used_at"))
      return { meta: { changes: 1 } };
    throw new Error(`unexpected run SQL: ${this.sql}`);
  }

  async all() {
    if (
      this.sql.startsWith(
        "SELECT name, hash, created_at, updated_at FROM configs",
      )
    )
      return { results: this.db.configs };
    throw new Error(`unexpected all SQL: ${this.sql}`);
  }
}
