// Browser-side preview of a GenericJSON node's template. Not a faithful
// port of Go text/template — covers the action/pipeline/range/if subset
// that authors actually write, plus the helper functions exposed in
// core/functions/genericjson.go (genericJSONFuncs). jq queries are
// delegated to jq-wasm so jq semantics match the runtime exactly.
//
// Drift caveats (kept honest because callers see the rendered string
// labelled "preview"):
//   - Number formatting uses JS Number.toString, which agrees with Go's
//     strconv.FormatFloat(_, 'g', -1, 64) for all values the IVR will
//     read aloud in practice but differs on extreme magnitudes.
//   - {{with}}, comments {{/* */}}, variables ($x), template inheritance
//     ({{define}} / {{template}}) are not supported.
//   - Method calls on values aren't supported — only field access on
//     map-shaped data, which is exactly what the JSON decoder produces.
//
// Status is fixed at 200 in the preview context — we don't surface the
// upstream HTTP status here because the upstream-failure path returns
// before render at runtime anyway.

import * as jq from "jq-wasm";

export type RenderResult =
  | { ok: true; rendered: string }
  | { ok: false; error: string };

export async function renderGenericJSONTemplate(
  jsonBody: string,
  template: string,
  status: number,
): Promise<RenderResult> {
  let data: unknown = null;
  const trimmed = jsonBody.trim();
  if (trimmed) {
    try {
      data = JSON.parse(trimmed);
    } catch (e) {
      return { ok: false, error: `JSON parse: ${e instanceof Error ? e.message : String(e)}` };
    }
  }
  // Vars is empty in preview — flow state only exists at call time. Providing
  // the key keeps templates that reference {{.Vars.*}} from erroring here.
  const ctx: Ctx = { Data: data, Status: status, Raw: jsonBody, Vars: {} };
  try {
    const nodes = parseTemplate(template);
    const out = await renderNodes(nodes, ctx, ctx);
    return { ok: true, rendered: out };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
}

// -------------------------------------------------------------------- AST

type Ctx = { Data: unknown; Status: number; Raw: string; Vars: Record<string, unknown> };

type Node =
  | { kind: "text"; text: string }
  | { kind: "action"; pipe: Pipeline }
  | { kind: "range"; pipe: Pipeline; body: Node[]; elseBody: Node[] }
  | { kind: "if"; pipe: Pipeline; thenBody: Node[]; elseBody: Node[] };

type Pipeline = { cmds: Cmd[] };
type Cmd = { ops: Operand[] };
type Operand =
  | { kind: "field"; path: string[] }
  | { kind: "str"; value: string }
  | { kind: "num"; value: number }
  | { kind: "bool"; value: boolean }
  | { kind: "ident"; name: string }
  | { kind: "pipe"; pipe: Pipeline };

// -------------------------------------------------------------------- Lex/Parse

// parseTemplate splits the template into text and action nodes, then
// recursively parses {{range}}/{{if}} block structure.
function parseTemplate(src: string): Node[] {
  const flat = splitFlat(src);
  const [nodes, end] = buildBlocks(flat, 0, null);
  if (end !== flat.length) {
    throw new Error("unexpected {{end}}");
  }
  return nodes;
}

type Flat =
  | { kind: "text"; text: string }
  | { kind: "action"; body: string }
  | { kind: "range"; body: string }
  | { kind: "if"; body: string }
  | { kind: "else" }
  | { kind: "end" };

function splitFlat(src: string): Flat[] {
  const out: Flat[] = [];
  let i = 0;
  let textStart = 0;
  while (i < src.length) {
    const open = src.indexOf("{{", i);
    if (open < 0) break;
    // Capture any text before the action
    let textEnd = open;
    let trimLead = false;
    // Lookahead for {{-
    let bodyStart = open + 2;
    if (src[bodyStart] === "-" && /\s/.test(src[bodyStart + 1] ?? "")) {
      trimLead = true;
      bodyStart += 1;
    }
    // Find closing }}
    const close = src.indexOf("}}", bodyStart);
    if (close < 0) throw new Error("unterminated {{");
    let bodyEnd = close;
    let trimTrail = false;
    if (src[bodyEnd - 1] === "-" && /\s/.test(src[bodyEnd - 2] ?? "")) {
      trimTrail = true;
      bodyEnd -= 1;
    }
    let text = src.slice(textStart, textEnd);
    if (trimLead) text = text.replace(/\s+$/, "");
    if (text.length > 0) out.push({ kind: "text", text });
    const body = src.slice(bodyStart, bodyEnd).trim();
    out.push(classifyAction(body));
    i = close + 2;
    textStart = i;
    if (trimTrail) {
      // Consume leading whitespace of the upcoming text
      while (textStart < src.length && /\s/.test(src[textStart])) textStart += 1;
      i = textStart;
    }
  }
  if (textStart < src.length) {
    out.push({ kind: "text", text: src.slice(textStart) });
  }
  return out;
}

function classifyAction(body: string): Flat {
  if (body === "end") return { kind: "end" };
  if (body === "else") return { kind: "else" };
  if (/^range\b/.test(body)) return { kind: "range", body: body.slice(5).trim() };
  if (/^if\b/.test(body)) return { kind: "if", body: body.slice(2).trim() };
  return { kind: "action", body };
}

// buildBlocks walks the flat list and groups range/if blocks recursively.
// stopAt: which terminator(s) end the current scope (null = top-level: only
// EOF; "end" = inside range/if; "elseOrEnd" = inside if's then-branch).
function buildBlocks(
  flat: Flat[],
  start: number,
  stopAt: null | "end" | "elseOrEnd",
): [Node[], number] {
  const out: Node[] = [];
  let i = start;
  while (i < flat.length) {
    const f = flat[i];
    if (f.kind === "end") {
      if (stopAt === null) throw new Error("unexpected {{end}}");
      return [out, i + 1];
    }
    if (f.kind === "else") {
      if (stopAt !== "elseOrEnd") throw new Error("unexpected {{else}}");
      return [out, i + 1];
    }
    if (f.kind === "text") {
      out.push({ kind: "text", text: f.text });
      i += 1;
      continue;
    }
    if (f.kind === "action") {
      out.push({ kind: "action", pipe: parsePipeline(f.body) });
      i += 1;
      continue;
    }
    if (f.kind === "range") {
      const pipe = parsePipeline(f.body);
      const [body, next] = buildBlocks(flat, i + 1, "elseOrEnd");
      // Did we stop on else or end? Check the consumed terminator.
      const terminator = flat[next - 1];
      let elseBody: Node[] = [];
      let after = next;
      if (terminator && terminator.kind === "else") {
        const [eb, after2] = buildBlocks(flat, next, "end");
        elseBody = eb;
        after = after2;
      }
      out.push({ kind: "range", pipe, body, elseBody });
      i = after;
      continue;
    }
    if (f.kind === "if") {
      const pipe = parsePipeline(f.body);
      const [thenBody, next] = buildBlocks(flat, i + 1, "elseOrEnd");
      const terminator = flat[next - 1];
      let elseBody: Node[] = [];
      let after = next;
      if (terminator && terminator.kind === "else") {
        const [eb, after2] = buildBlocks(flat, next, "end");
        elseBody = eb;
        after = after2;
      }
      out.push({ kind: "if", pipe, thenBody, elseBody });
      i = after;
      continue;
    }
  }
  if (stopAt !== null) throw new Error("unterminated block: missing {{end}}");
  return [out, i];
}

// parsePipeline tokenises a single action body and groups commands by `|`.
function parsePipeline(src: string): Pipeline {
  const toks = tokenize(src);
  // Split on top-level pipe tokens
  const cmds: Cmd[] = [];
  let depth = 0;
  let curr: Tok[] = [];
  for (const t of toks) {
    if (t.kind === "lparen") depth += 1;
    if (t.kind === "rparen") depth -= 1;
    if (t.kind === "pipe" && depth === 0) {
      cmds.push(parseCmd(curr));
      curr = [];
    } else {
      curr.push(t);
    }
  }
  cmds.push(parseCmd(curr));
  return { cmds };
}

type Tok =
  | { kind: "ident"; v: string }
  | { kind: "field"; v: string[] }
  | { kind: "str"; v: string }
  | { kind: "num"; v: number }
  | { kind: "bool"; v: boolean }
  | { kind: "lparen" }
  | { kind: "rparen" }
  | { kind: "pipe" };

function tokenize(src: string): Tok[] {
  const out: Tok[] = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (/\s/.test(c)) {
      i += 1;
      continue;
    }
    if (c === "(") {
      out.push({ kind: "lparen" });
      i += 1;
      continue;
    }
    if (c === ")") {
      out.push({ kind: "rparen" });
      i += 1;
      continue;
    }
    if (c === "|") {
      out.push({ kind: "pipe" });
      i += 1;
      continue;
    }
    if (c === ".") {
      // Field access: . OR .ident(.ident)*
      i += 1;
      const path: string[] = [];
      while (i < src.length && /[A-Za-z_]/.test(src[i])) {
        let start = i;
        while (i < src.length && /[A-Za-z0-9_]/.test(src[i])) i += 1;
        path.push(src.slice(start, i));
        if (src[i] === ".") i += 1;
        else break;
      }
      out.push({ kind: "field", v: path });
      continue;
    }
    if (c === '"' || c === "`") {
      const [str, ni] = readString(src, i, c);
      out.push({ kind: "str", v: str });
      i = ni;
      continue;
    }
    if (c === "-" || /[0-9]/.test(c)) {
      let start = i;
      if (c === "-") i += 1;
      while (i < src.length && /[0-9.eE+\-]/.test(src[i])) i += 1;
      const raw = src.slice(start, i);
      const n = Number(raw);
      if (Number.isNaN(n)) throw new Error(`bad number "${raw}"`);
      out.push({ kind: "num", v: n });
      continue;
    }
    if (/[A-Za-z_]/.test(c)) {
      let start = i;
      while (i < src.length && /[A-Za-z0-9_]/.test(src[i])) i += 1;
      const w = src.slice(start, i);
      if (w === "true") out.push({ kind: "bool", v: true });
      else if (w === "false") out.push({ kind: "bool", v: false });
      else if (w === "nil") out.push({ kind: "str", v: "" }); // best-effort
      else out.push({ kind: "ident", v: w });
      continue;
    }
    throw new Error(`unexpected character "${c}" in template action`);
  }
  return out;
}

