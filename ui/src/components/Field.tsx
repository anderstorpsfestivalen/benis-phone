import { ReactNode } from "react";
import HelpDot from "./HelpDot";

export function Field({
  label,
  children,
  hint,
  help,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
  help?: string;
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="text-blue-slate text-xs uppercase tracking-wide flex items-center">
        {label}
        {help && <HelpDot help={help} />}
      </span>
      {children}
      {hint && <span className="text-xs text-blue-slate/70">{hint}</span>}
    </label>
  );
}

export function TextInput({
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: "text" | "number";
}) {
  return (
    <input
      type={type}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      className="px-2 py-1 rounded font-mono text-sm w-full min-w-0"
    />
  );
}

export function NumberInput({
  value,
  onChange,
}: {
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <input
      type="number"
      value={value}
      onChange={(e) => onChange(Number(e.target.value) || 0)}
      className="px-2 py-1 rounded font-mono text-sm w-32"
    />
  );
}

export function CheckboxInput({
  label,
  value,
  onChange,
  help,
}: {
  label: string;
  value: boolean;
  onChange: (v: boolean) => void;
  help?: string;
}) {
  return (
    <label className="flex items-center gap-2 text-sm cursor-pointer">
      <input
        type="checkbox"
        checked={value}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span className="flex items-center">
        {label}
        {help && <HelpDot help={help} />}
      </span>
    </label>
  );
}

export function TextArea({
  value,
  onChange,
  rows = 4,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  rows?: number;
  placeholder?: string;
}) {
  return (
    <textarea
      value={value}
      onChange={(e) => onChange(e.target.value)}
      rows={rows}
      placeholder={placeholder}
      className="px-2 py-1 rounded font-mono text-sm w-full"
    />
  );
}
