import { useEffect, useMemo, useState } from "react";
import { useParams, Link } from "react-router-dom";
import {
  api,
  credentialKeys,
  type CredentialKey,
  type CredentialState,
  type SIPStatusSnapshot,
} from "../lib/api";
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
type Tab = "fn" | "general" | "sip" | "credentials" | "toml";

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
  const [credentialState, setCredentialState] =
    useState<CredentialState | null>(null);
  const [credentialEdits, setCredentialEdits] = useState<
    Partial<Record<CredentialKey, string | null>>
  >({});
  const [registrationID, setRegistrationID] = useState("");
  const [savingCredentials, setSavingCredentials] = useState(false);

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
        setRegistrationID(p.registration_id);
      })
      .catch((e) => setErr(String(e)));
    api
      .credentials(name)
      .then((payload) => setCredentialState(payload.state))
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

  async function saveCredentials() {
    if (Object.keys(credentialEdits).length === 0) return;
    setSavingCredentials(true);
    setErr(null);
    try {
      const result = await api.patchCredentials(name, credentialEdits);
      setCredentialState(result.state);
      setSavedHash(result.hash);
      setCredentialEdits({});
    } catch (caught) {
      setErr(String(caught));
    } finally {
      setSavingCredentials(false);
    }
  }

  async function rotateRegistration() {
    if (
      !confirm(
        "Rotate this registration ID? Existing bridges stay active, but the old ID can no longer enroll new bridges.",
      )
    )
      return;
    try {
      const result = await api.rotateRegistration(name);
      setRegistrationID(result.registration_id);
    } catch (caught) {
      setErr(String(caught));
    }
  }

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
          {(["fn", "general", "sip", "credentials", "toml"] as Tab[]).map(
            (t) => (
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
            ),
          )}
        </div>

        <div className="ml-auto flex gap-2">
          <button
            onClick={tab === "credentials" ? saveCredentials : save}
            disabled={
              tab === "credentials"
                ? savingCredentials || Object.keys(credentialEdits).length === 0
                : saving
            }
            className="px-4 py-1.5 bg-blue-slate text-white rounded hover:bg-shadow-grey disabled:opacity-50 text-sm"
          >
            {tab === "credentials"
              ? savingCredentials
                ? "Saving credentials…"
                : "Save credentials"
              : saving
                ? "Saving…"
                : "Save"}
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

      {tab === "credentials" && (
        <CredentialsEditor
          registrationID={registrationID}
          state={credentialState}
          edits={credentialEdits}
          saving={savingCredentials}
          onChange={(key, value) =>
            setCredentialEdits((current) => {
              const next = { ...current };
              if (value === undefined) delete next[key];
              else next[key] = value;
              return next;
            })
          }
          onSave={saveCredentials}
          onRotate={rotateRegistration}
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

const credentialLabels: Record<CredentialKey, string> = {
  r2_access_key: "R2 access key",
  r2_secret_key: "R2 secret key",
  r2_account_id: "R2 account ID",
  r2_bucket: "R2 bucket",
  polly_key: "Polly access key",
  polly_secret: "Polly secret key",
  elevenlabs_api_key: "ElevenLabs API key",
  backend_username: "Backend username",
  backend_password: "Backend password",
  trafikverket_key: "Trafikverket key",
  media_server_url: "Media-server URL",
  http_username: "Local HTTP username",
  http_password: "Local HTTP password",
};

function CredentialsEditor({
  registrationID,
  state,
  edits,
  saving,
  onChange,
  onSave,
  onRotate,
}: {
  registrationID: string;
  state: CredentialState | null;
  edits: Partial<Record<CredentialKey, string | null>>;
  saving: boolean;
  onChange: (key: CredentialKey, value: string | null | undefined) => void;
  onSave: () => void;
  onRotate: () => void;
}) {
  async function copyRegistration() {
    await navigator.clipboard.writeText(registrationID);
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <section className="border border-shadow-grey rounded p-4 bg-gunmetal">
        <h2 className="font-mono text-sm mb-2">Registration ID</h2>
        <p className="text-xs text-blue-slate mb-3">
          Start a new bridge with{" "}
          <span className="font-mono">
            go run benis-phone.go -register &lt;registration-id&gt;
          </span>
          .
        </p>
        <div className="flex gap-2">
          <code className="flex-1 border border-shadow-grey rounded px-3 py-2 text-xs break-all">
            {registrationID || "loading…"}
          </code>
          <button
            className="px-3 py-1 border border-shadow-grey rounded text-xs"
            disabled={!registrationID}
            onClick={copyRegistration}
          >
            Copy
          </button>
          <button
            className="px-3 py-1 border border-shadow-grey rounded text-xs"
            onClick={onRotate}
          >
            Rotate
          </button>
        </div>
      </section>

      <section className="border border-shadow-grey rounded p-4 bg-gunmetal">
        <div className="flex items-start justify-between gap-4 mb-4">
          <div>
            <h2 className="font-mono text-sm">Runtime credentials</h2>
            <p className="text-xs text-blue-slate mt-1">
              Values are write-only. The API returns configured state only.
            </p>
          </div>
          <button
            className="px-4 py-1.5 bg-blue-slate rounded text-sm disabled:opacity-50"
            disabled={saving || Object.keys(edits).length === 0}
            onClick={onSave}
          >
            {saving ? "Saving…" : "Save credentials"}
          </button>
        </div>
        <div className="grid gap-3">
          {credentialKeys.map((key) => {
            const edit = edits[key];
            const configured = state?.[key] ?? false;
            return (
              <div
                key={key}
                className="grid grid-cols-[180px_1fr_auto_auto] gap-2 items-center"
              >
                <label className="text-xs">{credentialLabels[key]}</label>
                <input
                  type="password"
                  autoComplete="new-password"
                  className="px-3 py-2 rounded text-sm font-mono"
                  placeholder={
                    configured ? "enter replacement" : "not configured"
                  }
                  value={typeof edit === "string" ? edit : ""}
                  onChange={(event) =>
                    onChange(key, event.target.value || undefined)
                  }
                />
                <span className="text-xs text-blue-slate w-24">
                  {edit === null
                    ? "will clear"
                    : typeof edit === "string"
                      ? "will replace"
                      : configured
                        ? "configured"
                        : "unconfigured"}
                </span>
                {edit !== undefined ? (
                  <button
                    className="text-xs px-2 py-1 border border-shadow-grey rounded"
                    onClick={() => onChange(key, undefined)}
                  >
                    Undo
                  </button>
                ) : (
                  <button
                    className="text-xs px-2 py-1 border border-shadow-grey rounded disabled:opacity-40"
                    disabled={!configured}
                    onClick={() => onChange(key, null)}
                  >
                    Clear
                  </button>
                )}
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}
