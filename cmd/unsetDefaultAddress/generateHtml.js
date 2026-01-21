const fs = require('fs');
const path = require('path');

// Paths
const jsonPath = path.join(__dirname, 'index.json');
const outPath = path.join(__dirname, 'index.generated.html');

function main() {
  const raw = fs.readFileSync(jsonPath, 'utf8');
  const data = JSON.parse(raw);
  const product = data.product;

  // Basic HTML from product data
  let html = '';

  html += `<h1>${escapeHtml(product.title)}</h1>\n`;

  // body_html đã là HTML, chèn trực tiếp
  if (product.body_html) {
    html += product.body_html + '\n';
  }

  // Render bảng variants: Thickness (option1) / Material (option2)
  if (Array.isArray(product.variants) && product.variants.length > 0) {
    html += '\n<h3>Available Variants</h3>\n';
    html += '<table border="1" cellpadding="4" cellspacing="0">';
    html += '<thead><tr><th>Thickness</th><th>Material</th><th>SKU</th><th>Price</th></tr></thead><tbody>';

    for (const v of product.variants) {
      html += '<tr>' +
        `<td>${escapeHtml(v.option1 || '')}</td>` +
        `<td>${escapeHtml(v.option2 || '')}</td>` +
        `<td>${escapeHtml(v.sku || '')}</td>` +
        `<td>${escapeHtml(v.price || '')}</td>` +
        '</tr>';
    }

    html += '</tbody></table>\n';
  }

  // Render options list (Thickness, Material)
  if (Array.isArray(product.options) && product.options.length > 0) {
    html += '\n<h3>Options</h3>\n';
    for (const opt of product.options) {
      html += `<h4>${escapeHtml(opt.name || '')}</h4>`;
      if (Array.isArray(opt.values)) {
        html += '<ul>';
        for (const val of opt.values) {
          html += `<li>${escapeHtml(val)}</li>`;
        }
        html += '</ul>';
      }
    }
  }

  fs.writeFileSync(outPath, html, 'utf8');
  console.log('Generated HTML ->', outPath);
}

function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

main();