function readString(src: string, i: number, quote: string): [string, number] {
  i += 1;
  let out = "";
  if (quote === "`") {
    // Go raw string — no escapes
    while (i < src.length && src[i] !== "`") {
      out += src[i];
      i += 1;
    }
    if (src[i] !== "`") throw new Error("unterminated raw string");
    return [out, i + 1];
  }
  while (i < src.length && src[i] !== '"') {
    if (src[i] === "\\") {
      i += 1;
      const e = src[i];
      switch (e) {
        case "n": out += "\n"; break;
        case "t": out += "\t"; break;
        case "r": out += "\r"; break;
        case "\\": out += "\\"; break;
        case '"': out += '"'; break;
        case "/": out += "/"; break;
        case "0": out += "\0"; break;
        default: out += e ?? "";
      }
      i += 1;
    } else {
      out += src[i];
      i += 1;
    }
  }
  if (src[i] !== '"') throw new Error("unterminated string");
  return [out, i + 1];
}

function parseCmd(toks: Tok[]): Cmd {
  // A command is one-or-more operands. Sub-pipelines `( ... )` collapse
  // to a single operand. We recursively descend so nested groups parse.
  const ops: Operand[] = [];
  let i = 0;
  while (i < toks.length) {
    const t = toks[i];
    if (t.kind === "lparen") {
      // Find matching rparen
      let depth = 1;
      let j = i + 1;
      while (j < toks.length && depth > 0) {
        if (toks[j].kind === "lparen") depth += 1;
        else if (toks[j].kind === "rparen") depth -= 1;
        if (depth === 0) break;
        j += 1;
      }
      if (depth !== 0) throw new Error("unmatched (");
      const inner = toks.slice(i + 1, j);
      // Inner can have pipes; parse as pipeline
      ops.push({ kind: "pipe", pipe: parsePipelineFromToks(inner) });
      i = j + 1;
      continue;
    }
    if (t.kind === "rparen") throw new Error("unexpected )");
    if (t.kind === "pipe") throw new Error("unexpected | inside command");
    switch (t.kind) {
      case "field": ops.push({ kind: "field", path: t.v }); break;
      case "str": ops.push({ kind: "str", value: t.v }); break;
      case "num": ops.push({ kind: "num", value: t.v }); break;
      case "bool": ops.push({ kind: "bool", value: t.v }); break;
      case "ident": ops.push({ kind: "ident", name: t.v }); break;
    }
    i += 1;
  }
  if (ops.length === 0) throw new Error("empty command");
  return { ops };
}

