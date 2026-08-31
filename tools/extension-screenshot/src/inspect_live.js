import { detectEngine, extractEngineAssets, checkLoginCookies, ENGINE_SELECTORS } from "./capture.js";
import { prepareDayDayMapSearch } from "./daydaymap.js";

const REPORT_URL = "http://127.0.0.1:18927/report";
const SPA_WAIT_MS = 9000;

function sanitizeURL(raw) {
  try {
    const parsed = new URL(raw);
    for (const key of [...parsed.searchParams.keys()]) {
      if (/ticket|token|code|session|auth|password|jwt|csrf/i.test(key)) {
        parsed.searchParams.set(key, "[redacted]");
      }
    }
    if (parsed.hash && /ticket|token|code=/i.test(parsed.hash)) {
      parsed.hash = "[redacted]";
    }
    return parsed.toString();
  } catch {
    return String(raw || "").split("?")[0];
  }
}

function searchURLFor(engine, currentURL) {
  let host = "";
  try { host = new URL(currentURL).host; } catch { /* ignore */ }
  const queries = {
    fofa: 'port="80"',
    hunter: "port=443",
    zoomeye: "port:80",
    quake: "port:80",
    shodan: "port:80",
    censys: "services.port:80",
    daydaymap: "port=80"
  };
  const query = queries[engine] || "port=80";
  const b64 = btoa(unescape(encodeURIComponent(query)));
  const enc = encodeURIComponent(query);
  const encB64 = encodeURIComponent(b64);
  switch (engine) {
    case "fofa":
      return `https://fofa.info/result?qbase64=${encB64}`;
    case "hunter":
      return `https://hunter.qianxin.com/home/list?search=${encB64}&conditions=`;
    case "quake":
      return `https://quake.360.net/quake/#/searchResult?searchVal=${enc}&selectIndex=quake_service&latest=true`;
    case "zoomeye": {
      const zoomHost = /zoomeye\.(ai|hk|com)$/i.test(host) ? host : "www.zoomeye.org";
      return `https://${zoomHost}/searchResult?q=${enc}`;
    }
    case "shodan":
      return `https://www.shodan.io/search?query=${enc}`;
    case "censys":
      return `https://platform.censys.io/search?resource=hosts&sort=RELEVANCE&per_page=25&virtual_hosts=EXCLUDE&q=${enc}`;
    case "daydaymap":
      return `https://www.daydaymap.com/searchResult?keyword=${enc}`;
    default:
      return currentURL;
  }
}

function looksLikeResultURL(engine, rawURL) {
  const lower = String(rawURL || "").toLowerCase();
  switch (engine) {
    case "fofa": return lower.includes("/result");
    case "hunter": return lower.includes("/list") || lower.includes("search=");
    case "zoomeye": return lower.includes("searchresult") || lower.includes("search?q=");
    case "quake": return lower.includes("searchresult") || lower.includes("searchval=");
    case "shodan": return lower.includes("/search?");
    case "censys": return lower.includes("/search?") || lower.includes("resource=hosts");
    case "daydaymap": return lower.includes("/searchresult");
    default: return false;
  }
}

