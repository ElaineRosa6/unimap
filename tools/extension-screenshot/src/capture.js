// Tab pool for reuse - limits memory usage
let tabPool = [];
const MAX_TAB_POOL_SIZE = 3;
const TAB_REUSE_TIMEOUT_MS = 30000;
let lastTabReuseTime = 0;

/**
 * Detect search engine type from the page URL.
 * @param {string} url - The page URL
 * @returns {string} Engine name or "unknown"
 */
function detectEngine(url) {
  if (!url) return "unknown";
  const lower = url.toLowerCase();
  if (lower.includes("fofa.info")) return "fofa";
  if (lower.includes("hunter.qianxin.com")) return "hunter";
  if (lower.includes("zoomeye.org")) return "zoomeye";
  if (lower.includes("quake.360.cn")) return "quake";
  return "unknown";
}

/**
 * Open or create a tab with the target URL.
 * @param {string} targetUrl - URL to open
 * @returns {Promise<number>} Tab ID
 */
export async function ensureTab(targetUrl) {
  const tabs = await chrome.tabs.query({});

  // Check if we have a reusable tab in the pool
  const now = Date.now();
  if (tabPool.length > 0 && now - lastTabReuseTime < TAB_REUSE_TIMEOUT_MS) {
    const reusableTab = tabPool.pop();
    if (reusableTab && reusableTab.id) {
      try {
        await chrome.tabs.get(reusableTab.id);
        await chrome.tabs.update(reusableTab.id, { url: targetUrl, active: true });
        return reusableTab.id;
      } catch (e) {
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

  const created = await chrome.tabs.create({ url: targetUrl, active: true });
  return created.id;
}

/**
 * Return tab to pool for reuse, or close if pool is full.
 * @param {number} tabId - Tab ID to release
 */
export async function releaseTab(tabId) {
  try {
    const tab = await chrome.tabs.get(tabId);
    if (!tab) return;

    if (tabPool.length < MAX_TAB_POOL_SIZE) {
      tabPool.push({ id: tabId, url: tab.url });
      lastTabReuseTime = Date.now();
      await chrome.tabs.update(tabId, { url: "about:blank" });
    } else {
      await chrome.tabs.remove(tabId);
    }
  } catch (e) {
    tabPool = tabPool.filter(t => t.id !== tabId);
  }
}

/**
 * Clean up stale tabs from pool.
 */
export async function cleanupTabPool() {
  const now = Date.now();
  if (now - lastTabReuseTime > TAB_REUSE_TIMEOUT_MS) {
    for (const pooledTab of tabPool) {
      try {
        await chrome.tabs.remove(pooledTab.id);
      } catch (e) { /* ignore */ }
    }
    tabPool = [];
  }
}

/**
 * Wait for page to be ready with multiple strategies.
 * @param {number} tabId - Tab ID
 * @param {string} strategy - "load", "delay", "networkidle", "spa"
 * @param {number} timeoutMs - Timeout in milliseconds
 */
export async function waitForPageReady(tabId, strategy, timeoutMs) {
  const timeout = Math.max(1000, timeoutMs || 15000);

  if (strategy === "delay") {
    await new Promise((resolve) => setTimeout(resolve, timeout));
    return;
  }

  // For SPA-heavy pages (search engines), use a hybrid approach:
  // 1. Wait for tab status "complete"
  // 2. Then wait extra time for dynamic content rendering
  const current = await chrome.tabs.get(tabId);
  if (current && current.status === "complete" && strategy === "load") {
    // For search engines, always wait extra for SPA rendering
    await new Promise((resolve) => setTimeout(resolve, 2000));
    return;
  }

  if (strategy === "spa" || strategy === "networkidle") {
    // SPA strategy: wait for complete + extra render time
    await new Promise((resolve) => setTimeout(resolve, Math.min(timeout, 5000)));
  }

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error("plugin_timeout: page load timeout"));
    }, timeout);

    function onUpdated(updatedTabId, info) {
      if (updatedTabId === tabId && info.status === "complete") {
        cleanup();
        // Extra wait for SPA rendering
        setTimeout(resolve, strategy === "spa" ? 3000 : 1000);
      }
    }

    function cleanup() {
      clearTimeout(timer);
      chrome.tabs.onUpdated.removeListener(onUpdated);
    }

    chrome.tabs.onUpdated.addListener(onUpdated);
  });
}

/**
 * Capture visible tab as PNG data URL.
 * @returns {Promise<string>} Data URL
 */
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

/**
 * Build screenshot result payload.
 */
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

/**
 * Build collect result payload with structured data.
 */
export function normalizeCollectPayload(items, title, requestId, startedAt) {
  const durationMs = Math.max(1, Date.now() - startedAt);
  return {
    request_id: requestId,
    success: true,
    image_path: "",
    image_data: "",
    duration_ms: durationMs,
    collected_data: title || "",
    structured_collected_data: {
      title: title || "",
      items: items || [],
      total: items ? items.length : 0,
      has_more: false
    }
  };
}

