import type { TTS } from "../generated/config";
import { Field, TextInput, TextArea } from "./Field";

// Message-first layout: the textarea gets the full width and enough height
// to read at a glance; voice/lang/engine/provider go under an "override
// TTS" collapsible since 90 % of callers use the defaults set in
// [general]. The collapsible auto-opens when any override is set so the
// user can see why their TTS sounds different.

const TTS_PROVIDERS = ["", "polly", "elevenlabs"] as const;

export default function TTSEditor({
  value,
  onChange,
  hideMessage = false,
}: {
  value: TTS;
  onChange: (v: TTS) => void;
  // hideMessage skips the Message field — useful when the message text is
  // produced elsewhere (e.g. by a GenericJSON template render) and the TTS
  // struct is only carrying voice/lang/engine overrides.
  hideMessage?: boolean;
}) {
  const set = <K extends keyof TTS>(k: K, v: TTS[K]) => onChange({ ...value, [k]: v });
  const hasOverride = !!(value.voice || value.lang || value.engine || value.provider);

  return (
    <div className="flex flex-col gap-3 p-3 bg-ink-black border border-shadow-grey rounded">
      {!hideMessage && (
        <Field label="Message">
          <TextArea
            value={value.msg}
            onChange={(v) => set("msg", v)}
            rows={6}
          />
        </Field>
      )}

      <details className="border border-shadow-grey rounded" open={hideMessage || hasOverride}>
        <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase tracking-wider">
          Override TTS settings
        </summary>
        <div className="p-3 grid grid-cols-2 gap-3">
          <Field
            label="Voice"
            help="Provider-specific voice id (e.g. Polly: Joanna, Matthew; ElevenLabs: a voice UUID). Leave blank to inherit General.default_tts_voice."
          >
            <TextInput value={value.voice} onChange={(v) => set("voice", v)} />
          </Field>
          <Field
            label="Lang"
            help="BCP-47 language code (en-US, sv-SE). Leave blank to inherit General.default_tts_lang."
          >
            <TextInput value={value.lang} onChange={(v) => set("lang", v)} />
          </Field>
          <Field
            label="Engine"
            help="Provider-specific engine. Polly: standard | neural | generative. ElevenLabs: model_id, e.g. eleven_multilingual_v2. Blank = inherit General.default_tts_engine."
          >
            <TextInput value={value.engine} onChange={(v) => set("engine", v)} />
          </Field>
          <Field
            label="Provider"
            help="Which TTS backend renders this message. Blank = inherit General.default_tts_provider."
          >
            <select
              value={value.provider}
              onChange={(e) => set("provider", e.target.value)}
              className="px-2 py-1 rounded font-mono text-sm w-full min-w-0"
            >
              {TTS_PROVIDERS.map((p) => (
                <option key={p} value={p}>{p === "" ? "(inherit)" : p}</option>
              ))}
            </select>
          </Field>
        </div>
      </details>
    </div>
  );
}
