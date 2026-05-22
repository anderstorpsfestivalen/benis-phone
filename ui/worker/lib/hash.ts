// SHA-256 of a UTF-8 string, lowercase hex. Used as the config version
// identifier the Go binary polls.
export async function sha256Hex(s: string): Promise<string> {
  const buf = new TextEncoder().encode(s);
  const out = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(out)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

// Constant-time string comparison for bearer tokens.
export function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}