/**
 * DOM selector configurations per engine.
 * These can be updated without redeploying the extension.
 */
const ENGINE_SELECTORS = {
  fofa: {
    // Primary selectors (try in order)
    row: [
      ".list_content > tbody > tr",
      ".result-table tbody tr",
      "[class*='result'] table tbody tr",
      "table[class*='list'] tbody tr"
    ],
    cells: {
      ip: { selector: "td:nth-child(1) a", fallback: "td:nth-child(1)" },
      port: { selector: "td:nth-child(2)" },
      protocol: { selector: "td:nth-child(3)" },
      host: { selector: "td:nth-child(4) a", fallback: "td:nth-child(4)" },
      title: { selector: "td:nth-child(5)" },
      country_code: { selector: "td:nth-child(6)" },
      banner: { selector: "td:nth-child(7)" }
    },
    total: [".total-count", ".total_count", "[class*='total']"],
    nextPage: [".next", ".next-page", "[class*='next']"]
  },
  hunter: {
    row: [
      ".result-list > .result-item",
      ".result-item",
      "[class*='result-list'] > [class*='item']",
      "div[class*='result-item']"
    ],
    cells: {
      ip: { selector: ".ip-address, [data-ip], [class*='ip']" },
      port: { selector: ".port, [data-port], [class*='port']" },
      protocol: { selector: ".protocol, .service, [data-protocol]" },
      host: { selector: ".domain, .hostname, [class*='domain']" },
      title: { selector: ".web-title, .title, [class*='web-title']" },
      banner: { selector: ".header-info, .banner" }
    },
    total: [".total-count", ".total", "[class*='total-count']"],
    nextPage: [".next", ".el-pagination__next", "[class*='next']"]
  },
  zoomeye: {
    row: [
      "div[class*='search-result-item']",
      ".result-list > .item",
      ".search-result-item",
      "[class*='result-item']"
    ],
    cells: {
      ip: { selector: ".ip, [data-ip], [class*='ip']" },
      port: { selector: ".port, [data-port], [class*='port']" },
      protocol: { selector: ".service, .protocol, [data-service]" },
      host: { selector: ".domain, [class*='domain']" },
      title: { selector: ".title, [class*='title']" },
      country_code: { selector: ".location, [class*='location']" },
      banner: { selector: ".banner, [class*='banner']" }
    },
    total: [".total", "[class*='total']", "[class*='count']"],
    nextPage: [".next", ".pagination-next", "[class*='next']"]
  },
  quake: {
    row: [
      ".result-list > .result-row",
      ".result-row",
      "[class*='result-list'] > [class*='row']",
      "[class*='result-row']"
    ],
    cells: {
      ip: { selector: ".ip, [class*='ip']" },
      port: { selector: ".port, [class*='port']" },
      protocol: { selector: ".transport, .protocol" },
      host: { selector: ".hostname, [class*='hostname']" },
      title: { selector: ".title, [class*='title']" },
      server: { selector: ".server, [class*='server']" },
      city: { selector: ".city, [class*='city']" },
      isp: { selector: ".isp, [class*='isp']" }
    },
    total: [".total-count", ".total", "[class*='total']"],
    nextPage: [".next", ".next-page", "[class*='next']"]
  }
};

/**
 * Safely query a single element using the first matching selector.
 * @param {Document|Element} root - Root element to query
 * @param {string|string[]} selectors - CSS selector(s)
 * @returns {Element|null}
 */
function queryOne(root, selectors) {
  const list = Array.isArray(selectors) ? selectors : [selectors];
  for (const sel of list) {
    const el = root.querySelector(sel);
    if (el) return el;
  }
  return null;
}

/**
 * Query all matching elements across multiple selector variants.
 * @param {Document|Element} root
 * @param {string|string[]} selectors
 * @returns {NodeListOf<Element>|Element[]}
 */
function queryAll(root, selectors) {
  const list = Array.isArray(selectors) ? selectors : [selectors];
  for (const sel of list) {
    const els = root.querySelectorAll(sel);
    if (els.length > 0) return els;
  }
  return [];
}

/**
 * Extract cell text using selector config with fallback.
 * @param {Element} row - Row element
 * @param {Object} cellConfig - {selector, fallback}
 * @returns {string}
 */
function extractCellText(row, cellConfig) {
  const el = row.querySelector(cellConfig.selector);
  if (el) return el.textContent.trim();
  if (cellConfig.fallback) {
    const fb = row.querySelector(cellConfig.fallback);
    if (fb) return fb.textContent.trim();
  }
  return "";
}

/**
 * Check if the page looks like a login wall.
 * @param {Document} doc
 * @returns {boolean}
 */
function isLoginWall(doc) {
  const text = doc.body.textContent.toLowerCase();
  const loginKeywords = [
    "请登录", "请先登录", "login required", "sign in to continue",
    "session expired", "session expired", "please log in",
    "登录", "登入", "サインイン", "로그인"
  ];
  // Only trigger if the page is short (likely a login form, not a full results page)
  if (text.length > 5000) return false;
  return loginKeywords.some(kw => text.includes(kw));
}

