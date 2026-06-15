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
  if (lower.includes("zoomeye.org") || lower.includes("zoomeye.com")) return "zoomeye";
  if (lower.includes("quake.360.cn") || lower.includes("quake.360.net")) return "quake";
  if (lower.includes("shodan.io")) return "shodan";
  if (lower.includes("censys.io")) return "censys";
  if (lower.includes("daydaymap.com")) return "daydaymap";
  if (lower.includes("onyphe.io")) return "onyphe";
  if (lower.includes("greynoise.io")) return "greynoise";
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
    // SPA strategy: give the page time to start rendering
    await new Promise((resolve) => setTimeout(resolve, Math.min(timeout, 5000)));
    // If the tab already reached "complete" during the SPA delay,
    // the onUpdated listener below would never fire — resolve now.
    const afterDelay = await chrome.tabs.get(tabId);
    if (afterDelay && afterDelay.status === "complete") {
      await new Promise((resolve) => setTimeout(resolve, 3000));
      return;
    }
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
    // FOFA uses Vue SPA with hsxa-* prefixed class names.
    // CDP DOM inspection (2026-06-03):
    //   Row: .hsxa-meta-data-item (result card container)
    //   Cells: <a> links with qbase64 params (stable across FOFA updates)
    //   Key span: .hsxa-host = "IP:Port" directly
    row: [
      ".hsxa-meta-data-item",
      "[class*='meta-data-item']",
      ".result-card", "[class*='result-card']",
      "[class*='result-item']", ".result-item",
      "[class*='result'] > div"
    ],
    cells: {
      // Link-based extraction: each link's text IS the field value.
      // qbase64 param patterns are FOFA's internal query format — stable/rarely change.
      // IMPORTANT: a[href*='qbase64=aXA9'] matches country links too (not just IP).
      // span.hsxa-host is the PRIMARY IP selector: it shows "IP:Port" directly.
      // CDP verified (2026-06-03): span.hsxa-host returns "8.8.8.8:53" correctly.
      ip: { selector: "span.hsxa-host, a[href*='qbase64=aXA9']" },
      port: { selector: "a[href*='qbase64=cG9ydD0']" },
      protocol: { selector: "a[href*='qbase64=cHJvdG9jb2w9'], a[href*='qbase64=cHJvdG9jb2xf']" },
      host: { selector: "a[href*='qbase64=ZG9tYWluPS'], a[href*='qbase64=aG9zdD0'], span.hsxa-host" },
      title: { selector: "[class*='title'] a, [class*='title'] span, [class*='name']" },
      country_code: { selector: "a[href*='qbase64=Y291bnRyeT0']" },
      asn: { selector: "a[href*='qbase64=YXNuPS']" },
      org: { selector: "a[href*='qbase64=b3JnPS']" },
      banner: { selector: "a[href*='qbase64=YmFubmVyX2hhc2g9'], pre, [class*='banner-content']" }
    },
    total: [".total-count", ".total_count", "[class*='total']", "[class*='count']"],
    nextPage: [".next", ".next-page", "[class*='next']", ".el-pagination__next"]
  },
  hunter: {
    // Hunter uses Quasar UI. Data stored in .q-tooltip spans inside <div class="cell"> wrappers.
    // Columns: 1=序号, 2=IP, 3=域名, 4=端口/服务, 5=标题, 6=状态码, 7=ICP, 8=应用, 9=标签, 10=地区, 11=更新时间
    row: [
      ".q-table tbody tr",
      ".q-table__body tr",
      ".list-table tbody tr",
      ".page-list-body_table tr",
      ".result-list > .result-item",
      ".result-item",
      "div[class*='result-item']",
      "[class*='result-list'] > div",
      "[class*='result'] > div",
      ".page-list-body > div"
    ],
    cells: {
      ip: { selector: "td:nth-child(2)" },
      host: { selector: "td:nth-child(3)" },
      port: { selector: "td:nth-child(4)" },
      protocol: { selector: "td:nth-child(4)" },
      title: { selector: "td:nth-child(5)" },
      status_code: { selector: "td:nth-child(6)" },
      org: { selector: "td:nth-child(7)" },
      product: { selector: "td:nth-child(8)" },
      country_code: { selector: "td:nth-child(10)" },
    },
    total: [".total-count", ".total", "[class*='total-count']", "[class*='total']", ".page-list-body_statistic"],
    nextPage: [".next", ".q-pagination button", "[class*='next']", ".pagination-next", ".page-list-pagination button"]
  },
  zoomeye: {
    // ZoomEye uses card-based layout (2026-06-15 verified from saved HTML).
    // Container: div.search-result-item-container (each result block)
    // Header: div.header-bar contains IP:Port in div.url-container
    // Port/Protocol: div.protocol-port-box button elements
    // IP: span._public-hover_uxlu6_1 or div.ip-detail-box span
    // Pagination: ul.ant-pagination
    row: [
      // Primary — card container (2026-06-15 verified)
      ".search-result-item-container",
      // Broader card matches
      "[class*='search-result-item-container']",
      ".search-result-item",
      "[class*='search-result-item']",
      // Generic fallbacks
      "[class*='result-item']",
      "[class*='result-list'] > div",
      "[class*='result'] > div",
      // Ant Design table — DEPRECATED, kept as last fallback
      ".ant-table tbody tr",
      ".ant-table-tbody tr"
    ],
    cells: {
      // IP is inside header-bar > div.url-container > span/a text (e.g. "132.232.231.41:8888")
      // or inside ip-detail-box > span._public-hover_uxlu6_1
      ip: { selector: "span._public-hover_uxlu6_1, div.url-container span, div.ip-detail-box span" },
      // Port is first button in div.protocol-port-box (e.g. "8888")
      port: { selector: "div.protocol-port-box button:first-child, div.protocol-port-box button" },
      // Protocol is second button's span in div.protocol-port-box (e.g. "http")
      protocol: { selector: "div.protocol-port-box button:last-child span, div.protocol-box span" },
      // Host/domain from header-bar link
      host: { selector: "div.header-bar a[href], div.url-container a" },
      // Title from search-result-item body
      title: { selector: ".title, [class*='title'], div.search-result-item-info" },
      // Banner from pre tab panels
      banner: { selector: "div.ant-tabs-tabpane-active pre, pre" },
      country_code: { selector: ".location, [class*='location'], [class*='country']" }
    },
    total: ["li.ant-pagination-total-text span", ".total", "[class*='total']", "[class*='count']"],
    nextPage: ["li.ant-pagination-next:not(.ant-pagination-disabled) a", ".ant-pagination-next a", ".next", "[class*='next']"]
  },
  quake: {
    // Quake 360 search results (Vue SPA, Element UI).
    // Result container: div.item-container
    // IP: span.copy_btn with data-clipboard-text="IP:port"
    // Port: span.port.common-tag
    // Protocol: span.server-protocol.common-tag
    // Country: span.country-container span.address
    // Title: span.ellipse-text inside div.title-line
    // ASN/Org/ISP: div.item span.label + span.ellipse-text
    row: [
      ".item-container",
      "[class*='result-item']",
      "[class*='result-card']",
      ".el-table tbody tr",
      ".el-table__body tr"
    ],
    cells: {
      ip: { selector: "div.ip span.copy_btn, [data-clipboard-text]" },
      port: { selector: "span.port" },
      protocol: { selector: "span.server-protocol" },
      title: { selector: ".title-line span.ellipse-text, [class*='title']" },
      country_code: { selector: ".country-container .address" },
      asn: { selector: ".item .label + .ellipse-text" },
    },
    total: [".total-count", ".total", "[class*='total']", ".pagination-info"],
    nextPage: [".next", ".next-page", "[class*='next']", ".el-pagination__next"]
  },
  shodan: {
    // Shodan search results (verified from HTML).
    // Result: div.result > div.heading + div.result-details + div.banner-data
    // IP: a.title[href='/host/X.X.X.X'] href → extract IP
    // Port: second a[href] in div.heading → extract port from URL
    // Title: a.title.text-dark text content
    // Org: a.filter-link.filter-org text
    // Country: a.filter-link.text-dark (first one with flag img)
    // Banner: div.banner-data pre text
    row: [
      ".result",
      "[class*='result'] > div"
    ],
    cells: {
      ip: { selector: "div.heading a.title", attr: "href", extract: "ip_from_path" },
      port: { selector: "div.heading a[href^='http://'], a.bg-primary[href^='#']", attr: "href", extract: "port_from_url" },
      title: { selector: "div.heading a.title" },
      org: { selector: "a.filter-link.filter-org" },
      country_code: { selector: ".result-details .flag + a, .result-details li:has(.flag) a" },
      banner: { selector: "div.banner-data pre" },
    },
    total: [".total", "[class*='total']", ".result-count", "[class*='result-count']"],
    nextPage: [".next", ".pagination-next", "[class*='next']", "a[rel='next']"]
  },
  censys: {
    // Censys uses a modern SPA layout with result cards.
    row: [
      "[class*='result-card']", "[class*='search-result']",
      "[class*='result-list'] > div", "[class*='result'] > div",
      "table tbody tr"
    ],
    cells: {
      ip: { selector: "[class*='ip'], [data-ip]" },
      port: { selector: "[class*='port'], [data-port]" },
      host: { selector: "[class*='hostname'], [class*='domain']" },
      title: { selector: "[class*='title'], h2, h3" },
      country_code: { selector: "[class*='country'], [class*='location']" },
      org: { selector: "[class*='org'], [class*='organization']" }
    },
    total: ["[class*='total']", "[class*='count']"],
    nextPage: ["[class*='next']", "button[aria-label='next']"]
  },
  daydaymap: {
    // DayDayMap uses a layout similar to FOFA.
    row: [
      "[class*='result-item']", "[class*='result-card']",
      "[class*='result-list'] > div", "[class*='result'] > div",
      "table tbody tr"
    ],
    cells: {
      ip: { selector: "[class*='ip'], [data-ip]" },
      port: { selector: "[class*='port'], [data-port]" },
      host: { selector: "[class*='domain'], [class*='host']" },
      title: { selector: "[class*='title'], [class*='name']" },
      country_code: { selector: "[class*='country'], [class*='location']" },
      org: { selector: "[class*='org'], [class*='company']" }
    },
    total: ["[class*='total']", "[class*='count']"],
    nextPage: ["[class*='next']", ".el-pagination__next"]
  },
  onyphe: {
    // Onyphe uses a table-based layout.
    row: [
      "table tbody tr", "[class*='result-row']",
      "[class*='result-list'] > div", "[class*='result'] > div"
    ],
    cells: {
      ip: { selector: "[class*='ip'], [data-ip]" },
      port: { selector: "[class*='port'], [data-port]" },
      host: { selector: "[class*='hostname'], [class*='domain']" },
      title: { selector: "[class*='title']" },
      country_code: { selector: "[class*='country'], [class*='location']" },
      org: { selector: "[class*='org'], [class*='organization']" }
    },
    total: ["[class*='total']", "[class*='count']"],
    nextPage: ["[class*='next']", "a[rel='next']"]
  },
  greynoise: {
    // GreyNoise uses a table-based layout for IP intelligence.
    row: [
      "table tbody tr", "[class*='result-row']",
      "[class*='result-list'] > div", "[class*='result'] > div"
    ],
    cells: {
      ip: { selector: "[class*='ip'], [data-ip]" },
      classification: { selector: "[class*='classification'], [class*='status']" },
      org: { selector: "[class*='org'], [class*='organization']" },
      country_code: { selector: "[class*='country'], [class*='location']" }
    },
    total: ["[class*='total']", "[class*='count']"],
    nextPage: ["[class*='next']", "a[rel='next']"]
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
        const bodyText = (document.body?.innerText || "").toLowerCase();
        const loginRequired = /登录|登陆|请先登录|login|sign in|signin|unauthorized/.test(bodyText + " " + title.toLowerCase());

        // Check for login wall first
        if (isLoginWallFn(document)) {
          return { items: [], total: 0, has_more: false, title, engine: eng, is_login_wall: true, extraction_method: "login_wall" };
        }

        const engineSelectors = selectors[eng];
        if (!engineSelectors) {
          return fallbackExtraction();
        }

        // Try each row selector variant
        let rows = [];
        let rowSelectorUsed = "";
        for (const rowSel of engineSelectors.row) {
          rows = document.querySelectorAll(rowSel);
          if (rows.length > 0) {
            rowSelectorUsed = rowSel;
            break;
          }
        }

        if (rows.length === 0) {
          // No rows found — try fallback extraction
          return fallbackExtraction();
        }

        // Cell text extractor with attribute support
        function extractCellText(row, cellConfig) {
          const selectors = cellConfig.selector.split(/,\s*/);
          let el = null;
          for (const sel of selectors) {
            el = row.querySelector(sel);
            if (el) break;
          }
          if (!el && cellConfig.fallback) {
            const fbs = cellConfig.fallback.split(/,\s*/);
            for (const fb of fbs) {
              el = row.querySelector(fb);
              if (el) break;
            }
          }
          if (!el) return "";

          // Support attribute extraction (e.g. href, src, data-*)
          if (cellConfig.attr) {
            const val = el.getAttribute(cellConfig.attr) || "";
            // Post-process: extract IP or port from URL paths
            if (cellConfig.extract) {
              if (cellConfig.extract === "ip_from_path") {
                const m = val.match(/\/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
                return m ? m[1] : val;
              }
              if (cellConfig.extract === "port_from_url") {
                const m = val.match(/:(\d{1,5})\//);
                if (m) return m[1];
                const m2 = val.match(/#(\d{1,5})/);
                if (m2) return m2[1];
                return "";
              }
            }
            return val.trim();
          }

          return el.textContent.trim();
        }

        // Clean Hunter UI filter labels from text
        function cleanHunterText(text) {
          if (!text) return "";
          text = text.replace(/只看该[^\s]*不看该[^\s]*/g, "");
          text = text.replace(/只看空[^\s]*不看空[^\s]*/g, "");
          text = text.replace(/看相似(网站|icon)/g, "");
          text = text.replace(/访问[^\s]*/g, "");
          text = text.replace(/复制/g, "");
          text = text.replace(/云厂商/g, "");
          text = text.replace(/\s+/g, " ").trim();
          return text;
        }

        // Global tooltip scan: ALL .q-tooltip in document (include hidden for position matching)
        const allTips = Array.from(document.querySelectorAll(".q-tooltip"));
        const tipData = []; // [{top, bottom, text}]
        allTips.forEach(tip => {
          const text = tip.textContent.trim();
          if (!text || text.includes("只看") || text.includes("不看") || text.length > 50) return;
          const rect = tip.getBoundingClientRect();
          // Include tooltips even if height=0 (Quasar may hide them)
          tipData.push({ top: rect.top || 0, bottom: rect.bottom || rect.top || 0, text });
        });

        // Extract data from each row/card
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
            // Fill from global tooltips: find tooltips whose vertical range overlaps this row
            const rowRect = row.getBoundingClientRect();
            if (rowRect.height > 0) {
              for (const tip of tipData) {
                // Check if tooltip vertical range overlaps row vertical range
                if (tip.bottom >= rowRect.top && tip.top <= rowRect.bottom) {
                  if (!item.status_code && /^\d{3}$/.test(tip.text)) item.status_code = parseInt(tip.text, 10);
                  if (!item.country_code && /[\u4e00-\u9fa5]{2,}/.test(tip.text) && tip.text.length < 15) item.country_code = tip.text;
                }
              }
            }
          } else {
            // Card/div-based layout: extract by selectors
            Object.keys(cellConfig).forEach((key) => {
              const cfg = cellConfig[key];
              item[key] = extractCellText(row, cfg);
            });
          }

          // Clean Hunter UI text from title/ip/host
          if (typeof item.title === "string") item.title = cleanHunterText(item.title);
          if (typeof item.ip === "string") item.ip = cleanHunterText(item.ip);
          if (typeof item.host === "string") item.host = cleanHunterText(item.host);

          // Clean Shodan country/org: extract from multi-line result-details
          if (typeof item.country_code === "string" && item.country_code.includes("\n")) {
            const lines = item.country_code.split(/\n/).map(l => l.trim()).filter(l => l.length > 1);
            // Country: look for "Country, City" pattern
            for (const l of lines) {
              if (/^[A-Z][a-z]+,\s*[A-Z]/.test(l)) {
                item.country_code = l.trim();
                break;
              }
              // Chinese locations
              if (/^中国/.test(l) || /^[\u4e00-\u9fa5]{2,}省/.test(l)) {
                item.country_code = l.trim();
                break;
              }
            }
            // Fallback: line after org
            if (!item.country_code || item.country_code.includes("\n")) {
              const orgIdx = lines.findIndex(l => /Cloud|Inc|Ltd|Corp|Company/.test(l));
              if (orgIdx >= 0 && orgIdx + 1 < lines.length) {
                item.country_code = lines[orgIdx + 1].trim();
              }
            }
          }
          if (typeof item.org === "string" && item.org.includes("\n")) {
            const lines = item.org.split(/\n/).map(l => l.trim()).filter(l => l.length > 3 && !/^\d/.test(l));
            const orgLine = lines.find(l => l !== item.ip && /Cloud|Inc|Ltd|Corp|Company|LLC|University/.test(l));
            if (orgLine) item.org = orgLine.trim();
          }

          // Port: ensure number, fallback to protocol
          if (typeof item.port === "string") item.port = parseInt(item.port, 10) || 0;
          if (!item.port && item.protocol) {
            const pm = String(item.protocol).match(/(\d{1,5})/);
            if (pm) item.port = parseInt(pm[1], 10);
          }

          // Post-fix: if ip is empty but host contains IP, move it
          if (!item.ip && item.host) {
            const m = String(item.host).match(/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
            if (m) item.ip = m[1];
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

        return { items, total, has_more: hasMore, title, engine: eng, is_login_wall: false, row_selector_used: rowSelectorUsed, rows_found: rows.length, extraction_method: "selector" };

        function isLoginWallFn(doc) {
          const text = doc.body.textContent.toLowerCase();
          const loginKeywords = [
            "请登录", "请先登录", "login required", "sign in to continue",
            "session expired", "please log in"
          ];
          if (text.length > 5000) return false;
          return loginKeywords.some(kw => text.includes(kw));
        }

        // queryOne MUST be defined inside the injected function — chrome.scripting
        // .executeScript serializes only the function body, so module-level helpers
        // are undefined in the page scope and throw ReferenceError (same class of bug
        // as the extractCellText scope fix). Missing this caused selector-based
        // extraction to throw at the pagination step → empty items → 0 assets.
        function queryOne(root, selectors) {
          if (!selectors) return null;
          const list = Array.isArray(selectors) ? selectors : [selectors];
          for (const sel of list) {
            const el = root.querySelector(sel);
            if (el) return el;
          }
          return null;
        }

        function extractCellTextFromCells(cells, cfg) {
          const match = cfg.selector.match(/td:nth-child\((\d+)\)/);
          if (!match) return "";
          const idx = parseInt(match[1], 10) - 1;
          if (idx < 0 || idx >= cells.length) return "";
          const target = cells[idx];
          // Hunter stores data in .q-tooltip spans inside cells
          const tooltips = target.querySelectorAll(".q-tooltip");
          for (const tip of tooltips) {
            const text = (tip.textContent || "").trim();
            if (text && !text.includes("只看") && !text.includes("不看")) {
              return text;
            }
          }
          // Fallback: try all span elements (for non-tooltip columns)
          const allSpans = target.querySelectorAll("span");
          for (const sp of allSpans) {
            const text = (sp.textContent || "").trim();
            if (text && !text.includes("只看") && !text.includes("不看") && text !== "-" && text.length < 50) {
              return text;
            }
          }
          // Last resort: raw cell text
          return target.textContent.trim();
        }

        function fallbackExtraction() {
          // Try table-based extraction first
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

          if (fallbackItems.length > 0) {
            return { items: fallbackItems, total: 0, has_more: false, title, engine: eng, is_login_wall: false, extraction_method: "table_fallback" };
          }

          // Try card-based extraction using link patterns
          const cardResult = cardBasedExtraction();
          if (cardResult.items.length > 0) return cardResult;

          return { items: [], total: 0, has_more: false, title, engine: eng, is_login_wall: false, extraction_method: "no_match" };
        }

        function cardBasedExtraction() {
          const cardItems = [];

          // Find potential result cards by looking for elements with IP-like content
          const allLinks = Array.from(document.querySelectorAll("a"));
          const ipLinks = allLinks.filter(a => {
            const href = a.href || "";
            const text = a.textContent.trim();
            return /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(text) ||
                   href.includes("qbase64=aXA9") ||
                   href.includes("ip=") ||
                   (a.className && a.className.includes("ip"));
          });

          if (ipLinks.length === 0) {
            // Try finding card containers
            const cardSelectors = [
              "[class*='result-card']", "[class*='result-item']",
              "[class*='asset-item']", "[class*='data-item']",
              ".result-list > div", ".list_content > div"
            ];
            for (const sel of cardSelectors) {
              const cards = document.querySelectorAll(sel);
              if (cards.length > 0) {
                cards.forEach((card) => {
                  const item = {};
                  const links = Array.from(card.querySelectorAll("a"));
                  if (links.length >= 2) {
                    // Heuristic: first link with IP pattern is IP, second is port, etc.
                    for (const link of links) {
                      const text = link.textContent.trim();
                      if (/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(text)) {
                        if (!item.ip) item.ip = text;
                      } else if (/^\d{1,5}$/.test(text) && !item.port) {
                        item.port = text;
                      } else if (!item.title && text.length > 3 && text.length < 100) {
                        item.title = text;
                      }
                    }
                    if (Object.keys(item).length > 0) cardItems.push(item);
                  }
                });
                break;
              }
            }
          } else {
            // Group IP links into items (each IP link + nearby links = one item)
            for (const ipLink of ipLinks.slice(0, 100)) {
              const item = {};
              const ipText = ipLink.textContent.trim();
              item.ip = /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(ipText) ? ipText : "";

              // Look for nearby elements for port, protocol, etc.
              const parent = ipLink.closest("div, li, tr, section, article");
              if (parent) {
                const siblings = Array.from(parent.querySelectorAll("a, span, div"));
                for (const sib of siblings) {
                  const text = sib.textContent.trim();
                  if (!text || text === ipText) continue;
                  if (/^\d{1,5}$/.test(text) && !item.port) item.port = text;
                  else if (/^[a-zA-Z][\w-]*$/.test(text) && text.length < 30 && !item.protocol) item.protocol = text;
                  else if (text.length > 3 && text.length < 100 && !item.title && !item.banner) item.title = text;
                }
              }

              if (Object.keys(item).length > 0) cardItems.push(item);
            }
          }

          return { items: cardItems, total: 0, has_more: false, title, engine: eng, is_login_wall: false, extraction_method: "card_fallback" };
        }
        return {
          items,
          total,
          has_more: hasMore,
          title,
          engine: eng,
          is_login_wall: false,
          login_required: loginRequired && items.length === 0
        };
      },
      args: [engine, ENGINE_SELECTORS]
    });

    if (results && results[0]) {
      if (results[0].result) {
        return results[0].result;
      }
      if (results[0].error) {
        return { items: [], total: 0, has_more: false, title: "", engine, login_required: false, error: "injection_error: " + String(results[0].error) };
      }
    }
    return { items: [], total: 0, has_more: false, title: "", engine, login_required: false, error: "no_injection_result" };
  } catch (err) {
    // DOM extraction failed — return empty result, let caller handle
    return { items: [], total: 0, has_more: false, title: "", engine, login_required: false, error: String(err) };
  }
}
