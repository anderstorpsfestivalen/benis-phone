import assert from "node:assert/strict";
import test from "node:test";

import { applyJSONPatch } from "../worker/lib/config-changes.ts";

test("restricted JSON Patch applies object and array edits without mutating input", () => {
  const input = {
    general: { default_tts_voice: "Maja" },
    fn: [{ name: "main" }],
  };
  const result = applyJSONPatch(input, [
    { op: "test", path: "/general/default_tts_voice", value: "Maja" },
    { op: "replace", path: "/general/default_tts_voice", value: "Elin" },
    { op: "add", path: "/fn/-", value: { name: "after-hours" } },
    { op: "remove", path: "/fn/0" },
  ]);
  assert.deepEqual(input, {
    general: { default_tts_voice: "Maja" },
    fn: [{ name: "main" }],
  });
  assert.deepEqual(result.value, {
    general: { default_tts_voice: "Elin" },
    fn: [{ name: "after-hours" }],
  });
  assert.deepEqual(
    result.diff.map((item) => [item.op, item.path]),
    [
      ["test", "/general/default_tts_voice"],
      ["replace", "/general/default_tts_voice"],
      ["add", "/fn/-"],
      ["remove", "/fn/0"],
    ],
  );
});

test("restricted JSON Patch rejects unsafe paths, missing values, and failed tests", () => {
  assert.throws(
    () =>
      applyJSONPatch({ general: {} }, [
        { op: "add", path: "/__proto__/polluted", value: true },
      ]),
    /unsafe path/,
  );
  assert.throws(
    () =>
      applyJSONPatch({ general: {} }, [
        { op: "replace", path: "/general/missing", value: true },
      ]),
    /missing value/,
  );
  assert.throws(
    () =>
      applyJSONPatch({ enabled: false }, [
        { op: "test", path: "/enabled", value: true },
      ]),
    /test operation .* failed/,
  );
});

test("root replacement supports audited rollback while root removal is rejected", () => {
  assert.deepEqual(
    applyJSONPatch({ old: true }, [
      { op: "replace", path: "", value: { restored: true } },
    ]).value,
    { restored: true },
  );
  assert.throws(
    () => applyJSONPatch({ old: true }, [{ op: "remove", path: "" }]),
    /root cannot be removed/,
  );
});
