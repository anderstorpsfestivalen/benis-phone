import type { Env } from "./auth";

export interface ConfigRow {
  name: string;
  doc: string;
  toml: string;
  hash: string;
  created_at: number;
  updated_at: number;
}

export async function listConfigs(env: Env): Promise<Pick<ConfigRow, "name" | "hash" | "created_at" | "updated_at">[]> {
  const r = await env.DB.prepare(
    "SELECT name, hash, created_at, updated_at FROM configs ORDER BY updated_at DESC",
  ).all<Pick<ConfigRow, "name" | "hash" | "created_at" | "updated_at">>();
  return r.results ?? [];
}

export async function getConfig(env: Env, name: string): Promise<ConfigRow | null> {
  const r = await env.DB.prepare("SELECT * FROM configs WHERE name = ?")
    .bind(name)
    .first<ConfigRow>();
  return r ?? null;
}

export async function upsertConfig(env: Env, row: ConfigRow): Promise<void> {
  await env.DB.prepare(
    `INSERT INTO configs (name, doc, toml, hash, created_at, updated_at)
     VALUES (?, ?, ?, ?, ?, ?)
     ON CONFLICT(name) DO UPDATE SET
       doc = excluded.doc,
       toml = excluded.toml,
       hash = excluded.hash,
       updated_at = excluded.updated_at`,
  )
    .bind(row.name, row.doc, row.toml, row.hash, row.created_at, row.updated_at)
    .run();
}

export async function deleteConfig(env: Env, name: string): Promise<boolean> {
  const r = await env.DB.prepare("DELETE FROM configs WHERE name = ?")
    .bind(name)
    .run();
  return (r.meta?.changes ?? 0) > 0;
}
