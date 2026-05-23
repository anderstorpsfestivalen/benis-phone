import { useState } from "react";
import type { Action, ActionKind } from "../generated/config";
import { actionKind, ACTION_KINDS } from "../generated/config";
import { SERVICE_NAMES } from "../generated/services";
import { Field, TextInput, NumberInput, CheckboxInput } from "./Field";
import HelpDot from "./HelpDot";
import TTSEditor from "./TTSEditor";
import ServiceArgsForm from "./ServiceArgsForm";
import TemplateFieldPicker from "./TemplateFieldPicker";
import FilePicker from "./FilePicker";

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
  clear: "Stop any currently-playing audio in this call session without otherwise affecting state.",
};

function headersToText(h: Record<string, string> | undefined | null): string {
  if (!h) return "";
  return Object.entries(h)
    .map(([k, v]) => `${k}: ${v}`)
    .join("\n");
}

function textToHeaders(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    const idx = line.indexOf(":");
    if (idx < 0) continue;
    const k = line.slice(0, idx).trim();
    const v = line.slice(idx + 1).trim();
    if (k) out[k] = v;
  }
  return out;
}

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
      genericjson: {
        url: "",
        method: "",
        body: "",
        headers: {},
        tmpl: "",
        timeout_seconds: 0,
        tts: { msg: "", voice: "", lang: "", engine: "", provider: "" },
      },
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
        seeded.genericjson = {
          url: "https://example.com/api",
          method: "GET",
          body: "",
          headers: {},
          tmpl: "The value is {{.Data.value}}.",
          timeout_seconds: 0,
          tts: { msg: "", voice: "", lang: "", engine: "", provider: "" },
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
            label="Headers (one per line, Key: Value)"
            help="Extra request headers. Example: Authorization: Bearer abc123"
          >
            <textarea
              value={headersToText(value.genericjson.headers)}
              onChange={(e) =>
                set("genericjson", {
                  ...value.genericjson,
                  headers: textToHeaders(e.target.value),
                })
              }
              rows={3}
              className="px-2 py-1 rounded font-mono text-sm w-full"
              placeholder="Authorization: Bearer abc123"
            />
          </Field>
          <div className="flex flex-col gap-1">
            <label className="text-xs text-blue-slate uppercase">
              Template (Go text/template over decoded JSON)
            </label>
            <textarea
              value={value.genericjson.tmpl}
              onChange={(e) => set("genericjson", { ...value.genericjson, tmpl: e.target.value })}
              rows={10}
              className="px-2 py-1 rounded font-mono text-sm w-full resize-y"
              style={{ minHeight: 200 }}
              placeholder={'The temperature is {{int .Data.temp}} celsius.\n{{range .Data.items}}{{.name}}: {{.value}}; {{end}}'}
            />
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

      {kind === "hangup" && (
        <div className="text-xs text-blue-slate">Hangs up the call.</div>
      )}
      {kind === "clear" && (
        <div className="text-xs text-blue-slate">Stops any currently-playing audio.</div>
      )}
    </div>
  );
}