/**
 * Extract structured assets from a search engine result page DOM.
 * This is the KEY function for collect mode.
 * @param {number} tabId - Chrome tab ID
 * @returns {Promise<{items: Array, total: number, has_more: boolean, title: string, engine: string, is_login_wall: boolean, error?: string}>}
 */
export async function extractEngineAssets(tabId) {
  const tab = await chrome.tabs.get(tabId);
  const engine = detectEngine(tab?.url);

  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId },
      func: (eng, selectors) => {
        const items = [];
        let total = 0;
        let hasMore = false;
        const title = document.title || "";

        // Check for login wall first
        if (isLoginWallFn(document)) {
          return { items: [], total: 0, has_more: false, title, engine: eng, is_login_wall: true };
        }

        const engineSelectors = selectors[eng];
        if (!engineSelectors) {
          // Fallback: try to extract any tabular data
          return fallbackExtraction();
        }

        // Try each row selector variant
        let rows = [];
        for (const rowSel of engineSelectors.row) {
          rows = document.querySelectorAll(rowSel);
          if (rows.length > 0) break;
        }

        if (rows.length === 0) {
          // No rows found — may be SPA not yet rendered, or page structure changed
          return fallbackExtraction();
        }

        // Extract data from each row
        rows.forEach((row) => {
          const cells = row.querySelectorAll("td");
          const item = {};
          const cellConfig = engineSelectors.cells;

          if (cells.length > 0) {
            // Table-based layout: extract by cell index
            Object.keys(cellConfig).forEach((key) => {
              const cfg = cellConfig[key];
              item[key] = extractCellTextFromCells(cells, cfg);
            });
          } else {
            // Card/div-based layout: extract by selectors
            Object.keys(cellConfig).forEach((key) => {
              const cfg = cellConfig[key];
              item[key] = extractCellText(row, cfg);
            });
          }

          // Skip completely empty rows
          const hasAnyValue = Object.values(item).some(v => v !== "" && v !== 0);
          if (hasAnyValue) items.push(item);
        });

        // Extract pagination info
        const totalEl = queryOne(document, engineSelectors.total);
        if (totalEl) {
          const text = totalEl.textContent.trim().replace(/[^0-9]/g, "");
          total = parseInt(text, 10) || 0;
        }

        const nextEl = queryOne(document, engineSelectors.nextPage);
        hasMore = !!nextEl && !nextEl.classList.contains("disabled");

        return { items, total, has_more: hasMore, title, engine: eng, is_login_wall: false };

        function isLoginWallFn(doc) {
          const text = doc.body.textContent.toLowerCase();
          const loginKeywords = [
            "请登录", "请先登录", "login required", "sign in to continue",
            "session expired", "please log in"
          ];
          if (text.length > 5000) return false;
          return loginKeywords.some(kw => text.includes(kw));
        }

        function extractCellTextFromCells(cells, cfg) {
          // Try selector within cells context first
          const el = cfg.selector.includes("td:nth-child")
            ? (() => {
                const match = cfg.selector.match(/td:nth-child\((\d+)\)/);
                if (match) {
                  const idx = parseInt(match[1], 10) - 1;
                  if (idx >= 0 && idx < cells.length) {
                    const target = cells[idx];
                    if (cfg.selector.includes(" a")) {
                      const a = target.querySelector("a");
                      return a ? a.textContent.trim() : target.textContent.trim();
                    }
                    return target.textContent.trim();
                  }
                }
                return "";
              })()
            : "";
          if (el) return el;
          // Fallback
          if (cfg.fallback) {
            const fbMatch = cfg.fallback.match(/td:nth-child\((\d+)\)/);
            if (fbMatch) {
              const idx = parseInt(fbMatch[1], 10) - 1;
              if (idx >= 0 && idx < cells.length) {
                return cells[idx].textContent.trim();
              }
            }
          }
          return "";
        }

        function fallbackExtraction() {
          const fallbackItems = [];
          const tables = document.querySelectorAll("table");
          tables.forEach((table) => {
            const tRows = table.querySelectorAll("tbody tr, tr");
            tRows.forEach((row) => {
              const tCells = row.querySelectorAll("td");
              if (tCells.length >= 2) {
                const item = {};
                tCells.forEach((cell, idx) => {
                  item[`col_${idx}`] = cell.textContent.trim().substring(0, 200);
                });
                fallbackItems.push(item);
              }
            });
          });
          return { items: fallbackItems, total: 0, has_more: false, title, engine: eng, is_login_wall: false };
        }
      },
      args: [engine, ENGINE_SELECTORS]
    });

    if (results && results[0] && results[0].result) {
      return results[0].result;
    }
    return { items: [], total: 0, has_more: false, title: "", engine };
  } catch (err) {
    return { items: [], total: 0, has_more: false, title: "", engine, error: String(err) };
  }
}
