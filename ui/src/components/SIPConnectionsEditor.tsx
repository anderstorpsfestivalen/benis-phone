import { useState } from "react";
import type {
  Fn,
  SIPConfig,
  SIPConnection,
  SIPRoute,
} from "../generated/config";
import type { SIPStatusSnapshot } from "../lib/api";
import { emptySIPConnection, emptySIPRoute } from "../lib/empty";
import { CheckboxInput, Field, NumberInput, TextInput } from "./Field";

export default function SIPConnectionsEditor({
  value,
  fns,
  secretState,
  secretEdits,
  status,
  legacy,
  onChange,
  onSecretEdit,
}: {
  value: SIPConfig;
  fns: Fn[];
  secretState: Record<string, boolean>;
  secretEdits: Record<string, string | null>;
  status: SIPStatusSnapshot;
  legacy: boolean;
  onChange: (value: SIPConfig) => void;
  onSecretEdit: (
    connectionID: string,
    value: string | null | undefined,
  ) => void;
}) {
  const names = fns.map((fn) => fn.name).filter(Boolean);

  function addConnection() {
    onChange({
      ...value,
      connection: [...value.connection, emptySIPConnection(names[0] ?? "")],
    });
  }
  function updateConnection(index: number, connection: SIPConnection) {
    const next = [...value.connection];
    next[index] = connection;
    onChange({ ...value, connection: next });
  }
  function removeConnection(index: number) {
    const connection = value.connection[index];
    if (
      !confirm(`Remove SIP connection "${connection.name || connection.id}"?`)
    )
      return;
    onChange({
      ...value,
      connection: value.connection.filter((_, i) => i !== index),
    });
    // Removing the connection also removes its stored secret server-side.
    // Drop any pending edit instead of sending a mutation for an ID that no
    // longer exists in the document.
    onSecretEdit(connection.id, undefined);
  }
  function moveConnection(index: number, delta: number) {
    const target = index + delta;
    if (target < 0 || target >= value.connection.length) return;
    const next = [...value.connection];
    [next[index], next[target]] = [next[target], next[index]];
    onChange({ ...value, connection: next });
  }

  return (
    <div className="flex flex-col gap-4">
      {legacy && (
        <div className="border border-warning bg-warning/10 text-warning rounded p-3 text-sm">
          This is a legacy single-SIP config. Review the prefilled connection,
          choose its entrypoint, enter its password, and save to complete the
          required migration.
        </div>
      )}
      <div className="grid grid-cols-2 gap-3 max-w-2xl">
        <Field
          label="Maximum concurrent calls"
          help="Shared cap across every SIP connection in this config."
        >
          <NumberInput
            value={value.max_concurrent_calls}
            onChange={(max_concurrent_calls) =>
              onChange({ ...value, max_concurrent_calls })
            }
          />
        </Field>
        <Field label="Recording base path">
          <TextInput
            value={value.record_path}
            onChange={(record_path) => onChange({ ...value, record_path })}
          />
        </Field>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-xs text-blue-slate uppercase tracking-wide">
          Connections ({value.connection.length})
        </span>
        <button
          onClick={addConnection}
          className="px-3 py-1.5 rounded bg-blue-slate text-white text-sm"
        >
          + add connection
        </button>
      </div>

      {value.connection.map((connection, index) => (
        <ConnectionCard
          key={connection.id}
          connection={connection}
          fnNames={names}
          configured={
            secretEdits[connection.id] === null
              ? false
              : typeof secretEdits[connection.id] === "string" ||
                !!secretState[connection.id]
          }
          passwordEdit={secretEdits[connection.id]}
          statuses={statusesFor(status, connection.id)}
          onChange={(next) => updateConnection(index, next)}
          onRemove={() => removeConnection(index)}
          onMoveUp={index > 0 ? () => moveConnection(index, -1) : undefined}
          onMoveDown={
            index < value.connection.length - 1
              ? () => moveConnection(index, 1)
              : undefined
          }
          onSecretEdit={(next) => onSecretEdit(connection.id, next)}
        />
      ))}
      {value.connection.length === 0 && (
        <div className="border border-shadow-grey rounded p-8 text-center text-sm text-blue-slate">
          Add an endpoint or trunk to make this IVR reachable.
        </div>
      )}
    </div>
  );
}

