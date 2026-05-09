// Tab pool for reuse - limits memory usage
let tabPool = [];
const MAX_TAB_POOL_SIZE = 3;
const TAB_REUSE_TIMEOUT_MS = 30000;
let lastTabReuseTime = 0;

export async function ensureTab(targetUrl) {
  const tabs = await chrome.tabs.query({});

  // Check if we have a reusable tab in the pool
  const now = Date.now();
  if (tabPool.length > 0 && now - lastTabReuseTime < TAB_REUSE_TIMEOUT_MS) {
    const reusableTab = tabPool.pop();
    if (reusableTab && reusableTab.id) {
      try {
        // Check if tab still exists
        await chrome.tabs.get(reusableTab.id);
        await chrome.tabs.update(reusableTab.id, { url: targetUrl, active: true });
        return reusableTab.id;
      } catch (e) {
        // Tab no longer exists, remove from pool
        tabPool = tabPool.filter(t => t.id !== reusableTab.id);
      }
    }
  }

  // Check for existing tab with the same URL
  const existing = tabs.find((t) => t.url === targetUrl);
  if (existing && existing.id) {
    await chrome.tabs.update(existing.id, { active: true });
    return existing.id;
  }

  // Create new tab
  const created = await chrome.tabs.create({ url: targetUrl, active: true });
  return created.id;
}

// Return tab to pool for reuse, or close if pool is full
export async function releaseTab(tabId) {
  try {
    // Check if tab still exists
    const tab = await chrome.tabs.get(tabId);
    if (!tab) return;

    if (tabPool.length < MAX_TAB_POOL_SIZE) {
      // Return to pool for reuse
      tabPool.push({ id: tabId, url: tab.url });
      lastTabReuseTime = Date.now();
      // Navigate to blank page to free memory
      await chrome.tabs.update(tabId, { url: "about:blank" });
    } else {
      // Pool full, close the tab
      await chrome.tabs.remove(tabId);
    }
  } catch (e) {
    // Tab already closed or doesn't exist
    tabPool = tabPool.filter(t => t.id !== tabId);
  }
}

// Clean up stale tabs from pool
export async function cleanupTabPool() {
  const now = Date.now();
  if (now - lastTabReuseTime > TAB_REUSE_TIMEOUT_MS) {
    // Close all pooled tabs
    for (const pooledTab of tabPool) {
      try {
        await chrome.tabs.remove(pooledTab.id);
      } catch (e) {
        // Ignore errors
      }
    }
    tabPool = [];
  }
}

export async function waitForPageReady(tabId, strategy, timeoutMs) {
  const timeout = Math.max(1000, timeoutMs || 15000);

  if (strategy === "delay") {
    await new Promise((resolve) => setTimeout(resolve, timeout));
    return;
  }

  const current = await chrome.tabs.get(tabId);
  if (current && current.status === "complete") {
    return;
  }

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error("plugin_timeout: page load timeout"));
    }, timeout);

    function onUpdated(updatedTabId, info) {
      if (updatedTabId === tabId && info.status === "complete") {
        cleanup();
        resolve();
      }
    }

    function cleanup() {
      clearTimeout(timer);
      chrome.tabs.onUpdated.removeListener(onUpdated);
    }

    chrome.tabs.onUpdated.addListener(onUpdated);
  });
}

export async function captureVisible() {
  try {
    const dataUrl = await chrome.tabs.captureVisibleTab(undefined, { format: "png" });
    return dataUrl;
  } catch (err) {
    try {
      const currentWindow = await chrome.windows.getCurrent({ populate: false });
      const dataUrl = await chrome.tabs.captureVisibleTab(currentWindow?.id, { format: "png" });
      return dataUrl;
    } catch (fallbackErr) {
      throw new Error(`plugin_capture_failed: ${String(fallbackErr || err)}`);
    }
  }
}