function inspectorFunc(engine, selectors) {
  const out = {
    engine: engine || "unknown",
    url: location.href,
    title: document.title || "",
    pathname: location.pathname,
    hash: location.hash || "",
    readyState: document.readyState,
    bodyTextLength: (document.body && document.body.innerText || "").length,
    loginSignals: {},
    selectorCounts: {},
    currentRow: {},
    tables: [],
    relevantClasses: [],
    firstRow: null,
    sampleText: ""
  };

  const bodyText = (document.body && document.body.innerText || "").slice(0, 800);
  out.sampleText = bodyText.replace(/\s+/g, " ").trim().slice(0, 400);
  const lower = (bodyText + " " + out.title).toLowerCase();
  out.loginSignals = {
    hasLoginWord: /请登录|请先登录|sign in|log in|登录|登入/.test(lower),
    hasCaptcha: /captcha|验证码|challenge|cloudflare|just a moment/.test(lower),
    hasAvatar: !!document.querySelector("img[class*='avatar'], .avatar, [class*='user-info'], [class*='userinfo']"),
    passwordInput: !!document.querySelector("input[type='password']")
  };

  const classSet = new Set();
  document.querySelectorAll("[class]").forEach((el) => {
    const cls = el.className;
    if (typeof cls !== "string") return;
    cls.split(/\s+/).forEach((c) => {
      if (/result|list|table|row|data|cell|card|item|col|asset|ip|port|protocol|host|title|banner|count|total|page|search|meta|hsxa|q-table|item-container/i.test(c)) {
        classSet.add(c);
      }
    });
  });
  out.relevantClasses = [...classSet].slice(0, 80);

  const tables = document.querySelectorAll("table");
  out.tables = Array.from(tables).slice(0, 6).map((t) => ({
    className: String(t.className || "").slice(0, 120),
    rows: t.querySelectorAll("tr").length,
    headers: Array.from(t.querySelectorAll("th")).slice(0, 12).map((th) => th.textContent.trim().slice(0, 40))
  }));

  const extra = [
    ".hsxa-meta-data-item", "[class*='meta-data-item']",
    ".q-table tbody tr", ".q-table__body tr",
    "div.search-result-item-container", ".search-result-item",
    ".item-container",
    ".l-search-results .result", "div.result",
    "[class*='result-card']", "[class*='result-item']",
    "[class*='search-result']",
    ".el-table__row", "table tbody tr",
    "[data-testid*='result']",
    ".list_content > tbody > tr"
  ];
  const rowSels = [];
  if (selectors && selectors.row) rowSels.push(...selectors.row);
  extra.forEach((s) => { if (!rowSels.includes(s)) rowSels.push(s); });

  for (const sel of rowSels) {
    try {
      const count = document.querySelectorAll(sel).length;
      if (count > 0) out.selectorCounts[sel] = count;
    } catch {
      out.selectorCounts[sel] = "invalid";
    }
  }

  if (selectors) {
    const used = Object.keys(out.selectorCounts).find((s) => (selectors.row || []).includes(s));
    out.currentRow = {
      firstMatching: used || "",
      count: used ? out.selectorCounts[used] : 0
    };
  }

  const firstSel = Object.keys(out.selectorCounts)[0];
  if (firstSel && typeof out.selectorCounts[firstSel] === "number") {
    const el = document.querySelector(firstSel);
    if (el) {
      const cells = el.querySelectorAll("td");
      out.firstRow = {
        selector: firstSel,
        tag: el.tagName,
        className: String(el.className || "").slice(0, 200),
        childCount: el.children.length,
        tdCount: cells.length,
        tdTexts: Array.from(cells).slice(0, 8).map((c) => c.textContent.replace(/\s+/g, " ").trim().slice(0, 80)),
        text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 240),
        html: el.outerHTML.slice(0, 1600)
      };
    }
  }

  const totalSels = (selectors && selectors.total) || [];
  out.totalHits = {};
  for (const sel of totalSels) {
    try {
      const el = document.querySelector(sel);
      if (el) out.totalHits[sel] = el.textContent.replace(/\s+/g, " ").trim().slice(0, 80);
    } catch { /* ignore */ }
  }
  return out;
}

function summarizeExtract(result) {
  if (!result) return null;
  const items = Array.isArray(result.items) ? result.items.slice(0, 5).map((item) => ({
    ip: item.ip || "",
    port: item.port || 0,
    protocol: item.protocol || "",
    host: item.host || "",
    title: (item.title || "").slice(0, 80),
    source: item.source || ""
  })) : [];
  return {
    engine: result.engine,
    title: result.title,
    is_login_wall: !!result.is_login_wall,
    extraction_method: result.extraction_method || "",
    rowSelectorUsed: result.rowSelectorUsed || result.row_selector_used || "",
    rowsFound: result.rowsFound || result.rows_found || 0,
    extractionError: result.error || result.extractionError || "",
    total: result.total || 0,
    has_more: !!result.has_more,
    itemCount: Array.isArray(result.items) ? result.items.length : 0,
    items
  };
}

