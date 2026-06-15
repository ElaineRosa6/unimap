package screenshot

// engineSelectors defines CSS selectors for extracting structured asset data
// from a search engine's web results page.
type engineSelectors struct {
	RowSelector    string // CSS selector for a single result row
	ExtractJS      string // JavaScript snippet to run on the page for data extraction
	PaginationNext string // CSS selector for the "next page" button
	TotalSelector  string // CSS selector for the total results count indicator
}

// selectorsByEngine maps engine names to their DOM extraction selectors.
// These are best-effort based on current page structures and may need
// adjustment when engine frontends change.
var selectorsByEngine = map[string]*engineSelectors{
	"fofa": {
		RowSelector:    "table.el-table__body tr.el-table__row",
		ExtractJS:      extractFofaJS,
		PaginationNext: "button.btn-next",
		TotalSelector:  ".search-list-head span:first-child",
	},
	"hunter": {
		RowSelector:    "table.table tbody tr",
		ExtractJS:      extractHunterJS,
		PaginationNext: "li.next a",
		TotalSelector:  ".pull-left.m-b span",
	},
	"zoomeye": {
		RowSelector:    "div.search-result-item-container",
		ExtractJS:      extractZoomEyeJS,
		PaginationNext: "li.ant-pagination-next:not(.ant-pagination-disabled) a",
		TotalSelector:  "li.ant-pagination-total-text span",
	},
	"quake": {
		RowSelector:    "table tbody tr",
		ExtractJS:      extractQuakeJS,
		PaginationNext: "li.ant-pagination-next button",
		TotalSelector:  ".total-count",
	},
	"shodan": {
		RowSelector:    ".row.l-search-results .result",
		ExtractJS:      extractShodanJS,
		PaginationNext: ".pagination .next a",
		TotalSelector:  ".result-count",
	},
	"censys": {
		RowSelector:    "[class*='result-card']",
		PaginationNext: "[class*='next']",
		TotalSelector:  "[class*='total']",
	},
	"daydaymap": {
		RowSelector:    "[class*='result-item']",
		PaginationNext: ".el-pagination__next",
		TotalSelector:  "[class*='total']",
	},
	"onyphe": {
		RowSelector:    "table tbody tr",
		PaginationNext: "[class*='next']",
		TotalSelector:  "[class*='total']",
	},
	"greynoise": {
		RowSelector:    "table tbody tr",
		PaginationNext: "[class*='next']",
		TotalSelector:  "[class*='total']",
	},
}

func getSelectors(engine string) *engineSelectors {
	s, ok := selectorsByEngine[engine]
	if !ok {
		return nil
	}
	return s
}

// JavaScript snippets to extract structured data from each engine's result page.
// Each script returns a JSON string: {"assets":[{ip,port,protocol,...}],"total":N,"hasMore":bool}

const extractFofaJS = `
(function() {
  var rows = document.querySelectorAll('table.el-table__body tr.el-table__row');
  var assets = [];
  rows.forEach(function(row) {
    var cells = row.querySelectorAll('td');
    if (cells.length < 6) return;
    var asset = {};
    var ipCell = cells[0].textContent.trim();
    // FOFA column: "IP:Port" in first cell
    var parts = ipCell.split(':');
    asset.ip = parts[0] || '';
    asset.port = parseInt(parts[1]) || 0;
    asset.protocol = cells[1] ? cells[1].textContent.trim() : '';
    asset.host = cells[2] ? cells[2].textContent.trim() : '';
    asset.title = cells[3] ? cells[3].textContent.trim() : '';
    asset.country = cells[4] ? cells[4].textContent.trim() : '';
    asset.server = cells[5] ? cells[5].textContent.trim() : '';
    asset.source = 'fofa';
    assets.push(asset);
  });
  var totalText = document.querySelector('.search-list-head span:first-child');
  var total = 0;
  if (totalText) {
    var m = totalText.textContent.match(/(\d+)/);
    if (m) total = parseInt(m[1]);
  }
  var hasNext = !!document.querySelector('button.btn-next:not([disabled])');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`

const extractHunterJS = `
(function() {
  var rows = document.querySelectorAll('table.table tbody tr');
  var assets = [];
  rows.forEach(function(row) {
    var cells = row.querySelectorAll('td');
    if (cells.length < 6) return;
    var asset = {};
    asset.ip = cells[0] ? cells[0].textContent.trim() : '';
    asset.port = parseInt(cells[1] ? cells[1].textContent.trim() : '0') || 0;
    asset.protocol = cells[2] ? cells[2].textContent.trim() : '';
    asset.host = cells[3] ? cells[3].textContent.trim() : '';
    asset.title = cells[4] ? cells[4].textContent.trim() : '';
    asset.country = cells[5] ? cells[5].textContent.trim() : '';
    asset.server = cells[6] ? cells[6].textContent.trim() : '';
    asset.source = 'hunter';
    assets.push(asset);
  });
  var totalText = document.querySelector('.pull-left.m-b span');
  var total = 0;
  if (totalText) {
    var m = totalText.textContent.match(/(\d+)/);
    if (m) total = parseInt(m[1]);
  }
  var hasNext = !!document.querySelector('li.next:not(.disabled) a');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`

