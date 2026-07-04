import type { Definition, Fn, Action, Queue, QueuePrompt } from "../generated/config";

// Renders the editor's JSON doc into the exact TOML shape that
// BurntSushi/toml parses back into core/functions.Definition. The goal is
// strict roundtrip: encode -> Go decode -> identical Definition. We do
// these things smol-toml doesn't on its own (smol-toml's stringify only
// emits standard / section-based TOML, never inline tables):
//   1. Emit [[fn]] and [[queue]] as array-of-tables sections.
//   2. Drop empty / zero-value branches so we don't pollute TOML with
//      noise like `wait = false` on every action.
//   3. Emit `actions = [ {...}, {...} ]` as inline-table arrays to match
//      the hand-authored TOML style under configurations/.
//   4. Use triple-quoted literal strings ("""...""") for any value that
//      contains a newline so the result stays readable.

export function renderToml(d: Definition): string {
  const parts: string[] = [];

  if (notEmpty(d.general)) {
    parts.push("[general]\n" + kv(prune(d.general) as unknown as Record<string, unknown>));
  }
  if (notEmpty(d.sip)) {
    parts.push("[sip]\n" + kv(prune(d.sip) as unknown as Record<string, unknown>));
  }
  for (const fn of d.fn ?? []) {
    parts.push(renderFn(fn));
  }
  for (const q of d.queue ?? []) {
    parts.push(renderQueue(q));
  }
  return parts.join("\n\n") + "\n";
}

function renderFn(fn: Fn): string {
  const body: string[] = ["[[fn]]"];
  if (fn.name) body.push(`name = ${formatString(fn.name)}`);
  if (notEmpty(fn.recording)) body.push(`recording = ${inline(prune(fn.recording))}`);
  if (notEmpty(fn.prefix)) body.push(`prefix = ${inline(prune(fn.prefix))}`);
  if (notEmpty(fn.gate)) body.push(`gate = ${inline(prune(fn.gate))}`);
  if (fn.clear_callstack) body.push(`clear_callstack = true`);
  if (fn.inputlength && fn.inputlength > 0) body.push(`inputlength = ${fn.inputlength}`);
  if (fn.actions && fn.actions.length) {
    body.push("actions = [");
    body.push(
      fn.actions.map((a) => "    " + inline(prune(actionForToml(a)))).join(",\n"),
    );
    body.push("]");
  }
  return body.join("\n");
}

function actionForToml(a: Action): Record<string, unknown> {
  return {
    num: a.num,
    wait: a.wait,
    clear: a.clear,
    name: a.name,
    prefix: a.prefix,
    pmsg: a.pmsg,
    dst: a.dst,
    file: a.file,
    randomfile: a.randomfile,
    tts: a.tts,
    srv: a.srv,
    dispatcher: a.dispatcher,
    transfer: a.transfer,
    hangup: a.hangup,
    record: a.record,
    record_to: a.record_to,
    dtmf: a.dtmf,
    livefeed: a.livefeed ?? undefined,
    genericjson: a.genericjson,
    script: a.script,
    then: a.then,
    auto: a.auto,
  };
}

function renderQueue(q: Queue): string {
  const body: string[] = ["[[queue]]"];
  if (q.name) body.push(`name = ${formatString(q.name)}`);
  if (notEmpty(q.entrymsg)) body.push(`entrymsg = ${inline(prune(q.entrymsg))}`);
  if (q.minpos) body.push(`minpos = ${q.minpos}`);
  if (q.maxpos) body.push(`maxpos = ${q.maxpos}`);
  if (q.speed) body.push(`speed = ${q.speed}`);
  if (q.minprompt) body.push(`minprompt = ${q.minprompt}`);
  if (q.maxprompt) body.push(`maxprompt = ${q.maxprompt}`);
  if (notEmpty(q.currentpos)) body.push(`currentpos = ${inline(prune(q.currentpos))}`);
  if (notEmpty(q.bgmusic)) body.push(`bgmusic = ${inline(prune(q.bgmusic))}`);
  if (notEmpty(q.end)) body.push(`end = ${inline(prune(actionForToml(q.end)))}`);
  for (const p of q.prompt ?? []) {
    body.push(renderQueuePrompt(p));
  }
  return body.join("\n");
}