function ConnectionCard({
  connection,
  fnNames,
  configured,
  passwordEdit,
  statuses,
  onChange,
  onRemove,
  onMoveUp,
  onMoveDown,
  onSecretEdit,
}: {
  connection: SIPConnection;
  fnNames: string[];
  configured: boolean;
  passwordEdit: string | null | undefined;
  statuses: ConnectionInstanceStatus[];
  onChange: (value: SIPConnection) => void;
  onRemove: () => void;
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  onSecretEdit: (value: string | null | undefined) => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  const set = <K extends keyof SIPConnection>(
    key: K,
    value: SIPConnection[K],
  ) => onChange({ ...connection, [key]: value });
  const health = aggregateHealth(statuses);

  function switchKind(kind: string) {
    if (kind === "trunk") {
      onChange({
        ...connection,
        kind,
        entrypoint: "",
        route: connection.route.length
          ? connection.route
          : [emptySIPRoute(fnNames[0] ?? "")],
      });
    } else {
      onChange({
        ...connection,
        kind,
        entrypoint: connection.entrypoint || fnNames[0] || "",
        route: [],
      });
    }
  }

  function switchRegistration(registration: string) {
    onChange({ ...connection, registration });
  }

  return (
    <section className="border border-shadow-grey rounded bg-gunmetal/30">
      <div className="flex items-center gap-3 px-4 py-3 border-b border-shadow-grey">
        <span
          className={`w-2.5 h-2.5 rounded-full ${health.dot}`}
          title={health.label}
        />
        <strong className="font-mono text-sm flex-1">
          {connection.name || connection.id}
        </strong>
        <span className={`text-xs ${health.text}`}>{health.label}</span>
        <button
          onClick={onMoveUp}
          disabled={!onMoveUp}
          className="text-xs text-blue-slate disabled:opacity-20"
          title="Move connection up"
        >
          ↑
        </button>
        <button
          onClick={onMoveDown}
          disabled={!onMoveDown}
          className="text-xs text-blue-slate disabled:opacity-20"
          title="Move connection down"
        >
          ↓
        </button>
        <button
          onClick={onRemove}
          className="text-xs text-danger hover:text-white"
        >
          remove
        </button>
      </div>
      <div className="p-4 flex flex-col gap-4">
        <div className="grid grid-cols-3 gap-3">
          <Field label="Name">
            <TextInput
              value={connection.name}
              onChange={(v) => set("name", v)}
            />
          </Field>
          <SelectField
            label="Kind"
            value={connection.kind}
            onChange={switchKind}
            options={["endpoint", "trunk"]}
          />
          <SelectField
            label="Mode"
            value={connection.registration}
            onChange={switchRegistration}
            options={["registered", "inbound"]}
          />
        </div>

        {connection.kind === "endpoint" ? (
          <FunctionSelect
            label="Entrypoint"
            value={connection.entrypoint}
            fnNames={fnNames}
            onChange={(v) => set("entrypoint", v)}
          />
        ) : (
          <RouteEditor
            routes={connection.route}
            fnNames={fnNames}
            onChange={(route) => set("route", route)}
          />
        )}

        {connection.registration === "registered" ? (
          <div className="grid grid-cols-3 gap-3">
            <Field label="Server">
              <TextInput
                value={connection.server}
                onChange={(v) => set("server", v)}
                placeholder="pbx.example.com:5060"
              />
            </Field>
            <Field label="Extension">
              <TextInput
                value={connection.extension}
                onChange={(v) => set("extension", v)}
              />
            </Field>
            <Field label="Username">
              <TextInput
                value={connection.username}
                onChange={(v) => set("username", v)}
              />
            </Field>
          </div>
        ) : (
          <Field
            label="Authentication username"
            hint="Asterisk must use this username in its outbound auth object."
          >
            <TextInput
              value={connection.username}
              onChange={(v) => set("username", v)}
              placeholder="asterisk"
            />
          </Field>
        )}

        <div className="grid grid-cols-[1fr_auto] gap-3 items-end">
          <Field
            label={`Password — ${passwordEdit === null ? "cleared" : configured ? "configured" : "missing"}`}
            hint={
              connection.registration === "inbound"
                ? "Stored encrypted and verified using SIP Digest authentication."
                : undefined
            }
          >
            <input
              type="password"
              value={typeof passwordEdit === "string" ? passwordEdit : ""}
              onChange={(event) =>
                onSecretEdit(event.target.value || undefined)
              }
              placeholder={
                configured
                  ? "Enter a new password to replace"
                  : "Enter SIP password"
              }
              className="px-2 py-1 rounded font-mono text-sm w-full"
              autoComplete="new-password"
            />
          </Field>
          <button
            onClick={() => onSecretEdit(null)}
            className="px-3 py-1.5 border border-shadow-grey rounded text-xs text-danger"
          >
            clear
          </button>
        </div>

        <button
          onClick={() => setAdvanced(!advanced)}
          className="text-left text-xs text-blue-slate hover:text-white"
        >
          {advanced ? "▾" : "▸"} advanced network settings
        </button>
        {advanced && (
          <div className="grid grid-cols-3 gap-3 border-t border-shadow-grey pt-3">
            <Field label="Domain">
              <TextInput
                value={connection.domain}
                onChange={(v) => set("domain", v)}
              />
            </Field>
            <SelectField
              label="Transport"
              value={connection.transport}
              onChange={(v) => set("transport", v)}
              options={["udp", "tcp", "ws"]}
            />
            <Field
              label="Local SIP port"
              hint={
                connection.registration === "registered"
                  ? "0 assigns a dedicated port automatically"
                  : "Required for inbound trunks"
              }
            >
              <NumberInput
                value={connection.local_port}
                onChange={(v) => set("local_port", v)}
              />
            </Field>
            <Field label="REGISTER expiry (s)">
              <NumberInput
                value={connection.expiry_seconds}
                onChange={(v) => set("expiry_seconds", v)}
              />
            </Field>
            <Field label="External IP">
              <TextInput
                value={connection.external_ip}
                onChange={(v) => set("external_ip", v)}
              />
            </Field>
            <Field
              label="Allowed source CIDRs"
              hint="One per line; required in inbound mode."
            >
              <textarea
                rows={3}
                value={connection.allowed_cidrs.join("\n")}
                onChange={(event) =>
                  set(
                    "allowed_cidrs",
                    event.target.value
                      .split(/[\n,]/)
                      .map((v) => v.trim())
                      .filter(Boolean),
                  )
                }
                className="px-2 py-1 rounded font-mono text-sm w-full"
                placeholder="192.0.2.0/24"
              />
            </Field>
          </div>
        )}

        <StatusLog statuses={statuses} />
      </div>
    </section>
  );
}

function RouteEditor({
  routes,
  fnNames,
  onChange,
}: {
  routes: SIPRoute[];
  fnNames: string[];
  onChange: (routes: SIPRoute[]) => void;
}) {
  const update = (index: number, route: SIPRoute) => {
    const next = [...routes];
    next[index] = route;
    onChange(next);
  };
  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-between text-xs text-blue-slate uppercase">
        <span>Trunk routes</span>
        <button
          onClick={() => onChange([...routes, emptySIPRoute(fnNames[0] ?? "")])}
          className="hover:text-white"
        >
          + add route
        </button>
      </div>
      {routes.map((route, index) => (
        <div
          key={route.id}
          className="grid grid-cols-[1fr_1fr_auto_auto] gap-2 items-end border border-shadow-grey rounded p-2"
        >
          <Field label={route.catch_all ? "Number" : "Exact called number"}>
            <TextInput
              value={route.number}
              onChange={(number) => update(index, { ...route, number })}
              placeholder={route.catch_all ? "Catch-all" : "+461234567"}
            />
          </Field>
          <FunctionSelect
            label="Entrypoint"
            value={route.entrypoint}
            fnNames={fnNames}
            onChange={(entrypoint) => update(index, { ...route, entrypoint })}
          />
          <CheckboxInput
            label="Catch-all"
            value={route.catch_all}
            onChange={(catch_all) =>
              update(index, {
                ...route,
                catch_all,
                number: catch_all ? "" : route.number,
              })
            }
          />
          <button
            onClick={() => onChange(routes.filter((_, i) => i !== index))}
            className="text-xs text-danger pb-1"
          >
            remove
          </button>
        </div>
      ))}
    </div>
  );
}

