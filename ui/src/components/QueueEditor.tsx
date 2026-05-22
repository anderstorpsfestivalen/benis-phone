import { useState } from "react";
import type { File as FileT, Playable, Queue, QueuePrompt } from "../generated/config";
import { emptyQueuePrompt } from "../lib/empty";
import { Field, TextInput, NumberInput, CheckboxInput } from "./Field";
import TTSEditor from "./TTSEditor";
import FilePicker from "./FilePicker";

// Full-coverage queue editor. Mirrors FnEditor's shape: scalar fields up
// top, collapsible sub-objects below, and an Actions-style list for the
// prompts array. The queue's End action is its own node — clicking the
// link here jumps there.

export default function QueueEditor({
  value,
  onChange,
  onSelectEnd,
}: {
  value: Queue;
  onChange: (v: Queue) => void;
  onSelectEnd?: () => void;
}) {
  const set = <K extends keyof Queue>(k: K, v: Queue[K]) =>
    onChange({ ...value, [k]: v });

  function addPrompt() {
    set("prompt", [...value.prompt, emptyQueuePrompt()]);
  }
  function updatePrompt(i: number, p: QueuePrompt) {
    const next = [...value.prompt];
    next[i] = p;
    set("prompt", next);
  }
  function removePrompt(i: number) {
    set("prompt", value.prompt.filter((_, n) => n !== i));
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <Field
          label="Name"
          help="Unique queue name. Dispatcher actions reference this name to hand callers to the queue."
        >
          <TextInput value={value.name} onChange={(v) => set("name", v)} />
        </Field>
        <Field
          label="Speed (s)"
          help="Simulated rate at which the announced position advances, in seconds per step. Lower = faster perceived progress."
        >
          <NumberInput value={value.speed} onChange={(v) => set("speed", v)} />
        </Field>
        <Field
          label="Minpos"
          help="Lower bound on the announced queue position. Callers below this number are still told they're at this position."
        >
          <NumberInput value={value.minpos} onChange={(v) => set("minpos", v)} />
        </Field>
        <Field
          label="Maxpos"
          help="Upper bound on the announced queue position. Big queues feel less hopeless when capped."
        >
          <NumberInput value={value.maxpos} onChange={(v) => set("maxpos", v)} />
        </Field>
        <Field
          label="Min prompt (s)"
          help="Minimum gap between two prompt announcements played to a held caller."
        >
          <NumberInput value={value.minprompt} onChange={(v) => set("minprompt", v)} />
        </Field>
        <Field
          label="Max prompt (s)"
          help="Maximum gap. Actual interval is uniform-random between min and max."
        >
          <NumberInput value={value.maxprompt} onChange={(v) => set("maxprompt", v)} />
        </Field>
      </div>

      <details className="border border-shadow-grey rounded" open={!!value.entrymsg.tts.msg || !!value.entrymsg.file.src}>
        <summary className="px-3 py-2 cursor-pointer text-sm text-blue-slate">
          Entry message (played once on arrival)
        </summary>
        <div className="p-3">
          <PlayableEditor
            value={value.entrymsg}
            onChange={(v) => set("entrymsg", v)}
          />
        </div>
      </details>

      <details className="border border-shadow-grey rounded" open={!!value.currentpos.msg}>
        <summary className="px-3 py-2 cursor-pointer text-sm text-blue-slate">
          Current-position announcement
        </summary>
        <div className="p-3">
          <TTSEditor
            value={value.currentpos}
            onChange={(v) => set("currentpos", v)}
          />
          <p className="mt-2 text-xs text-blue-slate">
            Use <code>{"{{.Position}}"}</code> in the message to interpolate the caller's place in line.
          </p>
        </div>
      </details>

      <details className="border border-shadow-grey rounded" open={!!value.bgmusic.src}>
        <summary className="px-3 py-2 cursor-pointer text-sm text-blue-slate">
          Background music (loops while held)
        </summary>
        <div className="p-3">
          <FileEditor value={value.bgmusic} onChange={(v) => set("bgmusic", v)} />
        </div>
      </details>

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-blue-slate uppercase">
            Prompts ({value.prompt.length})
          </span>
          <button
            onClick={addPrompt}
            className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
          >
            + prompt
          </button>
        </div>
        {value.prompt.length === 0 && (
          <div className="text-xs text-blue-slate italic">
            No prompts yet. Add one — it'll be selected weighted-randomly between min/max prompt intervals.
          </div>
        )}
        <div className="flex flex-col gap-2">
          {value.prompt.map((p, i) => (
            <PromptRow
              key={i}
              value={p}
              onChange={(v) => updatePrompt(i, v)}
              onRemove={() => removePrompt(i)}
            />
          ))}
        </div>
      </div>

      <div className="border border-shadow-grey rounded p-3 flex items-center gap-3">
        <span className="text-xs text-blue-slate uppercase flex-1">End action</span>
        <button
          onClick={onSelectEnd}
          className="text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
        >
          edit end action →
        </button>
      </div>
    </div>
  );
}

