// DOM Inspector for Search Engine Result Pages
// Run this in the Chrome DevTools Console or via MCP chrome_javascript
// Outputs the actual structure and suggested selectors

(() => {
  const results = {
    engine: 'unknown',
    url: location.href,
    title: document.title,
    items: [],
    suggestedSelectors: {},
    debug: {}
  };

  // Detect engine
  const lower = location.href.toLowerCase();
  if (lower.includes('fofa.info')) results.engine = 'fofa';
  else if (lower.includes('hunter.qianxin.com')) results.engine = 'hunter';
  else if (lower.includes('zoomeye.org') || lower.includes('zoomeye.com')) results.engine = 'zoomeye';
  else if (lower.includes('quake.360.net') || lower.includes('quake.360.cn')) results.engine = 'quake';

  // Find all classes containing result/list/table/row/card/item
  const allClasses = new Set();
  document.querySelectorAll('[class]').forEach(el => {
    const cls = el.className;
    if (typeof cls === 'string') {
      cls.split(/\s+/).forEach(c => {
        if (/result|list|table|row|data|cell|card|item|col|asset|ip|port|protocol|host|title|banner|count|total|page/i.test(c)) {
          allClasses.add(c);
        }
      });
    }
  });
  results.debug.allRelevantClasses = [...allClasses];

  // Find table-like structures
  const tables = document.querySelectorAll('table');
  results.debug.tableCount = tables.length;
  results.debug.tables = Array.from(tables).map((t, i) => ({
    class: t.className,
    rows: t.querySelectorAll('tr').length,
    headerTexts: Array.from(t.querySelectorAll('th, thead td')).map(th => th.textContent.trim().substring(0, 50))
  }));

  // Test all possible row selectors
  const rowSelectors = [
    '.list_content > tbody > tr',
    '.list_content tr',
    '.result-table tbody tr',
    '[class*="result"] table tbody tr',
    'table[class*="list"] tbody tr',
    'table tbody tr',
    '.result-list > .result-item',
    '.result-item',
    '[class*="result-list"] > [class*="item"]',
    'div[class*="result-item"]',
    '.el-table tbody tr',
    '.data-list tr',
    '.result-list > .result-row',
    '.result-row',
    '[class*="result-list"] > [class*="row"]',
    '[class*="result-row"]',
    '.search-result-item',
    'div[class*="search-result-item"]',
    '.result-list .item',
    '[class*="asset"]',
    '[class*="result"] [class*="item"]',
    '[class*="result"] [class*="row"]',
    '[class*="result"] > div',
    '.asset-item',
    '[data-ip]'
  ];

  const selectorCounts = {};
  for (const sel of rowSelectors) {
    try {
      const count = document.querySelectorAll(sel).length;
      if (count > 0) selectorCounts[sel] = count;
    } catch(e) {}
  }
  results.debug.selectorCounts = selectorCounts;

  // If we found a matching selector, extract first item structure
  for (const [sel, count] of Object.entries(selectorCounts)) {
    const el = document.querySelector(sel);
    if (el) {
      results.suggestedSelectors.row = sel;
      results.debug.firstRowHTML = el.outerHTML.substring(0, 1000);
      results.debug.rowText = el.textContent.substring(0, 300);

      // Try to identify cells
      const cells = el.querySelectorAll('td');
      if (cells.length > 0) {
        results.debug.tdCount = cells.length;
        results.debug.tdTexts = Array.from(cells).slice(0, 7).map(c => c.textContent.trim().substring(0, 80));
      } else {
        // Card-based: find child divs
        const children = Array.from(el.children);
        results.debug.childCount = children.length;
        results.debug.children = children.slice(0, 10).map(c => ({
          tag: c.tagName.toLowerCase(),
          class: c.className,
          text: c.textContent.trim().substring(0, 80)
        }));
      }
      break;
    }
  }

  // Total selector
  const totalSelectors = ['.total-count', '.total_count', '[class*="total"]', '[class*="count"]', '.pagination-info'];
  for (const sel of totalSelectors) {
    const el = document.querySelector(sel);
    if (el) {
      results.suggestedSelectors.total = sel;
      results.debug.totalText = el.textContent.trim().substring(0, 100);
      break;
    }
  }

  // Next page selector
  const nextSelectors = ['.next', '.next-page', '[class*="next"]', '.el-pagination__next', '.pagination-next'];
  for (const sel of nextSelectors) {
    const el = document.querySelector(sel);
    if (el) {
      results.suggestedSelectors.nextPage = sel;
      break;
    }
  }

  // Login wall check
  const bodyText = document.body.textContent.toLowerCase();
  const loginKeywords = ['请登录', '请先登录', 'login required', 'sign in to continue', 'please log in'];
  results.isLoginWall = bodyText.length < 5000 && loginKeywords.some(kw => bodyText.includes(kw));

  // Body text length
  results.debug.bodyTextLength = document.body.textContent.length;

  return results;
})()
