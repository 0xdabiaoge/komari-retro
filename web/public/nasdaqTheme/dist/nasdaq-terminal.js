/**
 * NASDAQ Financial Trading Terminal Client Engine
 * High-Frequency Infrastructure Telemetry for Komari-Next
 */

(function () {
  'use strict';

  // Force dark mode
  try {
    document.documentElement.classList.add('dark');
    if (localStorage.getItem('theme') !== 'dark') {
      localStorage.setItem('theme', 'dark');
    }
  } catch (e) {}

  const tickers = [
    { symbol: '^NDX', name: 'NASDAQ 100', price: 18245.81, change: '+1.15%', up: true },
    { symbol: '^GSPC', name: 'S&P 500', price: 5751.13, change: '+0.77%', up: true },
    { symbol: 'NVDA', name: 'NVIDIA', price: 128.55, change: '+3.20%', up: true },
    { symbol: 'AAPL', name: 'APPLE', price: 227.37, change: '+0.85%', up: true },
    { symbol: 'MSFT', name: 'MICROSOFT', price: 448.20, change: '+0.65%', up: true },
    { symbol: 'AMZN', name: 'AMAZON', price: 186.40, change: '+1.42%', up: true },
    { symbol: 'TSLA', name: 'TESLA', price: 250.08, change: '+2.41%', up: true },
    { symbol: 'BTC/USD', name: 'BITCOIN', price: 63820.00, change: '+2.85%', up: true },
    { symbol: 'ETH/USD', name: 'ETHEREUM', price: 2465.50, change: '+1.90%', up: true },
    { symbol: '[TELEMETRY]', name: 'CLUSTER', price: '4/4 ONLINE', change: '100% NOMINAL', highlight: true },
    { symbol: '[BANDWIDTH]', name: 'TRAFFIC', price: '1.97 TB IN', change: '88.9 GB OUT', highlight: true },
    { symbol: '[HFT-NET]', name: 'SPEED', price: '399 KB/s ▲', change: '61 KB/s ▼', up: true },
    { symbol: '[LATENCY]', name: 'PING', price: '24ms avg', change: 'HFT OPTIMIZED', up: true }
  ];

  function renderTickerItem(item) {
    if (item.highlight) {
      return `<div class="nasdaq-item"><span class="symbol">${item.symbol}</span> <span class="highlight">${item.name}: ${item.price} (${item.change})</span></div>`;
    }
    const colorClass = item.up ? 'up' : 'down';
    const arrow = item.up ? '▲' : '▼';
    const priceStr = typeof item.price === 'number' ? item.price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : item.price;
    return `<div class="nasdaq-item"><span class="symbol">${item.symbol}</span> <span class="price">${priceStr}</span> <span class="${colorClass}">${arrow} ${item.change}</span></div>`;
  }

  function mountTickerTape() {
    if (document.getElementById('nasdaq-ticker-bar')) return;

    const bar = document.createElement('div');
    bar.id = 'nasdaq-ticker-bar';

    const itemsHtml = tickers.map(renderTickerItem).join('');
    // Duplicate for smooth seamless loop
    const fullItemsHtml = itemsHtml + itemsHtml;

    bar.innerHTML = `
      <div class="nasdaq-badge-left">
        <span class="nasdaq-pulse-dot"></span>
        <span>NASDAQ // HFT TERMINAL</span>
      </div>
      <div class="nasdaq-ticker-track">
        <div class="nasdaq-ticker-items" id="nasdaq-ticker-container">
          ${fullItemsHtml}
        </div>
      </div>
      <div class="nasdaq-badge-right" id="nasdaq-clock">
        MARKET: OPEN | --:--:-- UTC
      </div>
    `;

    document.body.prepend(bar);

    // Update Clock
    function updateClock() {
      const clockEl = document.getElementById('nasdaq-clock');
      if (!clockEl) return;
      const now = new Date();
      const h = String(now.getUTCHours()).padStart(2, '0');
      const m = String(now.getUTCMinutes()).padStart(2, '0');
      const s = String(now.getUTCSeconds()).padStart(2, '0');
      clockEl.innerText = `MARKET: OPEN | ${h}:${m}:${s} UTC`;
    }
    setInterval(updateClock, 1000);
    updateClock();

    // Micro fluctuations for realistic live market ticker
    setInterval(() => {
      const idx = Math.floor(Math.random() * 7); // pick one stock
      const stock = tickers[idx];
      if (typeof stock.price === 'number') {
        const delta = (Math.random() - 0.48) * (stock.price * 0.002);
        stock.price = Math.round((stock.price + delta) * 100) / 100;
        const container = document.getElementById('nasdaq-ticker-container');
        if (container) {
          const newHtml = tickers.map(renderTickerItem).join('');
          container.innerHTML = newHtml + newHtml;
        }
      }
    }, 3500);
  }

  // Safely mount AFTER React hydration
  if (document.readyState === 'complete') {
    setTimeout(mountTickerTape, 200);
  } else {
    window.addEventListener('load', () => {
      setTimeout(mountTickerTape, 200);
    });
  }

  // Periodic check to ensure dark mode class stays on html
  setInterval(() => {
    if (!document.documentElement.classList.contains('dark')) {
      document.documentElement.classList.add('dark');
    }
  }, 1000);
})();
