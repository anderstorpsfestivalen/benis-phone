export function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}

export function text(body: string, status = 200): Response {
  return new Response(body, {
    status,
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}

export function badRequest(msg: string): Response {
  return json({ error: msg }, 400);
}
export function notFound(msg = "not found"): Response {
  return json({ error: msg }, 404);
}
export function unauthorized(): Response {
  return json({ error: "unauthorized" }, 401);
}
