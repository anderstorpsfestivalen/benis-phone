import { DurableObject } from "cloudflare:workers";
import type { Env } from "../lib/auth";

type Attachment = {
  role: "runtime" | "editor";
  name: string;
  bridgeId?: string;
};

export interface StoredStatusEvent {
  connection_id: string;
  state: string;
  code?: string;
  message?: string;
  local_port?: number;
  at: string;
}

interface ConnectionStatus {
  current: StoredStatusEvent;
  events: StoredStatusEvent[];
}

interface InstanceStatus {
  instance_id: string;
  last_seen: string;
}

const RETENTION_MS = 7 * 24 * 60 * 60 * 1000;
const MAX_EVENTS = 50;

export class ConfigBroker extends DurableObject<Env> {
  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    ctx.setWebSocketAutoResponse(
      new WebSocketRequestResponsePair("ping", "pong"),
    );
  }

  async fetch(req: Request): Promise<Response> {
    const url = new URL(req.url);

    if (url.pathname === "/subscribe" || url.pathname === "/status/subscribe") {
      const name = url.searchParams.get("name") ?? "";
      if (!name) return new Response("missing name", { status: 400 });
      if (req.headers.get("Upgrade") !== "websocket") {
        return new Response("expected websocket upgrade", { status: 426 });
      }
      const role = url.pathname === "/subscribe" ? "runtime" : "editor";
      const bridgeId = url.searchParams.get("bridge_id") ?? undefined;
      if (
        role === "runtime" &&
        (!bridgeId || !/^[0-9a-f-]{36}$/i.test(bridgeId))
      ) {
        return new Response("missing bridge_id", { status: 400 });
      }
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);
      this.ctx.acceptWebSocket(server);
      server.serializeAttachment({
        role,
        name,
        bridgeId,
      } satisfies Attachment);
      if (role === "editor") {
        server.send(
          JSON.stringify({
            type: "status-snapshot",
            ...(await this.snapshot(name)),
          }),
        );
      }
      return new Response(null, { status: 101, webSocket: client });
    }

    if (url.pathname === "/status" && req.method === "GET") {
      const name = url.searchParams.get("name") ?? "";
      if (!name) return new Response("missing name", { status: 400 });
      return Response.json(await this.snapshot(name));
    }

    if (url.pathname === "/notify" && req.method === "POST") {
      const name = url.searchParams.get("name") ?? "";
      const hash = url.searchParams.get("hash") ?? "";
      const payload = JSON.stringify({ type: "config-updated", name, hash });
      let fanout = 0;
      for (const ws of this.ctx.getWebSockets()) {
        const att = ws.deserializeAttachment() as Attachment | null;
        if (att?.role !== "runtime" || att.name !== name) continue;
        try {
          ws.send(payload);
          fanout++;
        } catch {
          /* runtime disconnected */
        }
      }
      return Response.json({ ok: true, fanout });
    }

    if (url.pathname === "/revoke" && req.method === "POST") {
      const bridgeId = url.searchParams.get("bridge_id") ?? "";
      let closed = 0;
      for (const ws of this.ctx.getWebSockets()) {
        const att = ws.deserializeAttachment() as Attachment | null;
        if (att?.role !== "runtime" || att.bridgeId !== bridgeId) continue;
        try {
          ws.close(4003, "bridge revoked");
          closed++;
        } catch {
          /* already disconnected */
        }
      }
      return Response.json({ ok: true, closed });
    }

    return new Response("not found", { status: 404 });
  }

  async webSocketMessage(ws: WebSocket, raw: ArrayBuffer | string) {
    const att = ws.deserializeAttachment() as Attachment | null;
    if (att?.role !== "runtime" || !att.bridgeId || typeof raw !== "string")
      return;
    let msg: Record<string, unknown>;
    try {
      msg = JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return;
    }
    const type = typeof msg.type === "string" ? msg.type : "";
    if (type === "runtime-hello" || type === "heartbeat") {
      await this.touchInstance(att.name, att.bridgeId);
      return;
    }
    if (type !== "sip-status" || msg.instance_id !== att.bridgeId) return;
    const connectionID =
      typeof msg.connection_id === "string"
        ? msg.connection_id.slice(0, 128)
        : "";
    const state = typeof msg.state === "string" ? msg.state.slice(0, 32) : "";
    if (!connectionID || !state) return;
    const now = new Date().toISOString();
    const event: StoredStatusEvent = {
      connection_id: connectionID,
      state,
      code: typeof msg.code === "string" ? msg.code.slice(0, 64) : undefined,
      message:
        typeof msg.message === "string" ? msg.message.slice(0, 500) : undefined,
      local_port:
        typeof msg.local_port === "number" ? msg.local_port : undefined,
      at: now,
    };
    const key = this.connectionKey(att.name, att.bridgeId, connectionID);
    const existing = await this.ctx.storage.get<ConnectionStatus>(key);
    const cutoff = Date.now() - RETENTION_MS;
    const events = [...(existing?.events ?? []), event]
      .filter((item) => Date.parse(item.at) >= cutoff)
      .slice(-MAX_EVENTS);
    await this.ctx.storage.put({
      [key]: { current: event, events } satisfies ConnectionStatus,
      [this.instanceKey(att.name, att.bridgeId)]: {
        instance_id: att.bridgeId,
        last_seen: now,
      } satisfies InstanceStatus,
    });
    this.fanoutEditors(att.name, {
      type: "sip-status",
      instance_id: att.bridgeId,
      last_seen: now,
      event,
    });
  }

  async webSocketClose(
    ws: WebSocket,
    _code: number,
    _reason: string,
    _wasClean: boolean,
  ) {
    try {
      ws.close();
    } catch {
      /* already closed */
    }
  }

  async webSocketError(ws: WebSocket, _err: unknown) {
    try {
      ws.close();
    } catch {
      /* already closed */
    }
  }

  private async touchInstance(name: string, instanceId: string) {
    const last_seen = new Date().toISOString();
    await this.env.DB.prepare(
      "UPDATE bridges SET last_seen = ? WHERE bridge_id = ? AND revoked_at IS NULL",
    )
      .bind(Date.now(), instanceId)
      .run();
    await this.ctx.storage.put(this.instanceKey(name, instanceId), {
      instance_id: instanceId,
      last_seen,
    } satisfies InstanceStatus);
    this.fanoutEditors(name, {
      type: "heartbeat",
      instance_id: instanceId,
      last_seen,
    });
  }

  private fanoutEditors(name: string, value: unknown) {
    const payload = JSON.stringify(value);
    for (const socket of this.ctx.getWebSockets()) {
      const att = socket.deserializeAttachment() as Attachment | null;
      if (att?.role !== "editor" || att.name !== name) continue;
      try {
        socket.send(payload);
      } catch {
        /* editor disconnected */
      }
    }
  }

  private async snapshot(name: string) {
    const instances = await this.ctx.storage.list<InstanceStatus>({
      prefix: `instance:${name}:`,
    });
    const connections = await this.ctx.storage.list<ConnectionStatus>({
      prefix: `connection:${name}:`,
    });
    const cutoff = Date.now() - RETENTION_MS;
    const expiredKeys: string[] = [];
    const liveInstances = [...instances.entries()].filter(([key, instance]) => {
      const live = Date.parse(instance.last_seen) >= cutoff;
      if (!live) expiredKeys.push(key);
      return live;
    });
    const retainedConnections = new Map<string, ConnectionStatus>();
    for (const [key, status] of connections) {
      const events = status.events
        .filter((event) => Date.parse(event.at) >= cutoff)
        .slice(-MAX_EVENTS);
      if (events.length === 0) {
        expiredKeys.push(key);
        continue;
      }
      retainedConnections.set(key, {
        current: events[events.length - 1],
        events,
      });
    }
    if (expiredKeys.length) await this.ctx.storage.delete(expiredKeys);
    return {
      instances: liveInstances.map(([, instance]) => ({
        ...instance,
        connections: [...retainedConnections.entries()]
          .filter(([key]) =>
            key.startsWith(`connection:${name}:${instance.instance_id}:`),
          )
          .map(([, status]) => status),
      })),
    };
  }

  private instanceKey(name: string, instanceId: string) {
    return `instance:${name}:${instanceId}`;
  }

  private connectionKey(
    name: string,
    instanceId: string,
    connectionID: string,
  ) {
    return `connection:${name}:${instanceId}:${connectionID}`;
  }
}
