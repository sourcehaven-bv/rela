// The host hands the SDK its bridge port a moment after load and signals
// it with the one-time 'rela:ready' event. The SDK queues calls made
// before the port arrives, so we could call straight away — but waiting
// avoids a flash of an empty table if the handshake is slow.
let started = false;
function whenReady(fn) {
  if (started) return;
  started = true;
  fn();
}
window.addEventListener('rela:ready', () => whenReady(load), { once: true });
// Safety net: if the event already fired (or fires very fast), start anyway.
setTimeout(() => whenReady(load), 500);

const statusEl = document.getElementById('status');
const tableEl = document.getElementById('table');
const rowsEl = document.getElementById('rows');
const refreshBtn = document.getElementById('refresh');

async function load() {
  statusEl.textContent = 'Loading tickets…';
  try {
    // Page through tickets and tally status. A real app would request a
    // server-side aggregate; this keeps the demo dependency-free.
    const res = await window.rela.list({ type: 'ticket', params: { per_page: 200 } });
    const items = (res && res.data) || [];
    const counts = {};
    for (const t of items) {
      const s = (t.properties && t.properties.status) || 'unknown';
      counts[s] = (counts[s] || 0) + 1;
    }
    rowsEl.replaceChildren();
    Object.keys(counts)
      .sort()
      .forEach((s) => {
        // Build cells with textContent (auto-escaping) rather than
        // innerHTML string concatenation — status values come from the
        // entity store and must never be treated as HTML.
        const tr = document.createElement('tr');
        const label = document.createElement('td');
        label.textContent = s;
        const count = document.createElement('td');
        count.className = 'count';
        count.textContent = String(counts[s]);
        tr.append(label, count);
        rowsEl.appendChild(tr);
      });
    statusEl.textContent = items.length + ' tickets';
    tableEl.hidden = false;
    refreshBtn.hidden = false;
  } catch (e) {
    statusEl.className = 'err';
    statusEl.textContent = 'Error: ' + (e && e.message ? e.message : e);
  }
}

refreshBtn.addEventListener('click', load);
