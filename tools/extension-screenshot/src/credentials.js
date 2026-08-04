export async function readBrowserCredentials(url, chromeApi = chrome) {
  const parsed = new URL(url);
  const allTabs = await chromeApi.tabs.query({});
  const candidates = allTabs.filter((candidate) => {
    try {
      return new URL(candidate.url || "").origin === parsed.origin;
    } catch {
      return false;
    }
  }).sort((left, right) => Number(Boolean(right.active)) - Number(Boolean(left.active)));

  let lastError = null;
  for (const candidate of candidates) {
    try {
      return await readFromTab(candidate, parsed.hostname, chromeApi, false);
    } catch (error) {
      lastError = error;
    }
  }

  const created = await chromeApi.tabs.create({ url: parsed.origin + "/", active: true });
  try {
    return await readFromTab(created, parsed.hostname, chromeApi, true);
  } catch (error) {
    throw new Error(`credential_page_unavailable: ${String(error || lastError || "unknown")}`);
  }
}

async function readFromTab(tab, hostname, chromeApi, removeAfter) {
  try {
    await waitForTabComplete(tab.id, chromeApi);
    const cookies = await chromeApi.cookies.getAll({ domain: hostname });
    const results = await chromeApi.scripting.executeScript({
      target: { tabId: tab.id },
      func: () => {
        const local = {};
        const session = {};
        for (let index = 0; index < localStorage.length; index++) {
          const key = localStorage.key(index);
          if (key !== null) local[key] = localStorage.getItem(key) ?? "";
        }
        for (let index = 0; index < sessionStorage.length; index++) {
          const key = sessionStorage.key(index);
          if (key !== null) session[key] = sessionStorage.getItem(key) ?? "";
        }
        return { local, session };
      }
    });
    const finalTab = await chromeApi.tabs.get(tab.id);
    return {
      cookies,
      storage: results?.[0]?.result || { local: {}, session: {} },
      finalURL: finalTab.url || ""
    };
  } finally {
    if (removeAfter) await chromeApi.tabs.remove(tab.id).catch(() => {});
  }
}

async function waitForTabComplete(tabId, chromeApi) {
  const current = await chromeApi.tabs.get(tabId);
  if (current.status === "complete") return;
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      chromeApi.tabs.onUpdated.removeListener(listener);
      reject(new Error("credential_page_timeout"));
    }, 15000);
    const listener = (updatedTabId, changeInfo) => {
      if (updatedTabId !== tabId || changeInfo.status !== "complete") return;
      clearTimeout(timer);
      chromeApi.tabs.onUpdated.removeListener(listener);
      resolve();
    };
    chromeApi.tabs.onUpdated.addListener(listener);
  });
}
