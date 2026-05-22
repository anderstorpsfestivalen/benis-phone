import type { Action, ActionKind } from "../generated/config";
import { actionKind, ACTION_KINDS } from "../generated/config";
import { SERVICE_NAMES } from "../generated/services";
import { Field, TextInput, TextArea, NumberInput, CheckboxInput } from "./Field";
import TTSEditor from "./TTSEditor";

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
      case "clear":
        seeded.clear = true;
        break;
    }
    onChange({ ...value, ...reset, ...seeded });
  }

  return (
    <div className="bg-gunmetal border border-shadow-grey rounded p-3 mb-3">
      <div className="flex items-center gap-3 mb-3">
        <div className="flex flex-col">
          <span className="text-xs text-blue-slate uppercase">Key</span>
          <NumberInput value={value.num} onChange={(v) => set("num", v)} />
        </div>
        <div className="flex flex-col flex-1">
          <span className="text-xs text-blue-slate uppercase">Kind</span>
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
        <CheckboxInput label="wait" value={value.wait} onChange={(v) => set("wait", v)} />
        <CheckboxInput label="clear" value={value.clear} onChange={(v) => set("clear", v)} />
        <button
          onClick={onRemove}
          className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
        >
          Remove
        </button>
      </div>

      {kind === "dst" && (
        <Field label="Destination menu">
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
          <Field label="Path">
            <TextInput
              value={value.file.src}
              onChange={(v) => set("file", { ...value.file, src: v })}
            />
          </Field>
          <CheckboxInput label="block" value={value.file.block} onChange={(v) => set("file", { ...value.file, block: v })} />
          <CheckboxInput label="clear" value={value.file.clear} onChange={(v) => set("file", { ...value.file, clear: v })} />
        </div>
      )}

      {kind === "randomfile" && (
        <Field label="Folder">
          <TextInput
            value={value.randomfile.folder}
            onChange={(v) => set("randomfile", { folder: v })}
          />
        </Field>
      )}

      {kind === "tts" && (
        <TTSEditor value={value.tts} onChange={(v) => set("tts", v)} />
      )}

      {kind === "srv" && (
        <div className="grid grid-cols-1 gap-3">
          <Field label="Service">
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
          <Field label="Template (Go text/template, e.g. {{.Field}})">
            <TextArea
              value={value.srv.tmpl}
              onChange={(v) => set("srv", { ...value.srv, tmpl: v })}
              rows={5}
            />
          </Field>
          <ArgsEditor
            value={value.srv.args}
            onChange={(v) => set("srv", { ...value.srv, args: v })}
          />
        </div>
      )}

      {kind === "dispatcher" && (
        <Field label="Queue name">
          <TextInput value={value.dispatcher} onChange={(v) => set("dispatcher", v)} />
        </Field>
      )}

      {kind === "transfer" && (
        <Field label="Transfer target" hint="sip:200@host, user@host, or extension 200">
          <TextInput value={value.transfer} onChange={(v) => set("transfer", v)} />
        </Field>
      )}

      {kind === "record" && (
        <div className="grid grid-cols-2 gap-2">
          <Field label="Mode" hint="start | stop">
            <TextInput value={value.record} onChange={(v) => set("record", v)} />
          </Field>
          <Field label="Subfolder" hint="under SIP.RecordPath">
            <TextInput value={value.record_to} onChange={(v) => set("record_to", v)} />
          </Field>
        </div>
      )}

      {kind === "dtmf" && (
        <Field label="DTMF digits" hint="200 ms gap between each">
          <TextInput value={value.dtmf} onChange={(v) => set("dtmf", v)} />
        </Field>
      )}

      {kind === "livefeed" && value.livefeed && (
        <div className="grid grid-cols-2 gap-2">
          <Field label="Device" hint="case-insensitive substring, empty = system default">
            <TextInput
              value={value.livefeed.device}
              onChange={(v) => set("livefeed", { ...value.livefeed!, device: v })}
            />
          </Field>
          <Field label="Channel">
            <NumberInput
              value={value.livefeed.channel}
              onChange={(v) => set("livefeed", { ...value.livefeed!, channel: v })}
            />
          </Field>
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

function ArgsEditor({
  value,
  onChange,
}: {
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
}) {
  const entries = Object.entries(value);
  function update(k: string, v: string) {
    onChange({ ...value, [k]: v });
  }
  function rename(oldK: string, newK: string) {
    if (newK === oldK || !newK) return;
    const next: Record<string, string> = {};
    for (const [k, v] of Object.entries(value)) next[k === oldK ? newK : k] = v;
    onChange(next);
  }
  function remove(k: string) {
    const next = { ...value };
    delete next[k];
    onChange(next);
  }
  function add() {
    const k = `key${entries.length + 1}`;
    onChange({ ...value, [k]: "" });
  }
  return (
    <div>
      <span className="text-xs text-blue-slate uppercase">Args</span>
      <div className="flex flex-col gap-2 mt-1">
        {entries.map(([k, v]) => (
          <div key={k} className="flex gap-2">
            <input
              className="px-2 py-1 rounded font-mono text-sm w-1/3"
              value={k}
              onChange={(e) => rename(k, e.target.value)}
            />
            <input
              className="px-2 py-1 rounded font-mono text-sm flex-1"
              value={v}
              onChange={(e) => update(k, e.target.value)}
            />
            <button
              onClick={() => remove(k)}
              className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
            >
              x
            </button>
          </div>
        ))}
        <button
          onClick={add}
          className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded self-start"
        >
          + arg
        </button>
      </div>
    </div>
  );
}