function parsePipelineFromToks(toks: Tok[]): Pipeline {
  const cmds: Cmd[] = [];
  let depth = 0;
  let curr: Tok[] = [];
  for (const t of toks) {
    if (t.kind === "lparen") depth += 1;
    if (t.kind === "rparen") depth -= 1;
    if (t.kind === "pipe" && depth === 0) {
      cmds.push(parseCmd(curr));
      curr = [];
    } else {
      curr.push(t);
    }
  }
  cmds.push(parseCmd(curr));
  return { cmds };
}

// -------------------------------------------------------------------- Eval

async function renderNodes(nodes: Node[], ctx: Ctx, dot: unknown): Promise<string> {
  let out = "";
  for (const n of nodes) {
    out += await renderNode(n, ctx, dot);
  }
  return out;
}

async function renderNode(n: Node, ctx: Ctx, dot: unknown): Promise<string> {
  switch (n.kind) {
    case "text":
      return n.text;
    case "action": {
      const v = await evalPipeline(n.pipe, ctx, dot);
      return formatValue(v);
    }
    case "if": {
      const v = await evalPipeline(n.pipe, ctx, dot);
      return isTruthy(v)
        ? renderNodes(n.thenBody, ctx, dot)
        : renderNodes(n.elseBody, ctx, dot);
    }
    case "range": {
      const v = await evalPipeline(n.pipe, ctx, dot);
      if (Array.isArray(v) && v.length > 0) {
        let out = "";
        for (const item of v) out += await renderNodes(n.body, ctx, item);
        return out;
      }
      if (v && typeof v === "object" && Object.keys(v as object).length > 0) {
        let out = "";
        for (const k of Object.keys(v as object)) {
          out += await renderNodes(n.body, ctx, (v as Record<string, unknown>)[k]);
        }
        return out;
      }
      return renderNodes(n.elseBody, ctx, dot);
    }
  }
}

