// Platform tabs in the installer.
const tabs = [...document.querySelectorAll('.platform-tabs [role="tab"]')];
function selectPlatform(platform, focus = false) {
  tabs.forEach(tab => {
    const selected = tab.id === `tab-${platform}`;
    tab.setAttribute('aria-selected', String(selected));
    tab.tabIndex = selected ? 0 : -1;
    document.getElementById(tab.getAttribute('aria-controls')).hidden = !selected;
    if (selected && focus) tab.focus();
  });
}
tabs.forEach((tab, index) => {
  tab.addEventListener('click', () => selectPlatform(tab.id.slice(4)));
  tab.addEventListener('keydown', event => {
    let next;
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') next = (index + 1) % tabs.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = tabs.length - 1;
    else return;
    event.preventDefault();
    selectPlatform(tabs[next].id.slice(4), true);
  });
});

// Copy buttons.
document.querySelectorAll('[data-copy]').forEach(button => {
  button.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(button.dataset.copy);
      button.textContent = 'Copied';
      document.getElementById('copy-status').textContent = 'Commands copied to clipboard.';
      setTimeout(() => { button.textContent = 'Copy'; }, 1800);
    } catch {
      button.textContent = 'Select text';
      document.getElementById('copy-status').textContent = 'Copy unavailable. Select and copy the commands below.';
    }
  });
});

// Windows visitors get the Windows download first and the Windows steps open.
if (/Windows/.test(navigator.userAgent)) {
  const mac = document.querySelector('[data-download="mac"]');
  const win = document.querySelector('[data-download="windows"]');
  if (mac && win) {
    mac.classList.replace('primary', 'secondary');
    win.classList.replace('secondary', 'primary');
    win.parentNode.insertBefore(win, mac);
  }
  selectPlatform('windows');
}

// Hero wall: copy each column's windows once so the CSS drift loops without a seam.
// The copies are decoration: hidden from screen readers and skipped by Tab.
document.querySelectorAll('.wall-col').forEach(col => {
  [...col.children].forEach(tile => {
    const copy = tile.cloneNode(true);
    copy.setAttribute('aria-hidden', 'true');
    copy.tabIndex = -1;
    copy.querySelector('img')?.removeAttribute('fetchpriority');
    col.appendChild(copy);
  });
});

// Screenshots: every wall tile and "See ..." button opens the lightbox.
const shots = {
  sessions: 'Sessions marked needs you, running and done, an alert card, and the API server’s log in the panel under the terminal',
  agents: 'A subagent open in the panel under its session, reading like Claude’s own transcript',
  board: 'The agents board, opened with Cmd+Shift+A: what needs you first, then each session’s agents',
  palette: 'The palette, opened with Cmd+K: waiting sessions first, then everything else',
  workspaces: 'A workspace card with its API, Web and Health members running',
  settings: 'Appearance settings with five themes and a live preview',
  about: 'About Kaze on macOS'
};
const order = Object.keys(shots);
const box = document.getElementById('lightbox');
const boxImg = document.getElementById('lightbox-img');
const boxCap = document.getElementById('lightbox-cap');
let current = 0, opener = null;
function show(i) {
  current = (i + order.length) % order.length;
  const key = order[current];
  boxImg.src = `site-assets/kaze-${key}.png`;
  boxImg.alt = shots[key];
  boxCap.textContent = shots[key];
}
function openShot(key, from) {
  opener = from;
  show(order.indexOf(key));
  if (!box.open) box.showModal();
}
document.querySelectorAll('.tile[data-shot]').forEach(tile => {
  tile.setAttribute('aria-label', `Open screenshot: ${shots[tile.dataset.shot]}`);
  tile.addEventListener('click', () => openShot(tile.dataset.shot, tile));
});
document.querySelectorAll('[data-open]').forEach(button => button.addEventListener('click', () => openShot(button.dataset.open, button)));
box.querySelectorAll('[data-step]').forEach(button => button.addEventListener('click', () => show(current + Number(button.dataset.step))));
box.querySelector('[data-close]').addEventListener('click', () => box.close());
box.addEventListener('click', event => { if (event.target === box) box.close(); });
box.addEventListener('keydown', event => {
  if (event.key === 'ArrowRight') show(current + 1);
  if (event.key === 'ArrowLeft') show(current - 1);
});
box.addEventListener('close', () => { if (opener) opener.focus(); });

// The moments: four real Kaze screenshots, switched by the chips (or keys 1 to 4).
const moments = {
  needs: {
    img: 'site-assets/kaze-moment-needs.png',
    alt: 'Kaze with a card at the bottom right: dark-mode, Claude needs your permission to use Bash',
    cap: 'Claude asked for permission in another session. The card names it the way the sidebar does; ⌘ J takes you there.'
  },
  servers: {
    img: 'site-assets/kaze-moment-servers.png',
    alt: 'Kaze with the API workspace member open as a tab in the panel under the session, showing its server log',
    cap: 'Click a workspace member and its terminal opens as a tab in the panel under Claude. Click it to type; click Claude to take the keyboard back.'
  },
  agents: {
    img: 'site-assets/kaze-agents.png',
    alt: 'A subagent open in the panel under its session: its notes, then Read 3 files, an edit and a failed test run',
    cap: 'A subagent’s own words lead, then its steps. It sits in the same panel as your servers.'
  },
  done: {
    img: 'site-assets/kaze-moment-done.png',
    alt: 'Kaze with a done card at the bottom right: search-index, Search now matches whole words',
    cap: 'Done cards show the start of Claude’s reply and how long it ran, then hide by themselves.'
  }
};
const momentImg = document.querySelector('[data-moment-img]');
const cap = document.querySelector('[data-cap]');
const chips = [...document.querySelectorAll('[data-moment]')];
Object.values(moments).forEach(m => { const i = new Image(); i.src = m.img; }); // switch without a blank frame
function moment(name) {
  const m = moments[name];
  chips.forEach(c => c.setAttribute('aria-pressed', String(c.dataset.moment === name)));
  momentImg.src = m.img;
  momentImg.alt = m.alt;
  cap.textContent = m.cap;
}
chips.forEach(c => c.addEventListener('click', () => moment(c.dataset.moment)));
document.addEventListener('keydown', event => {
  if (box.open || event.ctrlKey || event.metaKey || event.altKey) return;
  if (/^(INPUT|TEXTAREA|SELECT)$/.test(document.activeElement.tagName)) return;
  const i = Number(event.key) - 1;
  if (i >= 0 && i < chips.length) moment(chips[i].dataset.moment);
});
const fromHash = location.hash.match(/^#moment-(\w+)$/);
moment(fromHash && moments[fromHash[1]] ? fromHash[1] : 'needs');

// Sum release ZIP downloads, including the ZIPs used for app updates.
// Keep the figure hidden if GitHub is unavailable rather than showing a partial total.
(async () => {
  const fact = document.getElementById('download-fact');
  const count = document.getElementById('download-count');
  if (!fact || !count) return;
  try {
    let total = 0;
    for (let page = 1; ; page++) {
      const response = await fetch(`https://api.github.com/repos/beefysalad/kaze-terminal/releases?per_page=100&page=${page}`, { signal: AbortSignal.timeout(8000) });
      if (!response.ok) return;
      const releases = await response.json();
      for (const release of releases) for (const asset of release.assets || []) if (asset.name.endsWith('.zip')) total += asset.download_count;
      if (releases.length < 100) break;
    }
    if (total > 0) { count.textContent = total.toLocaleString('en-US'); fact.hidden = false; }
  } catch {
    // Downloads stay usable offline or after rate limiting.
  }
})();
