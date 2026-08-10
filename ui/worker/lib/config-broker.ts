import type { Env } from "./auth.ts";

export function notifyConfigChanged(
  env: Env,
  ctx: ExecutionContext,
  name: string,
  hash: string,
) {
  const id = env.CONFIG_BROKER.idFromName("global");
  const stub = env.CONFIG_BROKER.get(id);
  const url = `https://broker/notify?name=${encodeURIComponent(name)}&hash=${encodeURIComponent(hash)}`;
  ctx.waitUntil(stub.fetch(url, { method: "POST" }));
}
