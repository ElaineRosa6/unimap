const puppeteer = require('puppeteer-core');

// Extension's extractEngineAssets logic (injected into page)
const EXTRACT_SCRIPT = `(() => {
  // Detect engine from URL
  function detectEngine(url) {
    if (!url) return "unknown";
    const lower = url.toLowerCase();
    if (lower.includes("fofa.info")) return "fofa";
    if (lower.includes("hunter.qianxin.com")) return "hunter";
    if (lower.includes("zoomeye.org")) return "zoomeye";
    if (lower.includes("quake.360.net") || lower.includes("quake.360.net")) return "quake";
    return "unknown";
  }

  // Selectors per engine (updated for card-based layouts)
  const ENGINE_SELECTORS = {
    fofa: {
      row: [
        ".result-card", "[class*='result-card']", "[class*='result-item']", ".result-item",
        ".list_content > div", "[class*='list'] > [class*='item']",
        ".list_content > tbody > tr", ".result-table tbody tr",
        "[class*='result'] table tbody tr", "table[class*='list'] tbody tr",
        "[class*='result'] > div"
      ],
      cells: {
        ip: { selector: "[class*='ip'] a, a[href*='qbase64=aXA9']", fallback: "td:nth-child(1) a" },
        port: { selector: "[class*='port'] a, a[href*='qbase64=cG9ydD0']", fallback: "td:nth-child(2)" },
        protocol: { selector: "[class*='protocol'] a, a[href*='qbase64=cHJvdG9jb2w9']", fallback: "td:nth-child(3)" },
        host: { selector: "[class*='host'] a, [class*='domain'] a", fallback: "td:nth-child(4) a" },
        title: { selector: "[class*='title'], [class*='name']", fallback: "td:nth-child(5)" },
        country_code: { selector: "[class*='country'] a, a[href*='qbase64=Y291bnRyeT0']", fallback: "td:nth-child(6)" },
        banner: { selector: "[class*='banner'], [class*='header']", fallback: "td:nth-child(7)" }
      },
      total: [".total-count", ".total_count", "[class*='total']", "[class*='count']", ".pagination-info"],
      nextPage: [".next", ".next-page", "[class*='next']", ".el-pagination__next"]
    },
    hunter: {
      row: [
        ".result-list > .result-item", ".result-item",
        "[class*='result-list'] > [class*='item']", "div[class*='result-item']",
        ".el-table tbody tr", "[class*='result-list'] > div"
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
    quake: {
      row: [
        ".result-list > .result-row", ".result-row",
        "[class*='result-list'] > [class*='row']", "[class*='result-row']",
        ".el-table tbody tr", "[class*='result-list'] > div"
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
    },
    zoomeye: {
      row: [
        "div[class*='search-result-item']", ".result-list > .item",
        ".search-result-item", "[class*='result-item']",
        "[class*='result-list'] > div"
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
    }
  };

  function queryOne(root, selectors) {
    const list = Array.isArray(selectors) ? selectors : [selectors];
    for (const sel of list) {
      const el = root.querySelector(sel);
      if (el) return el;
    }
    return null;
  }

  function extractCellText(row, cellConfig) {
    const el = row.querySelector(cellConfig.selector);
    if (el) return el.textContent.trim();
    if (cellConfig.fallback) {
      const fb = row.querySelector(cellConfig.fallback);
      if (fb) return fb.textContent.trim();
    }
    return "";
  }

  function isLoginWall(doc) {
    const text = doc.body.textContent.toLowerCase();
    if (text.length > 5000) return false;
    const keywords = ["请登录", "请先登录", "login required", "sign in to continue", "please log in"];
    return keywords.some(kw => text.includes(kw));
  }

  const engine = detectEngine(location.href);
  const result = { engine, url: location.href, title: document.title, items: [], total: 0, has_more: false, is_login_wall: false };

  if (isLoginWall(document)) {
    result.is_login_wall = true;
    return result;
  }

  const sel = ENGINE_SELECTORS[engine];
  if (!sel) {
    result.error = "unsupported_engine: " + engine;
    return result;
  }

  // Try each row selector
  let rows = [];
  for (const rowSel of sel.row) {
    rows = document.querySelectorAll(rowSel);
    if (rows.length > 0) break;
  }

  result.row_selector_used = sel.row.find(s => document.querySelectorAll(s).length > 0) || "none";
  result.row_count = rows.length;

  if (rows.length === 0) {
    result.error = "no_rows_found";
    // Dump some page info for debugging
    result.page_text_preview = document.body.textContent.substring(0, 500);
    return result;
  }

  rows.forEach((row) => {
    const cells = row.querySelectorAll("td");
    const item = {};
    if (cells.length > 0) {
      Object.keys(sel.cells).forEach((key) => {
        const cfg = sel.cells[key];
        if (cfg.selector.includes && cfg.selector.includes("td:nth-child")) {
          const match = cfg.selector.match(/td:nth-child\\((\\d+)\\)/);
          if (match) {
            const idx = parseInt(match[1], 10) - 1;
            if (idx >= 0 && idx < cells.length) {
              const target = cells[idx];
              if (cfg.selector.includes(" a")) {
                const a = target.querySelector("a");
                item[key] = a ? a.textContent.trim() : target.textContent.trim();
              } else {
                item[key] = target.textContent.trim();
              }
              return;
            }
          }
        }
        item[key] = extractCellText(row, cfg);
      });
    } else {
      Object.keys(sel.cells).forEach((key) => {
        item[key] = extractCellText(row, sel.cells[key]);
      });
    }
    const hasAnyValue = Object.values(item).some(v => v !== "" && v !== 0);
    if (hasAnyValue) result.items.push(item);
  });

  const totalEl = queryOne(document, sel.total);
  if (totalEl) {
    const text = totalEl.textContent.trim().replace(/[^0-9]/g, "");
    result.total = parseInt(text, 10) || 0;
  }

  const nextEl = queryOne(document, sel.nextPage);
  result.has_more = !!nextEl;

  return result;
})();`;