async function inspectTab(tab) {
  const engine = detectEngine(tab.url);
  const cookies = await checkLoginCookies(tab.url);
  const snapshot = {
    tabId: tab.id,
    engine,
    url: sanitizeURL(tab.url),
    title: tab.title || "",
    cookie_count: cookies.cookie_count,
    has_login_cookies: cookies.has_login_cookies,
    cookie_names: cookies.cookie_names || []
  };
  try {
    const injected = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: inspectorFunc,
      args: [engine, ENGINE_SELECTORS[engine] || null]
    });
    snapshot.dom = injected?.[0]?.result || { error: "empty_inject" };
  } catch (err) {
    snapshot.dom = { error: String(err) };
  }
  try {
    snapshot.extract = summarizeExtract(await extractEngineAssets(tab.id));
  } catch (err) {
    snapshot.extract = { error: String(err) };
  }
  return snapshot;
}

async function waitReady(tabId, ms) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    try {
      const tab = await chrome.tabs.get(tabId);
      if (tab && tab.status === "complete") {
        await new Promise((r) => setTimeout(r, 2500));
        return;
      }
    } catch { /* ignore */ }
    await new Promise((r) => setTimeout(r, 400));
  }
}

function setStatus(lines) {
  let el = document.getElementById("inspectStatus");
  if (!el) {
    el = document.createElement("pre");
    el.id = "inspectStatus";
    el.style.cssText = "white-space:pre-wrap;background:#111;color:#d4f4d4;padding:16px;border-radius:8px;font:12px/1.4 ui-monospace,monospace;";
    document.body.prepend(el);
  }
  el.textContent = Array.isArray(lines) ? lines.join("\n") : String(lines);
}

function dumpEngineDetails(engine) {
  const out = { engine, url: location.href, title: document.title, pathname: location.pathname, hash: location.hash };
  const body = (document.body && document.body.innerText || "").replace(/\s+/g, " ").trim();
  out.sample = body.slice(0, 500);
  out.loading = !!document.querySelector("[class*='loading'], .search-result-loading, .el-loading-mask, .ant-spin-spinning");
  const classSet = new Set();
  document.querySelectorAll("[class]").forEach((el) => {
    const cls = typeof el.className === "string" ? el.className : "";
    cls.split(/\s+/).forEach((c) => {
      if (/result|item|card|host|ip|port|row|table|meta|hsxa|container|list/i.test(c)) classSet.add(c);
    });
  });
  out.relevantClasses = [...classSet].slice(0, 100);

  const probe = [
    ".hsxa-meta-data-item", ".hsxa-host", ".hsxa-copy-btn",
    ".q-table tbody tr", ".q-table__row",
    ".search-result-item-container", ".search-result-content", ".search-result-left", ".search-result-loading",
    ".item-container", ".english-mode-result-search", ".search-wrapper",
    ".l-search-results .result", "div.result",
    "[class*='result-card']", "[data-testid*='result']",
    ".search-result-header", ".ant-empty-description"
  ];
  out.probe = {};
  for (const sel of probe) {
    try { out.probe[sel] = document.querySelectorAll(sel).length; } catch { out.probe[sel] = "invalid"; }
  }

  if (engine === "fofa") {
    const row = document.querySelector(".hsxa-meta-data-item");
    if (row) {
      out.fofa = {
        copy: Array.from(row.querySelectorAll("[data-clipboard-text]")).slice(0, 12).map((el) => el.getAttribute("data-clipboard-text")),
        hostEls: Array.from(row.querySelectorAll(".hsxa-host, [class*='hsxa-host']")).map((el) => ({ cls: el.className, text: el.textContent.trim().slice(0, 80) })),
        links: Array.from(row.querySelectorAll("a[href*='qbase64']")).slice(0, 25).map((a) => ({
          text: a.textContent.replace(/\s+/g, " ").trim().slice(0, 80),
          href: (a.getAttribute("href") || "").slice(0, 120)
        })),
        childClasses: Array.from(row.querySelectorAll("[class]")).slice(0, 50).map((el) => String(el.className).slice(0, 120))
      };
    }
  }
  if (engine === "quake") {
    out.quakeHtml = (document.querySelector(".search-wrapper, .english-mode-result-search, .index-page") || document.body).innerHTML.slice(0, 4000);
  }
  if (engine === "zoomeye") {
    out.zoomeye = {
      loading: !!document.querySelector(".search-result-loading"),
      leftHtml: (document.querySelector(".search-result-left, .search-result-content, .result-container") || {}).innerHTML?.slice(0, 4000) || ""
    };
  }
  if (engine === "censys") {
    out.censysHtml = (document.querySelector("main, [class*='resultsWrapper'], [class*='pageContent']") || document.body).innerHTML.slice(0, 3500);
  }
  if (engine === "daydaymap") {
    out.dayday = {
      empty: (document.querySelector(".ant-empty-description") || {}).textContent || "",
      summary: (document.querySelector("[class*='StyleSummary'], .search-result-header") || {}).textContent || ""
    };
  }
  return out;
}