async function evalPipeline(p: Pipeline, ctx: Ctx, dot: unknown): Promise<unknown> {
  let carry: unknown = undefined;
  let first = true;
  for (const cmd of p.cmds) {
    carry = await evalCmd(cmd, ctx, dot, first ? undefined : carry);
    first = false;
  }
  return carry;
}

async function evalCmd(cmd: Cmd, ctx: Ctx, dot: unknown, piped: unknown): Promise<unknown> {
  // Single operand: just evaluate it (with piped appended only if it's a
  // function reference — which we treat as a 0-arg call).
  if (cmd.ops.length === 1) {
    const op = cmd.ops[0];
    if (op.kind === "ident") {
      // Function reference, possibly piped into
      const args = piped !== undefined ? [piped] : [];
      return callHelper(op.name, args);
    }
    const v = await evalOperand(op, ctx, dot);
    return piped !== undefined ? piped : v;
  }
  // Multi-operand: first must be a function name. Remaining operands are
  // args. If we have a piped value, it appends as the LAST arg.
  const head = cmd.ops[0];
  if (head.kind !== "ident") {
    throw new Error("command with multiple operands must start with a function name");
  }
  const args: unknown[] = [];
  for (let i = 1; i < cmd.ops.length; i += 1) {
    args.push(await evalOperand(cmd.ops[i], ctx, dot));
  }
  if (piped !== undefined) args.push(piped);
  return callHelper(head.name, args);
}

async function evalOperand(op: Operand, ctx: Ctx, dot: unknown): Promise<unknown> {
  switch (op.kind) {
    case "str": return op.value;
    case "num": return op.value;
    case "bool": return op.value;
    case "field": return readField(dot, op.path);
    case "ident":
      // Function name used as a value with no args
      return callHelper(op.name, []);
    case "pipe":
      return evalPipeline(op.pipe, ctx, dot);
  }
}

function readField(dot: unknown, path: string[]): unknown {
  // {{.}} is dot itself; {{.Foo}} is dot.Foo. Top-level dot is the
  // ctx ({Data,Status,Raw}); inside range it rebinds to the iterated
  // item.
  let v: unknown = dot;
  for (const k of path) {
    if (v == null) return null;
    if (typeof v !== "object") return null;
    v = (v as Record<string, unknown>)[k];
  }
  return v;
}

// -------------------------------------------------------------------- Helpers

