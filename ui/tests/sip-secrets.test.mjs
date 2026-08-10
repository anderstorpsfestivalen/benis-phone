import assert from "node:assert/strict";
import test from "node:test";

import {
  credentialState,
  decryptCredentialBundle,
  emptyCredentialBundle,
  encryptCredentialBundle,
  filterSIPSecrets,
  staleSIPSecrets,
} from "../worker/lib/secrets.ts";

const rows = [
  {
    config_name: "festival",
    connection_id: "registered",
    version: 1,
    iv: "iv",
    ciphertext: "secret",
    updated_at: 1,
  },
  {
    config_name: "festival",
    connection_id: "inbound",
    version: 1,
    iv: "iv",
    ciphertext: "unused",
    updated_at: 1,
  },
];

test("inbound and removed connections are selected for secret deletion", () => {
  assert.deepEqual(
    staleSIPSecrets(rows, new Set(["registered"])).map(
      (row) => row.connection_id,
    ),
    ["inbound"],
  );
});

test("runtime bundles include only explicitly allowed registered secrets", () => {
  assert.deepEqual(
    filterSIPSecrets(rows, new Set(["registered"])).map(
      (row) => row.connection_id,
    ),
    ["registered"],
  );
  assert.deepEqual(filterSIPSecrets(rows, new Set()), []);
});

test("credential bundles use the existing encryption master and expose state without plaintext", async () => {
  const encryptionMaster = Buffer.alloc(32, 7).toString("base64");
  const bundle = emptyCredentialBundle();
  bundle.r2_access_key = "write-only-access";
  bundle.http_password = "write-only-password";
  const encrypted = await encryptCredentialBundle(
    encryptionMaster,
    "festival",
    bundle,
  );
  assert.equal(encrypted.version, 1);
  assert.equal(encrypted.ciphertext.includes("write-only"), false);
  const restored = await decryptCredentialBundle(encryptionMaster, {
    config_name: "festival",
    updated_at: Date.now(),
    ...encrypted,
  });
  assert.deepEqual(restored, bundle);
  const state = credentialState(restored);
  assert.equal(state.r2_access_key, true);
  assert.equal(state.http_password, true);
  assert.equal(state.elevenlabs_api_key, false);
  assert.equal(JSON.stringify(state).includes("write-only"), false);
});
