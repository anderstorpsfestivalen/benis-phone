import { useState } from "react";
import type { Action, ActionKind, Script } from "../generated/config";
import { actionKind, ACTION_KINDS } from "../generated/config";
import { SERVICE_NAMES } from "../generated/services";
import { api } from "../lib/api";
import { emptyGenericJSON, emptyScript } from "../lib/empty";
import { Field, TextInput, NumberInput, CheckboxInput } from "./Field";
import HelpDot from "./HelpDot";
import TTSEditor from "./TTSEditor";
import ServiceArgsForm from "./ServiceArgsForm";
import TemplateFieldPicker from "./TemplateFieldPicker";
import FilePicker from "./FilePicker";
import CodeEditor from "./CodeEditor";
import { runScriptTest, type TranscriptEvent } from "../lib/script-runner";

const ACTION_KIND_HELP: Record<ActionKind, string> = {
  dst: "Jump to another menu by name. Use this to chain menus together — pressing this DTMF key sends the caller to the chosen menu.",
  file: "Play an audio file from disk (ogg/wav/mp3). Block holds the call until playback finishes; clear stops any prior audio first.",
  randomfile: "Pick a random file from a folder and play it. Useful for jingles, hold music variations, etc.",
  tts: "Speak text using a TTS provider (Polly / ElevenLabs). Voice/lang/engine override the defaults from General.",
  srv: "Call an external service (weather, traintimes, etc.), then speak the result through TTS using the template. Args drive the service.",
  dispatcher: "Hand the caller to a named queue. Queues handle wait music, position announcements, and agent routing.",
  transfer: "Blind SIP REFER to another endpoint. Format: sip:200@host, user@host, or just an extension like 200.",
  hangup: "Terminate the call. After this action, the session ends.",
  record: "Start or stop recording the live call. 'start' begins, 'stop' ends. Subfolder is appended under SIP.record_path.",
  dtmf: "Transmit a string of DTMF digits to the remote end, 200 ms apart. Useful for chaining into upstream IVRs.",
  livefeed: "Stream a host audio capture device into the caller's outbound RTP. Device is a case-insensitive substring match; channel picks the audio channel.",
  genericjson: "Fetch a JSON HTTP endpoint, render the response through a Go text/template, and speak it through TTS. Navigate untyped JSON with {{.Data.foo.bar}}, iterate with {{range .Data.items}}, or use the full jq language via {{jq .Data \".[] | select(.name == \\\"X\\\") | .temperature\"}}. Helpers: int, round, default, jq, jqAll, first, last, join, add, sub, mul, div, keys, length.",
  script: "Write the whole flow as JavaScript (run by goja). Bindings: speak(text), readKey() (returns a DTMF key or null), http.get(url, opts?) / http.post(url, bodyObjOrString, opts?) → { status, json, text } with json a native JS value, vars.get/set (shared .Vars), args (this node's args), goto(fn, param?) to hand off to another menu on exit, and log(...). Navigate JSON with plain JS (.find/.filter/?.) — no jq. Note: Test it runs in the browser (V8), the runtime is goja (ES5.1+), so avoid the newest JS APIs.",
  clear: "Stop any currently-playing audio in this call session without otherwise affecting state.",
};

type Props = {
  value: Action;
  onChange: (v: Action) => void;
  onRemove: () => void;
  knownFnNames: string[];
};

