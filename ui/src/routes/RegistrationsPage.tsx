import { useCallback, useEffect, useState } from "react";
import { api, type RegistrationsPayload } from "../lib/api";

export default function RegistrationsPage() {
  const [data, setData] = useState<RegistrationsPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setData(await api.registrations());
      setError(null);
    } catch (caught) {
      setError(String(caught));
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = setInterval(refresh, 30_000);
    return () => clearInterval(timer);
  }, [refresh]);

  async function approve(requestID: string, fingerprint: string) {
    const confirmation = prompt(
      `Compare the bridge output with:\n\n${fingerprint}\n\nPaste the complete fingerprint to approve:`,
    );
    if (confirmation === null) return;
    try {
      await api.approveRegistration(requestID, confirmation.trim());
      await refresh();
    } catch (caught) {
      setError(String(caught));
    }
  }

  async function deny(requestID: string) {
    if (!confirm("Deny this bridge enrollment?")) return;
    try {
      await api.denyRegistration(requestID);
      await refresh();
    } catch (caught) {
      setError(String(caught));
    }
  }

  async function revoke(bridgeID: string) {
    if (
      !confirm(
        `Revoke bridge ${bridgeID}? Its live socket will be closed immediately.`,
      )
    )
      return;
    try {
      await api.revokeBridge(bridgeID);
      await refresh();
    } catch (caught) {
      setError(String(caught));
    }
  }

  const pending =
    data?.enrollments.filter((item) => item.status === "pending") ?? [];
  const history =
    data?.enrollments.filter((item) => item.status !== "pending") ?? [];
  const active = data?.bridges.filter((item) => item.revoked_at === null) ?? [];
  const revoked =
    data?.bridges.filter((item) => item.revoked_at !== null) ?? [];

  return (
    <div className="max-w-7xl mx-auto space-y-8">
      <div>
        <h1 className="font-mono text-lg">Registrations</h1>
        <p className="text-sm text-blue-slate mt-1">
          Approve only after comparing the complete fingerprint with the one
          printed by the bridge.
        </p>
      </div>
      {error && <div className="text-blue-slate text-sm">{error}</div>}

      <Section title={`Pending (${pending.length})`}>
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-blue-slate">
              <Headers enrollment />
            </tr>
          </thead>
          <tbody>
            {pending.map((item) => (
              <tr
                key={item.request_id}
                className="border-t border-shadow-grey align-top"
              >
                <td className="py-3 pr-3 font-mono">{item.config_name}</td>
                <td className="py-3 pr-3 font-mono break-all max-w-xs">
                  {item.fingerprint}
                </td>
                <td className="py-3 pr-3">{item.hostname}</td>
                <td className="py-3 pr-3">{item.platform}</td>
                <td className="py-3 pr-3">{item.version}</td>
                <td className="py-3 pr-3">{date(item.created_at)}</td>
                <td className="py-3 whitespace-nowrap">
                  <button
                    className="px-2 py-1 bg-blue-slate rounded mr-2"
                    onClick={() => approve(item.request_id, item.fingerprint)}
                  >
                    Approve
                  </button>
                  <button
                    className="px-2 py-1 border border-shadow-grey rounded"
                    onClick={() => deny(item.request_id)}
                  >
                    Deny
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {pending.length === 0 && <Empty>No pending requests.</Empty>}
      </Section>

      <Section title={`Active (${active.length})`}>
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-blue-slate">
              <Headers />
            </tr>
          </thead>
          <tbody>
            {active.map((item) => (
              <tr
                key={item.bridge_id}
                className="border-t border-shadow-grey align-top"
              >
                <td className="py-3 pr-3 font-mono break-all">
                  {item.bridge_id}
                </td>
                <td className="py-3 pr-3 font-mono">{item.config_name}</td>
                <td className="py-3 pr-3 font-mono break-all max-w-xs">
                  {item.fingerprint}
                </td>
                <td className="py-3 pr-3">{date(item.approved_at)}</td>
                <td className="py-3 pr-3">
                  {item.last_seen ? date(item.last_seen) : "never"}
                </td>
                <td className="py-3 pr-3">
                  <span
                    className={item.online ? "text-white" : "text-blue-slate"}
                  >
                    {item.online ? "online" : "offline"}
                  </span>
                </td>
                <td className="py-3">
                  <button
                    className="px-2 py-1 border border-shadow-grey rounded"
                    onClick={() => revoke(item.bridge_id)}
                  >
                    Revoke
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {active.length === 0 && <Empty>No active bridges.</Empty>}
      </Section>

      <Section title="History">
        <div className="grid gap-2 text-xs">
          {history.map((item) => (
            <div
              key={item.request_id}
              className="border border-shadow-grey rounded p-3"
            >
              <span className="font-mono mr-3">{item.config_name}</span>
              <span className="mr-3">{item.status}</span>
              <span className="font-mono break-all">{item.fingerprint}</span>
            </div>
          ))}
          {revoked.map((item) => (
            <div
              key={item.bridge_id}
              className="border border-shadow-grey rounded p-3"
            >
              <span className="font-mono mr-3">{item.config_name}</span>
              <span className="mr-3">revoked</span>
              <span className="font-mono break-all">{item.bridge_id}</span>
            </div>
          ))}
          {history.length + revoked.length === 0 && (
            <Empty>
              No denied, expired, approved, or revoked registrations yet.
            </Empty>
          )}
        </div>
      </Section>
    </div>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="bg-gunmetal border border-shadow-grey rounded p-4 overflow-x-auto">
      <h2 className="font-mono text-sm mb-4">{title}</h2>
      {children}
    </section>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <div className="text-blue-slate text-xs py-3">{children}</div>;
}

function Headers({ enrollment = false }: { enrollment?: boolean }) {
  const values = enrollment
    ? [
        "config",
        "fingerprint",
        "host",
        "platform",
        "version",
        "requested",
        "actions",
      ]
    : [
        "bridge ID",
        "config",
        "fingerprint",
        "approved",
        "last seen",
        "state",
        "actions",
      ];
  return (
    <>
      {values.map((value) => (
        <th key={value} className="pb-2 pr-3 font-normal">
          {value}
        </th>
      ))}
    </>
  );
}

function date(value: number) {
  return new Date(value).toLocaleString();
}