function SelectField({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
}) {
  return (
    <Field label={label}>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="px-2 py-1 rounded font-mono text-sm"
      >
        {options.map((option) => (
          <option key={option}>{option}</option>
        ))}
      </select>
    </Field>
  );
}

function FunctionSelect({
  label,
  value,
  fnNames,
  onChange,
}: {
  label: string;
  value: string;
  fnNames: string[];
  onChange: (value: string) => void;
}) {
  return (
    <Field label={label}>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="px-2 py-1 rounded font-mono text-sm"
      >
        <option value="">Select a menu…</option>
        {fnNames.map((name) => (
          <option key={name}>{name}</option>
        ))}
      </select>
    </Field>
  );
}

interface ConnectionInstanceStatus {
  instanceID: string;
  lastSeen: string;
  current: {
    state: string;
    code?: string;
    message?: string;
    at: string;
    local_port?: number;
  };
  events: Array<{
    state: string;
    code?: string;
    message?: string;
    at: string;
    local_port?: number;
  }>;
}

function statusesFor(
  snapshot: SIPStatusSnapshot,
  connectionID: string,
): ConnectionInstanceStatus[] {
  return snapshot.instances.flatMap((instance) =>
    instance.connections
      .filter((connection) => connection.current.connection_id === connectionID)
      .map((connection) => ({
        instanceID: instance.instance_id,
        lastSeen: instance.last_seen,
        ...connection,
      })),
  );
}

