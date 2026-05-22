import type { Env } from "../lib/auth";
import { badRequest, json, notFound } from "../lib/responses";

// /api/files/*  — R2-backed file manager API.
//
//   GET    /api/files?prefix=&cursor=          list objects
//   PUT    /api/files/object?key=<key>         upload (raw body, Content-Type honoured)
//   GET    /api/files/object?key=<key>         download (streams the object back)
//   DELETE /api/files/object?key=<key>         delete

const MAX_KEY_LEN = 1024;

export async function handleFiles(req: Request, env: Env, pathname: string): Promise<Response> {
  const rest = pathname.replace(/^\/api\/files/, "");

  if (rest === "" || rest === "/") {
    if (req.method !== "GET") return badRequest("method not allowed");
    return listFiles(req, env);
  }

  if (rest === "/object") {
    const url = new URL(req.url);
    const key = url.searchParams.get("key") ?? "";
    if (!key) return badRequest("key is required");
    if (key.length > MAX_KEY_LEN) return badRequest("key too long");
    // R2 accepts any byte sequence — we just block path-traversal-style
    // weirdness that's almost certainly an editor bug rather than an
    // intentional key.
    if (key.startsWith("/") || key.includes("..")) return badRequest("invalid key");

    switch (req.method) {
      case "GET":
        return getObject(env, key);
      case "PUT":
        return putObject(req, env, key);
      case "DELETE":
        return deleteObject(env, key);
      default:
        return badRequest("method not allowed");
    }
  }

  return notFound();
}

async function listFiles(req: Request, env: Env): Promise<Response> {
  const url = new URL(req.url);
  const prefix = url.searchParams.get("prefix") ?? undefined;
  const cursor = url.searchParams.get("cursor") ?? undefined;
  const limit = Number(url.searchParams.get("limit") ?? "1000") || 1000;

  // The types in @cloudflare/workers-types don't expose the `include`
  // option yet, but the runtime accepts it and returns httpMetadata.
  const out = await env.BUCKET.list({
    prefix,
    cursor,
    limit: Math.min(limit, 1000),
    include: ["httpMetadata"],
  } as R2ListOptions);

  return json({
    objects: out.objects.map((o) => ({
      key: o.key,
      size: o.size,
      uploaded: o.uploaded.toISOString(),
      etag: o.etag,
      contentType: o.httpMetadata?.contentType ?? null,
    })),
    cursor: "cursor" in out ? out.cursor : null,
    truncated: out.truncated,
  });
}

async function getObject(env: Env, key: string): Promise<Response> {
  const obj = await env.BUCKET.get(key);
  if (!obj) return notFound();
  const headers = new Headers();
  obj.writeHttpMetadata(headers);
  headers.set("etag", obj.httpEtag);
  return new Response(obj.body, { headers });
}

async function putObject(req: Request, env: Env, key: string): Promise<Response> {
  if (!req.body) return badRequest("empty body");
  const contentType = req.headers.get("Content-Type") ?? "application/octet-stream";
  const out = await env.BUCKET.put(key, req.body, {
    httpMetadata: { contentType },
  });
  if (!out) return badRequest("upload failed");
  return json({
    key: out.key,
    size: out.size,
    uploaded: out.uploaded.toISOString(),
    etag: out.etag,
    contentType,
  });
}

async function deleteObject(env: Env, key: string): Promise<Response> {
  await env.BUCKET.delete(key);
  return new Response(null, { status: 204 });
}
