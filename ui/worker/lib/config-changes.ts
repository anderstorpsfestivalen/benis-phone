import { definitionSchema } from "../../src/generated/schemas.ts";
import type { Definition } from "../../src/generated/config.ts";
import { validateSIPDefinition } from "../../src/lib/sip-validation.ts";
import { renderToml } from "../../src/lib/toml-render.ts";
import type { Env } from "./auth.ts";
import { notifyConfigChanged } from "./config-broker.ts";
import { getConfig, type ConfigRow } from "./db.ts";
import { sha256Hex } from "./hash.ts";
import {
  deleteSIPSecretStatement,
  listSIPSecrets,
  staleSIPSecrets,
} from "./secrets.ts";

export type ConfigActor = {
  kind: "mcp" | "human";
  id: string;
  label: string;
};

export type ConfigPatchOperation = {
  op: "add" | "remove" | "replace" | "test";
  path: string;
  value?: unknown;
};

export type ConfigDiffItem = {
  op: ConfigPatchOperation["op"];
  path: string;
  before?: unknown;
  after?: unknown;
};

export interface ConfigChangeSummary {
  change_id: string;
  config_name: string;
  actor_kind: ConfigActor["kind"];
  actor_id: string;
  actor_label: string;
  before_hash: string;
  after_hash: string;
  patch: ConfigPatchOperation[];
  diff: ConfigDiffItem[];
  source_change_id: string | null;
  created_at: number;
}

export interface ConfigChangeDetail extends ConfigChangeSummary {
  before_doc: Definition;
  after_doc: Definition;
}

type PreparedChange = {
  row: ConfigRow;
  doc: Definition;
  toml: string;
  hash: string;
  secretRevision: number;
  removedSecretIDs: string[];
  patch: ConfigPatchOperation[];
  diff: ConfigDiffItem[];
};

export class ConfigChangeError extends Error {
  readonly code: "not_found" | "conflict" | "invalid_patch" | "validation";
  readonly errors: string[];

  constructor(
    code: "not_found" | "conflict" | "invalid_patch" | "validation",
    message: string,
    errors: string[] = [message],
  ) {
    super(message);
    this.code = code;
    this.errors = errors;
  }
}

const UNSAFE_SEGMENTS = new Set(["__proto__", "prototype", "constructor"]);
const MAX_PATCH_OPERATIONS = 100;
const MAX_PATCH_BYTES = 256 * 1024;

export async function validateConfigPatch(
  env: Env,
  name: string,
  baseHash: string,
  patch: ConfigPatchOperation[],
) {
  const prepared = await preparePatch(env, name, baseHash, patch);
  return preview(prepared);
}

export async function applyConfigPatch(
  env: Env,
  ctx: ExecutionContext,
  name: string,
  baseHash: string,
  patch: ConfigPatchOperation[],
  actor: ConfigActor,
) {
  const prepared = await preparePatch(env, name, baseHash, patch);
  return commitPrepared(env, ctx, name, baseHash, prepared, actor, null);
}

export async function rollbackConfigChange(
  env: Env,
  ctx: ExecutionContext,
  name: string,
  baseHash: string,
  sourceChangeID: string,
  actor: ConfigActor,
) {
  const source = await getConfigChange(env, name, sourceChangeID);
  if (!source)
    throw new ConfigChangeError("not_found", "config change not found");
  const patch: ConfigPatchOperation[] = [
    { op: "replace", path: "", value: source.before_doc },
  ];
  const prepared = await preparePatch(env, name, baseHash, patch);
  return commitPrepared(
    env,
    ctx,
    name,
    baseHash,
    prepared,
    actor,
    sourceChangeID,
  );
}

export async function listConfigChanges(
  env: Env,
  name: string,
  limit = 50,
): Promise<ConfigChangeSummary[]> {
  const safeLimit = Math.max(1, Math.min(100, Math.floor(limit)));
  const result = await env.DB.prepare(
    `SELECT change_id, config_name, actor_kind, actor_id, actor_label,
            before_hash, after_hash, patch, diff, source_change_id, created_at
     FROM config_changes WHERE config_name = ?
     ORDER BY created_at DESC LIMIT ?`,
  )
    .bind(name, safeLimit)
    .all<
      Omit<ConfigChangeSummary, "patch" | "diff"> & {
        patch: string;
        diff: string;
      }
    >();
  return (result.results ?? []).map(parseSummary);
}