async function callHelper(name: string, args: unknown[]): Promise<unknown> {
  switch (name) {
    case "int":
    case "round":
      return toInt(args[0]);
    case "float":
    case "num":
      return toFloat(args[0]);
    case "str":
      return toString(args[0]);
    case "default": {
      const [fallback, value] = args;
      return isEmpty(value) ? fallback : value;
    }
    case "jq":
      return await jqFirst(args[0], args[1]);
    case "jqAll":
    case "jqall":
      return await jqAllFn(args[0], args[1]);
    case "first":
      return Array.isArray(args[0]) && args[0].length > 0 ? args[0][0] : null;
    case "last":
      return Array.isArray(args[0]) && args[0].length > 0 ? args[0][args[0].length - 1] : null;
    case "join":
      return joinFn(args[0] as string, args[1]);
    case "add":
      return toFloat(args[0]) + toFloat(args[1]);
    case "sub":
      return toFloat(args[0]) - toFloat(args[1]);
    case "mul":
      return toFloat(args[0]) * toFloat(args[1]);
    case "div": {
      const b = toFloat(args[1]);
      return b === 0 ? 0 : toFloat(args[0]) / b;
    }
    case "keys":
      return args[0] && typeof args[0] === "object" && !Array.isArray(args[0])
        ? Object.keys(args[0] as object)
        : [];
    case "length":
      return lengthFn(args[0]);
    default:
      throw new Error(`unknown helper "${name}"`);
  }
}

function toFloat(v: unknown): number {
  if (v == null) return 0;
  if (typeof v === "number") return v;
  if (typeof v === "boolean") return v ? 1 : 0;
  if (typeof v === "string") {
    const n = parseFloat(v.trim());
    return Number.isNaN(n) ? 0 : n;
  }
  return 0;
}

// Mirrors core/functions/genericjson.go toInt — round half away from zero,
// not the JS Math.round half-toward-+inf default.
function toInt(v: unknown): number {
  const f = toFloat(v);
  return f >= 0 ? Math.floor(f + 0.5) : Math.ceil(f - 0.5);
}

function toString(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "boolean") return v ? "true" : "false";
  if (typeof v === "number") return numberToString(v);
  return JSON.stringify(v);
}

function isEmpty(v: unknown): boolean {
  if (v == null) return true;
  if (typeof v === "string") return v === "";
  if (typeof v === "boolean") return !v;
  if (typeof v === "number") return v === 0;
  if (Array.isArray(v)) return v.length === 0;
  if (typeof v === "object") return Object.keys(v as object).length === 0;
  return false;
}

function isTruthy(v: unknown): boolean {
  return !isEmpty(v);
}

function joinFn(sep: string, v: unknown): string {
  if (!Array.isArray(v)) return "";
  return v.map((x) => toString(x)).join(sep ?? "");
}

function lengthFn(v: unknown): number {
  if (v == null) return 0;
  if (Array.isArray(v)) return v.length;
  if (typeof v === "string") return v.length;
  if (typeof v === "object") return Object.keys(v as object).length;
  return 0;
}

async function jqFirst(input: unknown, query: unknown): Promise<unknown> {
  const all = await runJq(input, query);
  return all.length > 0 ? all[0] : null;
}

async function jqAllFn(input: unknown, query: unknown): Promise<unknown[]> {
  return await runJq(input, query);
}

async function runJq(input: unknown, query: unknown): Promise<unknown[]> {
  if (typeof query !== "string") throw new Error("jq query must be a string");
  // -c forces compact one-result-per-line output so we can split on \n
  // without worrying about pretty-printed multi-line objects.
  const r = await jq.raw(JSON.stringify(input ?? null), query, ["-c"]);
  if (r.exitCode !== 0) {
    const err = (r.stderr || "").split("\n")[0] || `jq exit ${r.exitCode}`;
    throw new Error(`jq: ${err}`);
  }
  const lines = r.stdout.split("\n").filter((l) => l.length > 0);
  return lines.map((l) => JSON.parse(l));
}

// numberToString prints a number the way Go's text/template would when
// the value is rendered into output via the default formatter (%v on a
// float64 collapses 23.0 → "23", keeps 23.4375 as-is). JS's
// Number.toString does the same for finite values within the IVR's
// expected range.
function numberToString(n: number): string {
  if (Number.isInteger(n)) return n.toString();
  return n.toString();
}

function formatValue(v: unknown): string {
  if (v == null) return "<no value>";
  if (typeof v === "string") return v;
  if (typeof v === "number") return numberToString(v);
  if (typeof v === "boolean") return v ? "true" : "false";
  if (Array.isArray(v)) return "[" + v.map(formatValue).join(" ") + "]";
  return JSON.stringify(v);
}
