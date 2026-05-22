import { useRef } from "react";
import { SERVICE_SCHEMAS, type ServiceSchema } from "../generated/services";

// Chip list of available template paths for a service. Clicking a chip
// inserts `{{.Path}}` at the textarea's cursor. The actual textarea lives
// in ActionEditor — we just wire a ref through.

export default function TemplateFieldPicker({
  service,
  value,
  onChange,
  rows = 5,
}: {
  service: string;
  value: string;
  onChange: (v: string) => void;
  rows?: number;
}) {
  const taRef = useRef<HTMLTextAreaElement | null>(null);
  const schema: ServiceSchema | undefined = (SERVICE_SCHEMAS as Record<string, ServiceSchema>)[service];

  function insert(path: string) {
    const ta = taRef.current;
    const insertion = `{{${path}}}`;
    if (!ta) {
      onChange(value + insertion);
      return;
    }
    const start = ta.selectionStart ?? value.length;
    const end = ta.selectionEnd ?? value.length;
    const next = value.slice(0, start) + insertion + value.slice(end);
    onChange(next);
    // Refocus after React re-renders so cursor lands after the insertion.
    requestAnimationFrame(() => {
      ta.focus();
      const pos = start + insertion.length;
      ta.setSelectionRange(pos, pos);
    });
  }

  const fields = schema?.templateFields ?? [];

  return (
    <div className="flex flex-col gap-2">
      <label className="text-xs text-blue-slate uppercase">
        Template (Go text/template, e.g. {"{{.Field}}"})
      </label>
      <textarea
        ref={taRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        rows={rows}
        className="px-2 py-1 rounded font-mono text-sm w-full"
      />
      {fields.length > 0 && (
        <div>
          <span className="text-xs text-blue-slate">Available fields — click to insert:</span>
          <div className="flex flex-wrap gap-1 mt-1">
            {fields.map((f) => (
              <button
                key={f.path}
                type="button"
                onClick={() => insert(f.path)}
                title={`${f.type}${f.description ? " — " + f.description : ""}`}
                className="text-xs font-mono px-2 py-0.5 border border-shadow-grey text-blue-slate hover:text-white hover:border-blue-slate rounded"
              >
                {f.path}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
