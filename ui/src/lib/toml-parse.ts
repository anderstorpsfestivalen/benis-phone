import { parse } from "smol-toml";
import type {
  Action,
  Definition,
  Fn,
  Playable,
  Prefix,
  Queue,
  QueuePrompt,
  Service,
} from "../generated/config";
import {
  emptyAction,
  emptyDefinition,
  emptyFn,
  emptyPlayable,
  emptyPrefix,
  emptyQueue,
  emptyService,
  emptyTTS,
} from "./empty";

// parseTomlConfig parses a TOML string (as produced by BurntSushi/toml from
// core/functions) into our editor's Definition shape, with every nested
// object backfilled from the empty-factory defaults so controlled inputs
// never see undefined.
export function parseTomlConfig(text: string): Definition {
  const raw = parse(text) as Record<string, unknown>;
  return normalizeDefinition(raw);
}

type AnyRec = Record<string, unknown>;
const asObj = (x: unknown): AnyRec => (x && typeof x === "object" ? (x as AnyRec) : {});
const asArr = (x: unknown): unknown[] => (Array.isArray(x) ? x : []);
const asStr = (x: unknown, d = ""): string => (typeof x === "string" ? x : d);
const asNum = (x: unknown, d = 0): number => (typeof x === "number" ? x : d);
const asBool = (x: unknown): boolean => x === true;

function normalizeDefinition(raw: AnyRec): Definition {
  const e = emptyDefinition();
  return {
    general: { ...e.general, ...asObj(raw.general) } as Definition["general"],
    sip: { ...e.sip, ...asObj(raw.sip) } as Definition["sip"],
    fn: asArr(raw.fn).map(normalizeFn),
    queue: asArr(raw.queue).map(normalizeQueue),
  };
}

function normalizeFn(raw: unknown): Fn {
  const r = asObj(raw);
  const e = emptyFn();
  return {
    name: asStr(r.name),
    recording: { ...e.recording, ...asObj(r.recording) },
    prefix: normalizePrefix(r.prefix),
    gate: { ...e.gate, ...asObj(r.gate) },
    clear_callstack: asBool(r.clear_callstack),
    inputlength: asNum(r.inputlength),
    actions: asArr(r.actions).map(normalizeAction),
  };
}

function normalizePrefix(raw: unknown): Prefix {
  const r = asObj(raw);
  const e = emptyPrefix();
  return {
    file: { ...e.file, ...asObj(r.file) },
    tts: { ...e.tts, ...asObj(r.tts) },
    wait: asBool(r.wait),
    ignoreclear: asBool(r.ignoreclear),
  };
}

function normalizeAction(raw: unknown): Action {
  const r = asObj(raw);
  const e = emptyAction();
  const livefeed = r.livefeed
    ? {
        device: asStr(asObj(r.livefeed).device),
        channel: asNum(asObj(r.livefeed).channel),
      }
    : null;
  return {
    num: asNum(r.num),
    wait: asBool(r.wait),
    clear: asBool(r.clear),
    name: asStr(r.name),
    prefix: normalizePrefix(r.prefix),
    pmsg: normalizePrefix(r.pmsg),
    dst: asStr(r.dst),
    file: { ...e.file, ...asObj(r.file) },
    randomfile: { ...e.randomfile, ...asObj(r.randomfile) },
    tts: { ...e.tts, ...asObj(r.tts) },
    srv: normalizeService(r.srv),
    dispatcher: asStr(r.dispatcher),
    transfer: asStr(r.transfer),
    hangup: asBool(r.hangup),
    record: asStr(r.record),
    record_to: asStr(r.record_to),
    dtmf: asStr(r.dtmf),
    livefeed,
    genericjson: normalizeGenericJSON(r.genericjson),
  };
}

function normalizeGenericJSON(raw: unknown) {
  const r = asObj(raw);
  const headers: Record<string, string> = {};
  for (const [k, v] of Object.entries(asObj(r.headers))) {
    if (typeof v === "string") headers[k] = v;
  }
  return {
    url: asStr(r.url),
    method: asStr(r.method),
    body: asStr(r.body),
    headers,
    tmpl: asStr(r.tmpl),
    timeout_seconds: asNum(r.timeout_seconds),
    tts: { ...emptyTTS(), ...asObj(r.tts) },
  };
}

function normalizeService(raw: unknown): Service {
  const r = asObj(raw);
  const e = emptyService();
  const args: Record<string, string> = {};
  for (const [k, v] of Object.entries(asObj(r.args))) {
    if (typeof v === "string") args[k] = v;
  }
  return {
    dst: asStr(r.dst),
    tmpl: asStr(r.tmpl),
    args,
    tts: { ...e.tts, ...asObj(r.tts) },
  };
}

function normalizeQueue(raw: unknown): Queue {
  const r = asObj(raw);
  const e = emptyQueue();
  return {
    name: asStr(r.name),
    entrymsg: normalizePlayable(r.entrymsg),
    minpos: asNum(r.minpos, e.minpos),
    maxpos: asNum(r.maxpos, e.maxpos),
    speed: asNum(r.speed, e.speed),
    minprompt: asNum(r.minprompt, e.minprompt),
    maxprompt: asNum(r.maxprompt, e.maxprompt),
    currentpos: { ...e.currentpos, ...asObj(r.currentpos) },
    prompt: asArr(r.prompt).map(normalizeQueuePrompt),
    bgmusic: { ...e.bgmusic, ...asObj(r.bgmusic) },
    end: normalizeAction(r.end),
  };
}

function normalizePlayable(raw: unknown): Playable {
  const r = asObj(raw);
  const e = emptyPlayable();
  return {
    file: { ...e.file, ...asObj(r.file) },
    tts: { ...e.tts, ...asObj(r.tts) },
    wait: asBool(r.wait),
    clear: asBool(r.clear),
  };
}

function normalizeQueuePrompt(raw: unknown): QueuePrompt {
  const r = asObj(raw);
  return {
    prompt: normalizePlayable(r.prompt),
    empty: asBool(r.empty),
    weight: asNum(r.weight, 1),
  };
}