// PlayableEditor — { file, tts, wait, clear }
function PlayableEditor({
  value,
  onChange,
}: {
  value: Playable;
  onChange: (v: Playable) => void;
}) {
  const set = <K extends keyof Playable>(k: K, v: Playable[K]) =>
    onChange({ ...value, [k]: v });

  return (
    <div className="flex flex-col gap-3">
      <div>
        <span className="text-xs text-blue-slate uppercase">TTS</span>
        <div className="mt-1">
          <TTSEditor value={value.tts} onChange={(v) => set("tts", v)} />
        </div>
      </div>
      <details className="border border-shadow-grey rounded" open={!!value.file.src}>
        <summary className="px-3 py-2 cursor-pointer text-xs text-blue-slate uppercase">
          File (overrides TTS when set)
        </summary>
        <div className="p-3">
          <FileEditor value={value.file} onChange={(v) => set("file", v)} />
        </div>
      </details>
      <div className="flex items-center gap-4">
        <CheckboxInput
          label="wait"
          value={value.wait}
          onChange={(v) => set("wait", v)}
          help="Block whatever follows until the announcement finishes playing."
        />
        <CheckboxInput
          label="clear"
          value={value.clear}
          onChange={(v) => set("clear", v)}
          help="Stop any currently-playing audio before this plays."
        />
      </div>
    </div>
  );
}

// FileEditor — { src, block, clear } with the Pick-from-R2 affordance.
function FileEditor({
  value,
  onChange,
}: {
  value: FileT;
  onChange: (v: FileT) => void;
}) {
  const [pick, setPick] = useState(false);
  return (
    <div className="grid grid-cols-3 gap-2">
      <div className="col-span-3">
        <Field label="Path" help="Path under files/. Use Pick to browse the R2 bucket.">
          <div className="flex gap-2">
            <TextInput value={value.src} onChange={(v) => onChange({ ...value, src: v })} />
            <button
              type="button"
              onClick={() => setPick(true)}
              className="shrink-0 text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
            >
              Pick…
            </button>
          </div>
        </Field>
      </div>
      <CheckboxInput
        label="block"
        value={value.block}
        onChange={(v) => onChange({ ...value, block: v })}
        help="Hold until playback finishes."
      />
      <CheckboxInput
        label="clear"
        value={value.clear}
        onChange={(v) => onChange({ ...value, clear: v })}
        help="Stop any currently-playing audio first."
      />
      {pick && (
        <FilePicker
          mode="file"
          onClose={() => setPick(false)}
          onPick={(key) => {
            onChange({ ...value, src: "files/" + key });
            setPick(false);
          }}
        />
      )}
    </div>
  );
}

// PromptRow — compact editor for one QueuePrompt.
function PromptRow({
  value,
  onChange,
  onRemove,
}: {
  value: QueuePrompt;
  onChange: (v: QueuePrompt) => void;
  onRemove: () => void;
}) {
  const [open, setOpen] = useState(false);
  const preview = value.empty
    ? "(silent placeholder)"
    : value.prompt.tts.msg || value.prompt.file.src || "(empty)";

  return (
    <div className="border border-shadow-grey rounded">
      <div className="flex items-center gap-2 p-2 text-xs font-mono">
        <button
          onClick={() => setOpen(!open)}
          className="text-blue-slate hover:text-white w-4 text-left"
        >
          {open ? "▼" : "▶"}
        </button>
        <span className="text-white truncate flex-1">{preview}</span>
        <span className="text-blue-slate">w={value.weight}</span>
        {value.empty && <span className="text-blue-slate uppercase text-[10px]">empty</span>}
        <button
          onClick={onRemove}
          className="text-blue-slate hover:text-white border border-shadow-grey rounded px-2 py-0.5"
        >
          ×
        </button>
      </div>
      {open && (
        <div className="p-3 border-t border-shadow-grey flex flex-col gap-3">
          <div className="flex items-center gap-4">
            <Field
              label="Weight"
              help="Selection weight for this prompt. Higher = more likely to play."
            >
              <NumberInput
                value={value.weight}
                onChange={(v) => onChange({ ...value, weight: v })}
              />
            </Field>
            <CheckboxInput
              label="empty slot"
              value={value.empty}
              onChange={(v) => onChange({ ...value, empty: v })}
              help="When picked, play nothing — useful as a probability of silence between prompts."
            />
          </div>
          {!value.empty && (
            <PlayableEditor
              value={value.prompt}
              onChange={(v) => onChange({ ...value, prompt: v })}
            />
          )}
        </div>
      )}
    </div>
  );
}