export async function getConfigChange(
  env: Env,
  name: string,
  changeID: string,
): Promise<ConfigChangeDetail | null> {
  const row = await env.DB.prepare(
    `SELECT change_id, config_name, actor_kind, actor_id, actor_label,
            before_hash, after_hash, patch, diff, before_doc, after_doc,
            source_change_id, created_at
     FROM config_changes WHERE config_name = ? AND change_id = ?`,
  )
    .bind(name, changeID)
    .first<
      Omit<
        ConfigChangeDetail,
        "patch" | "diff" | "before_doc" | "after_doc"
      > & {
        patch: string;
        diff: string;
        before_doc: string;
        after_doc: string;
      }
    >();
  if (!row) return null;
  return {
    ...parseSummary(row),
    before_doc: JSON.parse(row.before_doc) as Definition,
    after_doc: JSON.parse(row.after_doc) as Definition,
  };
}

async function preparePatch(
  env: Env,
  name: string,
  baseHash: string,
  patch: ConfigPatchOperation[],
): Promise<PreparedChange> {
  const row = await getConfig(env, name);
  if (!row) throw new ConfigChangeError("not_found", "config not found");
  if (row.hash !== baseHash) {
    throw new ConfigChangeError(
      "conflict",
      `config changed since it was read; current hash is ${row.hash}`,
    );
  }
  let applied: { value: unknown; diff: ConfigDiffItem[] };
  try {
    applied = applyJSONPatch(JSON.parse(row.doc) as unknown, patch);
  } catch (caught) {
    throw new ConfigChangeError(
      "invalid_patch",
      caught instanceof Error ? caught.message : String(caught),
    );
  }
  const parsed = definitionSchema.safeParse(applied.value);
  if (!parsed.success) {
    throw new ConfigChangeError(
      "validation",
      "patched config does not match the config schema",
      parsed.error.issues.map(
        (issue) => `${issue.path.join(".") || "config"}: ${issue.message}`,
      ),
    );
  }
  const doc = parsed.data as Definition;
  const errors = validateSIPDefinition(doc);
  const connectionIDs = new Set(doc.sip.connection.map((item) => item.id));
  const secrets = await listSIPSecrets(env, name);
  const secretIDs = new Set(secrets.map((secret) => secret.connection_id));
  for (const connection of doc.sip.connection) {
    if (!secretIDs.has(connection.id)) {
      errors.push(
        `${connection.name || connection.id}: a SIP password is required`,
      );
    }
  }
  if (errors.length) {
    throw new ConfigChangeError(
      "validation",
      "patched config failed validation",
      [...new Set(errors)],
    );
  }
  const removedSecrets = staleSIPSecrets(secrets, connectionIDs);
  const secretRevision = row.secret_revision + (removedSecrets.length ? 1 : 0);
  const toml = renderToml(doc);
  const hash = await sha256Hex(`${toml}\nsecret-revision:${secretRevision}`);
  return {
    row,
    doc,
    toml,
    hash,
    secretRevision,
    removedSecretIDs: removedSecrets.map((secret) => secret.connection_id),
    patch,
    diff: applied.diff,
  };
}

