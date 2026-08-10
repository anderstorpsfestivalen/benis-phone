import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  authenticateBridge,
  base64ToBytes,
  canonicalBridgeRequest,
} from "../worker/lib/bridge-auth.ts";

const vector = JSON.parse(
  await readFile(
    new URL("../../testdata/bridge-signing.json", import.meta.url),
    "utf8",
  ),
);

test("TypeScript canonical signing content matches the shared Go golden vector", () => {
  assert.equal(
    canonicalBridgeRequest(
      vector.method,
      vector.escaped_path,
      vector.body_hash,
      vector.timestamp,
      vector.nonce,
    ),
    vector.canonical,
  );
});

test("Cloudflare-compatible Web Crypto verifies the shared Ed25519 signature", async () => {
  const publicKey = await crypto.subtle.importKey(
    "raw",
    base64ToBytes(vector.public_key),
    { name: "Ed25519" },
    false,
    ["verify"],
  );
  assert.equal(
    await crypto.subtle.verify(
      { name: "Ed25519" },
      publicKey,
      base64ToBytes(vector.signature),
      new TextEncoder().encode(vector.canonical),
    ),
    true,
  );
});

test("signed authentication rejects nonce replay and returns only the stored binding", async () => {
  const db = new AuthDB({
    bridge_id: vector.bridge_id,
    config_name: "bound-config",
    public_key: vector.public_key,
    fingerprint: "fingerprint",
    revoked_at: null,
  });
  const request = signedGoldenRequest(
    "https://worker.test/bridge/runtime?name=other-config",
  );
  const authenticated = await authenticateBridge(
    request,
    { DB: db },
    new Uint8Array(),
    Number(vector.timestamp),
  );
  assert.equal(authenticated?.bridgeId, vector.bridge_id);
  assert.equal(authenticated?.configName, "bound-config");

  assert.equal(
    await authenticateBridge(
      signedGoldenRequest(
        "https://worker.test/bridge/runtime?name=other-config",
      ),
      { DB: db },
      new Uint8Array(),
      Number(vector.timestamp),
    ),
    null,
  );
});

test("signed authentication rejects stale timestamps and revoked bridges", async () => {
  const activeDB = new AuthDB({
    bridge_id: vector.bridge_id,
    config_name: "bound-config",
    public_key: vector.public_key,
    fingerprint: "fingerprint",
    revoked_at: null,
  });
  assert.equal(
    await authenticateBridge(
      signedGoldenRequest("https://worker.test/bridge/runtime"),
      { DB: activeDB },
      new Uint8Array(),
      Number(vector.timestamp) + 301,
    ),
    null,
  );

  const revokedDB = new AuthDB({
    bridge_id: vector.bridge_id,
    config_name: "bound-config",
    public_key: vector.public_key,
    fingerprint: "fingerprint",
    revoked_at: Number(vector.timestamp) * 1000,
  });
  assert.equal(
    await authenticateBridge(
      signedGoldenRequest("https://worker.test/bridge/runtime"),
      { DB: revokedDB },
      new Uint8Array(),
      Number(vector.timestamp),
    ),
    null,
  );
});

function signedGoldenRequest(url) {
  return new Request(url, {
    headers: {
      "X-Bridge-ID": vector.bridge_id,
      "X-Bridge-Timestamp": vector.timestamp,
      "X-Bridge-Nonce": vector.nonce,
      "X-Bridge-Signature": vector.signature,
    },
  });
}

class AuthDB {
  constructor(row) {
    this.row = row;
    this.nonces = new Set();
  }

  prepare(sql) {
    return new AuthStatement(this, sql);
  }
}

class AuthStatement {
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
    if (this.sql.includes("FROM bridges WHERE bridge_id")) return this.db.row;
    throw new Error(`unexpected first SQL: ${this.sql}`);
  }

  async run() {
    if (this.sql.startsWith("DELETE FROM bridge_nonces")) {
      return { meta: { changes: 0 } };
    }
    if (this.sql.includes("INSERT OR IGNORE INTO bridge_nonces")) {
      const key = `${this.values[0]}:${this.values[1]}`;
      if (this.db.nonces.has(key)) return { meta: { changes: 0 } };
      this.db.nonces.add(key);
      return { meta: { changes: 1 } };
    }
    throw new Error(`unexpected run SQL: ${this.sql}`);
  }
}