async function waitForRows(tabId, selectors, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const injected = await chrome.scripting.executeScript({
        target: { tabId },
        func: (sels) => {
          const loading = !!document.querySelector(".search-result-loading, .el-loading-mask, [class*='loading'][class*='result']");
          const counts = {};
          let found = 0;
          for (const sel of sels) {
            try {
              const n = document.querySelectorAll(sel).length;
              counts[sel] = n;
              if (n > 0) found += n;
            } catch { counts[sel] = "invalid"; }
          }
          return { loading, found, counts, href: location.href, title: document.title };
        },
        args: [selectors]
      });
      const st = injected?.[0]?.result;
      if (st && st.found > 0 && !st.loading) return st;
    } catch { /* ignore */ }
    await new Promise((r) => setTimeout(r, 1000));
  }
  return { timeout: true };
}

export async function runLiveInspectPass2() {
  const report = { started_at: new Date().toISOString(), pass: 2, engines: {} };
  const log = [];
  const say = (msg) => { log.push(msg); setStatus(log); };
  say("pass 2 start");
  const tabs = await chrome.tabs.query({});
  const byEngine = {};
  for (const tab of tabs) {
    const engine = detectEngine(tab.url);
    if (engine !== "unknown") byEngine[engine] = tab;
  }

  async function details(tab, engine) {
    const injected = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: dumpEngineDetails,
      args: [engine]
    });
    let extract = null;
    try { extract = summarizeExtract(await extractEngineAssets(tab.id)); } catch (err) { extract = { error: String(err) }; }
    return { url: sanitizeURL(tab.url), title: tab.title, details: injected?.[0]?.result, extract };
  }

  if (byEngine.fofa) {
    say("fofa dump fields");
    report.engines.fofa = await details(byEngine.fofa, "fofa");
  }
  if (byEngine.hunter) {
    say("hunter dump fields");
    report.engines.hunter = await details(byEngine.hunter, "hunter");
  }
  if (byEngine.shodan) {
    say("shodan dump fields");
    report.engines.shodan = await details(byEngine.shodan, "shodan");
  }

  if (byEngine.zoomeye) {
    say("zoomeye wait for cards");
    await waitForRows(byEngine.zoomeye.id, [
      "div.search-result-item-container",
      ".search-result-item",
      "[class*='search-result-item-container']"
    ], 25000);
    report.engines.zoomeye = await details(byEngine.zoomeye, "zoomeye");
    report.engines.zoomeye.extract = summarizeExtract(await extractEngineAssets(byEngine.zoomeye.id));
  }

  if (byEngine.quake) {
    say("quake wait for items");
    await waitForRows(byEngine.quake.id, [".item-container", ".el-table__row", "[class*='result-item']"], 25000);
    report.engines.quake = await details(byEngine.quake, "quake");
  }

  if (byEngine.censys) {
    say("censys requery");
    const q = encodeURIComponent('host.services.port:"80"');
    await chrome.tabs.update(byEngine.censys.id, {
      url: `https://platform.censys.io/search?resource=hosts&sort=RELEVANCE&per_page=25&virtual_hosts=EXCLUDE&q=${q}`
    });
    await waitReady(byEngine.censys.id, 12000);
    await waitForRows(byEngine.censys.id, ["[class*='result-card']", "[data-testid*='result']", "table tbody tr"], 20000);
    report.engines.censys = await details(byEngine.censys, "censys");
  }

  if (byEngine.daydaymap) {
    say("daydaymap quoted query");
    try {
      await prepareDayDayMapSearch(byEngine.daydaymap.id, 'port="80"');
      await waitReady(byEngine.daydaymap.id, 12000);
      await waitForRows(byEngine.daydaymap.id, ["[class*='result-item']", "table tbody tr", "[class*='table-row']"], 15000);
    } catch (err) {
      report.engines.daydaymap = { error: String(err) };
    }
    if (!report.engines.daydaymap?.error) {
      report.engines.daydaymap = await details(byEngine.daydaymap, "daydaymap");
    }
  }

  report.finished_at = new Date().toISOString();
  try {
    const resp = await fetch(REPORT_URL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(report)
    });
    say(`report post ${resp.status}`);
  } catch (err) {
    say(`report post failed ${String(err)}`);
    await chrome.storage.local.set({ liveInspectReport2: report });
  }
  return report;
}

