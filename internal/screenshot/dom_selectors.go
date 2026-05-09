package screenshot

// engineSelectors defines CSS selectors for extracting structured asset data
// from a search engine's web results page.
type engineSelectors struct {
	RowSelector    string            // CSS selector for a single result row
	ExtractJS      string            // JavaScript snippet to run on the page for data extraction
	PaginationNext string            // CSS selector for the "next page" button
	TotalSelector  string            // CSS selector for the total results count indicator
}

// selectorsByEngine maps engine names to their DOM extraction selectors.
// These are best-effort based on current page structures and may need
// adjustment when engine frontends change.
var selectorsByEngine = map[string]*engineSelectors{
	"fofa": {
		RowSelector:   "table.el-table__body tr.el-table__row",
		ExtractJS:     extractFofaJS,
		PaginationNext: "button.btn-next",
		TotalSelector:  ".search-list-head span:first-child",
	},
	"hunter": {
		RowSelector:   "table.table tbody tr",
		ExtractJS:     extractHunterJS,
		PaginationNext: "li.next a",
		TotalSelector:  ".pull-left.m-b span",
	},
	"zoomeye": {
		RowSelector:   "table.table-condensed tbody tr",
		ExtractJS:     extractZoomEyeJS,
		PaginationNext: "ul.pagination li.next a",
		TotalSelector:  ".result-count",
	},
	"quake": {
		RowSelector:   "table tbody tr",
		ExtractJS:     extractQuakeJS,
		PaginationNext: "li.ant-pagination-next button",
		TotalSelector:  ".total-count",
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
  var rows = document.querySelectorAll('table.table-condensed tbody tr');
  var assets = [];
  rows.forEach(function(row) {
    var cells = row.querySelectorAll('td');
    if (cells.length < 5) return;
    var asset = {};
    asset.ip = cells[0] ? cells[0].textContent.trim() : '';
    asset.port = parseInt(cells[1] ? cells[1].textContent.trim() : '0') || 0;
    asset.protocol = cells[2] ? cells[2].textContent.trim() : '';
    asset.title = cells[3] ? cells[3].textContent.trim() : '';
    asset.country = cells[4] ? cells[4].textContent.trim() : '';
    asset.server = cells[5] ? cells[5].textContent.trim() : '';
    asset.source = 'zoomeye';
    assets.push(asset);
  });
  var total = 0;
  var hasNext = !!document.querySelector('ul.pagination li.next:not(.disabled) a');
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
