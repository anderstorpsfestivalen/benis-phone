import assert from "node:assert/strict";
import test from "node:test";

import { handleBridge } from "../worker/handlers/bridge.ts";
import {
  bytesToBase64,
  enrollmentCanonicalBody,
} from "../worker/lib/bridge-auth.ts";

const registrationID = "223e4567-e89b-42d3-a456-426614174000";

test("enrollment is self-signed and idempotent for the same public key", async () => {
  const db = new EnrollmentDB(registrationID);
  const body = await enrollmentBody(registrationID);
  const first = await handleBridge(
    enrollmentRequest(body),
    { DB: db },
    "/bridge/enroll",
  );
  assert.equal(first.status, 202);
  assert.equal((await first.json()).status, "pending");
  assert.equal(db.enrollments.length, 1);

  const repeat = await handleBridge(
    enrollmentRequest(body),
    { DB: db },
    "/bridge/enroll",
  );
  assert.equal(repeat.status, 200);
  assert.equal((await repeat.json()).request_id, body.request_id);
  assert.equal(db.enrollments.length, 1);
});

test("enrollment expires after 24 hours and pending requests are capped at five", async () => {
  const db = new EnrollmentDB(registrationID);
  const expired = await enrollmentBody(registrationID);
  db.enrollments.push({
    request_id: expired.request_id,
    config_name: db.configName,
    public_key: expired.public_key,
    fingerprint: "fingerprint",
    hostname: expired.hostname,
    platform: expired.platform,
    version: expired.version,
    created_at: Date.now() - 25 * 60 * 60 * 1000,
    expires_at: Date.now() - 1000,
    status: "pending",
    decided_at: null,
    bridge_id: null,
  });
  const poll = await handleBridge(
    new Request(`https://worker.test/bridge/enroll/${expired.request_id}`),
    { DB: db },
    `/bridge/enroll/${expired.request_id}`,
  );
  assert.equal((await poll.json()).status, "expired");

  db.enrollments = [];
  for (let index = 0; index < 5; index++) {
    db.enrollments.push({
      request_id: crypto.randomUUID(),
      config_name: db.configName,
      public_key: `key-${index}`,
      status: "pending",
      created_at: Date.now(),
      expires_at: Date.now() + 60_000,
      bridge_id: null,
    });
  }
  const sixth = await enrollmentBody(registrationID);
  const response = await handleBridge(
    enrollmentRequest(sixth),
    { DB: db },
    "/bridge/enroll",
  );
  assert.equal(response.status, 429);
  assert.equal(db.enrollments.length, 5);
});

test("rotating a registration ID blocks new enrollment with the old ID", async () => {
  const db = new EnrollmentDB("323e4567-e89b-42d3-a456-426614174000");
  const oldID = db.registrationID;
  db.registrationID = "423e4567-e89b-42d3-a456-426614174000";
  const response = await handleBridge(
    enrollmentRequest(await enrollmentBody(oldID)),
    { DB: db },
    "/bridge/enroll",
  );
  assert.equal(response.status, 404);
});

test("unauthenticated bridge endpoints never return runtime data", async () => {
  const response = await handleBridge(
    new Request("https://worker.test/bridge/runtime"),
    { DB: new EnrollmentDB(registrationID) },
    "/bridge/runtime",
  );
  assert.equal(response.status, 401);
  assert.deepEqual(await response.json(), { error: "unauthorized" });
});

async function enrollmentBody(id) {
  const keyPair = await crypto.subtle.generateKey("Ed25519", true, [
    "sign",
    "verify",
  ]);
  const publicKey = bytesToBase64(
    new Uint8Array(await crypto.subtle.exportKey("raw", keyPair.publicKey)),
  );
  const body = {
    registration_id: id,
    request_id: crypto.randomUUID(),
    public_key: publicKey,
    hostname: "bridge.test",
    platform: "linux/arm64",
    version: "test",
    timestamp: String(Math.floor(Date.now() / 1000)),
  };
  const signature = await crypto.subtle.sign(
    "Ed25519",
    keyPair.privateKey,
    new TextEncoder().encode(await enrollmentCanonicalBody(body)),
  );
  return { ...body, signature: bytesToBase64(new Uint8Array(signature)) };
}

function enrollmentRequest(body) {
  return new Request("https://worker.test/bridge/enroll", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

class EnrollmentDB {
  constructor(id) {
    this.registrationID = id;
    this.configName = "festival";
    this.enrollments = [];
    this.bridges = [];
  }

  prepare(sql) {
    return new EnrollmentStatement(this, sql.replace(/\s+/g, " ").trim());
  }
}

class EnrollmentStatement {
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
    if (this.sql.includes("SELECT name FROM configs WHERE registration_id")) {
      return this.values[0] === this.db.registrationID
        ? { name: this.db.configName }
        : null;
    }
    if (this.sql.includes("SELECT bridge_id FROM bridges")) {
      return (
        this.db.bridges.find(
          (bridge) =>
            bridge.config_name === this.values[0] &&
            bridge.public_key === this.values[1] &&
            bridge.revoked_at === null,
        ) ?? null
      );
    }
    if (
      this.sql.includes(
        "FROM bridge_enrollments WHERE config_name = ? AND public_key = ?",
      )
    ) {
      return (
        [...this.db.enrollments]
          .filter(
            (row) =>
              row.config_name === this.values[0] &&
              row.public_key === this.values[1],
          )
          .sort((left, right) => right.created_at - left.created_at)[0] ?? null
      );
    }
    if (this.sql.includes("count(*) AS count FROM bridge_enrollments")) {
      return {
        count: this.db.enrollments.filter(
          (row) =>
            row.config_name === this.values[0] && row.status === "pending",
        ).length,
      };
    }
    if (this.sql.includes("FROM bridge_enrollments WHERE request_id = ?")) {
      return (
        this.db.enrollments.find((row) => row.request_id === this.values[0]) ??
        null
      );
    }
    throw new Error(`unexpected first SQL: ${this.sql}`);
  }

  async run() {
    if (
      this.sql.startsWith(
        "UPDATE bridge_enrollments SET status = 'expired' WHERE config_name",
      )
    ) {
      for (const row of this.db.enrollments) {
        if (
          row.config_name === this.values[0] &&
          row.status === "pending" &&
          row.expires_at <= this.values[1]
        )
          row.status = "expired";
      }
      return { meta: { changes: 1 } };
    }
    if (
      this.sql.startsWith(
        "UPDATE bridge_enrollments SET status = 'expired' WHERE request_id",
      )
    ) {
      for (const row of this.db.enrollments) {
        if (
          row.request_id === this.values[0] &&
          row.status === "pending" &&
          row.expires_at <= this.values[1]
        )
          row.status = "expired";
      }
      return { meta: { changes: 1 } };
    }
    if (this.sql.startsWith("INSERT INTO bridge_enrollments")) {
      const [
        request_id,
        config_name,
        public_key,
        fingerprint,
        hostname,
        platform,
        version,
        created_at,
        expires_at,
      ] = this.values;
      this.db.enrollments.push({
        request_id,
        config_name,
        public_key,
        fingerprint,
        hostname,
        platform,
        version,
        created_at,
        expires_at,
        status: "pending",
        decided_at: null,
        bridge_id: null,
      });
      return { meta: { changes: 1 } };
    }
    throw new Error(`unexpected run SQL: ${this.sql}`);
  }
}