const extractZoomEyeJS = `
(function() {
  var containers = document.querySelectorAll('div.search-result-item-container');
  var assets = [];
  containers.forEach(function(container) {
    var asset = {};
    // Extract IP from ip-detail-box > span._public-hover_uxlu6_1
    var ipEl = container.querySelector('span._public-hover_uxlu6_1');
    if (ipEl) {
      asset.ip = ipEl.textContent.trim();
    }
    // Extract host:port from header-bar > div.url-container
    var urlContainer = container.querySelector('div.url-container');
    if (urlContainer) {
      var urlText = urlContainer.textContent.trim();
      // Parse "domain:port" or "ip:port" format
      var match = urlText.match(/^(.+?):(\d+)$/);
      if (match) {
        if (!asset.ip || /\d+\.\d+\.\d+\.\d+/.test(match[1])) {
          asset.ip = match[1];
        }
        asset.host = /\d+\.\d+\.\d+\.\d+/.test(match[1]) ? '' : match[1];
        asset.port = parseInt(match[2]) || 0;
      } else {
        asset.host = urlText;
      }
    }
    // Extract port from protocol-port-box first button
    var portBtn = container.querySelector('div.protocol-port-box button:first-child');
    if (portBtn) {
      var portVal = parseInt(portBtn.textContent.trim());
      if (portVal > 0) asset.port = portVal;
    }
    // Extract protocol from protocol-port-box last button span
    var protoBtn = container.querySelector('div.protocol-port-box button:last-child span');
    if (protoBtn) {
      asset.protocol = protoBtn.textContent.trim();
    }
    // Extract banner from pre tab panel
    var preEl = container.querySelector('pre');
    if (preEl) {
      asset.banner = preEl.textContent.trim().substring(0, 500);
    }
    asset.source = 'zoomeye';
    if (asset.ip || asset.host) {
      assets.push(asset);
    }
  });
  // Get total from pagination
  var totalEl = document.querySelector('li.ant-pagination-total-text span');
  var total = 0;
  if (totalEl) {
    var m = totalEl.textContent.match(/[\d,]+/);
    if (m) total = parseInt(m[0].replace(/,/g, '')) || 0;
  }
  var hasNext = !!document.querySelector('li.ant-pagination-next:not(.ant-pagination-disabled) a');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`

const extractQuakeJS = `
(function() {
  var rows = document.querySelectorAll('table tbody tr');
  var assets = [];
  rows.forEach(function(row) {
    var cells = row.querySelectorAll('td');
    if (cells.length < 6) return;
    var asset = {};
    asset.ip = cells[0] ? cells[0].textContent.trim() : '';
    asset.port = parseInt(cells[1] ? cells[1].textContent.trim() : '0') || 0;
    asset.protocol = cells[2] ? cells[2].textContent.trim() : '';
    asset.host = cells[3] ? cells[3].textContent.trim() : '';
    asset.title = cells[4] ? cells[4].textContent.trim() : '';
    asset.country = cells[5] ? cells[5].textContent.trim() : '';
    asset.server = cells[6] ? cells[6].textContent.trim() : '';
    asset.source = 'quake';
    assets.push(asset);
  });
  var total = 0;
  var hasNext = !!document.querySelector('li.ant-pagination-next:not(.ant-pagination-disabled) button');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`

const extractShodanJS = `
(function() {
  var results = document.querySelectorAll('.row.l-search-results .result');
  var assets = [];
  results.forEach(function(el) {
    var asset = {};
    var ipLink = el.querySelector("a[href*='/host/']");
    if (ipLink) {
      var m = ipLink.getAttribute('href').match(/\\/host\\/([^/?#]+)/);
      if (m) asset.ip = m[1];
    }
    var portLink = el.querySelector("a[href*='/port/']");
    if (portLink) asset.port = parseInt(portLink.textContent.trim()) || 0;
    var heading = el.querySelector('.heading a, a.title');
    if (heading) asset.title = heading.textContent.trim();
    var details = el.querySelector('.result-details');
    if (details) asset.org = details.textContent.trim().split('\\n')[0];
    var banner = el.querySelector('.banner-data, pre');
    if (banner) asset.banner = banner.textContent.trim().substring(0, 200);
    asset.source = 'shodan';
    if (asset.ip) assets.push(asset);
  });
  var total = 0;
  var totalEl = document.querySelector('.result-count, [class*="total"]');
  if (totalEl) { var m = totalEl.textContent.match(/(\\d+)/); if (m) total = parseInt(m[1]); }
  var hasNext = !!document.querySelector('.pagination .next:not(.disabled) a, a[rel="next"]');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`
