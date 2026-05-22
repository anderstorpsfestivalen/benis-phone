import { useEffect, useMemo, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api } from "../lib/api";
import { emptyDefinition, emptyFn, emptyQueue } from "../lib/empty";
import { renderToml } from "../lib/toml-render";
import type { Definition } from "../generated/config";
import { Field, TextInput, NumberInput, CheckboxInput } from "../components/Field";
import FnEditor from "../components/FnEditor";

type Tab = "general" | "sip" | "fn" | "queue" | "toml";

export default function ConfigEditor() {
  const { name = "" } = useParams();
  const [doc, setDoc] = useState<Definition>(emptyDefinition());
  const [tab, setTab] = useState<Tab>("general");
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [savedHash, setSavedHash] = useState<string>("");

  useEffect(() => {
    api.get(name)
      .then((p) => {
        setDoc(p.doc);
        setSavedHash(p.hash);
      })
      .catch((e) => setErr(String(e)));
  }, [name]);

  const toml = useMemo(() => renderToml(doc), [doc]);
  const fnNames = useMemo(() => doc.fn.map((f) => f.name).filter(Boolean), [doc]);

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      const p = await api.save(name, doc, toml);
      setSavedHash(p.hash);
    } catch (e) {
      setErr(String(e));
    } finally {
      setSaving(false);
    }
  }

  const setGeneral = (k: keyof Definition["general"], v: string) =>
    setDoc({ ...doc, general: { ...doc.general, [k]: v } });
  const setSip = <K extends keyof Definition["sip"]>(k: K, v: Definition["sip"][K]) =>
    setDoc({ ...doc, sip: { ...doc.sip, [k]: v } });

  return (
    <div className="max-w-5xl mx-auto">
      <div className="flex items-center gap-3 mb-4">
        <Link to="/" className="text-blue-slate hover:text-white text-sm">← back</Link>
        <h1 className="font-mono text-white">{name}</h1>
        <span className="text-xs text-blue-slate font-mono">{savedHash.slice(0, 12)}</span>
        <div className="ml-auto flex gap-2">
          <button
            onClick={save}
            disabled={saving}
            className="px-4 py-2 bg-blue-slate text-white rounded hover:bg-shadow-grey disabled:opacity-50"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {err && <div className="text-blue-slate mb-3 text-sm">{err}</div>}

      <div className="border-b border-shadow-grey mb-4 flex gap-1">
        {(["general", "sip", "fn", "queue", "toml"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-mono ${
              tab === t
                ? "border-b-2 border-blue-slate text-white"
                : "text-blue-slate hover:text-white"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "general" && (
        <div className="grid grid-cols-2 gap-3 max-w-2xl">
          <Field label="Entrypoint" hint="name of the starting menu">
            <TextInput
              value={doc.general.entrypoint}
              onChange={(v) => setGeneral("entrypoint", v)}
            />
          </Field>
          <Field label="Default TTS provider" hint="polly | elevenlabs">
            <TextInput
              value={doc.general.default_tts_provider}
              onChange={(v) => setGeneral("default_tts_provider", v)}
            />
          </Field>
          <Field label="Default TTS voice">
            <TextInput
              value={doc.general.default_tts_voice}
              onChange={(v) => setGeneral("default_tts_voice", v)}
            />
          </Field>
          <Field label="Default TTS language">
            <TextInput
              value={doc.general.default_tts_lang}
              onChange={(v) => setGeneral("default_tts_lang", v)}
            />
          </Field>
          <Field label="Default TTS engine">
            <TextInput
              value={doc.general.default_tts_engine}
              onChange={(v) => setGeneral("default_tts_engine", v)}
            />
          </Field>
        </div>
      )}

      {tab === "sip" && (
        <div className="grid grid-cols-2 gap-3 max-w-2xl">
          <Field label="Server">
            <TextInput value={doc.sip.server} onChange={(v) => setSip("server", v)} />
          </Field>
          <Field label="Extension">
            <TextInput value={doc.sip.extension} onChange={(v) => setSip("extension", v)} />
          </Field>
          <Field label="Username">
            <TextInput value={doc.sip.username} onChange={(v) => setSip("username", v)} />
          </Field>
          <Field label="Domain">
            <TextInput value={doc.sip.domain} onChange={(v) => setSip("domain", v)} />
          </Field>
          <Field label="Transport" hint="udp | tcp | ws | wss">
            <TextInput value={doc.sip.transport} onChange={(v) => setSip("transport", v)} />
          </Field>
          <Field label="Local port">
            <NumberInput value={doc.sip.local_port} onChange={(v) => setSip("local_port", v)} />
          </Field>
          <Field label="Max concurrent calls">
            <NumberInput
              value={doc.sip.max_concurrent_calls}
              onChange={(v) => setSip("max_concurrent_calls", v)}
            />
          </Field>
          <Field label="Record path">
            <TextInput value={doc.sip.record_path} onChange={(v) => setSip("record_path", v)} />
          </Field>
          <Field label="Expiry (s)">
            <NumberInput
              value={doc.sip.expiry_seconds}
              onChange={(v) => setSip("expiry_seconds", v)}
            />
          </Field>
          <Field label="External IP">
            <TextInput value={doc.sip.external_ip} onChange={(v) => setSip("external_ip", v)} />
          </Field>
          <CheckboxInput
            label="Direct mode (no PBX registration)"
            value={doc.sip.direct}
            onChange={(v) => setSip("direct", v)}
          />
        </div>
      )}

      {tab === "fn" && (
        <div className="flex flex-col gap-3">
          <button
            onClick={() => setDoc({ ...doc, fn: [...doc.fn, emptyFn(`fn${doc.fn.length + 1}`)] })}
            className="self-start px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded text-sm"
          >
            + add fn
          </button>
          {doc.fn.map((f, i) => (
            <FnEditor
              key={i}
              value={f}
              knownFnNames={fnNames}
              onChange={(v) => {
                const next = [...doc.fn];
                next[i] = v;
                setDoc({ ...doc, fn: next });
              }}
              onRemove={() => setDoc({ ...doc, fn: doc.fn.filter((_, n) => n !== i) })}
            />
          ))}
        </div>
      )}

      {tab === "queue" && (
        <div className="flex flex-col gap-3">
          <button
            onClick={() =>
              setDoc({ ...doc, queue: [...doc.queue, emptyQueue(`queue${doc.queue.length + 1}`)] })
            }
            className="self-start px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded text-sm"
          >
            + add queue
          </button>
          {doc.queue.map((q, i) => (
            <div key={i} className="bg-gunmetal border border-shadow-grey rounded p-3">
              <div className="flex items-center gap-3 mb-3">
                <Field label="Name">
                  <TextInput
                    value={q.name}
                    onChange={(v) => {
                      const next = [...doc.queue];
                      next[i] = { ...q, name: v };
                      setDoc({ ...doc, queue: next });
                    }}
                  />
                </Field>
                <button
                  onClick={() => setDoc({ ...doc, queue: doc.queue.filter((_, n) => n !== i) })}
                  className="ml-auto text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
                >
                  Remove
                </button>
              </div>
              <div className="grid grid-cols-3 gap-2 text-sm">
                <Field label="Minpos">
                  <NumberInput
                    value={q.minpos}
                    onChange={(v) => {
                      const next = [...doc.queue];
                      next[i] = { ...q, minpos: v };
                      setDoc({ ...doc, queue: next });
                    }}
                  />
                </Field>
                <Field label="Maxpos">
                  <NumberInput
                    value={q.maxpos}
                    onChange={(v) => {
                      const next = [...doc.queue];
                      next[i] = { ...q, maxpos: v };
                      setDoc({ ...doc, queue: next });
                    }}
                  />
                </Field>
                <Field label="Speed (s)">
                  <NumberInput
                    value={q.speed}
                    onChange={(v) => {
                      const next = [...doc.queue];
                      next[i] = { ...q, speed: v };
                      setDoc({ ...doc, queue: next });
                    }}
                  />
                </Field>
              </div>
              <p className="text-xs text-blue-slate mt-3">
                Queue prompts and templates can be edited via the TOML tab for now —
                richer queue editing coming next.
              </p>
            </div>
          ))}
        </div>
      )}

      {tab === "toml" && (
        <pre className="bg-ink-black border border-shadow-grey rounded p-4 text-xs font-mono whitespace-pre-wrap overflow-x-auto">
          {toml}
        </pre>
      )}
    </div>
  );
}
