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
  sessions: 'The session sidebar: sessions grouped by repo, marked needs you, running and done',
  agents: 'Subagents listed under their session, with one open beside the terminal',
  board: 'The agents board, opened with Cmd+Shift+A',
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

// The live window: four moments, switched by the chips (or keys 1 to 4).
const moments = {
  needs: {
    cap: 'Claude asked for permission in another session. The card names it the way the sidebar does; ⌘ J takes you there.',
    term: '<span class="a">●</span> I\'ll run the auth tests before reviewing the diff.\n\n<span class="d">  Bash(npm run test -- --runInBand src/auth)</span>\n\n  Do you want to proceed?\n  <span class="a">❯ 1. Yes</span>\n    2. Yes, and don\'t ask again for npm run test\n    3. No, and tell Claude what to do differently',
    state: ['needs you', 'needs'], wait: '1 waiting',
    card: { kind: 'needs', ctx: '<span class="br">⎇ feature/V3-3149</span> · <span class="c">/review-pr</span>', title: 'auth-tokens', body: 'Claude needs your permission to use Bash: npm run test -- --runInBand src/auth', status: 'Needs your permission', dot: 'var(--needs)' }
  },
  servers: {
    cap: 'Click a workspace member and its terminal opens as a tab under Claude. Click it to type; click Claude to take the keyboard back.',
    term: '<span class="d">&gt; </span>add rate limiting to the token endpoint\n\n<span class="a">●</span> Adding a sliding-window limiter to POST /auth/token.\n\n<span class="d">  Update(src/auth/routes.ts)</span>\n<span class="g">  + router.post("/token", limit({ window: "1m", max: 10 }), issueToken)</span>',
    state: ['running', 'run'], wait: '',
    dock: 'api',
    body: '<span class="st">$ <b>uvicorn app.main:app --reload --port 8000</b></span>\nINFO:     Uvicorn running on http://127.0.0.1:8000\nINFO:     Application startup complete.\nINFO:     127.0.0.1 - "POST /auth/token HTTP/1.1" 200\nINFO:     127.0.0.1 - "POST /auth/token HTTP/1.1" <span class="y">429</span>'
  },
  agents: {
    cap: 'A subagent’s own words lead, then its steps. It sits in the same panel as your servers.',
    term: '<span class="a">●</span> I\'ll have an Explore agent map the auth code while I read the diff.\n\n<span class="d">  Agent(Find the auth middleware)</span>\n<span class="d">  ⎿ Running…</span>',
    state: ['running', 'run'], wait: '',
    dock: 'agent',
    body: '<span class="say">Looking for where tokens are checked. The middleware is likely under src/auth; I\'ll confirm before reading the routes.</span><span class="st">› Searched the code <b>3 times</b></span>\n<span class="st">› Read <b>src/auth/middleware.ts</b></span>\n<span class="st">› Read <b>src/auth/routes.ts</b></span>'
  },
  done: {
    cap: 'Done cards show the start of Claude’s reply and how long it ran, then hide by themselves.',
    term: '<span class="d">✻ Worked for 6m 40s</span>\n\n<span class="a">●</span> Review done. Two things to fix before merging:\n  1. The refresh token is logged at debug level.\n  2. /auth/token has no rate limit.\n\n<span class="d">&gt; </span>',
    state: ['done', 'done'], wait: '',
    card: { kind: 'done', ctx: '<span class="br">⎇ feature/V3-3149</span> · <span class="c">/review-pr</span>', title: 'auth-tokens', body: 'Review done. Two things to fix before merging: the refresh token is logged at debug level, and /auth/token has no rate limit.', status: 'Done · ran 6m 40s', dot: 'var(--done)' }
  }
};
const term = document.querySelector('[data-term]');
const dock = document.querySelector('[data-dock]');
const dockBody = document.querySelector('[data-body]');
const card = document.querySelector('[data-card]');
const sideState = document.querySelector('[data-side-state]');
const sideWait = document.querySelector('[data-side-wait]');
const cap = document.querySelector('[data-cap]');
const chips = [...document.querySelectorAll('[data-moment]')];
function moment(name) {
  const m = moments[name];
  chips.forEach(c => c.setAttribute('aria-pressed', String(c.dataset.moment === name)));
  term.innerHTML = m.term;
  sideState.textContent = m.state[0];
  sideState.className = `s ${m.state[1]}`;
  sideWait.textContent = m.wait;
  dock.hidden = !m.dock;
  if (m.dock) {
    dock.querySelectorAll('[data-tab]').forEach(t => t.classList.toggle('on', t.dataset.tab === m.dock));
    document.querySelector('[data-expand]').hidden = m.dock !== 'api';
    dockBody.innerHTML = m.body;
  }
  card.hidden = !m.card;
  if (m.card) {
    const c = m.card;
    card.innerHTML = `<span class="av">✳<span class="bd ${c.kind}">${c.kind === 'needs' ? '!' : '✓'}</span></span><div><div class="ctx">${c.ctx}</div><div class="tt">${c.title}</div><div class="bo">${c.body}</div><div class="sl"><i style="background:${c.dot}"></i>${c.status}</div><div class="ft"><span>Open session</span><kbd>Ctrl</kbd><kbd>Shift</kbd><kbd>J</kbd></div></div>`;
  }
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
