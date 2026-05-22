import type { Fn } from "../generated/config";
import { emptyAction } from "../lib/empty";
import { actionDetail, dtmfLabel } from "../lib/fn-graph";
import { actionKind } from "../generated/config";
import TTSEditor from "./TTSEditor";
import { Field, TextInput, NumberInput, CheckboxInput } from "./Field";

// Side-panel editor for an Fn (menu) node. Shows only what's configurable on
// the menu itself; actions are represented in the graph as their own nodes
// and edited there, so this lists them as a compact summary with a jump
// button.

export default function FnEditor({
  value,
  onChange,
  onSelectAction,
}: {
  value: Fn;
  onChange: (v: Fn) => void;
  onSelectAction?: (index: number) => void;
}) {
  const set = <K extends keyof Fn>(k: K, v: Fn[K]) => onChange({ ...value, [k]: v });

  function addAction() {
    set("actions", [...value.actions, emptyAction()]);
    onSelectAction?.(value.actions.length);
  }

  function removeAction(i: number) {
    set("actions", value.actions.filter((_, n) => n !== i));
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <Field
          label="Name"
          help="Unique identifier for this menu. Referenced by other actions via dst, by the General entrypoint, and by dispatcher targets. Lowercase, no spaces."
        >
          <TextInput value={value.name} onChange={(v) => set("name", v)} />
        </Field>
        <Field
          label="Recording path"
          help="Optional subfolder (under SIP.record_path) where calls reaching this menu are recorded. Leave blank to inherit / disable."
        >
          <TextInput
            value={value.recording.path}
            onChange={(v) => set("recording", { path: v })}
          />
        </Field>
        <Field
          label="Input length"
          help="If > 0, the menu collects N DTMF digits as input before matching against an action; 0 means single-digit menus (the default)."
        >
          <NumberInput
            value={value.inputlength}
            onChange={(v) => set("inputlength", v)}
          />
        </Field>
        <CheckboxInput
          label="clear callstack on entry"
          value={value.clear_callstack}
          onChange={(v) => set("clear_callstack", v)}
          help="When entered, drop any prior menu history so pressing 0 won't pop the caller back to a previous menu. Useful for top-level entries."
        />
      </div>

      <details className="border border-shadow-grey rounded">
        <summary className="px-3 py-2 cursor-pointer text-sm text-blue-slate">
          Prefix (announcement on entry)
        </summary>
        <div className="p-3">
          <TTSEditor
            value={value.prefix.tts}
            onChange={(v) => set("prefix", { ...value.prefix, tts: v })}
          />
        </div>
      </details>

      <details className="border border-shadow-grey rounded">
        <summary className="px-3 py-2 cursor-pointer text-sm text-blue-slate">
          Gate (input validation)
        </summary>
        <div className="p-3 grid grid-cols-2 gap-3">
          <Field
            label="Validator service (dst)"
            help="Name of a service that decides whether to admit the caller. The service receives the collected input and returns ok / not-ok."
          >
            <TextInput
              value={value.gate.dst}
              onChange={(v) => set("gate", { ...value.gate, dst: v })}
            />
          </Field>
          <Field
            label="Accept menu"
            help="Menu name to jump to when the validator accepts the input."
          >
            <TextInput
              value={value.gate.accept}
              onChange={(v) => set("gate", { ...value.gate, accept: v })}
            />
          </Field>
          <Field
            label="Prompt"
            help="Text spoken to the caller asking them to enter the value the gate validates (e.g. a code)."
          >
            <TextInput
              value={value.gate.prompt}
              onChange={(v) => set("gate", { ...value.gate, prompt: v })}
            />
          </Field>
          <Field
            label="Deny template"
            help="Go text/template used to render the rejection message — the validator service's response is the template data."
          >
            <TextInput
              value={value.gate.deny_tmpl}
              onChange={(v) => set("gate", { ...value.gate, deny_tmpl: v })}
            />
          </Field>
        </div>
      </details>

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-blue-slate uppercase">
            Actions ({value.actions.length})
          </span>
          <button
            onClick={addAction}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
          >
            + action
          </button>
        </div>
        {value.actions.length === 0 && (
          <div className="text-xs text-blue-slate italic">
            No actions yet. Add one — it will appear as its own node in the graph.
          </div>
        )}
        <ul className="flex flex-col gap-1">
          {value.actions.map((a, i) => {
            const kind = actionKind(a);
            const detail = actionDetail(a);
            return (
              <li
                key={i}
                className="flex items-center gap-2 px-2 py-1 border border-shadow-grey rounded text-xs font-mono"
              >
                <span className="text-blue-slate w-6 text-center">{dtmfLabel(a.num)}</span>
                <span className="text-white">{kind ?? "empty"}</span>
                <span className="text-blue-slate truncate flex-1">{detail}</span>
                <button
                  onClick={() => onSelectAction?.(i)}
                  className="px-2 py-0.5 border border-shadow-grey text-blue-slate hover:text-white rounded"
                  title="Open this action node"
                >
                  edit
                </button>
                <button
                  onClick={() => removeAction(i)}
                  className="px-2 py-0.5 border border-shadow-grey text-blue-slate hover:text-white rounded"
                  title="Remove this action"
                >
                  ×
                </button>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
