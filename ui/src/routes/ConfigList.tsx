import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, type ConfigSummary } from "../lib/api";
import { emptyDefinition } from "../lib/empty";
import { renderToml } from "../lib/toml-render";
import { parseTomlConfig } from "../lib/toml-parse";

export default function ConfigList() {
  const [configs, setConfigs] = useState<ConfigSummary[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const nav = useNavigate();

  async function refresh() {
    try {
      setConfigs(await api.list());
    } catch (e) {
      setErr(String(e));
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function create() {
    if (!newName.trim()) return;
    const doc = emptyDefinition();
    await api.save(newName.trim(), doc, renderToml(doc));
    setNewName("");
    nav(`/editor/${encodeURIComponent(newName.trim())}`);
  }

  async function importFile(file: File) {
    setErr(null);
    try {
      const text = await file.text();
      const doc = parseTomlConfig(text);
      // Default name = filename without extension; user can override.
      const defaultName = file.name.replace(/\.toml$/i, "").replace(/[^a-zA-Z0-9_-]/g, "_");
      const name = prompt("Save imported config as:", defaultName);
      if (!name) return;
      const trimmed = name.trim();
      if (configs?.some((c) => c.name === trimmed) &&
          !confirm(`"${trimmed}" already exists. Overwrite?`)) return;
      await api.save(trimmed, doc, renderToml(doc));
      nav(`/editor/${encodeURIComponent(trimmed)}`);
    } catch (e) {
      setErr(`Import failed: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  async function duplicate(name: string) {
    const to = prompt(`Duplicate "${name}" as:`);
    if (!to) return;
    await api.duplicate(name, to);
    refresh();
  }

  async function remove(name: string) {
    if (!confirm(`Delete config "${name}"? This cannot be undone.`)) return;
    await api.remove(name);
    refresh();
  }

  return (
    <div className="max-w-4xl mx-auto">
      <div className="flex items-center gap-2 mb-6">
        <input
          className="flex-1 px-3 py-2 rounded font-mono text-sm"
          placeholder="new config name (e.g. simonstorp)"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && create()}
        />
        <button
          className="px-4 py-2 bg-blue-slate text-white rounded hover:bg-shadow-grey"
          onClick={create}
        >
          Create
        </button>
        <button
          className="px-4 py-2 border border-shadow-grey text-blue-slate hover:text-white rounded"
          onClick={() => fileRef.current?.click()}
        >
          Import TOML…
        </button>
        <input
          ref={fileRef}
          type="file"
          accept=".toml,text/plain"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) importFile(f);
            // Reset so picking the same file twice re-fires onChange.
            e.target.value = "";
          }}
        />
      </div>

      {err && <div className="text-blue-slate mb-4">{err}</div>}

      <div className="grid gap-3">
        {configs?.length === 0 && (
          <div className="text-blue-slate text-sm">
            No configs yet. Create one above.
          </div>
        )}
        {configs?.map((c) => (
          <div
            key={c.name}
            className="bg-gunmetal border border-shadow-grey rounded p-4 flex items-center gap-4"
          >
            <Link
              to={`/editor/${encodeURIComponent(c.name)}`}
              className="font-mono text-white hover:text-blue-slate flex-1"
            >
              {c.name}
            </Link>
            <span className="text-xs text-blue-slate font-mono">
              {c.hash.slice(0, 12)}
            </span>
            <span className="text-xs text-blue-slate">
              {new Date(c.updated_at).toLocaleString()}
            </span>
            <button
              className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
              onClick={() => duplicate(c.name)}
            >
              Duplicate
            </button>
            <button
              className="text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
              onClick={() => remove(c.name)}
            >
              Delete
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
