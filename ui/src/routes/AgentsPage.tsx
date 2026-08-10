import { useCallback, useEffect, useState } from "react";
import { api, type AgentGrant } from "../lib/api";

export default function AgentsPage() {
  const [agents, setAgents] = useState<AgentGrant[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setAgents(await api.agents());
      setError(null);
    } catch (caught) {
      setError(String(caught));
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function revoke(agent: AgentGrant) {
    if (
      !confirm(
        `Revoke ${agent.client_name}? Its access and refresh tokens will stop working immediately.`,
      )
    )
      return;
    try {
      await api.revokeAgent(agent.grant_id);
      await refresh();
    } catch (caught) {
      setError(String(caught));
    }
  }

  const active = agents?.filter((agent) => agent.revoked_at === null) ?? [];
  const revoked = agents?.filter((agent) => agent.revoked_at !== null) ?? [];

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div>
        <h1 className="font-mono text-lg">Agent access</h1>
        <p className="text-sm text-blue-slate mt-1">
          MCP clients are authorized through the browser and can edit non-secret
          config only. They cannot read credentials, files, registration IDs, or
          bridge administration data.
        </p>
      </div>

      <section className="bg-gunmetal border border-shadow-grey rounded p-4">
        <h2 className="font-mono text-sm mb-3">Connect an agent</h2>
        <p className="text-xs text-blue-slate mb-3">
          Add the remote MCP URL, then run the client&apos;s login command. Your
          browser will open an authorization page protected by Cloudflare
          Access.
        </p>
        <Command>codex mcp add benis-phone --url {location.origin}/mcp</Command>
        <Command>codex mcp login benis-phone</Command>
        <Command>
          claude mcp add --transport http benis-phone {location.origin}/mcp
        </Command>
        <Command>claude mcp login benis-phone</Command>
      </section>

      {error && <div className="text-blue-slate text-sm">{error}</div>}

      <AgentSection title={`Active (${active.length})`}>
        {active.map((agent) => (
          <AgentRow key={agent.grant_id} agent={agent} onRevoke={revoke} />
        ))}
        {active.length === 0 && <Empty>No active MCP clients.</Empty>}
      </AgentSection>

      <AgentSection title={`Revoked (${revoked.length})`}>
        {revoked.map((agent) => (
          <AgentRow key={agent.grant_id} agent={agent} />
        ))}
        {revoked.length === 0 && <Empty>No revoked MCP clients.</Empty>}
      </AgentSection>
    </div>
  );
}

function Command({ children }: { children: React.ReactNode }) {
  return (
    <code className="block text-xs bg-ink-black border border-shadow-grey rounded px-3 py-2 mb-2 overflow-x-auto">
      {children}
    </code>
  );
}

function AgentSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="bg-gunmetal border border-shadow-grey rounded p-4">
      <h2 className="font-mono text-sm mb-3">{title}</h2>
      <div className="grid gap-3">{children}</div>
    </section>
  );
}

function AgentRow({
  agent,
  onRevoke,
}: {
  agent: AgentGrant;
  onRevoke?: (agent: AgentGrant) => void;
}) {
  return (
    <div className="border border-shadow-grey rounded p-3 flex items-start gap-4 text-xs">
      <div className="flex-1 min-w-0">
        <div className="font-mono text-sm">{agent.client_name}</div>
        <div className="text-blue-slate mt-1 break-all">
          approved by {agent.access_identity}
        </div>
        <div className="text-blue-slate mt-1">
          authorized {date(agent.created_at)} · last used{" "}
          {agent.last_used_at ? date(agent.last_used_at) : "never"}
        </div>
        <div className="font-mono text-blue-slate mt-1 break-all">
          {agent.scopes.join(" · ")}
        </div>
      </div>
      {onRevoke && (
        <button
          className="px-3 py-1 border border-shadow-grey rounded"
          onClick={() => onRevoke(agent)}
        >
          Revoke
        </button>
      )}
    </div>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <div className="text-blue-slate text-xs py-2">{children}</div>;
}

function date(value: number) {
  return new Date(value).toLocaleString();
}
