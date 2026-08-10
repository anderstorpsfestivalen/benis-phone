import assert from "node:assert/strict";
import test from "node:test";

import { validateSIPDefinition } from "../src/lib/sip-validation.ts";

function inboundDefinition(username) {
  return {
    sip: {
      connection: [
        {
          id: "asterisk-trunk",
          name: "Asterisk trunk",
          kind: "endpoint",
          registration: "inbound",
          username,
          transport: "udp",
          local_port: 5062,
          allowed_cidrs: ["192.0.2.10/32"],
          entrypoint: "main",
          route: [],
        },
      ],
    },
    fn: [{ name: "main" }],
  };
}

test("inbound SIP connections require an explicit authentication username", () => {
  assert.ok(
    validateSIPDefinition(inboundDefinition("")).some((error) =>
      error.includes("authentication username"),
    ),
  );
  assert.deepEqual(validateSIPDefinition(inboundDefinition("asterisk")), []);
});
