import { useEffect, useMemo, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api } from "../lib/api";
import { emptyDefinition } from "../lib/empty";
import { renderToml } from "../lib/toml-render";
import type { Definition } from "../generated/config";
import { Field, TextInput, NumberInput, CheckboxInput } from "../components/Field";
import FnGraph from "../components/FnGraph";

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

  useEffect(() => {
    api.get(name)
      .then((p) => {
        setDoc(p.doc);
        setSavedHash(p.hash);
      })
      .catch((e) => setErr(String(e)));
  }, [name]);

  const toml = useMemo(() => renderToml(doc), [doc]);

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
    <div className={tab === "fn" ? "px-4" : "max-w-5xl mx-auto px-4"}>
      <div className="flex items-center gap-3 py-2">
        <Link to="/" className="text-blue-slate hover:text-white text-sm">← back</Link>
        <h1 className="font-mono text-white text-sm">{name}</h1>
        <span className="text-xs text-blue-slate font-mono">{savedHash.slice(0, 12)}</span>

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
          entrypoint={doc.general.entrypoint}
          onFnsChange={(fns) => setDoc({ ...doc, fn: fns })}
          onQueuesChange={(queue) => setDoc({ ...doc, queue })}
        />
      )}

      {tab === "general" && (
        <div className="grid grid-cols-2 gap-3 max-w-2xl">
          <Field
            label="Entrypoint"
            help="Name of the menu (fn) where inbound calls land. Usually 'main'. Must match a defined fn."
          >
            <TextInput
              value={doc.general.entrypoint}
              onChange={(v) => setGeneral("entrypoint", v)}
            />
          </Field>
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
        <div className="grid grid-cols-2 gap-3 max-w-2xl">
          <Field
            label="Server"
            help="Host:port of the upstream SIP PBX to register with (e.g. pbx.example.com:5060). In -direct mode this is ignored."
          >
            <TextInput value={doc.sip.server} onChange={(v) => setSip("server", v)} />
          </Field>
          <Field
            label="Extension"
            help="The extension number this IVR registers as. Inbound calls to this extension are answered by the IVR."
          >
            <TextInput value={doc.sip.extension} onChange={(v) => setSip("extension", v)} />
          </Field>
          <Field
            label="Username"
            help="SIP auth username, usually the same as the extension. Paired with the password in creds.json."
          >
            <TextInput value={doc.sip.username} onChange={(v) => setSip("username", v)} />
          </Field>
          <Field
            label="Domain"
            help="SIP realm/domain (the part after @ in SIP URIs). Often matches the PBX hostname."
          >
            <TextInput value={doc.sip.domain} onChange={(v) => setSip("domain", v)} />
          </Field>
          <Field
            label="Transport"
            help="SIP transport. udp is the default for PBX registration. tcp/ws/wss are supported by the underlying library but rarely used here."
          >
            <TextInput value={doc.sip.transport} onChange={(v) => setSip("transport", v)} />
          </Field>
          <Field
            label="Local port"
            help="UDP/TCP port this binary listens on for SIP signaling. 5060 is the SIP default."
          >
            <NumberInput value={doc.sip.local_port} onChange={(v) => setSip("local_port", v)} />
          </Field>
          <Field
            label="Max concurrent calls"
            help="Hard cap on simultaneous IVR sessions. Additional inbound INVITEs are rejected with 486 Busy."
          >
            <NumberInput
              value={doc.sip.max_concurrent_calls}
              onChange={(v) => setSip("max_concurrent_calls", v)}
            />
          </Field>
          <Field
            label="Record path"
            help="Filesystem path where call recordings are written. The Fn-level recording subfolder is appended to this."
          >
            <TextInput value={doc.sip.record_path} onChange={(v) => setSip("record_path", v)} />
          </Field>
          <Field
            label="Expiry (s)"
            help="REGISTER refresh interval in seconds. Shorter = faster recovery from PBX restart, more network chatter. 300 is typical."
          >
            <NumberInput
              value={doc.sip.expiry_seconds}
              onChange={(v) => setSip("expiry_seconds", v)}
            />
          </Field>
          <Field
            label="External IP"
            help="Public IP advertised in SDP for RTP. Leave blank if the host has a public IP; set explicitly when behind NAT."
          >
            <TextInput value={doc.sip.external_ip} onChange={(v) => setSip("external_ip", v)} />
          </Field>
          <CheckboxInput
            label="Direct mode (no PBX registration)"
            value={doc.sip.direct}
            onChange={(v) => setSip("direct", v)}
            help="Skip REGISTER and accept any unauthenticated INVITE directly to this host:port. Used for local testing with a softphone."
          />
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