function dumpDayDayMapPage() {
  const ipv4 = /\b(?:\d{1,3}\.){3}\d{1,3}\b/;
  const probes = {
    "[class*='result-item']": 0,
    "[class*='result-card']": 0,
    "[class*='result-list'] > div": 0,
    ".el-table__row": 0,
    "table tbody tr": 0,
    ".ant-table-row": 0,
    "[role='row']": 0,
    ".rc-virtual-list-holder-inner > div": 0,
    "[class*='virtual']": 0,
    "[class*='table-row']": 0,
    "[class*='list-row']": 0,
    "[class*='asset']": 0,
    ".search-btn": 0,
    ".search-input": 0,
    ".ant-empty-description": 0,
    "[class*='StyleSummary']": 0
  };
  for (const sel of Object.keys(probes)) {
    try { probes[sel] = document.querySelectorAll(sel).length; } catch { probes[sel] = "invalid"; }
  }
  const classes = new Set();
  document.querySelectorAll("[class]").forEach((el) => {
    const cls = typeof el.className === "string" ? el.className : "";
    cls.split(/\s+/).forEach((c) => {
      if (/result|item|card|row|table|list|ip|port|host|asset|search|virtual|cell|summary/i.test(c)) classes.add(c);
    });
  });
  const inputs = Array.from(document.querySelectorAll("input, textarea")).slice(0, 20).map((el) => ({
    tag: el.tagName,
    type: el.type || "",
    placeholder: (el.getAttribute("placeholder") || "").slice(0, 80),
    value: String(el.value || "").slice(0, 80),
    className: String(el.className || "").slice(0, 120)
  }));
  const ipLeaves = [];
  const seen = {};
  document.querySelectorAll("tr.ant-table-row td span.ellipsis, tr.ant-table-row td").forEach((el) => {
    const text = (el.textContent || "").trim();
    const m = text.match(/^(?:\d{1,3}\.){3}\d{1,3}$/);
    if (!m || seen[m[0]]) return;
    seen[m[0]] = true;
    ipLeaves.push({
      ip: m[0],
      tag: el.tagName,
      className: String(el.className || "").slice(0, 160),
      parentClass: String(el.parentElement && el.parentElement.className || "").slice(0, 160),
      text: text.slice(0, 80)
    });
  });
  const firstHitSel = [".ant-table-row", "[class*='table-row']", "[class*='result-item']", "table tbody tr"].find((sel) => {
    try { return document.querySelectorAll(sel).length > 0; } catch { return false; }
  });
  let firstRow = null;
  if (firstHitSel) {
    const el = Array.from(document.querySelectorAll(firstHitSel)).find((node) => !String(node.className || "").includes("measure-row")) || document.querySelector(firstHitSel);
    if (el) {
      firstRow = {
        selector: firstHitSel,
        tag: el.tagName,
        className: String(el.className || "").slice(0, 200),
        text: (el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 400),
        tdTexts: Array.from(el.querySelectorAll("td")).slice(0, 12).map((td) => td.textContent.replace(/\s+/g, " ").trim().slice(0, 80)),
        headers: Array.from(document.querySelectorAll("thead th, .ant-table-thead th")).map((th) => th.textContent.replace(/\s+/g, " ").trim().slice(0, 40)),
        html: el.outerHTML.slice(0, 5000)
      };
    }
  }
  if (!firstRow && ipLeaves[0]) {
    let el = document.querySelectorAll("a, span, div, td");
    // already have leaf metadata
  }
  return {
    url: location.href,
    title: document.title,
    pathname: location.pathname,
    bodyTextLength: (document.body && document.body.innerText || "").length,
    sample: (document.body && document.body.innerText || "").replace(/\s+/g, " ").trim().slice(0, 500),
    loginSignals: {
      hasAvatar: !!document.querySelector("img[class*='avatar'], .avatar, [class*='user'] img"),
      hasLoginWord: /请登录|登录/.test((document.body && document.body.innerText || "").slice(0, 2000)),
      passwordInput: !!document.querySelector("input[type='password']"),
      memberText: /会员|个人中心|退出/.test(document.body && document.body.innerText || "")
    },
    probes,
    relevantClasses: [...classes].slice(0, 80),
    inputs,
    ipLeafCount: ipLeaves.length,
    ipLeaves: ipLeaves.slice(0, 8),
    firstRow,
    empty: (document.querySelector(".ant-empty-description") || {}).textContent || "",
    summary: (document.querySelector("[class*='StyleSummary'], .search-result-header") || {}).textContent || ""
  };
}