function renderQueuePrompt(p: QueuePrompt): string {
  const body: string[] = ["[[queue.prompt]]"];
  if (notEmpty(p.prompt)) body.push(`prompt = ${inline(prune(p.prompt))}`);
  if (p.empty) body.push(`empty = true`);
  if (p.weight) body.push(`weight = ${p.weight}`);
  return body.join("\n");
}

// kv writes top-level key=value pairs (used inside [general], [sip]).
function kv(o: Record<string, unknown>): string {
  const lines: string[] = [];
  for (const [k, v] of Object.entries(o)) {
    lines.push(`${k} = ${tomlValue(v)}`);
  }
  return lines.join("\n");
}

// inline emits a value in inline-TOML form: strings become quoted; arrays
// become `[v1, v2]`; objects become `{ k1 = v1, k2 = v2 }`. Multiline
// strings get triple-quoted literal-string form.
function inline(v: unknown): string {
  return tomlValue(v);
}

function tomlValue(v: unknown): string {
  if (v === null || v === undefined) return '""';
  if (typeof v === "string") return formatString(v);
  if (typeof v === "number") return Number.isFinite(v) ? String(v) : "0";
  if (typeof v === "boolean") return v ? "true" : "false";
  if (Array.isArray(v)) {
    if (v.length === 0) return "[]";
    return "[" + v.map((x) => tomlValue(x)).join(", ") + "]";
  }
  if (typeof v === "object") {
    const entries = Object.entries(v as Record<string, unknown>);
    if (entries.length === 0) return "{}";
    return (
      "{ " +
      entries.map(([k, val]) => `${k} = ${tomlValue(val)}`).join(", ") +
      " }"
    );
  }
  return '""';
}

// formatString picks single-line "..." vs triple-quoted """...""" based on
// content. Single-line strings get JSON-escaped (which is a subset of
// TOML's basic string syntax: \n, \t, \r, \", \\, and \uXXXX all match).
function formatString(s: string): string {
  if (!s.includes("\n") && !s.includes("\r")) return JSON.stringify(s);
  // Multi-line literal-ish form. TOML """...""" is a basic multi-line
  // string: \ and the same escapes apply, but newlines and " are allowed
  // verbatim (except for """ which we have to escape).
  const escaped = s.replace(/\\/g, "\\\\").replace(/"""/g, '"\\"\\"');
  return `"""${escaped}"""`;
}

// prune removes fields whose values are empty strings, zero, false, empty
// arrays, or fully-empty nested objects. This matches the BurntSushi/toml
// "omitempty" mindset and keeps rendered TOML close to what a human
// would have written by hand.
function prune<T>(x: T): T {
  if (x === null || x === undefined) return x;
  if (Array.isArray(x)) {
    const arr = x.map(prune).filter((v) => !isEmpty(v));
    return arr as unknown as T;
  }
  if (typeof x === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(x as Record<string, unknown>)) {
      const pv = prune(v);
      if (!isEmpty(pv)) out[k] = pv;
    }
    return out as T;
  }
  return x;
}

function isEmpty(v: unknown): boolean {
  if (v === null || v === undefined) return true;
  // Strings: treat whitespace-only as empty so e.g. a `name = "   "`
  // doesn't survive prune and bloat the TOML output. The runtime
  // ignores leading/trailing whitespace on labels anyway.
  if (typeof v === "string") return v.trim() === "";
  if (typeof v === "number") return v === 0;
  if (typeof v === "boolean") return v === false;
  if (Array.isArray(v)) return v.length === 0;
  if (typeof v === "object") return Object.keys(v).length === 0;
  return false;
}

function notEmpty(v: unknown): boolean {
  return !isEmpty(prune(v));
}
