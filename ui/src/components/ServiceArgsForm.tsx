import { useEffect } from "react";
import { SERVICE_SCHEMAS, type ServiceSchema } from "../generated/services";
import { Field, TextInput, CheckboxInput } from "./Field";

// Schema-driven typed form for a service's args. The TOML wire format is
// still Record<string,string>; this component is a render+validation layer
// on top of that map, driven entirely by the per-service ArgSchema list
// emitted by tools/typegen.

export default function ServiceArgsForm({
  service,
  value,
  onChange,
}: {
  service: string;
  value: Record<string, string>;
  onChange: (v: Record<string, string>) => void;
}) {
  const schema: ServiceSchema | undefined = (SERVICE_SCHEMAS as Record<string, ServiceSchema>)[service];

  // When the service name changes (e.g. user picks weather → traintimes),
  // drop any args not in the new schema and seed defaults for new ones.
  useEffect(() => {
    if (!schema) return;
    const declared = new Set(schema.args.map((a) => a.name));
    const next: Record<string, string> = {};
    let dirty = false;
    for (const [k, v] of Object.entries(value)) {
      if (declared.has(k)) next[k] = v;
      else dirty = true;
    }
    for (const a of schema.args) {
      if (!(a.name in next)) {
        next[a.name] = a.default;
        if (a.default !== "" || !(a.name in value)) dirty = true;
      }
    }
    if (dirty) onChange(next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service]);

  if (!schema) {
    return (
      <div className="border border-shadow-grey rounded p-3 text-xs text-blue-slate">
        Schema missing for "{service}". Run <code>go generate ./...</code> to refresh
        ui/src/generated/services.ts.
      </div>
    );
  }

  if (schema.args.length === 0) {
    return (
      <div className="text-xs text-blue-slate italic">
        This service takes no arguments.
      </div>
    );
  }

  function setArg(name: string, v: string) {
    onChange({ ...value, [name]: v });
  }

  return (
    <div className="flex flex-col gap-3">
      <span className="text-xs text-blue-slate uppercase">Args</span>
      <div className="grid grid-cols-2 gap-3">
        {schema.args.map((a) => {
          const current = value[a.name] ?? a.default ?? "";
          const label = a.required ? `${a.name} *` : a.name;
          if (a.type === "boolean") {
            return (
              <CheckboxInput
                key={a.name}
                label={label}
                value={current === "true"}
                onChange={(v) => setArg(a.name, v ? "true" : "false")}
              />
            );
          }
          return (
            <Field key={a.name} label={label} hint={a.description}>
              <TextInput
                value={current}
                onChange={(v) => setArg(a.name, v)}
                placeholder={a.default}
                type={a.type === "number" ? "number" : "text"}
              />
            </Field>
          );
        })}
      </div>
    </div>
  );
}