async function fillDayDayMapQuery(tabId, query) {
  await chrome.tabs.update(tabId, { url: "https://www.daydaymap.com/home", active: true });
  await waitReady(tabId, 12000);
  await new Promise((r) => setTimeout(r, 1500));
  const filled = await chrome.scripting.executeScript({
    target: { tabId },
    func: (nativeQuery) => {
      const nodes = Array.from(document.querySelectorAll("input, textarea"));
      const input = nodes.find((el) => {
        const ph = (el.getAttribute("placeholder") || "").toLowerCase();
        return ph.includes("search") || ph.includes("检索") || ph.includes("语法") || ph.includes("关键词") || ph.includes("ip=");
      }) || nodes.find((el) => el.type === "text" || el.tagName === "TEXTAREA");
      if (!input) return { ok: false, reason: "search_input_missing", placeholders: nodes.map((el) => el.getAttribute("placeholder") || "") };
      input.focus();
      const proto = input.tagName === "TEXTAREA" ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      const desc = Object.getOwnPropertyDescriptor(proto, "value");
      if (desc && desc.set) desc.set.call(input, nativeQuery);
      else input.value = nativeQuery;
      input.dispatchEvent(new InputEvent("input", { bubbles: true, composed: true, data: nativeQuery, inputType: "insertFromPaste" }));
      input.dispatchEvent(new Event("change", { bubbles: true }));
      input.dispatchEvent(new KeyboardEvent("keyup", { key: "a", bubbles: true }));
      return { ok: true, value: input.value, tag: input.tagName, placeholder: input.getAttribute("placeholder") || "", className: String(input.className || "") };
    },
    args: [query]
  });
  const fill = filled?.[0]?.result;
  if (!fill?.ok) throw new Error("daydaymap_search_prepare_failed:" + (fill?.reason || "unknown"));
  await new Promise((r) => setTimeout(r, 800));
  await chrome.scripting.executeScript({
    target: { tabId },
    func: () => {
      const btn = document.querySelector(".search-btn, button.search-btn, [class*='search-btn']");
      if (btn) btn.click();
      else {
        const input = document.querySelector("input, textarea");
        if (input) {
          input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true }));
          input.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter", code: "Enter", bubbles: true }));
        }
      }
    }
  });
  return fill;
}