async function commitPrepared(
  env: Env,
  ctx: ExecutionContext,
  name: string,
  baseHash: string,
  prepared: PreparedChange,
  actor: ConfigActor,
  sourceChangeID: string | null,
) {
  const changeID = crypto.randomUUID();
  const now = Date.now();
  const statements: D1PreparedStatement[] = [
    env.DB.prepare(
      `UPDATE configs
       SET doc = ?, toml = ?, hash = ?, secret_revision = ?,
           updated_at = ?, last_change_id = ?
       WHERE name = ? AND hash = ?`,
    ).bind(
      JSON.stringify(prepared.doc),
      prepared.toml,
      prepared.hash,
      prepared.secretRevision,
      now,
      changeID,
      name,
      baseHash,
    ),
  ];
  for (const connectionID of prepared.removedSecretIDs) {
    statements.push(deleteSIPSecretStatement(env, name, connectionID));
  }
  statements.push(
    env.DB.prepare(
      `INSERT INTO config_changes
       (change_id, config_name, actor_kind, actor_id, actor_label,
        before_hash, after_hash, patch, diff, before_doc, after_doc,
        source_change_id, created_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    ).bind(
      changeID,
      name,
      actor.kind,
      actor.id.slice(0, 255),
      actor.label.slice(0, 255),
      baseHash,
      prepared.hash,
      JSON.stringify(prepared.patch),
      JSON.stringify(prepared.diff),
      prepared.row.doc,
      JSON.stringify(prepared.doc),
      sourceChangeID,
      now,
    ),
  );
  try {
    await env.DB.batch(statements);
  } catch (caught) {
    const current = await getConfig(env, name);
    if (!current || current.hash !== baseHash) {
      throw new ConfigChangeError(
        "conflict",
        `config changed since it was read${current ? `; current hash is ${current.hash}` : ""}`,
      );
    }
    throw caught;
  }
  notifyConfigChanged(env, ctx, name, prepared.hash);
  return {
    ...preview(prepared),
    change_id: changeID,
    created_at: now,
  };
}

function preview(prepared: PreparedChange) {
  return {
    valid: true as const,
    name: prepared.row.name,
    base_hash: prepared.row.hash,
    resulting_hash: prepared.hash,
    doc: prepared.doc,
    toml: prepared.toml,
    diff: prepared.diff,
  };
}

export function applyJSONPatch(
  input: unknown,
  patch: ConfigPatchOperation[],
): { value: unknown; diff: ConfigDiffItem[] } {
  if (!Array.isArray(patch)) throw new Error("patch must be an array");
  if (patch.length === 0)
    throw new Error("patch must contain at least one operation");
  if (patch.length > MAX_PATCH_OPERATIONS)
    throw new Error(
      `patch may contain at most ${MAX_PATCH_OPERATIONS} operations`,
    );
  if (JSON.stringify(patch).length > MAX_PATCH_BYTES)
    throw new Error(`patch may be at most ${MAX_PATCH_BYTES} bytes`);
  let value = cloneJSON(input);
  const diff: ConfigDiffItem[] = [];
  for (const [index, operation] of patch.entries()) {
    if (
      !operation ||
      !["add", "remove", "replace", "test"].includes(operation.op)
    )
      throw new Error(`patch operation ${index} is not supported`);
    const segments = pointerSegments(operation.path);
    if (segments.some((segment) => UNSAFE_SEGMENTS.has(segment)))
      throw new Error(`patch operation ${index} contains an unsafe path`);
    if (segments.length === 0) {
      const before = cloneJSON(value);
      if (operation.op === "remove")
        throw new Error("the config root cannot be removed");
      if (operation.op === "test") {
        if (!deepEqual(value, operation.value))
          throw new Error(
            `test operation ${index} failed at the document root`,
          );
        diff.push({ op: "test", path: "", before });
        continue;
      }
      requireValue(operation, index);
      value = cloneJSON(operation.value);
      diff.push({
        op: operation.op,
        path: "",
        before,
        after: cloneJSON(value),
      });
      continue;
    }
    const parent = resolveParent(value, segments, index);
    const key = segments[segments.length - 1];
    const before = existingValue(parent, key, operation.op, index);
    if (operation.op === "test") {
      requireValue(operation, index);
      if (!deepEqual(before, operation.value))
        throw new Error(`test operation ${index} failed at ${operation.path}`);
      diff.push({
        op: "test",
        path: operation.path,
        before: cloneJSON(before),
      });
      continue;
    }
    if (operation.op === "remove") {
      removeValue(parent, key, index);
      diff.push({
        op: "remove",
        path: operation.path,
        before: cloneJSON(before),
      });
      continue;
    }
    requireValue(operation, index);
    const after = cloneJSON(operation.value);
    setValue(parent, key, after, operation.op === "add", index);
    diff.push({
      op: operation.op,
      path: operation.path,
      ...(before !== undefined ? { before: cloneJSON(before) } : {}),
      after: cloneJSON(after),
    });
  }
  return { value, diff };
}

function pointerSegments(path: string): string[] {
  if (typeof path !== "string") throw new Error("patch path must be a string");
  if (path === "") return [];
  if (!path.startsWith("/")) throw new Error(`invalid JSON pointer ${path}`);
  return path
    .slice(1)
    .split("/")
    .map((segment) => {
      if (/~(?![01])/u.test(segment))
        throw new Error(`invalid JSON pointer ${path}`);
      return segment.replace(/~1/g, "/").replace(/~0/g, "~");
    });
}

function resolveParent(
  root: unknown,
  segments: string[],
  index: number,
): object {
  let current = root;
  for (const segment of segments.slice(0, -1)) {
    if (Array.isArray(current)) {
      const arrayIndex = arrayOffset(segment, current.length, false, index);
      current = current[arrayIndex];
    } else if (isRecord(current) && Object.hasOwn(current, segment)) {
      current = current[segment];
    } else {
      throw new Error(`patch operation ${index} refers to a missing parent`);
    }
  }
  if (!Array.isArray(current) && !isRecord(current))
    throw new Error(
      `patch operation ${index} parent is not an object or array`,
    );
  return current;
}

function existingValue(
  parent: object,
  key: string,
  op: ConfigPatchOperation["op"],
  index: number,
): unknown {
  if (Array.isArray(parent)) {
    if (op === "add" && key === "-") return undefined;
    const offset = arrayOffset(key, parent.length, op === "add", index);
    return parent[offset];
  }
  const record = parent as Record<string, unknown>;
  if (!Object.hasOwn(record, key) && op !== "add")
    throw new Error(`patch operation ${index} refers to a missing value`);
  return record[key];
}

function setValue(
  parent: object,
  key: string,
  value: unknown,
  add: boolean,
  index: number,
) {
  if (Array.isArray(parent)) {
    if (add && key === "-") {
      parent.push(value);
      return;
    }
    const offset = arrayOffset(key, parent.length, add, index);
    if (add) parent.splice(offset, 0, value);
    else parent[offset] = value;
    return;
  }
  (parent as Record<string, unknown>)[key] = value;
}

function removeValue(parent: object, key: string, index: number) {
  if (Array.isArray(parent)) {
    parent.splice(arrayOffset(key, parent.length, false, index), 1);
    return;
  }
  const record = parent as Record<string, unknown>;
  if (!Object.hasOwn(record, key))
    throw new Error(`patch operation ${index} refers to a missing value`);
  delete record[key];
}

function arrayOffset(
  key: string,
  length: number,
  allowEnd: boolean,
  index: number,
) {
  if (!/^(0|[1-9]\d*)$/.test(key))
    throw new Error(`patch operation ${index} has an invalid array index`);
  const offset = Number(key);
  if (
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    offset > length ||
    (!allowEnd && offset === length)
  )
    throw new Error(`patch operation ${index} array index is out of range`);
  return offset;
}

function requireValue(operation: ConfigPatchOperation, index: number) {
  if (!("value" in operation))
    throw new Error(`patch operation ${index} requires a value`);
}

function cloneJSON<T>(value: T): T {
  if (value === undefined) return value;
  return JSON.parse(JSON.stringify(value)) as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function deepEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right))
    return (
      left.length === right.length &&
      left.every((item, index) => deepEqual(item, right[index]))
    );
  if (isRecord(left) && isRecord(right)) {
    const leftKeys = Object.keys(left);
    const rightKeys = Object.keys(right);
    return (
      leftKeys.length === rightKeys.length &&
      leftKeys.every(
        (key) => Object.hasOwn(right, key) && deepEqual(left[key], right[key]),
      )
    );
  }
  return false;
}

function parseSummary(
  row: Omit<ConfigChangeSummary, "patch" | "diff"> & {
    patch: string;
    diff: string;
  },
): ConfigChangeSummary {
  return {
    ...row,
    patch: JSON.parse(row.patch) as ConfigPatchOperation[],
    diff: JSON.parse(row.diff) as ConfigDiffItem[],
  };
}
