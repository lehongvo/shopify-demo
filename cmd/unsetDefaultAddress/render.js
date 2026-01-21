(async function () {
  const container = document.getElementById('app');

  try {
    const res = await fetch('index.json');
    if (!res.ok) {
      throw new Error('Failed to load index.json: ' + res.status);
    }
    const data = await res.json();
    const product = data.product || {};

    // Update document title
    if (product.title) {
      document.title = product.title;
    }

    // Render HTML
    let html = '';
    if (product.title) {
      html += `<div class="product-title">${escapeHtml(product.title)}</div>`;
    }
    if (product.body_html) {
      html += `<div class="product-body">${product.body_html}</div>`;
    }

    container.innerHTML = html || 'No product data found.';
  } catch (err) {
    console.error(err);
    container.textContent = 'Error loading product data. Check console for details.';
  }

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }
})();