export default function ActionEditor({ value, onChange, onRemove, knownFnNames }: Props) {
  const kind = actionKind(value) ?? "dst";
  const set = <K extends keyof Action>(k: K, v: Action[K]) =>
    onChange({ ...value, [k]: v });

  // Track which Pick modal (if any) is open. file = file.src for `file`
  // actions; folder = randomfile.folder for `randomfile` actions.
  const [picker, setPicker] = useState<"file" | "folder" | null>(null);

  function switchKind(k: ActionKind) {
    // Reset all kind-specific fields, then set just the chosen one to a
    // sentinel default so actionKind() returns it.
    const reset: Partial<Action> = {
      dst: "",
      file: { src: "", block: false, clear: false },
      randomfile: { folder: "" },
      tts: { msg: "", voice: "", lang: "", engine: "", provider: "" },
      srv: { dst: "", tmpl: "", args: {}, tts: { msg: "", voice: "", lang: "", engine: "", provider: "" } },
      dispatcher: "",
      transfer: "",
      hangup: false,
      record: "",
      dtmf: "",
      livefeed: null,
      // GenericJSON has its own empty factory in lib/empty.ts — use it
      // here so kind-switch and "Add Action" stay in lockstep.
      genericjson: emptyGenericJSON(),
      script: emptyScript(),
      then: "",
      auto: false,
      clear: false,
    };
    const seeded: Partial<Action> = {};
    switch (k) {
      case "dst":
        seeded.dst = "(choose menu)";
        break;
      case "file":
        seeded.file = { src: "(path)", block: false, clear: false };
        break;
      case "randomfile":
        seeded.randomfile = { folder: "(folder)" };
        break;
      case "tts":
        seeded.tts = { msg: "(say something)", voice: "", lang: "", engine: "", provider: "" };
        break;
      case "srv":
        seeded.srv = { dst: SERVICE_NAMES[0] ?? "", tmpl: "", args: {}, tts: { msg: "", voice: "", lang: "", engine: "", provider: "" } };
        break;
      case "dispatcher":
        seeded.dispatcher = "(queue name)";
        break;
      case "transfer":
        seeded.transfer = "200@host";
        break;
      case "hangup":
        seeded.hangup = true;
        break;
      case "record":
        seeded.record = "start";
        break;
      case "dtmf":
        seeded.dtmf = "1234";
        break;
      case "livefeed":
        seeded.livefeed = { device: "", channel: 0 };
        break;
      case "genericjson":
        // Start from emptyGenericJSON() so a kind-switch matches "Add
        // Action" defaults exactly; only layer placeholder hints on top
        // so the user immediately sees example values to edit.
        seeded.genericjson = {
          ...emptyGenericJSON(),
          url: "https://example.com/api",
          tmpl: "The value is {{.Data.value}}.",
        };
        break;
      case "script":
        seeded.script = {
          ...emptyScript(),
          code:
            '// Fetch, decide, speak — plain JS. See the ? for the full API.\n' +
            'var res = http.get(args.base_url + "/api/example");\n' +
            'speak("Status " + res.status);\n',
        };
        break;
      case "clear":
        seeded.clear = true;
        break;
    }
    onChange({ ...value, ...reset, ...seeded });
  }

  return (
    <div className="bg-gunmetal border border-shadow-grey rounded p-3 mb-3">
      <div className="flex flex-wrap items-end gap-3 mb-3">
        <div className="flex flex-col">
          <span className="text-xs text-blue-slate uppercase flex items-center">
            Key
            <HelpDot help="DTMF key the caller presses to trigger this action. 0-9 are normal keys; 10 maps to *, 11 maps to #." />
          </span>
          <NumberInput value={value.num} onChange={(v) => set("num", v)} />
        </div>
        <div className="flex flex-col flex-1 min-w-[120px]">
          <span className="text-xs text-blue-slate uppercase flex items-center">
            Kind
            <HelpDot help={ACTION_KIND_HELP[kind]} />
          </span>
          <select
            value={kind}
            onChange={(e) => switchKind(e.target.value as ActionKind)}
            className="px-2 py-1 rounded font-mono text-sm"
          >
            {ACTION_KINDS.map((k) => (
              <option key={k} value={k}>{k}</option>
            ))}
          </select>
        </div>
        <div className="flex flex-col flex-1 min-w-[160px]">
          <span className="text-xs text-blue-slate uppercase flex items-center">
            Name
            <HelpDot help="Human-readable label shown on this node in the graph. Stored in the config but ignored at runtime — purely an authoring aid." />
          </span>
          <TextInput
            // Coerce: a stale doc (saved before the field existed) can
            // hand us name=undefined, which would switch this input
            // into uncontrolled mode and leak the prior selection's
            // value when the user clicks a different node.
            value={value.name ?? ""}
            onChange={(v) => set("name", v)}
            placeholder="e.g. summalajnen temp"
          />
        </div>
        <div className="flex items-center gap-3 pb-1">
          <CheckboxInput
            label="wait"
            value={value.wait}
            onChange={(v) => set("wait", v)}
            help="Block the menu from accepting the next DTMF press until this action finishes. Used for TTS prompts that should play in full."
          />
          <CheckboxInput
            label="clear"
            value={value.clear}
            onChange={(v) => set("clear", v)}
            help="Stop any currently-playing audio before this action runs. Prevents prompts from overlapping when the caller jumps menus quickly."
          />
        </div>
        <button
          onClick={onRemove}
          className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded shrink-0"
        >
          Remove
        </button>
      </div>

      <details className="border border-shadow-grey rounded mb-3" open={!!value.prefix.tts.msg}>
        <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase tracking-wider flex items-center">
          Prefix (plays sequentially before action)
          <HelpDot help="Audio played BEFORE this action runs. Sequential — the action waits until the prefix finishes. Use for short announcements like 'Connecting…' before a transfer." />
        </summary>
        <div className="p-3">
          <TTSEditor
            value={value.prefix.tts}
            onChange={(v) => set("prefix", { ...value.prefix, tts: v })}
          />
        </div>
      </details>

      <details className="border border-shadow-grey rounded mb-3" open={!!value.pmsg.tts.msg}>
        <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase tracking-wider flex items-center">
          Pmsg (plays in parallel while action runs)
          <HelpDot help="Audio played WHILE this action runs (parallel). Useful for slow service calls — the caller hears 'The current forecast is…' while the weather lookup + TTS synthesis happen in the background. The action's result is held until pmsg finishes." />
        </summary>
        <div className="p-3">
          <TTSEditor
            value={value.pmsg.tts}
            onChange={(v) => set("pmsg", { ...value.pmsg, tts: v })}
          />
        </div>
      </details>

      {kind === "dst" && (
        <Field
          label="Destination menu"
          help="Name of the menu (fn) to jump into when this key is pressed. Must match an existing fn name."
        >
          <select
            value={value.dst}
            onChange={(e) => set("dst", e.target.value)}
            className="px-2 py-1 rounded font-mono text-sm"
          >
            <option value="">(none)</option>
            {knownFnNames.map((n) => (
              <option key={n} value={n}>{n}</option>
            ))}
          </select>
        </Field>
      )}

      {kind === "file" && (
        <div className="grid grid-cols-3 gap-2">
          <div className="col-span-3">
            <Field
              label="Path"
              help="Path to the audio file under files/. Supported formats: wav, mp3, ogg. Use Pick to browse the R2 bucket."
            >
              <div className="flex gap-2">
                <TextInput
                  value={value.file.src}
                  onChange={(v) => set("file", { ...value.file, src: v })}
                />
                <button
                  type="button"
                  onClick={() => setPicker("file")}
                  className="shrink-0 text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
                >
                  Pick…
                </button>
              </div>
            </Field>
          </div>
          <CheckboxInput
            label="block"
            value={value.file.block}
            onChange={(v) => set("file", { ...value.file, block: v })}
            help="Hold the menu until playback finishes. Without this, subsequent actions may fire before the file is done."
          />
          <CheckboxInput
            label="clear"
            value={value.file.clear}
            onChange={(v) => set("file", { ...value.file, clear: v })}
            help="Stop any currently-playing audio before this file starts."
          />
        </div>
      )}

      {kind === "randomfile" && (
        <Field
          label="Folder"
          help="Folder under files/ containing candidate audio files. One is picked uniformly at random per invocation. Use Pick to browse the R2 bucket."
        >
          <div className="flex gap-2">
            <TextInput
              value={value.randomfile.folder}
              onChange={(v) => set("randomfile", { folder: v })}
            />
            <button
              type="button"
              onClick={() => setPicker("folder")}
              className="shrink-0 text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
            >
              Pick…
            </button>
          </div>
        </Field>
      )}

      {picker === "file" && (
        <FilePicker
          mode="file"
          onClose={() => setPicker(null)}
          onPick={(key) => {
            set("file", { ...value.file, src: "files/" + key });
            setPicker(null);
          }}
        />
      )}
      {picker === "folder" && (
        <FilePicker
          mode="folder"
          onClose={() => setPicker(null)}
          onPick={(key) => {
            set("randomfile", { folder: key ? "files/" + key : "files" });
            setPicker(null);
          }}
        />
      )}

      {kind === "tts" && (
        <TTSEditor value={value.tts} onChange={(v) => set("tts", v)} />
      )}

      {kind === "srv" && (
        <div className="grid grid-cols-1 gap-3">
          <Field
            label="Service"
            help="Registered service to invoke. Each service declares its own typed args and TemplateData; the editor below adapts to the choice."
          >
            <select
              value={value.srv.dst}
              onChange={(e) => set("srv", { ...value.srv, dst: e.target.value })}
              className="px-2 py-1 rounded font-mono text-sm"
            >
              {SERVICE_NAMES.map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
          </Field>
          <ServiceArgsForm
            service={value.srv.dst}
            value={value.srv.args}
            onChange={(v) => set("srv", { ...value.srv, args: v })}
          />
          <TemplateFieldPicker
            service={value.srv.dst}
            value={value.srv.tmpl}
            onChange={(v) => set("srv", { ...value.srv, tmpl: v })}
          />
        </div>
      )}

      {kind === "dispatcher" && (
        <Field
          label="Queue name"
          help="Name of a queue defined under [[queue]]. The caller is held with announcements until an agent (or condition) picks up."
        >
          <TextInput value={value.dispatcher} onChange={(v) => set("dispatcher", v)} />
        </Field>
      )}

      {kind === "transfer" && (
        <Field
          label="Transfer target"
          help="Endpoint to blind-REFER the call to. Accepts a full SIP URI (sip:200@host), user@host, or a bare extension like 200 (resolved against the registered domain)."
        >
          <TextInput value={value.transfer} onChange={(v) => set("transfer", v)} />
        </Field>
      )}

      {kind === "record" && (
        <div className="grid grid-cols-2 gap-2">
          <Field
            label="Mode"
            help="'start' begins recording the live audio of this call; 'stop' ends an in-progress recording."
          >
            <TextInput value={value.record} onChange={(v) => set("record", v)} />
          </Field>
          <Field
            label="Subfolder"
            help="Subfolder appended under SIP.record_path. The full recording is written as <record_path>/<subfolder>/<call-id>.wav."
          >
            <TextInput value={value.record_to} onChange={(v) => set("record_to", v)} />
          </Field>
        </div>
      )}

      {kind === "dtmf" && (
        <Field
          label="DTMF digits"
          help="String of DTMF digits sent to the remote end with a 200 ms gap between each. Useful for navigating upstream IVRs."
        >
          <TextInput value={value.dtmf} onChange={(v) => set("dtmf", v)} />
        </Field>
      )}

      {kind === "livefeed" && value.livefeed && (
        <div className="grid grid-cols-2 gap-2">
          <Field
            label="Device"
            help="Case-insensitive substring matched against host audio capture device names. Empty selects the system default. Use the -list-audio-devices CLI flag to enumerate."
          >
            <TextInput
              value={value.livefeed.device}
              onChange={(v) => set("livefeed", { ...value.livefeed!, device: v })}
            />
          </Field>
          <Field
            label="Channel"
            help="Zero-indexed audio channel on the chosen device. 0 = first channel; useful for multi-channel interfaces."
          >
            <NumberInput
              value={value.livefeed.channel}
              onChange={(v) => set("livefeed", { ...value.livefeed!, channel: v })}
            />
          </Field>
        </div>
      )}

      {kind === "genericjson" && (
        <div className="grid grid-cols-1 gap-3">
          <Field
            label="URL"
            help="Endpoint to fetch. Must be reachable from the phone host. Response must be valid JSON."
          >
            <TextInput
              value={value.genericjson.url}
              onChange={(v) => set("genericjson", { ...value.genericjson, url: v })}
              placeholder="https://api.example.com/sensor"
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field
              label="Method"
              help="HTTP method. Defaults to GET when empty."
            >
              <TextInput
                value={value.genericjson.method}
                onChange={(v) => set("genericjson", { ...value.genericjson, method: v })}
                placeholder="GET"
              />
            </Field>
            <Field
              label="Timeout (s)"
              help="Request timeout in seconds. 0 = default (10s)."
            >
              <NumberInput
                value={value.genericjson.timeout_seconds}
                onChange={(v) => set("genericjson", { ...value.genericjson, timeout_seconds: v })}
              />
            </Field>
          </div>
          <Field
            label="Body"
            help="Request body, sent for non-GET methods. Defaults to Content-Type application/json (override via headers)."
          >
            <textarea
              value={value.genericjson.body}
              onChange={(e) => set("genericjson", { ...value.genericjson, body: e.target.value })}
              rows={3}
              className="px-2 py-1 rounded font-mono text-sm w-full"
            />
          </Field>
          <Field
            label="Headers"
            help="Extra request headers as key/value rows. Example: Authorization → Bearer abc123."
          >
            <HeadersGrid
              value={value.genericjson.headers}
              onChange={(h) => set("genericjson", { ...value.genericjson, headers: h })}
            />
          </Field>
          <GenericJSONTemplateAndPreview
            config={value.genericjson}
            onTmplChange={(t) => set("genericjson", { ...value.genericjson, tmpl: t })}
          />
          <Field
            label="Store (flow variables)"
            help="Save values from the response into flow variables for later nodes. Each row is varname → jq expression (the jq may itself use {{.Vars.*}}). Later nodes read them via {{.Vars.<name>}} in their URL / body / template. Example: roll → ."
          >
            <HeadersGrid
              value={value.genericjson.store}
              onChange={(st) => set("genericjson", { ...value.genericjson, store: st })}
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field
              label="Then (auto-advance)"
              help="After this node's audio finishes, jump to this menu with no keypress — the declarative-flow 'next step'."
            >
              <select
                value={value.then}
                onChange={(e) => set("then", e.target.value)}
                className="px-2 py-1 rounded font-mono text-sm"
              >
                <option value="">(none)</option>
                {knownFnNames.map((n) => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            </Field>
            <div className="flex items-end pb-1">
              <CheckboxInput
                label="auto (run on entry)"
                value={value.auto}
                onChange={(v) => set("auto", v)}
                help="Run this node automatically when its menu is entered (after the prefix), instead of waiting for a key. Use for 'fetch on arrival' steps."
              />
            </div>
          </div>
          <details className="border border-shadow-grey rounded">
            <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase tracking-wider">
              TTS overrides (voice / lang / engine / provider)
            </summary>
            <div className="p-3">
              <TTSEditor
                value={value.genericjson.tts}
                onChange={(v) => set("genericjson", { ...value.genericjson, tts: v })}
                hideMessage
              />
            </div>
          </details>
        </div>
      )}

      {kind === "script" && (
        <ScriptEditor
          value={value.script}
          onChange={(s) => set("script", s)}
        />
      )}

      {kind === "hangup" && (
        <div className="text-xs text-blue-slate">Hangs up the call.</div>
      )}
      {kind === "clear" && (
        <div className="text-xs text-blue-slate">Stops any currently-playing audio.</div>
      )}
    </div>
  );
}

// GenericJSONTemplateAndPreview bundles the template textarea, the "Test
// fetch"/"Test parse" buttons, the helper-function hint line, and the
// inline previews of the upstream response and rendered output. Keeping
// these together lets the preview state live next to the template the
// author is iterating on without re-rendering the rest of the form.
//
// Test parse runs the template through a browser-side renderer
// (lib/template-render.ts) that handles the helper subset and delegates
// jq to jq-wasm. The renderer is labelled "preview" because Go
// text/template features outside the documented subset can still drift
// from runtime behaviour.
function GenericJSONTemplateAndPreview({
  config,
  onTmplChange,
}: {
  config: {
    url: string;
    method: string;
    body: string;
    headers: Record<string, string>;
    tmpl: string;
  };
  onTmplChange: (tmpl: string) => void;
}) {
  const [state, setState] = useState<
    | { kind: "idle" }
    | { kind: "loading" }
    | { kind: "ok"; status: number; contentType: string; body: string; truncated: boolean }
    | { kind: "err"; msg: string }
  >({ kind: "idle" });

  // Parse-preview state is independent of fetch state so the user can
  // tweak the template and re-render without re-fetching.
  const [parseState, setParseState] = useState<
    | { kind: "idle" }
    | { kind: "loading" }
    | { kind: "ok"; rendered: string }
    | { kind: "err"; msg: string }
  >({ kind: "idle" });

  async function run() {
    setState({ kind: "loading" });
    try {
      const r = await api.previewGenericJSON({
        url: config.url,
        method: config.method || undefined,
        body: config.body || undefined,
        headers: config.headers,
      });
      setState({ kind: "ok", ...r });
    } catch (e) {
      setState({ kind: "err", msg: e instanceof Error ? e.message : String(e) });
    }
  }

  async function runParse() {
    if (state.kind !== "ok") return;
    setParseState({ kind: "loading" });
    // Dynamic import keeps jq-wasm (~1.4 MB WASM blob) out of the main
    // ConfigEditor bundle. It's only fetched the first time a user
    // actually clicks Test parse.
    try {
      const { renderGenericJSONTemplate } = await import("../lib/template-render");
      const r = await renderGenericJSONTemplate(state.body, config.tmpl, state.status);
      setParseState(r.ok ? { kind: "ok", rendered: r.rendered } : { kind: "err", msg: r.error });
    } catch (e) {
      setParseState({ kind: "err", msg: e instanceof Error ? e.message : String(e) });
    }
  }

  const canParse = state.kind === "ok" && config.tmpl.trim().length > 0;

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between">
        <label className="text-xs text-blue-slate uppercase">
          Template (Go text/template over decoded JSON)
        </label>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={run}
            disabled={!config.url || state.kind === "loading"}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {state.kind === "loading" ? "Fetching…" : "Test fetch"}
          </button>
          <button
            type="button"
            onClick={runParse}
            disabled={!canParse || parseState.kind === "loading"}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded disabled:opacity-40 disabled:cursor-not-allowed"
            title={
              state.kind === "ok"
                ? "Render the template against the fetched JSON (browser-side preview)"
                : "Run Test fetch first"
            }
          >
            {parseState.kind === "loading" ? "Rendering…" : "Test parse"}
          </button>
        </div>
      </div>
      <textarea
        value={config.tmpl}
        onChange={(e) => onTmplChange(e.target.value)}
        rows={10}
        className="px-2 py-1 rounded font-mono text-sm w-full resize-y"
        style={{ minHeight: 200 }}
        placeholder={
          "The temperature is {{int .Data.temp}} celsius.\n{{range .Data.items}}{{.name}}: {{.value}}; {{end}}"
        }
      />
      <div className="grid grid-cols-2 gap-2 mt-1">
        {state.kind === "ok" && (
          <div>
            <div className="text-xs text-blue-slate mb-1">
              HTTP {state.status}
              {state.contentType ? ` · ${state.contentType}` : ""}
              {state.truncated ? " · truncated at 1 MiB" : ""}
            </div>
            <pre className="bg-ink-black border border-shadow-grey rounded p-2 text-xs font-mono text-white max-h-96 overflow-auto whitespace-pre-wrap">
              {prettyJSON(state.body)}
            </pre>
          </div>
        )}
        {parseState.kind === "ok" && (
          <div>
            <div className="text-xs text-blue-slate mb-1">
              Rendered output (preview)
            </div>
            <pre className="bg-ink-black border border-shadow-grey rounded p-2 text-xs font-mono text-white max-h-96 overflow-auto whitespace-pre-wrap">
              {parseState.rendered}
            </pre>
          </div>
        )}
        {parseState.kind === "err" && (
          <div>
            <div className="text-xs text-red-300 mb-1">Rendered output</div>
            <pre className="bg-ink-black border border-red-900/40 bg-red-900/10 rounded p-2 text-xs font-mono text-red-300 max-h-96 overflow-auto whitespace-pre-wrap">
              {parseState.msg}
            </pre>
          </div>
        )}
      </div>
      {state.kind === "err" && (
        <div className="mt-1 text-xs text-red-300 border border-red-900/40 bg-red-900/10 rounded p-2">
          {state.msg}
        </div>
      )}
      <span className="text-xs text-blue-slate">
        JSON tree is bound as <code>.Data</code>. Use <code>.Status</code> and <code>.Raw</code> for the HTTP status and raw body.
        <br />
        <code>jq</code> runs a real jq expression (filter/select/map/iterate):{" "}
        <code>{"{{jq .Data \".[] | select(.name == \\\"Summalajnen\\\") | .temperature\"}}"}</code>.
        Use <code>jqAll</code> with <code>{"{{range}}"}</code> when a query yields multiple values.
        <br />
        Format helpers: <code>int</code>, <code>round</code>, <code>float</code>, <code>str</code>, <code>default</code>,{" "}
        <code>first</code>, <code>last</code>, <code>join</code>,{" "}
        <code>add</code>, <code>sub</code>, <code>mul</code>, <code>div</code>,{" "}
        <code>keys</code>, <code>length</code>.
      </span>
    </div>
  );
}

// ScriptEditor is the editor for a `script` action: a CodeMirror JS editor
// with inline syntax linting, a Format button (Prettier), an `args` map, a
// "Test it" harness that runs the script in an isolated Web Worker, and the
// TTS overrides for speak().
function ScriptEditor({
  value,
  onChange,
}: {
  value: Script;
  onChange: (v: Script) => void;
}) {
  const [formatErr, setFormatErr] = useState<string | null>(null);

  async function format() {
    setFormatErr(null);
    try {
      // Dynamic import keeps Prettier (+ its babel/estree plugins) out of the
      // main ConfigEditor bundle — only fetched when the user clicks Format.
      const prettier = await import("prettier/standalone");
      const babel = (await import("prettier/plugins/babel")).default;
      const estree = (await import("prettier/plugins/estree")).default;
      const formatted = await prettier.format(value.code, {
        parser: "babel",
        plugins: [babel, estree],
        semi: true,
      });
      onChange({ ...value, code: formatted.replace(/\n$/, "") });
    } catch (e) {
      // Prettier throws a SyntaxError with a location on invalid JS.
      setFormatErr(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="grid grid-cols-1 gap-3">
      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between">
          <label className="text-xs text-blue-slate uppercase flex items-center">
            Code (JavaScript, run by goja)
            <HelpDot help="speak(text), readKey(), http.get/post → {status,json,text}, vars.get/set, args, goto(fn,param?), log(...). Use plain JS to navigate JSON (.find/.filter) — no jq. Runs in the browser (V8) for Test it; the runtime is goja (ES5.1+)." />
          </label>
          <button
            type="button"
            onClick={format}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
          >
            Format
          </button>
        </div>
        <CodeEditor value={value.code} onChange={(c) => onChange({ ...value, code: c })} />
        {formatErr && (
          <pre className="text-xs text-red-300 bg-red-900/10 border border-red-900/40 rounded p-2 whitespace-pre-wrap">
            {formatErr}
          </pre>
        )}
      </div>

      <Field
        label="Args"
        help="Static config exposed to the script as the `args` object (all string values). Example: base_url → https://beer.anderstorpsfestivalen.se."
      >
        <HeadersGrid value={value.args} onChange={(a) => onChange({ ...value, args: a })} />
      </Field>

      <ScriptTestPanel code={value.code} args={value.args} />

      <details className="border border-shadow-grey rounded">
        <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase tracking-wider">
          TTS overrides (voice / lang / engine / provider)
        </summary>
        <div className="p-3">
          <TTSEditor
            value={value.tts}
            onChange={(v) => onChange({ ...value, tts: v })}
            hideMessage
          />
        </div>
      </details>
    </div>
  );
}

// ScriptTestPanel runs the script in a Web Worker (see lib/script-runner) with
// a simulated keypress sequence, and renders the resulting transcript. HTTP
// goes through the same /api/genericjson/preview proxy the genericjson editor
// uses, so no config save / live call is needed.
function ScriptTestPanel({
  code,
  args,
}: {
  code: string;
  args: Record<string, string>;
}) {
  const [keys, setKeys] = useState("");
  const [state, setState] = useState<
    | { kind: "idle" }
    | { kind: "running" }
    | { kind: "done"; transcript: TranscriptEvent[]; gotoTarget: string | null; error?: string }
  >({ kind: "idle" });

  async function run() {
    setState({ kind: "running" });
    const seq = keys
      .split(/[\s,]+/)
      .map((k) => k.trim())
      .filter(Boolean);
    const r = await runScriptTest({ code, args, keys: seq });
    setState({ kind: "done", transcript: r.transcript, gotoTarget: r.gotoTarget, error: r.error });
  }

  return (
    <div className="flex flex-col gap-2 border border-shadow-grey rounded p-3">
      <div className="flex items-end gap-2">
        <div className="flex flex-col flex-1">
          <label className="text-xs text-blue-slate uppercase flex items-center">
            Simulated keypresses
            <HelpDot help="Comma/space-separated DTMF keys fed to readKey() in order (e.g. `4, 1, 1`). readKey() returns null once they run out." />
          </label>
          <TextInput value={keys} onChange={setKeys} placeholder="4, 1, 1" />
        </div>
        <button
          type="button"
          onClick={run}
          disabled={state.kind === "running" || !code.trim()}
          className="text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {state.kind === "running" ? "Running…" : "Test it"}
        </button>
      </div>

      {state.kind === "done" && (
        <div className="flex flex-col gap-1">
          {state.error && (
            <pre className="text-xs text-red-300 bg-red-900/10 border border-red-900/40 rounded p-2 whitespace-pre-wrap">
              {state.error}
            </pre>
          )}
          {state.transcript.length === 0 && !state.error && (
            <span className="text-xs text-blue-slate italic">No output.</span>
          )}
          {state.transcript.length > 0 && (
            <pre className="text-xs text-white bg-ink-black border border-shadow-grey rounded p-2 whitespace-pre-wrap">
              {state.transcript.map(transcriptLine).join("\n")}
            </pre>
          )}
          {state.gotoTarget && (
            <span className="text-xs text-blue-slate">
              goto → <span className="text-white">{state.gotoTarget}</span>
            </span>
          )}
        </div>
      )}
    </div>
  );
}

function transcriptLine(e: TranscriptEvent): string {
  switch (e.type) {
    case "speak":
      return `🔊 ${e.text}`;
    case "readKey":
      return `⌨  readKey() → ${e.key === null ? "null" : e.key}`;
    case "http":
      return `🌐 ${e.method} ${e.url} → ${e.status}`;
    case "goto":
      return `➡  goto(${e.fn})`;
    case "log":
      return `· ${e.args.join(" ")}`;
  }
}

// Best-effort: pretty-print JSON, fall back to the raw body if parsing
// fails (e.g. the upstream returned HTML or text instead of JSON — the
// user still wants to see *something* to understand what came back).
function prettyJSON(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}

// HeadersGrid renders the GenericJSON headers map as a list of paired
// key/value inputs. The storage shape stays Record<string,string>, so
// nothing downstream changes — the prior textarea-with-line-parsing
// approach silently dropped malformed lines and reordered the map on
// every edit, which this avoids entirely.
//
// Implementation note: we mirror the map into an ordered local list of
// {k, v} rows on each render. New rows append with an empty key; we
// keep the original key as `originalKey` so renaming a key (which would
// otherwise drop+recreate the entry, losing position) updates in place.
function HeadersGrid({
  value,
  onChange,
}: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
}) {
  // Object.entries preserves insertion order in modern JS engines, so
  // round-tripping through this list is stable.
  const rows = Object.entries(value);

  function setRow(i: number, k: string, v: string) {
    const next: Record<string, string> = {};
    rows.forEach(([rk, rv], idx) => {
      if (idx === i) {
        if (k) next[k] = v;
      } else {
        next[rk] = rv;
      }
    });
    onChange(next);
  }

  function deleteRow(i: number) {
    const next: Record<string, string> = {};
    rows.forEach(([rk, rv], idx) => {
      if (idx !== i) next[rk] = rv;
    });
    onChange(next);
  }

  function addRow() {
    // Pick a fresh placeholder key that doesn't collide so React keys
    // stay stable. Empty string would collapse multiple new rows into
    // one entry.
    let i = 1;
    let candidate = `header-${i}`;
    while (candidate in value) {
      i += 1;
      candidate = `header-${i}`;
    }
    onChange({ ...value, [candidate]: "" });
  }

  return (
    <div className="flex flex-col gap-1">
      {rows.length === 0 && (
        <span className="text-xs text-blue-slate italic">No headers.</span>
      )}
      {rows.map(([k, v], i) => (
        <div key={i} className="flex gap-2 items-center">
          <input
            type="text"
            value={k}
            onChange={(e) => setRow(i, e.target.value, v)}
            placeholder="Header-Name"
            className="px-2 py-1 rounded font-mono text-sm flex-1 min-w-0"
          />
          <input
            type="text"
            value={v}
            onChange={(e) => setRow(i, k, e.target.value)}
            placeholder="value"
            className="px-2 py-1 rounded font-mono text-sm flex-1 min-w-0"
          />
          <button
            type="button"
            onClick={() => deleteRow(i)}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded shrink-0"
            aria-label={`Remove header ${k || "row " + (i + 1)}`}
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={addRow}
        className="self-start text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded mt-1"
      >
        + Add header
      </button>
    </div>
  );
}

