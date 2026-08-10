// SHA-256 of a UTF-8 string, lowercase hex. Used as the config version
// identifier the Go binary polls.
export async function sha256Hex(s: string): Promise<string> {
  const buf = new TextEncoder().encode(s);
  const out = await crypto.subtle.digest("SHA-256", buf);
  return [...new Uint8Array(out)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
