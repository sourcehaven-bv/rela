/* Shared chrome for the project-overview mockups: sidebar, variant switcher,
 * and a dark-mode toggle. The SPA toggles a `dark` class on <html>, so these
 * mockups do the same and get the real tokens for free. */

const VARIANTS = [
  ['index.html', 'Index'],
  ['1-accordion.html', '1 · Accordion'],
  ['1b-accordion-at-scale.html', '1b · At scale'],
  ['2-swimlanes.html', '2 · Swimlanes'],
  ['3-tree-table.html', '3 · Tree table'],
  ['4-cards.html', '4 · Epic cards'],
  ['5-timeline.html', '5 · Timeline'],
  ['6-detail-widget.html', '6 · Detail widget'],
];

function currentFile() {
  const name = window.location.pathname.split('/').pop();
  return name === '' ? 'index.html' : name;
}

function renderSidebar() {
  const here = currentFile();
  const links = VARIANTS.filter(([f]) => f !== 'index.html')
    .map(([file, label]) => {
      const cls = file === here ? ' class="active"' : '';
      return `<a href="${file}"${cls}>${label.replace(/^\d+\w* · /, '')}</a>`;
    })
    .join('');

  return `
    <nav class="sidebar">
      <div class="brand">rela</div>
      <a href="#">Dashboard</a>
      <a href="#">Projects</a>
      <a href="#">All tasks</a>
      <a href="#">Calendar</a>
      <div class="nav-group">Mockups</div>
      <a href="index.html"${here === 'index.html' ? ' class="active"' : ''}>Overview</a>
      ${links}
    </nav>`;
}

function renderVariants() {
  const here = currentFile();
  return `<div class="variants">${VARIANTS.map(([file, label]) => {
    const cls = file === here ? ' class="active"' : '';
    return `<a href="${file}"${cls}>${label}</a>`;
  }).join('')}</div>`;
}

function applyTheme(theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark');
  localStorage.setItem('rela-mockup-theme', theme);
  const btn = document.getElementById('theme-toggle');
  if (btn) btn.textContent = theme === 'dark' ? '☀ Light' : '☾ Dark';
}

/* Wraps the page's <main> content in the app shell. Call after DOM parse. */
function mountShell() {
  const main = document.querySelector('main.main');
  if (!main) return;

  const shell = document.createElement('div');
  shell.className = 'shell';
  main.parentNode.insertBefore(shell, main);
  shell.insertAdjacentHTML('afterbegin', renderSidebar());
  shell.appendChild(main);

  main.insertAdjacentHTML('afterbegin', renderVariants());

  const toggle = document.createElement('button');
  toggle.id = 'theme-toggle';
  toggle.className = 'btn';
  toggle.style.cssText = 'position:fixed;top:1rem;right:1rem;z-index:50;box-shadow:var(--shadow-sm)';
  toggle.onclick = () =>
    applyTheme(document.documentElement.classList.contains('dark') ? 'light' : 'dark');
  document.body.appendChild(toggle);

  applyTheme(localStorage.getItem('rela-mockup-theme') || 'light');
}

document.addEventListener('DOMContentLoaded', mountShell);
