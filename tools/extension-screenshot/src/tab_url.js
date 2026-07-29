const DEFAULT_RETRY_DELAY_MS = 100;
const DEFAULT_RETRIES = 3;

function wait(delayMs) {
  return new Promise((resolve) => setTimeout(resolve, delayMs));
}

// Resolve the URL actually loaded by a task tab. A successful Bridge result
// must never silently fall back to the requested URL because redirects may
// have moved the browser to a different origin.
export async function resolveTabFinalURL(tabId, options = {}) {
  const tabs = options.tabs || chrome.tabs;
  const scripting = options.scripting || chrome.scripting;
  const retries = Math.max(1, options.retries || DEFAULT_RETRIES);
  const retryDelayMs = Math.max(0, options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS);

  for (let attempt = 0; attempt < retries; attempt += 1) {
    try {
      const tab = await tabs.get(tabId);
      const finalURL = String(tab?.url || tab?.pendingUrl || "").trim();
      if (finalURL) {
        return finalURL;
      }
    } catch {
      // The tab may be between navigation states; retry below.
    }

    try {
      const results = await scripting.executeScript({
        target: { tabId },
        func: () => globalThis.location?.href || ""
      });
      const finalURL = String(results?.[0]?.result || "").trim();
      if (finalURL) {
        return finalURL;
      }
    } catch {
      // Content-script execution can also race a navigation; retry below.
    }

    if (attempt + 1 < retries && retryDelayMs > 0) {
      await wait(retryDelayMs);
    }
  }

  throw new Error("plugin_final_url_unavailable");
}
