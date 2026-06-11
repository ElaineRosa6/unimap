/**
 * Quick test script for extension data extraction.
 * Paste this into the browser DevTools Console on any search engine results page.
 * Works with: FOFA, Hunter, ZoomEye, Quake
 */
(() => {
  // === Copy of ENGINE_SELECTORS from capture.js ===
  const ENGINE_SELECTORS = {
    fofa: {
      row: [
        ".result-card", "[class*='result-card']", "[class*='result-item']", ".result-item",
        ".list_content > div", "[class*='list'] > [class*='item']",
        ".list_content > tbody > tr", ".result-table tbody tr",
        "[class*='result'] table tbody tr", "table[class*='list'] tbody tr",
        "[class*='result'] > div", "[class*='asset']"
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
        ".el-table tbody tr"
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
        "div[class*='search-result-item']", ".result-list > .item",
        ".search-result-item", "[class*='result-item']"
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
        ".result-list > .result-row", ".result-row",
        "[class*='result-list'] > [class*='row']", "[class*='result-row']"
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

  // === Helper functions ===
  function detectEngine(url) {
    if (!url) return "unknown";
    const lower = url.toLowerCase();
    if (lower.includes("fofa.info")) return "fofa";
    if (lower.includes("hunter.qianxin.com")) return "hunter";
    if (lower.includes("zoomeye.org") || lower.includes("zoomeye.com")) return "zoomeye";
    if (lower.includes("quake.360.net") || lower.includes("quake.360.cn")) return "quake";
    return "unknown";
  }

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

  function extractCellTextFromCells(cells, cfg) {
    if (cfg.selector.includes("td:nth-child")) {
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
    }
    return "";
  }

  function isLoginWall(doc) {
    const text = doc.body.textContent.toLowerCase();
    const loginKeywords = ["请登录", "请先登录", "login required", "sign in to continue", "please log in"];
    if (text.length > 5000) return false;
    return loginKeywords.some(kw => text.includes(kw));
  }

  function cardBasedExtraction() {
    const cardItems = [];
    const allLinks = Array.from(document.querySelectorAll("a"));
    const ipLinks = allLinks.filter(a => {
      const href = a.href || "";
      const text = a.textContent.trim();
      return /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(text) ||
             href.includes("qbase64=aXA9") || href.includes("ip=");
    });

    if (ipLinks.length === 0) {
      // Try finding card containers
      const cardSelectors = ["[class*='result-card']", "[class*='result-item']", "[class*='asset-item']", ".result-list > div", ".list_content > div"];
      for (const sel of cardSelectors) {
        const cards = document.querySelectorAll(sel);
        if (cards.length > 0) {
          cards.forEach((card) => {
            const item = {};
            const links = Array.from(card.querySelectorAll("a"));
            for (const link of links) {
              const text = link.textContent.trim();
              if (/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(text) && !item.ip) item.ip = text;
              else if (/^\d{1,5}$/.test(text) && !item.port) item.port = text;
              else if (text.length > 3 && text.length < 100 && !item.title) item.title = text;
            }
            if (Object.keys(item).length > 0) cardItems.push(item);
          });
          break;
        }
      }
    } else {
      for (const ipLink of ipLinks.slice(0, 100)) {
        const item = {};
        const ipText = ipLink.textContent.trim();
        if (/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(ipText)) item.ip = ipText;
        const parent = ipLink.closest("div, li, tr, section, article");
        if (parent) {
          const siblings = Array.from(parent.querySelectorAll("a, span, div"));
          for (const sib of siblings) {
            const text = sib.textContent.trim();
            if (!text || text === ipText) continue;
            if (/^\d{1,5}$/.test(text) && !item.port) item.port = text;
            else if (/^[a-zA-Z][\w-]*$/.test(text) && text.length < 30 && !item.protocol) item.protocol = text;
            else if (text.length > 3 && text.length < 100 && !item.title) item.title = text;
          }
        }
        if (Object.keys(item).length > 0) cardItems.push(item);
      }
    }
    return cardItems;
  }

  // === Main extraction ===
  const engine = detectEngine(location.href);
  console.log(`\n=== ${engine.toUpperCase()} Extraction Test ===`);
  console.log(`URL: ${location.href}`);
  console.log(`Title: ${document.title}`);

  // Check login wall
  if (isLoginWall(document)) {
    console.log("WARNING: Login wall detected!");
    return { engine, is_login_wall: true };
  }

  const sel = ENGINE_SELECTORS[engine];
  if (!sel) {
    console.log(`ERROR: Unsupported engine: ${engine}`);
    return { engine, error: "unsupported" };
  }

  // Try row selectors
  let rows = [];
  let rowSelectorUsed = "none";
  for (const rowSel of sel.row) {
    try {
      rows = document.querySelectorAll(rowSel);
      if (rows.length > 0) {
        rowSelectorUsed = rowSel;
        break;
      }
    } catch(e) {}
  }

  console.log(`\nRow selector: "${rowSelectorUsed}" (${rows.length} matches)`);

  let items = [];

  if (rows.length > 0) {
    // Extract from rows/cards
    rows.forEach((row) => {
      const cells = row.querySelectorAll("td");
      const item = {};
      const cellConfig = sel.cells;

      if (cells.length > 0) {
        Object.keys(cellConfig).forEach((key) => {
          item[key] = extractCellTextFromCells(cells, cellConfig[key]);
        });
      } else {
        Object.keys(cellConfig).forEach((key) => {
          item[key] = extractCellText(row, cellConfig[key]);
        });
      }

      const hasAnyValue = Object.values(item).some(v => v !== "" && v !== 0);
      if (hasAnyValue) items.push(item);
    });
    console.log(`Extracted ${items.length} items from rows/cards`);
  }

  // Fallback to card-based extraction if no items
  if (items.length === 0) {
    console.log("No items from selectors, trying card-based extraction...");
    items = cardBasedExtraction();
    console.log(`Card-based extraction: ${items.length} items`);
  }

  // Extract total count
  const totalEl = queryOne(document, sel.total);
  let total = 0;
  if (totalEl) {
    const text = totalEl.textContent.trim().replace(/[^0-9]/g, "");
    total = parseInt(text, 10) || 0;
    console.log(`Total: ${total} (selector: "${sel.total.find(s => document.querySelector(s))}")`);
  }

  // Extract has_more
  const nextEl = queryOne(document, sel.nextPage);
  const hasMore = !!nextEl && !nextEl.classList.contains("disabled");
  console.log(`Has more: ${hasMore}`);

  // Show first item
  if (items.length > 0) {
    console.log(`\nFirst item:`);
    console.table([items[0]]);
    if (items.length > 1) {
      console.log(`\nSecond item:`);
      console.table([items[1]]);
    }
  }

  const result = {
    engine,
    url: location.href,
    title: document.title,
    items,
    total,
    has_more: hasMore,
    is_login_wall: false,
    row_selector_used: rowSelectorUsed,
    extraction_method: items.length > 0 ? (rows.length > 0 ? "row_based" : "card_based") : "none"
  };

  console.log(`\n=== Summary ===`);
  console.log(JSON.stringify({
    engine: result.engine,
    items_count: result.items.length,
    total: result.total,
    has_more: result.has_more,
    extraction_method: result.extraction_method,
    row_selector_used: result.row_selector_used
  }, null, 2));

  return result;
})()
