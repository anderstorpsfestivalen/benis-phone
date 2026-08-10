import assert from "node:assert/strict";
import test from "node:test";

import { handleRegistrationsApi } from "../worker/handlers/registrations.ts";

test("approval requires an exact fingerprint and binds one bridge to the enrollment config", async () => {
  const enrollment = pendingEnrollment();
  const env = registrationEnv(enrollment);
  const mismatch = await handleRegistrationsApi(
    jsonRequest(`/api/registrations/${enrollment.request_id}/approve`, {
      fingerprint: "wrong",
    }),
    env,
    `/api/registrations/${enrollment.request_id}/approve`,
  );
  assert.equal(mismatch.status, 400);
  assert.equal(env.DB.bridges.length, 0);

  const approved = await handleRegistrationsApi(
    jsonRequest(`/api/registrations/${enrollment.request_id}/approve`, {
      fingerprint: enrollment.fingerprint,
    }),
    env,
    `/api/registrations/${enrollment.request_id}/approve`,
  );
  assert.equal(approved.status, 200);
  assert.equal(enrollment.status, "approved");
  assert.equal(env.DB.bridges.length, 1);
  assert.equal(env.DB.bridges[0].config_name, enrollment.config_name);
  assert.equal(env.DB.bridges[0].public_key, enrollment.public_key);
});

test("pending enrollment can be denied and an expired enrollment cannot be approved", async () => {
  const deniedEnrollment = pendingEnrollment();
  const deniedEnv = registrationEnv(deniedEnrollment);
  const denied = await handleRegistrationsApi(
    jsonRequest(`/api/registrations/${deniedEnrollment.request_id}/deny`, {}),
    deniedEnv,
    `/api/registrations/${deniedEnrollment.request_id}/deny`,
  );
  assert.equal(denied.status, 200);
  assert.equal(deniedEnrollment.status, "denied");

  const expiredEnrollment = pendingEnrollment();
  expiredEnrollment.expires_at = Date.now() - 1;
  const expiredEnv = registrationEnv(expiredEnrollment);
  const expired = await handleRegistrationsApi(
    jsonRequest(`/api/registrations/${expiredEnrollment.request_id}/approve`, {
      fingerprint: expiredEnrollment.fingerprint,
    }),
    expiredEnv,
    `/api/registrations/${expiredEnrollment.request_id}/approve`,
  );
  assert.equal(expired.status, 400);
  assert.equal(expiredEnrollment.status, "expired");
  assert.equal(expiredEnv.DB.bridges.length, 0);
});

test("revocation marks the bridge and asks the broker to close it immediately", async () => {
  const enrollment = pendingEnrollment();
  const env = registrationEnv(enrollment);
  const bridge = {
    bridge_id: crypto.randomUUID(),
    config_name: enrollment.config_name,
    public_key: enrollment.public_key,
    fingerprint: enrollment.fingerprint,
    approved_at: Date.now(),
    revoked_at: null,
  };
  env.DB.bridges.push(bridge);
  const response = await handleRegistrationsApi(
    jsonRequest(`/api/bridges/${bridge.bridge_id}/revoke`, {}),
    env,
    `/api/bridges/${bridge.bridge_id}/revoke`,
  );
  assert.equal(response.status, 200);
  assert.equal(typeof bridge.revoked_at, "number");
  assert.equal(env.brokerRequests.length, 1);
  assert.match(
    env.brokerRequests[0],
    new RegExp(`bridge_id=${bridge.bridge_id}`),
  );
});

function pendingEnrollment() {
  return {
    request_id: crypto.randomUUID(),
    config_name: "festival",
    public_key: "public-key",
    fingerprint: "aa:bb:cc",
    hostname: "bridge.test",
    platform: "linux/arm64",
    version: "test",
    created_at: Date.now(),
    expires_at: Date.now() + 60_000,
    status: "pending",
    decided_at: null,
    bridge_id: null,
  };
}

function jsonRequest(path, body) {
  return new Request(`https://worker.test${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

function registrationEnv(enrollment) {
  const brokerRequests = [];
  return {
    DB: new RegistrationDB(enrollment),
    brokerRequests,
    CONFIG_BROKER: {
      idFromName: () => "broker-id",
      get: () => ({
        fetch: async (url) => {
          brokerRequests.push(String(url));
          return Response.json({ ok: true });
        },
      }),
    },
  };
}

class RegistrationDB {
  constructor(enrollment) {
    this.enrollments = [enrollment];
    this.bridges = [];
  }

  prepare(sql) {
    return new RegistrationStatement(this, sql.replace(/\s+/g, " ").trim());
  }

  async batch(statements) {
    return Promise.all(statements.map((statement) => statement.run()));
  }
}

class RegistrationStatement {
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
    if (
      this.sql.includes("SELECT * FROM bridge_enrollments WHERE request_id")
    ) {
      return (
        this.db.enrollments.find((row) => row.request_id === this.values[0]) ??
        null
      );
    }
    throw new Error(`unexpected first SQL: ${this.sql}`);
  }

  async run() {
    if (
      this.sql.startsWith("UPDATE bridge_enrollments SET status = 'expired'")
    ) {
      const row = this.db.enrollments.find(
        (item) => item.request_id === this.values[0],
      );
      if (row) row.status = "expired";
      return { meta: { changes: row ? 1 : 0 } };
    }
    if (
      this.sql.startsWith("UPDATE bridge_enrollments SET status = 'denied'")
    ) {
      const row = this.db.enrollments.find(
        (item) => item.request_id === this.values[1],
      );
      if (row) {
        row.status = "denied";
        row.decided_at = this.values[0];
      }
      return { meta: { changes: row ? 1 : 0 } };
    }
    if (this.sql.startsWith("INSERT INTO bridges")) {
      const [bridge_id, config_name, public_key, fingerprint, approved_at] =
        this.values;
      this.db.bridges.push({
        bridge_id,
        config_name,
        public_key,
        fingerprint,
        approved_at,
        revoked_at: null,
      });
      return { meta: { changes: 1 } };
    }
    if (this.sql.includes("SET status = 'approved'")) {
      const row = this.db.enrollments.find(
        (item) => item.request_id === this.values[2],
      );
      if (row) {
        row.status = "approved";
        row.decided_at = this.values[0];
        row.bridge_id = this.values[1];
      }
      return { meta: { changes: row ? 1 : 0 } };
    }
    if (this.sql.startsWith("UPDATE bridges SET revoked_at")) {
      const row = this.db.bridges.find(
        (item) => item.bridge_id === this.values[1] && item.revoked_at === null,
      );
      if (row) row.revoked_at = this.values[0];
      return { meta: { changes: row ? 1 : 0 } };
    }
    throw new Error(`unexpected run SQL: ${this.sql}`);
  }
}
