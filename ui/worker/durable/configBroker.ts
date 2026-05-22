import { DurableObject } from "cloudflare:workers";
import type { Env } from "../lib/auth";

// ConfigBroker fans out config-change events to Go binaries that have
// subscribed via WebSocket. One DO instance (idFromName "global") owns all
// subscriptions; each subscriber stores its config name in its WebSocket
// attachment so /notify can filter without scanning a separate table.
//
// Follows the canonical hibernation pattern:
// https://developers.cloudflare.com/durable-objects/examples/websocket-hibernation-server/
//
// The DO is dormant whenever no save is happening. Cloudflare's runtime
// answers the per-socket "ping" keep-alive without invoking us thanks to
// setWebSocketAutoResponse below.

type Attachment = { name: string };

export class ConfigBroker extends DurableObject<Env> {
  constructor(ctx: DurableObjectState, env: Env) {
    super(ctx, env);
    ctx.setWebSocketAutoResponse(
      new WebSocketRequestResponsePair("ping", "pong"),
    );
  }

  async fetch(req: Request): Promise<Response> {
    const url = new URL(req.url);

    if (url.pathname === "/subscribe") {
      const name = url.searchParams.get("name") ?? "";
      if (!name) return new Response("missing name", { status: 400 });
      const upgrade = req.headers.get("Upgrade");
      if (upgrade !== "websocket") {
        return new Response("expected websocket upgrade", { status: 426 });
      }
      const pair = new WebSocketPair();
      const [client, server] = Object.values(pair);
      this.ctx.acceptWebSocket(server);
      server.serializeAttachment({ name } satisfies Attachment);
      return new Response(null, { status: 101, webSocket: client });
    }

    if (url.pathname === "/notify" && req.method === "POST") {
      const name = url.searchParams.get("name") ?? "";
      const hash = url.searchParams.get("hash") ?? "";
      const payload = JSON.stringify({ type: "config-updated", name, hash });
      let fanout = 0;
      for (const ws of this.ctx.getWebSockets()) {
        const att = ws.deserializeAttachment() as Attachment | null;
        if (att?.name !== name) continue;
        try {
          ws.send(payload);
          fanout++;
        } catch {
          // Dead socket; the runtime will reap it on the next close cycle.
        }
      }
      return new Response(JSON.stringify({ ok: true, fanout }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    return new Response("not found", { status: 404 });
  }

  // Hibernation lifecycle. Inbound app messages from clients are unused —
  // the "ping" keep-alive is handled entirely by setWebSocketAutoResponse
  // and never wakes the DO. We still need this method present so the
  // runtime knows the class is hibernation-aware.
  async webSocketMessage(_ws: WebSocket, _msg: ArrayBuffer | string) {}

  async webSocketClose(ws: WebSocket, _code: number, _reason: string, _wasClean: boolean) {
    try { ws.close(); } catch { /* already closed */ }
  }

  async webSocketError(ws: WebSocket, _err: unknown) {
    try { ws.close(); } catch { /* already closed */ }
  }
}