export async function runLiveInspectDayDayMap() {
  const report = { started_at: new Date().toISOString(), pass: "ddm", extension: chrome.runtime.getManifest() };
  const log = [];
  const say = (msg) => { log.push(msg); setStatus(log); };
  say("DayDayMap inspect start");
  const tabs = await chrome.tabs.query({});
  const tab = tabs.find((t) => detectEngine(t.url) === "daydaymap");
  if (!tab) {
    report.error = "no_daydaymap_tab";
    say("no daydaymap tab");
  } else {
    report.tab = { id: tab.id, url: sanitizeURL(tab.url), title: tab.title };
    report.cookies = await checkLoginCookies(tab.url);
    say("dump current " + tab.url);
    const currentDump = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: dumpDayDayMapPage
    });
    report.current = currentDump?.[0]?.result;
    try { report.currentExtract = summarizeExtract(await extractEngineAssets(tab.id)); } catch (err) { report.currentExtract = { error: String(err) }; }
    const hasRows = (report.current?.ipLeafCount || 0) > 0 || (report.currentExtract?.itemCount || 0) > 0;
    say("current items=" + (report.currentExtract?.itemCount || 0) + " ipLeaves=" + (report.current?.ipLeafCount || 0) + " loginCookies=" + report.cookies.has_login_cookies);
    if (!hasRows) {
      say("no rows, fill query from home");
      try {
        report.fill = await fillDayDayMapQuery(tab.id, 'port="80"');
        await waitReady(tab.id, 15000);
        await waitForRows(tab.id, [
          "[class*='result-item']", ".ant-table-row", "[role='row']",
          ".rc-virtual-list-holder-inner > div", "table tbody tr"
        ], 20000);
        await new Promise((r) => setTimeout(r, 3000));
        const afterDump = await chrome.scripting.executeScript({
          target: { tabId: tab.id },
          func: dumpDayDayMapPage
        });
        report.after = afterDump?.[0]?.result;
        report.afterExtract = summarizeExtract(await extractEngineAssets(tab.id));
        const afterTab = await chrome.tabs.get(tab.id);
        report.afterTab = { url: sanitizeURL(afterTab.url), title: afterTab.title };
        say("after items=" + (report.afterExtract?.itemCount || 0) + " ipLeaves=" + (report.after?.ipLeafCount || 0));
      } catch (err) {
        report.searchError = String(err);
        say("search error " + String(err));
      }
    }
  }
  report.finished_at = new Date().toISOString();
  try {
    const resp = await fetch(REPORT_URL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(report)
    });
    say("report post " + resp.status);
  } catch (err) {
    say("report post failed " + String(err));
    await chrome.storage.local.set({ liveInspectReportDdm: report });
  }
  return report;
}

export async function runLiveInspect() {
  const report = {
    started_at: new Date().toISOString(),
    extension: chrome.runtime.getManifest(),
    phases: {}
  };
  const log = [];
  const say = (msg) => { log.push(msg); setStatus(log); };

  say("Live inspect started");
  const tabs = await chrome.tabs.query({});
  const engineTabs = tabs.filter((t) => detectEngine(t.url) !== "unknown");
  report.all_tab_titles = tabs.map((t) => ({ id: t.id, title: t.title || "", url: sanitizeURL(t.url || ""), engine: detectEngine(t.url) }));
  say(`tabs=${tabs.length} engine_tabs=${engineTabs.length}`);

  report.phases.current = [];
  for (const tab of engineTabs) {
    say(`inspect current ${detectEngine(tab.url)} #${tab.id}`);
    report.phases.current.push(await inspectTab(tab));
  }

  report.phases.after_search = [];
  for (const tab of engineTabs) {
    const engine = detectEngine(tab.url);
    const current = report.phases.current.find((row) => row.tabId === tab.id);
    const itemCount = current?.extract?.itemCount || 0;
    const alreadyResult = looksLikeResultURL(engine, tab.url) && itemCount > 0;
    if (alreadyResult) {
      say(`${engine}: already has ${itemCount} items, skip navigation`);
      report.phases.after_search.push({ tabId: tab.id, engine, skipped: true, reason: "already_extracted" });
      continue;
    }
    say(`${engine}: navigate to search`);
    try {
      if (engine === "daydaymap") {
        await prepareDayDayMapSearch(tab.id, "port=80");
      } else {
        const target = searchURLFor(engine, tab.url);
        await chrome.tabs.update(tab.id, { url: target, active: false });
      }
      await waitReady(tab.id, SPA_WAIT_MS);
      await new Promise((r) => setTimeout(r, engine === "censys" || engine === "zoomeye" ? 6000 : 4000));
      const inspected = await inspectTab(tab);
      report.phases.after_search.push(inspected);
      say(`${engine}: items=${inspected.extract?.itemCount || 0} login_wall=${inspected.extract?.is_login_wall || false}`);
    } catch (err) {
      report.phases.after_search.push({ tabId: tab.id, engine, error: String(err) });
      say(`${engine}: ERROR ${String(err)}`);
    }
  }

  report.finished_at = new Date().toISOString();
  let posted = false;
  try {
    const resp = await fetch(REPORT_URL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(report)
    });
    posted = resp.ok;
    say(`report post ${resp.status}`);
  } catch (err) {
    say(`report post failed: ${String(err)}`);
  }
  if (!posted) {
    await chrome.storage.local.set({ liveInspectReport: report });
    say("report saved to chrome.storage.local.liveInspectReport");
  }
  return report;
}
