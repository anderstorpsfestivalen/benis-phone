import type { Fn } from "../generated/config";
import { emptyAction } from "../lib/empty";
import ActionEditor from "./ActionEditor";
import TTSEditor from "./TTSEditor";
import { Field, TextInput, NumberInput, CheckboxInput } from "./Field";

export default function FnEditor({
  value,
  onChange,
  knownFnNames,
}: {
  value: Fn;
  onChange: (v: Fn) => void;
  knownFnNames: string[];
}) {
  const set = <K extends keyof Fn>(k: K, v: Fn[K]) => onChange({ ...value, [k]: v });

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Name">
          <TextInput value={value.name} onChange={(v) => set("name", v)} />
        </Field>
        <Field label="Recording path" hint="subfolder under SIP.record_path">
          <TextInput
            value={value.recording.path}
            onChange={(v) => set("recording", { path: v })}
          />
        </Field>
        <Field label="Input length">
          <NumberInput
            value={value.inputlength}
            onChange={(v) => set("inputlength", v)}
          />
        </Field>
        <CheckboxInput
          label="clear callstack on entry"
          value={value.clear_callstack}
          onChange={(v) => set("clear_callstack", v)}
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
          <Field label="Validator service (dst)">
            <TextInput
              value={value.gate.dst}
              onChange={(v) => set("gate", { ...value.gate, dst: v })}
            />
          </Field>
          <Field label="Accept menu">
            <TextInput
              value={value.gate.accept}
              onChange={(v) => set("gate", { ...value.gate, accept: v })}
            />
          </Field>
          <Field label="Prompt">
            <TextInput
              value={value.gate.prompt}
              onChange={(v) => set("gate", { ...value.gate, prompt: v })}
            />
          </Field>
          <Field label="Deny template">
            <TextInput
              value={value.gate.deny_tmpl}
              onChange={(v) => set("gate", { ...value.gate, deny_tmpl: v })}
            />
          </Field>
        </div>
      </details>

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-blue-slate uppercase">Actions</span>
          <button
            onClick={() => set("actions", [...value.actions, emptyAction()])}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
          >
            + action
          </button>
        </div>
        {value.actions.map((a, i) => (
          <ActionEditor
            key={i}
            value={a}
            knownFnNames={knownFnNames}
            onChange={(v) => {
              const next = [...value.actions];
              next[i] = v;
              set("actions", next);
            }}
            onRemove={() => {
              const next = value.actions.filter((_, n) => n !== i);
              set("actions", next);
            }}
          />
        ))}
      </div>
    </div>
  );
}
