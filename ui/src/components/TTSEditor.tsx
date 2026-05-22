import type { TTS } from "../generated/config";
import { Field, TextInput, TextArea } from "./Field";

export default function TTSEditor({
  value,
  onChange,
}: {
  value: TTS;
  onChange: (v: TTS) => void;
}) {
  const set = <K extends keyof TTS>(k: K, v: TTS[K]) => onChange({ ...value, [k]: v });
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 p-3 bg-ink-black border border-shadow-grey rounded">
      <Field label="Message">
        <TextArea value={value.msg} onChange={(v) => set("msg", v)} rows={3} />
      </Field>
      <div className="grid grid-cols-2 gap-2">
        <Field label="Voice">
          <TextInput value={value.voice} onChange={(v) => set("voice", v)} />
        </Field>
        <Field label="Lang">
          <TextInput value={value.lang} onChange={(v) => set("lang", v)} />
        </Field>
        <Field label="Engine">
          <TextInput value={value.engine} onChange={(v) => set("engine", v)} />
        </Field>
        <Field label="Provider" hint="polly | elevenlabs">
          <TextInput value={value.provider} onChange={(v) => set("provider", v)} />
        </Field>
      </div>
    </div>
  );
}
