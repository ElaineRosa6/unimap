const { chromium } = require('playwright');

// Extension's extraction script (exact copy from capture.js)
const EXTRACT_SCRIPT = `(() => {
  function detectEngine(url) {
    if (!url) return "unknown";
    const lower = url.toLowerCase();
    if (lower.includes("fofa.info")) return "fofa";
    if (lower.includes("hunter.qianxin.com")) return "hunter";
    if (lower.includes("zoomeye.org")) return "zoomeye";
    if (lower.includes("quake.360.net")) return "quake";
    return "unknown";
  }

  const ENGINE_SELECTORS = {
    fofa: {
      row: [".list_content > tbody > tr", ".result-table tbody tr", "[class*='result'] table tbody tr"],
      cells: {
        ip: { selector: "td:nth-child(1) a", fallback: "td:nth-child(1)" },
        port: { selector: "td:nth-child(2)" },
        protocol: { selector: "td:nth-child(3)" },
        host: { selector: "td:nth-child(4) a", fallback: "td:nth-child(4)" },
        title: { selector: "td:nth-child(5)" },
        country_code: { selector: "td:nth-child(6)" },
        banner: { selector: "td:nth-child(7)" }
      },
      total: [".total-count", ".total_count"],
      nextPage: [".next", ".next-page"]
    },
    hunter: {
      row: [".result-list > .result-item", ".result-item", "[class*='result-list'] > [class*='item']"],
      cells: {
        ip: { selector: ".ip-address, [data-ip], [class*='ip']" },
        port: { selector: ".port, [data-port], [class*='port']" },
        protocol: { selector: ".protocol, .service, [data-protocol]" },
        host: { selector: ".domain, .hostname" },
        title: { selector: ".web-title, .title" },
        banner: { selector: ".header-info, .banner" }
      },
      total: [".total-count", ".total"],
      nextPage: [".next", ".el-pagination__next"]
    },
    zoomeye: {
      row: ["div[class*='search-result-item']", ".result-list > .item", ".search-result-item"],
      cells: {
        ip: { selector: ".ip, [data-ip]" },
        port: { selector: ".port, [data-port]" },
        protocol: { selector: ".service, .protocol" },
        host: { selector: ".domain" },
        title: { selector: ".title" },
        country_code: { selector: ".location" },
        banner: { selector: ".banner" }
      },
      total: [".total"],
      nextPage: [".next", ".pagination-next"]
    },
    quake: {
      row: [".result-list > .result-row", ".result-row"],
      cells: {
        ip: { selector: ".ip" },
        port: { selector: ".port" },
        protocol: { selector: ".transport, .protocol" },
        host: { selector: ".hostname" },
        title: { selector: ".title" },
        server: { selector: ".server" },
        city: { selector: ".city" },
        isp: { selector: ".isp" }
      },
      total: [".total-count", ".total"],
      nextPage: [".next", ".next-page"]
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
    result.page_text_preview = document.body.textContent.substring(0, 500);
    return result;
  }

  let rows = [];
  for (const rowSel of sel.row) {
    rows = document.querySelectorAll(rowSel);
    if (rows.length > 0) break;
  }

  result.row_selector_used = sel.row.find(s => document.querySelectorAll(s).length > 0) || "none";
  result.row_count = rows.length;

  if (rows.length === 0) {
    result.error = "no_rows_found";
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

async function testEngine(engine, query) {
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
  }

  console.log(`\n${'='.repeat(60)}`);
  console.log(`Testing ${engine.toUpperCase()}`);
  console.log(`Query: ${query}`);
  console.log(`URL: ${searchUrl}`);

  const browser = await chromium.launch({ headless: false });
  const page = await browser.newPage();

  try {
    await page.goto(searchUrl, { waitUntil: 'domcontentloaded', timeout: 30000 });
    console.log('Page loaded (DOMContentLoaded), waiting 5s for SPA rendering...');
    await page.waitForTimeout(5000);

    const result = await page.evaluate(EXTRACT_SCRIPT);
    console.log(`\nEngine: ${result.engine}`);
    console.log(`Title: ${result.title}`);
    console.log(`Login Wall: ${result.is_login_wall}`);
    console.log(`Row Selector Used: ${result.row_selector_used || 'N/A'}`);
    console.log(`Rows Found: ${result.row_count || 0}`);
    console.log(`Items Extracted: ${result.items.length}`);
    console.log(`Total: ${result.total}`);
    console.log(`Has More: ${result.has_more}`);

    if (result.error) {
      console.log(`\n⚠️ Error: ${result.error}`);
      if (result.page_text_preview) {
        console.log(`\nPage text preview (first 300 chars):`);
        console.log(result.page_text_preview.substring(0, 300));
      }
    }

    if (result.items.length > 0) {
      console.log(`\n✅ First item:`);
      console.log(JSON.stringify(result.items[0], null, 2));
    } else if (!result.is_login_wall) {
      console.log(`\n❌ No items extracted - selectors may not match current DOM!`);
    }
  } catch (err) {
    console.error(`Error: ${err.message}`);
  } finally {
    await browser.close();
  }
}

async function main() {
  const engine = process.argv[2] || 'fofa';
  const query = process.argv[3] || 'protocol="http"';
  await testEngine(engine, query);
}

main().catch(err => {
  console.error('Fatal error:', err.message);
  process.exit(1);
});
