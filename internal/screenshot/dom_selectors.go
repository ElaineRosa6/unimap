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
		RowSelector:    ".hsxa-meta-data-item",
		ExtractJS:      extractFofaJS,
		PaginationNext: "button.btn-next:not([disabled])",
		TotalSelector:  "[class*='total']",
	},
	"hunter": {
		RowSelector:    ".q-table tbody tr",
		ExtractJS:      extractHunterJS,
		PaginationNext: ".q-pagination button:last-child:not([disabled])",
		TotalSelector:  ".page-list-body_statistic",
	},
	"zoomeye": {
		RowSelector:    "div.search-result-item-container",
		ExtractJS:      extractZoomEyeJS,
		PaginationNext: "li.ant-pagination-next:not(.ant-pagination-disabled) a",
		TotalSelector:  "li.ant-pagination-total-text span",
	},
	"quake": {
		RowSelector:    ".item-container",
		ExtractJS:      extractQuakeJS,
		PaginationNext: ".el-pagination__next:not([disabled]) button",
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
  var rows = document.querySelectorAll('.hsxa-meta-data-item');
  var assets = [];
  rows.forEach(function(row) {
    var asset = {};
    // FOFA card layout: IP from .hsxa-host
    var ipEl = row.querySelector('.hsxa-host');
    if (ipEl) {
      var ipText = ipEl.textContent.trim();
      var parts = ipText.split(':');
      asset.ip = parts[0] || '';
      asset.port = parseInt(parts[1]) || 0;
    }
    // Port from qbase64 links
    var portLink = row.querySelector("a[href*='qbase64=cG9ydD0']");
    if (portLink) {
      var m = portLink.textContent.trim().match(/(\d+)/);
      if (m) asset.port = parseInt(m[1]);
    }
    // Protocol from qbase64 links
    var protoLinks = row.querySelectorAll("a[href*='qbase64=']");
    protoLinks.forEach(function(a) {
      var text = a.textContent.trim().toLowerCase();
      if (text === 'http' || text === 'https' || text === 'tcp') asset.protocol = text;
    });
    // Host, title, country, org from card fields
    var fields = row.querySelectorAll('.hsxa-meta-data-item__field, [class*="field"]');
    fields.forEach(function(f) {
      var label = (f.querySelector('[class*="label"]') || {}).textContent || '';
      var value = (f.querySelector('[class*="value"]') || f).textContent.trim();
      if (label.includes('域名') || label.includes('host')) asset.host = value;
      if (label.includes('标题') || label.includes('title')) asset.title = value;
      if (label.includes('国家') || label.includes('country')) asset.country = value;
      if (label.includes('组织') || label.includes('org')) asset.org = value;
      if (label.includes('Server') || label.includes('server')) asset.server = value;
    });
    asset.source = 'fofa';
    if (asset.ip || asset.host) assets.push(asset);
  });
  var totalEl = document.querySelector('[class*="total"]');
  var total = 0;
  if (totalEl) { var m = totalEl.textContent.match(/(\d[\d,]*)/); if (m) total = parseInt(m[0].replace(/,/g, '')); }
  var hasNext = !!document.querySelector('button.btn-next:not([disabled])');
  return JSON.stringify({assets: assets, total: total, hasMore: hasNext});
})()
`

const extractHunterJS = `
(function() {
  var rows = document.querySelectorAll('.q-table tbody tr');
  var assets = [];
  var seen = {};
  rows.forEach(function(row) {
    var cells = row.querySelectorAll('td');
    if (cells.length < 5) return;
    var asset = {};

    // Hunter Quasar UI columns: 0=checkbox, 1=序号, 2=IP, 3=域名, 4=端口/服务, 5=标题, 6=状态码, 7=ICP, 8=应用, 9=标签, 10=地区, 11=更新时间
    function getCellText(idx) {
      if (idx >= cells.length) return '';
      var cell = cells[idx];
      var cellDiv = cell.querySelector('.cell');
      var text = cellDiv ? cellDiv.textContent : cell.textContent;
      text = text.replace(/只看该[^\s]*不看该[^\s]*/g, '');
      text = text.replace(/只看空[^\s]*不看空[^\s]*/g, '');
      text = text.replace(/看相似(网站|icon)/g, '');
      text = text.replace(/访问[^\s]*/g, '');
      text = text.replace(/复制[^\s]*/g, '');
      text = text.replace(/云厂商/g, '');
      text = text.replace(/-/g, '');
      text = text.replace(/高危/g, '');
      text = text.replace(/中危/g, '');
      text = text.replace(/低危/g, '');
      return text.replace(/\s+/g, ' ').trim();
    }

    asset.ip = getCellText(2);
    var hostText = getCellText(3);
    if (hostText && hostText !== asset.ip) asset.host = hostText;
    // Port from column 4
    var portCell = cells[cells.length > 4 ? 4 : 0].textContent || '';
    var pm = portCell.match(/(\d{1,5})/);
    if (pm) asset.port = parseInt(pm[1]);
    // Protocol: extract known protocol name from column 4
    var protoMatch = portCell.match(/\b(http|https|tcp|udp|ssh|ftp|smtp|pop3|imap|mysql|rdp|smb|dns)\b/i);
    if (protoMatch) asset.protocol = protoMatch[1].toLowerCase();
    // Title from column 5 — keep only the first meaningful segment
    var titleRaw = getCellText(5);
    if (titleRaw) {
      // Take text before Chinese category labels like "企业办公", "邮件系统"
      var titleParts = titleRaw.split(/\s+(?:企业|个人|开源|政府|金融)/);
      asset.title = titleParts[0].trim();
    }
    asset.source = 'hunter';

    // Skip empty or duplicate rows
    if (!asset.ip && !asset.host) return;
    var key = asset.ip + ':' + asset.port;
    if (asset.port > 0 && seen[key]) return;
    if (asset.port > 0) seen[key] = true;
    assets.push(asset);
  });
  var totalEl = document.querySelector('.page-list-body_statistic');
  var total = 0;
  if (totalEl) { var m = totalEl.textContent.match(/(\\d[\\d,]*)/); if (m) total = parseInt(m[0].replace(/,/g, '')); }
  var hasNext = !!document.querySelector('.q-pagination button:last-child:not([disabled])');
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
  var containers = document.querySelectorAll('.item-container');
  var assets = [];
  containers.forEach(function(container) {
    var asset = {};
    // IP from div.ip span.copy_btn data-clipboard-text
    var copyBtn = container.querySelector('div.ip span.copy_btn, [data-clipboard-text]');
    if (copyBtn) {
      var clipText = copyBtn.getAttribute('data-clipboard-text') || '';
      var parts = clipText.split(':');
      asset.ip = parts[0] || '';
      if (parts.length > 1) asset.port = parseInt(parts[1]) || 0;
    }
    // Port from span.port
    var portEl = container.querySelector('span.port');
    if (portEl) {
      var p = parseInt(portEl.textContent.trim());
      if (p > 0) asset.port = p;
    }
    // Protocol from span.server-protocol
    var protoEl = container.querySelector('span.server-protocol');
    if (protoEl) asset.protocol = protoEl.textContent.trim();
    // Title from .title-line span.ellipse-text
    var titleEl = container.querySelector('.title-line span.ellipse-text');
    if (titleEl) asset.title = titleEl.textContent.trim();
    // Country from .country-container .address
    var countryEl = container.querySelector('.country-container .address');
    if (countryEl) asset.country = countryEl.textContent.trim();
    // Host from .item span.label matching "host" + sibling .ellipse-text
    var items = container.querySelectorAll('.item');
    items.forEach(function(item) {
      var label = item.querySelector('.label');
      if (label && /host|domain/i.test(label.textContent)) {
        var val = item.querySelector('.ellipse-text');
        if (val) asset.host = val.textContent.trim();
      }
    });
    asset.source = 'quake';
    if (asset.ip || asset.host) assets.push(asset);
  });
  var totalEl = document.querySelector('.total-count');
  var total = 0;
  if (totalEl) { var m = totalEl.textContent.match(/(\d[\d,]*)/); if (m) total = parseInt(m[0].replace(/,/g, '')); }
  var hasNext = !!document.querySelector('.el-pagination__next:not([disabled]) button');
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