async function main() {
  const browser = await puppeteer.connect({
    browserURL: 'http://127.0.0.1:9222',
    defaultViewport: null
  });

  const pages = await browser.pages();
  console.log(`Connected to Chrome with ${pages.length} tabs\n`);

  // Show all pages
  for (const page of pages) {
    const title = await page.title();
    const url = await page.url();
    console.log(`Tab: "${title}"`);
    console.log(`URL: ${url}`);
    console.log('---');
  }

  // Ask which engine to test
  const engine = process.argv[2] || 'fofa';
  const query = process.argv[3] || 'protocol="http"';

  // Build search URL
  const { URLSearchParams } = require('url');
  let searchUrl;
  switch (engine) {
    case 'fofa':
      searchUrl = `https://fofa.info/result?qbase64=${Buffer.from(query).toString('base64')}`;
      break;
    case 'hunter':
      searchUrl = `https://hunter.qianxin.com/list?searchValue=${encodeURIComponent(Buffer.from(query).toString('base64'))}`;
      break;
    case 'quake':
      searchUrl = `https://quake.360.net/quake/#/searchResult?searchVal=${encodeURIComponent(query)}`;
      break;
    case 'zoomeye':
      searchUrl = `https://www.zoomeye.org/searchResult?q=${encodeURIComponent(query)}`;
      break;
    default:
      console.log(`Unknown engine: ${engine}`);
      await browser.disconnect();
      return;
  }

  console.log(`\n=== Testing ${engine.toUpperCase()} ===`);
  console.log(`Query: ${query}`);
  console.log(`URL: ${searchUrl}\n`);

  // Find existing page or create new one
  let page = pages.find(p => p.url().includes(engine)) || pages[0];
  if (!page) {
    page = await browser.newPage();
  }

  await page.goto(searchUrl, { waitUntil: 'networkidle2', timeout: 30000 });
  console.log('Page loaded, waiting 3s for SPA rendering...');
  await new Promise(r => setTimeout(r, 3000));

  // Inject and run extraction
  const result = await page.evaluate(EXTRACT_SCRIPT);
  console.log('\n=== Extraction Result ===');
  console.log(JSON.stringify(result, null, 2));

  // Summary
  console.log('\n=== Summary ===');
  console.log(`Engine: ${result.engine}`);
  console.log(`URL: ${result.url}`);
  console.log(`Title: ${result.title}`);
  console.log(`Login Wall: ${result.is_login_wall}`);
  console.log(`Row Selector Used: ${result.row_selector_used || 'N/A'}`);
  console.log(`Rows Found: ${result.row_count || 0}`);
  console.log(`Items Extracted: ${result.items.length}`);
  console.log(`Total: ${result.total}`);
  console.log(`Has More: ${result.has_more}`);
  if (result.error) console.log(`Error: ${result.error}`);
  if (result.items.length > 0) {
    console.log(`\nFirst item: ${JSON.stringify(result.items[0], null, 2)}`);
  }
  if (result.items.length === 0 && !result.is_login_wall) {
    console.log('\n⚠️ No items extracted! DOM selectors may need updating.');
    if (result.page_text_preview) {
      console.log(`Page text preview: ${result.page_text_preview.substring(0, 300)}`);
    }
  }

  await browser.disconnect();
}

main().catch(err => {
  console.error('Error:', err.message);
  process.exit(1);
});
