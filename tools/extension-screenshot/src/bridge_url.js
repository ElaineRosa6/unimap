export function normalizeLoopbackAPIBaseURL(raw) {
  let parsed;
  try {
    parsed = new URL(String(raw || "").trim());
  } catch {
    throw new Error("Bridge URL must be a valid loopback HTTP URL.");
  }
  const hostname = parsed.hostname.toLowerCase();
  if (parsed.protocol !== "http:" || !["127.0.0.1", "localhost", "[::1]"].includes(hostname)) {
    throw new Error("Bridge URL must use HTTP on 127.0.0.1, localhost, or [::1].");
  }
  if (!parsed.port || parsed.username || parsed.password || parsed.search || parsed.hash) {
    throw new Error("Bridge URL requires an explicit port and must not contain credentials, query, or fragment.");
  }
  parsed.pathname = parsed.pathname.replace(/\/+$/, "");
  return parsed.toString().replace(/\/$/, "");
}
