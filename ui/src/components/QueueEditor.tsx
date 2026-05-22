import type { Queue } from "../generated/config";
import { Field, TextInput, NumberInput } from "./Field";

// Side-panel editor for a Queue. Surfaces the simple scalar fields; richer
// queue prompt configuration still lives in the TOML view for now.

export default function QueueEditor({
  value,
  onChange,
}: {
  value: Queue;
  onChange: (v: Queue) => void;
}) {
  const set = <K extends keyof Queue>(k: K, v: Queue[K]) =>
    onChange({ ...value, [k]: v });

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
          label="Minpos"
          help="Lower bound on the announced queue position. Callers who arrive at a real position below this are still told they're at this position — keeps small queues from sounding alarming."
        >
          <NumberInput value={value.minpos} onChange={(v) => set("minpos", v)} />
        </Field>
        <Field
          label="Maxpos"
          help="Upper bound on the announced queue position. Callers beyond this are told they're at this position so big queues feel less hopeless."
        >
          <NumberInput value={value.maxpos} onChange={(v) => set("maxpos", v)} />
        </Field>
        <Field
          label="Speed (s)"
          help="Simulated rate at which the announced position advances, in seconds per step. Lower = faster perceived progress."
        >
          <NumberInput value={value.speed} onChange={(v) => set("speed", v)} />
        </Field>
        <Field
          label="Min prompt (s)"
          help="Minimum gap between two prompt announcements played to the held caller."
        >
          <NumberInput value={value.minprompt} onChange={(v) => set("minprompt", v)} />
        </Field>
        <Field
          label="Max prompt (s)"
          help="Maximum gap between prompt announcements. Actual interval is randomised between min and max."
        >
          <NumberInput value={value.maxprompt} onChange={(v) => set("maxprompt", v)} />
        </Field>
      </div>
      <p className="text-xs text-blue-slate">
        Detailed queue prompt / template / bgmusic editing still happens in the TOML
        view for now.
      </p>
    </div>
  );
}
