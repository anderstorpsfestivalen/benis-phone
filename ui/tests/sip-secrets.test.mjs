import assert from "node:assert/strict";
import test from "node:test";

import { filterSIPSecrets, staleSIPSecrets } from "../worker/lib/secrets.ts";

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