export function normalizeImagePayload(dataUrl, requestId, startedAt) {
  const durationMs = Math.max(1, Date.now() - startedAt);
  return {
    request_id: requestId,
    success: true,
    image_path: "",
    image_data: dataUrl,
    duration_ms: durationMs
  };
}

// Normalize collect payload into the structured_collected_data format
export function normalizeCollectPayload(items, title, requestId, startedAt) {
  const durationMs = Math.max(1, Date.now() - startedAt);
  return {
    request_id: requestId,
    success: true,
    image_path: "",
    image_data: "",
    duration_ms: durationMs,
    structured_collected_data: {
      title: title || "",
      items: items || [],
      total: items ? items.length : 0,
      has_more: false
    }
  };
}

// Detect search engine type from the page URL
function detectEngine(url) {
  if (!url) return "unknown";
  const lower = url.toLowerCase();
  if (lower.includes("fofa.info")) return "fofa";
  if (lower.includes("hunter.qianxin.com")) return "hunter";
  if (lower.includes("zoomeye.org")) return "zoomeye";
  if (lower.includes("quake.360.cn")) return "quake";
  return "unknown";
}

// Extract structured assets from a search engine result page DOM
export async function extractEngineAssets(tabId) {
  const tab = await chrome.tabs.get(tabId);
  const engine = detectEngine(tab?.url);

  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId },
      func: (eng) => {
        const items = [];
        let total = 0;
        let hasMore = false;
        let title = document.title || "";
        const bodyText = (document.body?.innerText || "").toLowerCase();
        const loginRequired = /登录|登陆|请先登录|login|sign in|signin|unauthorized/.test(bodyText + " " + title.toLowerCase());

        switch (eng) {
          case "fofa": {
            const rows = document.querySelectorAll(".list_content > tbody > tr");
            rows.forEach((row) => {
              const cells = row.querySelectorAll("td");
              if (cells.length < 5) return;
              const item = {};
              const ipEl = cells[0]?.querySelector("a");
              item.ip = ipEl ? ipEl.textContent.trim() : cells[0]?.textContent.trim();
              item.port = parseInt(cells[1]?.textContent.trim(), 10) || 0;
              item.protocol = cells[2]?.textContent.trim();
              const hostEl = cells[3]?.querySelector("a");
              item.host = hostEl ? hostEl.textContent.trim() : cells[3]?.textContent.trim();
              item.title = cells[4]?.textContent.trim();
              if (cells[5]) item.country_code = cells[5]?.textContent.trim();
              if (cells[6]) item.banner = cells[6]?.textContent.trim();
              items.push(item);
            });
            const totalEl = document.querySelector(".total-count, .total_count");
            if (totalEl) total = parseInt(totalEl.textContent.trim(), 10) || 0;
            hasMore = !!document.querySelector(".next, .next-page, [class*='next']");
            break;
          }
          case "hunter": {
            const cards = document.querySelectorAll(".result-list > .result-item, .result-item");
            cards.forEach((card) => {
              const item = {};
              const ipEl = card.querySelector(".ip-address, [data-ip]");
              if (ipEl) item.ip = ipEl.textContent.trim();
              const portEl = card.querySelector(".port, [data-port]");
              if (portEl) item.port = parseInt(portEl.textContent.trim(), 10) || 0;
              const protoEl = card.querySelector(".protocol, .service, [data-protocol]");
              if (protoEl) item.protocol = protoEl.textContent.trim();
              const domainEl = card.querySelector(".domain, .hostname");
              if (domainEl) item.host = domainEl.textContent.trim();
              const titleEl = card.querySelector(".web-title, .title");
              if (titleEl) item.title = titleEl.textContent.trim();
              const bannerEl = card.querySelector(".header-info, .banner");
              if (bannerEl) item.banner = bannerEl.textContent.trim();
              items.push(item);
            });
            const totalEl = document.querySelector(".total-count, .total");
            if (totalEl) total = parseInt(totalEl.textContent.trim(), 10) || 0;
            hasMore = !!document.querySelector(".next, .el-pagination__next, [class*='next']");
            break;
          }
          case "zoomeye": {
            const resultItems = document.querySelectorAll(
              "div[class*='search-result-item'], .result-list > .item, .search-result-item"
            );
            resultItems.forEach((el) => {
              const item = {};
              const ipEl = el.querySelector(".ip, [data-ip]");
              if (ipEl) item.ip = ipEl.textContent.trim();
              const portEl = el.querySelector(".port, [data-port]");
              if (portEl) item.port = parseInt(portEl.textContent.trim(), 10) || 0;
              const serviceEl = el.querySelector(".service, .protocol, [data-service]");
              if (serviceEl) item.protocol = serviceEl.textContent.trim();
              const domainEl = el.querySelector(".domain");
              if (domainEl) item.host = domainEl.textContent.trim();
              const titleEl = el.querySelector(".title");
              if (titleEl) item.title = titleEl.textContent.trim();
              const locEl = el.querySelector(".location");
              if (locEl) item.country_code = locEl.textContent.trim();
              const bannerEl = el.querySelector(".banner");
              if (bannerEl) item.banner = bannerEl.textContent.trim();
              items.push(item);
            });
            const totalEl = document.querySelector(".total");
            if (totalEl) total = parseInt(totalEl.textContent.trim(), 10) || 0;
            hasMore = !!document.querySelector(".next, .pagination-next, [class*='next']");
            break;
          }
          case "quake": {
            const rows = document.querySelectorAll(".result-list > .result-row, .result-row");
            rows.forEach((row) => {
              const item = {};
              const ipEl = row.querySelector(".ip");
              if (ipEl) item.ip = ipEl.textContent.trim();
              const portEl = row.querySelector(".port");
              if (portEl) item.port = parseInt(portEl.textContent.trim(), 10) || 0;
              const protoEl = row.querySelector(".transport, .protocol");
              if (protoEl) item.protocol = protoEl.textContent.trim();
              const hostEl = row.querySelector(".hostname");
              if (hostEl) item.host = hostEl.textContent.trim();
              const titleEl = row.querySelector(".title");
              if (titleEl) item.title = titleEl.textContent.trim();
              const serverEl = row.querySelector(".server");
              if (serverEl) item.server = serverEl.textContent.trim();
              const cityEl = row.querySelector(".city");
              if (cityEl) item.city = cityEl.textContent.trim();
              const ispEl = row.querySelector(".isp");
              if (ispEl) item.isp = ispEl.textContent.trim();
              items.push(item);
            });
            const totalEl = document.querySelector(".total-count, .total");
            if (totalEl) total = parseInt(totalEl.textContent.trim(), 10) || 0;
            hasMore = !!document.querySelector(".next, .next-page, [class*='next']");
            break;
          }
          default: {
            // Fallback: try to extract any tabular data from the page
            const tables = document.querySelectorAll("table");
            tables.forEach((table) => {
              const rows = table.querySelectorAll("tbody tr, tr");
              rows.forEach((row) => {
                const cells = row.querySelectorAll("td");
                if (cells.length >= 2) {
                  const item = {};
                  cells.forEach((cell, idx) => {
                    item[`col_${idx}`] = cell.textContent.trim().substring(0, 200);
                  });
                  items.push(item);
                }
              });
            });
            break;
          }
        }

        return { items, total, has_more: hasMore, title, login_required: loginRequired && items.length === 0 };
      },
      args: [engine]
    });

    if (results && results[0] && results[0].result) {
      return results[0].result;
    }
    return { items: [], total: 0, has_more: false, title: "", login_required: false };
  } catch (err) {
    // DOM extraction failed — return empty result, let caller handle
    return { items: [], total: 0, has_more: false, title: "", login_required: false, error: String(err) };
  }
}