function aggregateHealth(statuses: ConnectionInstanceStatus[]) {
  if (statuses.length === 0)
    return {
      label: "no runtime status",
      dot: "bg-blue-slate",
      text: "text-blue-slate",
    };
  const live = statuses.filter(
    (status) => Date.now() - Date.parse(status.lastSeen) <= 90_000,
  );
  if (live.length === 0)
    return { label: "offline", dot: "bg-warning", text: "text-warning" };
  if (live.some((status) => status.current.state === "error"))
    return { label: "error", dot: "bg-danger", text: "text-danger" };
  if (
    live.every((status) =>
      ["ready", "listening"].includes(status.current.state),
    )
  )
    return { label: "healthy", dot: "bg-success", text: "text-success" };
  return {
    label: live.map((status) => status.current.state).join(", "),
    dot: "bg-warning",
    text: "text-warning",
  };
}

function StatusLog({ statuses }: { statuses: ConnectionInstanceStatus[] }) {
  if (statuses.length === 0) return null;
  return (
    <details className="border-t border-shadow-grey pt-3">
      <summary className="text-xs text-blue-slate cursor-pointer">
        runtime status log
      </summary>
      <div className="mt-2 flex flex-col gap-3">
        {statuses.map((status) => (
          <div key={status.instanceID}>
            <div className="font-mono text-xs text-white">
              {status.instanceID} · last seen{" "}
              {new Date(status.lastSeen).toLocaleString()}
            </div>
            <div className="mt-1 max-h-40 overflow-y-auto font-mono text-xs">
              {[...status.events].reverse().map((event, index) => (
                <div
                  key={`${event.at}-${index}`}
                  className={
                    event.state === "error" ? "text-danger" : "text-blue-slate"
                  }
                >
                  {new Date(event.at).toLocaleTimeString()} · {event.state}
                  {event.code ? ` (${event.code})` : ""}
                  {event.local_port ? ` · :${event.local_port}` : ""}
                  {event.message ? ` · ${event.message}` : ""}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </details>
  );
}
