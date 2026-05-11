/**
 * Test extension extraction logic using HTML loaded from file.
 * Usage: node test_extraction_node.js <html-file-path>
 *
 * Since Chrome MCP JS injection has limitations, this loads the HTML
 * directly and runs the extraction logic in Node.js.
 */

const fs = require('fs');
const path = require('path');

// Simple DOM simulation using regex-based extraction
function extractFromHTML(html, engine) {
  const ENGINE_SELECTORS = {
    fofa: {
      row: [
        ".result-card", "[class*='result-card']", "[class*='result-item']", ".result-item",
        ".list_content > tbody > tr", ".result-table tbody tr",
        "[class*='result'] table tbody tr", "table[class*='list'] tbody tr"
      ],
      cells: {
        ip: { selector: "td:nth-child(1) a", fallback: "td:nth-child(1)" },
        port: { selector: "td:nth-child(2)" },
        protocol: { selector: "td:nth-child(3)" },
        host: { selector: "td:nth-child(4) a", fallback: "td:nth-child(4)" },
        title: { selector: "td:nth-child(5)" },
        country_code: { selector: "td:nth-child(6)" },
        banner: { selector: "td:nth-child(7)" }
      }
    },
    hunter: {
      row: [
        ".result-list > .result-item", ".result-item",
        ".el-table tbody tr"
      ],
      cells: {
        ip: { selector: ".ip-address, [data-ip], [class*='ip']" },
        port: { selector: ".port, [data-port], [class*='port']" },
        protocol: { selector: ".protocol, .service, [data-protocol]" },
        host: { selector: ".domain, .hostname, [class*='domain']" },
        title: { selector: ".web-title, .title, [class*='web-title']" },
        banner: { selector: ".header-info, .banner" }
      }
    }
  };

  const result = {
    engine,
    items: [],
    total: 0,
    has_more: false,
    debug: {}
  };

  // Extract IPs from the HTML
  const ipRegex = /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/g;
  const ips = html.match(ipRegex) || [];
  result.debug.uniqueIPs = [...new Set(ips)].slice(0, 10);
  result.debug.ipCount = ips.length;

  // Find result count
  const totalMatch = html.match(/(\d[\d,]+)\s*条匹配结果/);
  if (totalMatch) {
    result.total = parseInt(totalMatch[1].replace(/,/g, ''), 10);
  }

  // Count tables
  const tableMatches = html.match(/<table/gi) || [];
  result.debug.tableCount = tableMatches.length;

  // Count divs with relevant classes
  const classRegex = /class="([^"]*)"/g;
  let match;
  const classes = new Set();
  while ((match = classRegex.exec(html)) !== null) {
    match[1].split(' ').forEach(c => {
      if (/result|list|table|row|data|cell|card|item|count|total/i.test(c)) {
        classes.add(c);
      }
    });
  }
  result.debug.relevantClasses = [...classes];

  // Try to extract card-based items using link patterns
  const linkRegex = /<a[^>]*href="([^"]*)"[^>]*>([^<]*(?:<[^>]*>[^<]*)*)<\/a>/gi;
  const links = [];
  while ((match = linkRegex.exec(html)) !== null) {
    const href = match[1];
    const text = match[2].replace(/<[^>]*>/g, '').trim();
    if (text) links.push({ href, text });
  }

  // Group links into items based on IP addresses
  const ipLinks = links.filter(l => /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(l.text) || l.href.includes('qbase64=aXA9'));
  result.debug.ipLinksFound = ipLinks.length;

  if (ipLinks.length > 0) {
    for (const ipLink of ipLinks.slice(0, 100)) {
      const item = {};
      const ipText = ipLink.text;
      if (/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/.test(ipText)) {
        item.ip = ipText;
      }
      result.items.push(item);
    }
  }

  return result;
}

// Main
const args = process.argv.slice(2);
if (args.length < 1) {
  console.log('Usage: node test_extraction_node.js <html-file>');
  process.exit(1);
}

const htmlFile = args[0];
const engine = args[1] || 'fofa';

const html = fs.readFileSync(htmlFile, 'utf8');
console.log(`HTML file: ${htmlFile}`);
console.log(`HTML length: ${html.length}`);
console.log(`Engine: ${engine}`);

const result = extractFromHTML(html, engine);

console.log('\n=== Extraction Result ===');
console.log(JSON.stringify(result, null, 2));

if (result.items.length === 0) {
  console.log('\nWARNING: No items extracted!');
  console.log(`Tables found: ${result.debug.tableCount}`);
  console.log(`Relevant classes: ${result.debug.relevantClasses.join(', ')}`);
  console.log(`IP links found: ${result.debug.ipLinksFound}`);
}
