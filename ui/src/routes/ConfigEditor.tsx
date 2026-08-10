import { useEffect, useMemo, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api, type SIPStatusSnapshot } from "../lib/api";
import { emptyDefinition } from "../lib/empty";
import { isLegacySIPToml, parseTomlConfig } from "../lib/toml-parse";
import { renderToml } from "../lib/toml-render";
import type { Definition } from "../generated/config";
import { Field, TextInput } from "../components/Field";
import FnGraph from "../components/FnGraph";
import SIPConnectionsEditor from "../components/SIPConnectionsEditor";
import { validateSIPDefinition } from "../lib/sip-validation";

// Tabs other than `fn` are secondary configuration — fn is the primary view
// the editor opens to and the only one that gets the full viewport width.
type Tab = "fn" | "general" | "sip" | "toml";

export default function ConfigEditor() {
  const { name = "" } = useParams();
  const [doc, setDoc] = useState<Definition>(emptyDefinition());
  const [tab, setTab] = useState<Tab>("fn");
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [savedHash, setSavedHash] = useState<string>("");
  const [secretState, setSecretState] = useState<Record<string, boolean>>({});
  const [secretEdits, setSecretEdits] = useState<Record<string, string | null>>(
    {},
  );
  const [sipStatus, setSipStatus] = useState<SIPStatusSnapshot>({
    instances: [],
  });
  const [legacySIP, setLegacySIP] = useState(false);

  useEffect(() => {
    api
      .get(name)
      .then((p) => {
        // Re-normalize through parseTomlConfig instead of trusting p.doc
        // verbatim: D1 stores the doc as JSON snapshotted at save time, so
        // older saves from before a field existed (e.g. action.name)
        // come back with that field undefined. Re-parsing from the TOML
        // backfills every field via the empty-factory defaults, which
        // keeps controlled inputs from flipping to uncontrolled and
        // leaking the previous selection's value into the new node.
        try {
          setLegacySIP(isLegacySIPToml(p.toml));
          setDoc(parseTomlConfig(p.toml));
        } catch {
          setDoc(p.doc);
        }
        setSavedHash(p.hash);
        setSecretState(p.sip_secret_state ?? {});
      })
      .catch((e) => setErr(String(e)));
  }, [name]);

  useEffect(() => {
    let closed = false;
    let socket: WebSocket | undefined;
    let reconnect: ReturnType<typeof setTimeout> | undefined;
    const refresh = () =>
      api
        .sipStatus(name)
        .then((snapshot) => {
          if (!closed) setSipStatus(snapshot);
        })
        .catch(() => {});
    refresh();
    const connect = () => {
      const protocol = location.protocol === "https:" ? "wss:" : "ws:";
      socket = new WebSocket(
        `${protocol}//${location.host}/api/configs/${encodeURIComponent(name)}/sip-status/ws`,
      );
      socket.onmessage = () => refresh();
      socket.onclose = () => {
        if (!closed) reconnect = setTimeout(connect, 2_000);
      };
      socket.onerror = () => socket?.close();
    };
    connect();
    const poll = setInterval(refresh, 30_000);
    return () => {
      closed = true;
      clearInterval(poll);
      if (reconnect) clearTimeout(reconnect);
      socket?.close();
    };
  }, [name]);

  const toml = useMemo(() => renderToml(doc), [doc]);

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      const validationErrors = validateSIPDefinition(doc);
      for (const connection of doc.sip.connection) {
        const hasPassword =
          secretEdits[connection.id] === null
            ? false
            : typeof secretEdits[connection.id] === "string" ||
              !!secretState[connection.id];
        if (connection.registration === "registered" && !hasPassword) {
          validationErrors.push(
            `${connection.name || connection.id}: enter a SIP password.`,
          );
        }
      }
      if (validationErrors.length) throw new Error(validationErrors.join(" "));
      const p = await api.save(name, doc, toml, secretEdits);
      setSavedHash(p.hash);
      setSecretState(p.sip_secret_state ?? {});
      setSecretEdits({});
      setLegacySIP(false);
    } catch (e) {
      setErr(String(e));
    } finally {
      setSaving(false);
    }
  }

  const setGeneral = (k: keyof Definition["general"], v: string) =>
    setDoc({ ...doc, general: { ...doc.general, [k]: v } });
  const setSecretEdit = (
    connectionID: string,
    value: string | null | undefined,
  ) => {
    setSecretEdits((current) => {
      const next = { ...current };
      if (value === undefined) delete next[connectionID];
      else next[connectionID] = value;
      return next;
    });
  };

  return (
    <div className={tab === "fn" ? "px-4" : "max-w-5xl mx-auto px-4"}>
      <div className="flex items-center gap-3 py-2">
        <Link to="/" className="text-blue-slate hover:text-white text-sm">
          ← back
        </Link>
        <h1 className="font-mono text-white text-sm">{name}</h1>
        <span className="text-xs text-blue-slate font-mono">
          {savedHash.slice(0, 12)}
        </span>

        <div className="flex gap-1 ml-4">
          {(["fn", "general", "sip", "toml"] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-3 py-1 text-xs font-mono rounded ${
                tab === t
                  ? "bg-blue-slate text-white"
                  : "text-blue-slate hover:text-white border border-shadow-grey"
              }`}
            >
              {t}
            </button>
          ))}
        </div>

        <div className="ml-auto flex gap-2">
          <button
            onClick={save}
            disabled={saving}
            className="px-4 py-1.5 bg-blue-slate text-white rounded hover:bg-shadow-grey disabled:opacity-50 text-sm"
          >
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {err && <div className="text-blue-slate mb-2 text-sm">{err}</div>}

      {tab === "fn" && (
        <FnGraph
          fns={doc.fn}
          queues={doc.queue}
          sip={doc.sip}
          onSelectSIP={() => setTab("sip")}
          onFnsChange={(fns) => setDoc({ ...doc, fn: fns })}
          onQueuesChange={(queue) => setDoc({ ...doc, queue })}
        />
      )}

      {tab === "general" && (
        <div className="grid grid-cols-2 gap-3 max-w-2xl">
          <Field
            label="Default TTS provider"
            help="Fallback TTS provider used by actions that don't specify their own. Supported: polly, elevenlabs."
          >
            <TextInput
              value={doc.general.default_tts_provider}
              onChange={(v) => setGeneral("default_tts_provider", v)}
            />
          </Field>
          <Field
            label="Default TTS voice"
            help="Provider-specific voice id (e.g. Polly: Joanna, Matthew). Overridden by per-action TTS.voice."
          >
            <TextInput
              value={doc.general.default_tts_voice}
              onChange={(v) => setGeneral("default_tts_voice", v)}
            />
          </Field>
          <Field
            label="Default TTS language"
            help="BCP-47 language code (e.g. en-US, sv-SE) passed to the provider when an action doesn't override."
          >
            <TextInput
              value={doc.general.default_tts_lang}
              onChange={(v) => setGeneral("default_tts_lang", v)}
            />
          </Field>
          <Field
            label="Default TTS engine"
            help="Provider-specific engine selector. Polly: standard | neural | generative. Ignored by providers without engines."
          >
            <TextInput
              value={doc.general.default_tts_engine}
              onChange={(v) => setGeneral("default_tts_engine", v)}
            />
          </Field>
        </div>
      )}

      {tab === "sip" && (
        <SIPConnectionsEditor
          value={doc.sip}
          fns={doc.fn}
          secretState={secretState}
          secretEdits={secretEdits}
          status={sipStatus}
          legacy={legacySIP}
          onChange={(sip) => setDoc({ ...doc, sip })}
          onSecretEdit={setSecretEdit}
        />
      )}

      {tab === "toml" && (
        <pre className="bg-ink-black border border-shadow-grey rounded p-4 text-xs font-mono whitespace-pre-wrap overflow-x-auto">
          {toml}
        </pre>
      )}
    </div>
  );
}
