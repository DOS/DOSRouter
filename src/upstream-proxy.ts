/**
 * Upstream Proxy Support
 *
 * Configures a global fetch dispatcher when BLOCKRUN_UPSTREAM_PROXY is set.
 * Supports http://, https://, and socks5:// proxy URLs.
 *
 * Usage:
 *   BLOCKRUN_UPSTREAM_PROXY=socks5://127.0.0.1:1080 clawrouter start
 *   BLOCKRUN_UPSTREAM_PROXY=http://127.0.0.1:8080 clawrouter start
 */

/**
 * Apply upstream proxy settings to the global fetch dispatcher.
 * Called once at proxy startup if BLOCKRUN_UPSTREAM_PROXY is set.
 * Returns the proxy URL that was configured, or undefined if none.
 */
export async function applyUpstreamProxy(proxyUrl?: string): Promise<string | undefined> {
  const url = proxyUrl ?? process.env.BLOCKRUN_UPSTREAM_PROXY;
  if (!url) return undefined;

  // Validate URL format
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    console.warn(`[ClawRouter] Invalid BLOCKRUN_UPSTREAM_PROXY URL: ${url} — skipping proxy setup`);
    return undefined;
  }

  const scheme = parsed.protocol; // "http:", "https:", "socks5:"

  try {
    if (scheme === "socks5:" || scheme === "socks4:") {
      const { Socks5ProxyAgent, setGlobalDispatcher } = await import("undici");
      setGlobalDispatcher(new Socks5ProxyAgent(url));
    } else if (scheme === "http:" || scheme === "https:") {
      const { ProxyAgent, setGlobalDispatcher } = await import("undici");
      setGlobalDispatcher(new ProxyAgent(url));
    } else {
      console.warn(
        `[ClawRouter] Unsupported proxy scheme "${scheme}" in BLOCKRUN_UPSTREAM_PROXY — use http:// or socks5://`,
      );
      return undefined;
    }
  } catch (err) {
    console.warn(
      `[ClawRouter] Failed to configure upstream proxy "${url}": ${err instanceof Error ? err.message : err}`,
    );
    return undefined;
  }

  return url;
}
