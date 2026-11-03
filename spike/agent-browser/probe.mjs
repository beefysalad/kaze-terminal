import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { once } from 'node:events';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'node:http';
import { CdpPeer, FocusGuardRelay, McpPeer, delay } from './protocol.mjs';
import { SHOW, WinTools, ownedEvent, startForegroundMonitor, treeSummary } from './win.mjs';

const directory = path.dirname(fileURLToPath(import.meta.url));
const artifacts = path.join(directory, 'artifacts');
await fs.mkdir(artifacts, { recursive: true });
const browsers = new Set();
const relays = new Set();
const mcps = new Set();
const focusEvents = [];
const windows = process.platform === 'win32';
// --isolation and --mcp are the macOS phases; --popup, --dialogs, --download and --memory are the extra
// Windows-run scenarios (they also run elsewhere when asked for). No flag: every phase this platform runs by default.
const knownPhases = ['isolation', 'mcp', 'popup', 'dialogs', 'download', 'memory'];
const askedPhases = knownPhases.filter(phase => process.argv.includes(`--${phase}`));
const phases = askedPhases.length ? askedPhases : windows ? knownPhases : ['isolation', 'mcp'];
const launchVariants = ['node-hidden', 'node-default', 'shownoactivate', 'showminnoactive'];
const launchVariant = process.env.KAZE_SPIKE_LAUNCH || (windows ? 'node-default' : 'open-g');
if (windows && !launchVariants.includes(launchVariant)) throw new Error(`KAZE_SPIKE_LAUNCH must be one of ${launchVariants.join(', ')}`);
const owned = new Map(); // browser-tree pid -> { from, to }: only these count as test-browser activations
const windowSnapshots = [];
const scenarioSpans = {};
let tools;
let runRoot;
let sampler;
const report = { date: new Date().toISOString(), platform: process.platform, release: os.release(), arch: os.arch(), node: process.version, nodeExecutable: process.execPath, phases, baseCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: directory, encoding: 'utf8' }).trim(), dependencies: JSON.parse(await fs.readFile(path.join(directory, 'package.json'), 'utf8')).dependencies, observations: {}, browserInstances: [] };
let monitor;
let fixture;

const windowsBrowsers = {
  edge: path.join(process.env['PROGRAMFILES(X86)'] || 'C:\\Program Files (x86)', 'Microsoft/Edge/Application/msedge.exe'),
  chrome: path.join(process.env.PROGRAMFILES || 'C:\\Program Files', 'Google/Chrome/Application/chrome.exe')
};

// KAZE_SPIKE_CHROME: a browser executable path, or "edge" / "chrome" for the installed default.
// The design picks Edge on Windows, so Edge is the Windows default.
function executable() {
  const chosen = process.env.KAZE_SPIKE_CHROME;
  if (chosen && windowsBrowsers[chosen.toLowerCase()]) return windowsBrowsers[chosen.toLowerCase()];
  if (chosen) return chosen;
  if (process.platform === 'darwin') return '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
  if (windows) return windowsBrowsers.edge;
  throw new Error('Set KAZE_SPIKE_CHROME to an installed Chrome executable');
}
const browserName = (() => { try { return /msedge/i.test(executable()) ? 'edge' : 'chrome'; } catch { return 'unknown'; } })();

function own(pid, at = Date.now()) {
  if (!pid) return;
  const span = owned.get(pid);
  if (!span) owned.set(pid, { from: at, to: null });
  else if (span.to !== null) span.to = null;
}

async function sampleTree(item) {
  if (!tools || item.closed) return [];
  const rows = await tools.tree(item.pid).catch(() => []);
  for (const row of rows) { own(row.pid); item.treePids.add(row.pid); }
  return rows;
}

async function snapshotWindows(label) {
  if (!tools) return undefined;
  const pids = [...owned].filter(([, span]) => span.to === null).map(([pid]) => pid);
  const rows = await tools.windows(pids).catch(error => [{ error: error.message }]);
  windowSnapshots.push({ label, at_ms: Date.now(), windows: rows });
  return rows;
}

// Runs one phase, records its time span (for per-scenario activation counts) and keeps going on failure.
async function scenario(name, fn) {
  const span = { startedAt: Date.now() };
  scenarioSpans[name] = span;
  try { await fn(); span.ok = true; } catch (error) {
    span.ok = false; span.error = error.stack;
    report.error ??= error.stack;
    console.error(name, error);
    process.exitCode = 1;
  } finally { span.endedAt = Date.now(); }
}

async function launchBrowser(label) {
  const started = performance.now();
  const profile = await fs.mkdtemp(path.join(runRoot, `${label}-`));
  const flags = [
    `--user-data-dir=${profile}`, '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0',
    '--no-first-run', '--no-default-browser-check', '--no-startup-window', '--disable-extensions',
    '--disable-background-networking', '--disable-component-update', '--disable-sync', '--disable-default-apps',
    '--metrics-recording-only', '--disable-features=Translate,MediaRouter', '--window-size=900,650'
  ];
  const item = { label, profile, closed: false, treePids: new Set() };
  browsers.add(item);
  if (process.platform === 'darwin') {
    execFileSync('/usr/bin/open', ['-g', '-n', '-a', executable().split('/Contents/')[0], '--args', ...flags], { timeout: 10000 });
  } else if (windows && SHOW[launchVariant]) {
    // CreateProcess with STARTF_USESHOWWINDOW and SW_SHOWNOACTIVATE / SW_SHOWMINNOACTIVE.
    item.launcherPid = (await tools.launch(executable(), flags, SHOW[launchVariant])).pid;
  } else {
    // node-hidden: STARTF_USESHOWWINDOW + SW_HIDE (Node's windowsHide); node-default: no show flags.
    item.process = spawn(executable(), flags, { stdio: 'ignore', windowsHide: launchVariant !== 'node-default' });
    item.process.on('error', error => { item.spawnError = error; });
    item.launcherPid = item.process.pid;
  }
  own(item.launcherPid);
  let lines;
  const deadline = performance.now() + 10000;
  while (performance.now() < deadline && !item.spawnError) {
    try { lines = (await fs.readFile(path.join(profile, 'DevToolsActivePort'), 'utf8')).trim().split('\n'); break; } catch { await delay(50); }
  }
  if (item.spawnError) throw item.spawnError;
  assert.ok(lines, `Chrome did not expose its temporary endpoint: ${label}`);
  item.http = `http://127.0.0.1:${lines[0]}`;
  item.ws = `ws://127.0.0.1:${lines[0]}${lines[1]}`;
  item.cdp = await CdpPeer.connect(item.ws);
  item.version = await item.cdp.call('Browser.getVersion');
  const processInfo = await item.cdp.call('SystemInfo.getProcessInfo');
  item.pid = processInfo.processInfo.find(p => p.type === 'browser').id;
  for (const p of processInfo.processInfo) own(p.id);
  item.startupMs = Math.round(performance.now() - started);
  item.startedAt = Date.now();
  await sampleTree(item);
  report.browserInstances.push({ label, pid: item.pid, ...(windows ? { launcherPid: item.launcherPid, launchVariant } : {}), startupMs: item.startupMs, startedAt: item.startedAt });
  report.chrome = item.version.product;
  return item;
}

async function stopBrowser(item) {
  if (item.closed) return;
  item.closed = true;
  try { await item.cdp.call('Browser.close'); } catch {}
  item.cdp?.close();
  item.pid ??= item.launcherPid; // launch failed before CDP reported the browser PID
  for (let n = 0; n < 30; n++) {
    try { process.kill(item.pid, 0); } catch { break; }
    await delay(50);
  }
  // PID was read from this run's private CDP connection, never from a browser name search.
  try { process.kill(item.pid, 0); process.kill(item.pid, 'SIGTERM'); } catch {}
  await delay(windows ? 500 : 150);
  const stoppedAt = Date.now();
  for (const pid of [item.pid, item.launcherPid, ...item.treePids]) { const span = owned.get(pid); if (span && span.to === null) span.to = stoppedAt; }
  await fs.rm(item.profile, { recursive: true, force: true, maxRetries: windows ? 20 : 5, retryDelay: windows ? 250 : 100 });
  browsers.delete(item);
}

async function newPage(browser, url, browserContextId) {
  const { targetId } = await browser.cdp.call('Target.createTarget', { url: 'about:blank', newWindow: true, background: true, ...(browserContextId ? { browserContextId } : {}) });
  const { sessionId } = await browser.cdp.call('Target.attachToTarget', { targetId, flatten: true });
  await browser.cdp.call('Page.enable', {}, sessionId);
  await browser.cdp.call('Page.navigate', { url }, sessionId);
  const deadline = performance.now() + 5000;
  while (performance.now() < deadline) {
    const ready = await browser.cdp.call('Runtime.evaluate', { expression: 'document.readyState', returnByValue: true }, sessionId);
    if (ready.result.value === 'complete') break;
    await delay(30);
  }
  return { targetId, sessionId, browserContextId, url };
}

async function evaluate(peer, page, expression) {
  const result = await peer.call('Runtime.evaluate', { expression, returnByValue: true }, page.sessionId);
  return result.result.value;
}

async function isolation(origin) {
  const shared = await launchBrowser('shared-browser');
  const a = await newPage(shared, `${origin}/session-a`);
  const b = await newPage(shared, `${origin}/session-b`);
  await evaluate(shared.cdp, a, "document.cookie='kaze_probe=shared; SameSite=Lax; path=/'");
  const cookieShared = (await evaluate(shared.cdp, b, 'document.cookie')).includes('kaze_probe=shared');
  assert.equal(cookieShared, true);
  const agent = await CdpPeer.connect(shared.ws);
  const targets = (await agent.call('Target.getTargets')).targetInfos;
  assert.ok(targets.some(t => t.targetId === b.targetId));
  const foreign = await agent.call('Target.attachToTarget', { targetId: b.targetId, flatten: true });
  const foreignTitle = await evaluate(agent, foreign, 'document.title');
  assert.equal(foreignTitle, 'Kaze fixture /session-b');
  report.observations.sharedWindows = { sharedSignIn: cookieShared, foreignWindowListed: true, foreignWindowReadable: true, isolated: false, startupMs: shared.startupMs };

  const { browserContextId } = await shared.cdp.call('Target.createBrowserContext');
  const isolated = await newPage(shared, `${origin}/isolated-context`, browserContextId);
  const isolatedCookie = await evaluate(shared.cdp, isolated, 'document.cookie');
  assert.equal(isolatedCookie.includes('kaze_probe'), false);
  const crossContext = await agent.call('Target.attachToTarget', { targetId: isolated.targetId, flatten: true });
  assert.equal(await evaluate(agent, crossContext, 'document.title'), 'Kaze fixture /isolated-context');
  report.observations.separateContexts = { cookiesSeparated: true, foreignContextReadableThroughBrowserEndpoint: true, isolatedConnection: false };

  const pageEndpoint = `ws://127.0.0.1:${new URL(shared.http).port}/devtools/page/${a.targetId}`;
  const pageAgent = await CdpPeer.connect(pageEndpoint);
  try {
    const listed = await pageAgent.call('Target.getTargets');
    const foreignFound = listed.targetInfos.some(t => t.targetId === b.targetId);
    let foreignReadable = false;
    try {
      const attached = await pageAgent.call('Target.attachToTarget', { targetId: b.targetId, flatten: true });
      foreignReadable = (await evaluate(pageAgent, attached, 'document.title')) === 'Kaze fixture /session-b';
    } catch (error) { report.observations.pageEndpointAttachError = error.message; }
    report.observations.pageEndpoint = { foreignWindowListed: foreignFound, foreignWindowReadable: foreignReadable, isolated: !foreignReadable && !foreignFound };
  } catch (error) { report.observations.pageEndpoint = { browserCommandsRejected: true, error: error.message }; }
  pageAgent.close();

  const separate = await launchBrowser('separate-session-browser');
  const separatePage = await newPage(separate, `${origin}/separate-session`);
  assert.equal((await evaluate(separate.cdp, separatePage, 'document.cookie')).includes('kaze_probe'), false);
  const separateTargets = (await separate.cdp.call('Target.getTargets')).targetInfos;
  assert.ok(!separateTargets.some(t => t.targetId === a.targetId || t.targetId === b.targetId));
  let attachRejected = false;
  try { await separate.cdp.call('Target.attachToTarget', { targetId: a.targetId, flatten: true }); } catch { attachRejected = true; }
  assert.equal(attachRejected, true);
  report.observations.separateInstances = { cookiesSeparated: true, foreignTargetsAbsent: true, foreignTargetAttachRejected: true, startupMs: separate.startupMs };

  if (process.platform === 'darwin') {
    report.observations.browserProcesses = [];
    for (const browser of [shared, separate]) {
      const info = await browser.cdp.call('SystemInfo.getProcessInfo');
      const pids = info.processInfo.map(p => p.id);
      const rows = execFileSync('/bin/ps', ['-p', pids.join(','), '-o', 'rss='], { encoding: 'utf8' });
      report.observations.browserProcesses.push({ label: browser.label, processCount: pids.length, rssKiB: rows.trim().split('\n').reduce((sum, line) => sum + Number(line.trim()), 0), note: 'RSS sum includes shared memory more than once; not physical footprint' });
    }
  }
  if (windows) {
    report.observations.browserProcesses = [];
    for (const browser of [shared, separate]) report.observations.browserProcesses.push({ label: browser.label, ...treeSummary(await sampleTree(browser)) });
    await snapshotWindows('isolation: shared and separate browsers open');
  }
  agent.close();
  await stopBrowser(separate);
  await stopBrowser(shared);
}

function summarize(result) {
  return { isError: !!result.isError, text: (result.content || []).filter(c => c.type === 'text').map(c => c.text).join('\n').slice(0, 14000), images: (result.content || []).filter(c => c.type === 'image').map(c => ({ mimeType: c.mimeType, bytes: Buffer.from(c.data, 'base64').length })) };
}

async function mcpProbe(kind, origin) {
  const observation = { kind, toolCalls: [], launches: 0 };
  report.observations[kind] = observation;
  let browser;
  let launching;
  const launch = async () => {
    if (!browser || browser.closed) {
      launching ??= (async () => {
        const next = await launchBrowser(`${kind}-lazy`);
        await newPage(next, `${origin}/${kind}-initial`);
        browser = next; observation.launches++;
        return next;
      })();
      await launching;
      launching = undefined;
    }
    return browser.ws;
  };
  const relay = await new FocusGuardRelay(launch).start(); relays.add(relay);
  const args = kind === 'playwright'
    ? [path.join(directory, 'node_modules/@playwright/mcp/cli.js'), '--cdp-endpoint', relay.url, '--output-dir', artifacts, '--timeout-navigation', '8000', '--timeout-action', '5000', '--no-webmcp']
    : [path.join(directory, 'node_modules/chrome-devtools-mcp/build/src/bin/chrome-devtools-mcp.js'), '--wsEndpoint', relay.url, '--no-usage-statistics', '--no-performance-crux', '--no-source-maps'];
  const mcp = new McpPeer(process.execPath, args, artifacts); mcps.add(mcp);
  observation.initialize = await mcp.initialize();
  const { tools } = await mcp.call('tools/list');
  await fs.writeFile(path.join(artifacts, `${kind}-tools.json`), JSON.stringify(tools, null, 2));
  observation.tools = tools.map(t => t.name);
  observation.afterDiscovery = { relayConnections: relay.connections, browserLaunches: observation.launches, nodeProcessRunning: mcp.process.exitCode === null };
  assert.equal(relay.connections, 0, `${kind} connected before its first tool call`);
  assert.equal(observation.launches, 0, `${kind} launched a browser during discovery`);
  console.log(kind, 'after discovery', JSON.stringify(observation.afterDiscovery));
  if (process.platform === 'darwin') {
    await delay(150);
    const cpuBefore = execFileSync('/bin/ps', ['-p', String(mcp.process.pid), '-o', 'time=,rss='], { encoding: 'utf8' }).trim();
    await delay(1000);
    const cpuAfter = execFileSync('/bin/ps', ['-p', String(mcp.process.pid), '-o', 'time=,rss='], { encoding: 'utf8' }).trim();
    observation.idleWorker = { intervalMs: 1000, cpuTimeAndRssBefore: cpuBefore, cpuTimeAndRssAfter: cpuAfter, browserLaunches: observation.launches, relayConnections: relay.connections };
    assert.equal(observation.launches, 0);
  }
  if (windows) {
    await delay(150);
    const [before] = await tools.tree(mcp.process.pid);
    await delay(1000);
    const [after] = await tools.tree(mcp.process.pid);
    observation.idleWorker = { intervalMs: 1000, cpuMsBefore: before?.cpuMs, cpuMsAfter: after?.cpuMs, workingSetBytes: after?.workingSetBytes, privateBytes: after?.privateBytes, browserLaunches: observation.launches, relayConnections: relay.connections };
    assert.equal(observation.launches, 0);
  }
  const call = async (name, args) => {
    const start = performance.now();
    const blockedBefore = relay.blocked.length;
    const result = await mcp.call('tools/call', { name, arguments: args });
    const summary = summarize(result);
    observation.toolCalls.push({ name, args, ms: Math.round(performance.now() - start), focusCommandsBlocked: relay.blocked.slice(blockedBefore), ...summary });
    console.log(kind, name, JSON.stringify({ ms: observation.toolCalls.at(-1).ms, ...summary }).slice(0, 2400));
    return summary;
  };
  if (kind === 'playwright') {
    const navigation = await call('browser_navigate', { url: `${origin}/playwright` });
    assert.equal(navigation.isError, false);
    const snapshot = await call('browser_snapshot', {});
    const input = snapshot.text.match(/textbox "Message" \[ref=([^\]]+)\]/)?.[1];
    const button = snapshot.text.match(/button "Apply" \[ref=([^\]]+)\]/)?.[1];
    assert.ok(input && button, 'fixture controls absent from snapshot');
    assert.equal((await call('browser_type', { target: input, text: 'playwright-applied' })).isError, false);
    assert.equal((await call('browser_click', { target: button })).isError, false);
    assert.match((await call('browser_snapshot', {})).text, /playwright-applied/);
    assert.match((await call('browser_console_messages', { level: 'error' })).text, /kaze-fixture-console-error/);
    assert.match((await call('browser_network_requests', { includeStatic: true })).text, /network-failure.*FAILED/);
    const screenshot = await call('browser_take_screenshot', { type: 'png', scale: 'css' });
    assert.ok(screenshot.images.some(image => image.bytes > 100));
    await call('browser_tabs', { action: 'list' });
    await call('browser_tabs', { action: 'new', url: `${origin}/playwright-new-page` });
    await call('browser_tabs', { action: 'select', index: 0 });
  } else {
    const listed = await call('list_pages', {});
    const pageId = Number(listed.text.match(/(?:^|\n)(\d+): /)?.[1]);
    assert.ok(Number.isInteger(pageId));
    assert.equal((await call('navigate_page', { pageId, type: 'url', url: `${origin}/devtools` })).isError, false);
    const snapshot = await call('take_snapshot', { pageId });
    const input = snapshot.text.match(/uid=([^\s]+) textbox "Message"/)?.[1];
    const button = snapshot.text.match(/uid=([^\s]+) button "Apply"/)?.[1];
    assert.ok(input && button, 'fixture controls absent from DevTools snapshot');
    assert.equal((await call('fill', { pageId, uid: input, value: 'devtools-applied' })).isError, false);
    assert.equal((await call('click', { pageId, uid: button })).isError, false);
    assert.match((await call('take_snapshot', { pageId })).text, /devtools-applied/);
    assert.match((await call('list_console_messages', { pageId, types: ['error'] })).text, /kaze-fixture-console-error/);
    assert.match((await call('list_network_requests', { pageId })).text, /network-failure/);
    const screenshot = await call('take_screenshot', { pageId, format: 'png' });
    assert.ok(screenshot.images.some(image => image.bytes > 100));
    await call('select_page', { pageId, bringToFront: true });
  }
  await snapshotWindows(`${kind}: after tool calls`);
  const foreign = await newPage(browser, `${origin}/foreign-window`);
  const foreignContext = await browser.cdp.call('Target.createBrowserContext');
  const foreignIsolated = await newPage(browser, `${origin}/foreign-context`, foreignContext.browserContextId);
  const allPages = kind === 'playwright' ? await call('browser_tabs', { action: 'list' }) : await call('list_pages', {});
  observation.unfilteredRelay = { foreignWindowListed: allPages.text.includes('/foreign-window'), foreignContextListed: allPages.text.includes('/foreign-context') };
  assert.equal(observation.unfilteredRelay.foreignWindowListed, true);
  await browser.cdp.call('Target.closeTarget', { targetId: foreign.targetId });
  await browser.cdp.call('Target.closeTarget', { targetId: foreignIsolated.targetId });
  await browser.cdp.call('Target.disposeBrowserContext', { browserContextId: foreignContext.browserContextId });

  const neighbor = await launchBrowser(`${kind}-neighbor`);
  await newPage(neighbor, `${origin}/neighbor-session`);
  const scopedPages = kind === 'playwright' ? await call('browser_tabs', { action: 'list' }) : await call('list_pages', {});
  observation.separateInstanceNeighborHidden = !scopedPages.text.includes('/neighbor-session');
  assert.equal(observation.separateInstanceNeighborHidden, true);
  await stopBrowser(neighbor);
  const beforeRestart = observation.launches;
  await stopBrowser(browser);
  await delay(200);
  const firstReconnect = kind === 'playwright'
    ? await call('browser_navigate', { url: `${origin}/playwright-reconnected` })
    : await call('list_pages', {});
  observation.reconnect = { firstCallSucceeded: !firstReconnect.isError, browserRelaunched: observation.launches > beforeRestart };
  if (firstReconnect.isError) {
    const retry = kind === 'playwright'
      ? await call('browser_navigate', { url: `${origin}/playwright-reconnected` })
      : await call('list_pages', {});
    observation.reconnect.retrySucceeded = !retry.isError;
    observation.reconnect.browserRelaunched = observation.launches > beforeRestart;
  }
  observation.connections = relay.connections;
  observation.focusCommandsBlocked = relay.blocked;
  observation.cdpMethods = [...new Set(relay.commands)];
  await mcp.close(); mcps.delete(mcp);
  if (browser) await stopBrowser(browser);
  await relay.close(); relays.delete(relay);
}

async function waitForEvent(peer, mark, match, timeoutMs) {
  const deadline = performance.now() + timeoutMs;
  while (performance.now() < deadline) {
    const found = peer.events.slice(mark).find(match);
    if (found) return found;
    await delay(50);
  }
  return undefined;
}

// Page-initiated windows do not pass through the relay's createTarget rewrite: does a popup activate the browser?
async function popupScenario(origin) {
  const browser = await launchBrowser('popup');
  const opener = await newPage(browser, `${origin}/popup-opener`);
  const seen = new Set((await browser.cdp.call('Target.getTargets')).targetInfos.map(t => t.targetId));
  const cases = [
    ['tab', true, "window.open('/popup-tab', '_blank')"],
    ['window', true, "window.open('/popup-window', 'kazepopup', 'popup,width=420,height=320')"],
    ['windowWithoutGesture', false, "window.open('/popup-no-gesture', 'kazepopup2', 'popup,width=420,height=320')"],
    ['openerCallsWindowFocus', true, 'window.focus()']
  ];
  const results = {};
  for (const [name, userGesture, expression] of cases) {
    const startedAt = Date.now();
    const value = await browser.cdp.call('Runtime.evaluate', { expression: `(() => { const r = ${expression}; return r === undefined ? 'undefined' : !!r; })()`, userGesture, returnByValue: true }, opener.sessionId);
    await delay(1500);
    await sampleTree(browser);
    const targets = (await browser.cdp.call('Target.getTargets')).targetInfos.filter(t => t.type === 'page' && !seen.has(t.targetId));
    for (const t of targets) seen.add(t.targetId);
    results[name] = { userGesture, returned: value.result.value, newPages: targets.map(t => ({ url: t.url.replace(origin, ''), hasOpener: !!t.openerId })), startedAt, endedAt: Date.now(), windows: await snapshotWindows(`popup: ${name}`) };
  }
  report.observations.popup = results;
  await stopBrowser(browser);
}

async function dialogScenario(origin) {
  const browser = await launchBrowser('dialogs');
  const page = await newPage(browser, `${origin}/dialogs`);
  const results = {};
  for (const kind of ['alert', 'confirm', 'prompt']) {
    const startedAt = Date.now();
    const mark = browser.cdp.events.length;
    await evaluate(browser.cdp, page, `setTimeout(() => { window.__kazeDialog = ${kind}('kaze ${kind}'); }, 50); true`);
    const opened = await waitForEvent(browser.cdp, mark, e => e.method === 'Page.javascriptDialogOpening' && e.sessionId === page.sessionId, 5000);
    await delay(1500); // give the dialog time to raise or flash its window, if it is going to
    const windowsWhileOpen = await snapshotWindows(`dialogs: ${kind} open`);
    let handled = false;
    if (opened) { await browser.cdp.call('Page.handleJavaScriptDialog', { accept: kind !== 'confirm', promptText: 'kaze' }, page.sessionId); handled = true; }
    results[kind] = { opened: !!opened, type: opened?.params.type, handled, valueAfter: await evaluate(browser.cdp, page, 'String(window.__kazeDialog)'), startedAt, endedAt: Date.now(), windows: windowsWhileOpen };
  }
  report.observations.dialogs = results;
  await stopBrowser(browser);
}

// Downloads go to this run's temporary profile (never the owner's Downloads folder).
async function downloadScenario(origin) {
  const browser = await launchBrowser('download');
  const downloadPath = path.join(browser.profile, 'downloads');
  await fs.mkdir(downloadPath, { recursive: true });
  await browser.cdp.call('Browser.setDownloadBehavior', { behavior: 'allow', downloadPath, eventsEnabled: true });
  const page = await newPage(browser, `${origin}/download-page`);
  const mark = browser.cdp.events.length;
  const startedAt = Date.now();
  await browser.cdp.call('Runtime.evaluate', { expression: "(() => { const a = document.createElement('a'); a.href = '/download-file'; a.download = 'kaze-spike.txt'; document.body.append(a); a.click(); return true; })()", userGesture: true, returnByValue: true }, page.sessionId);
  const begin = await waitForEvent(browser.cdp, mark, e => e.method === 'Browser.downloadWillBegin', 5000);
  const done = await waitForEvent(browser.cdp, mark, e => e.method === 'Browser.downloadProgress' && ['completed', 'canceled'].includes(e.params.state), 8000);
  await delay(1500); // a download bubble or shelf, if any, shows up here
  const files = await fs.readdir(downloadPath).catch(() => []);
  report.observations.download = { began: !!begin, state: done?.params.state ?? 'none', files, behavior: 'allow (temporary profile folder)', startedAt, endedAt: Date.now(), windows: await snapshotWindows('download: after completion') };
  await stopBrowser(browser);
}

// Per-instance memory for one and for three separate browser instances, each with one fixture page.
async function memoryScenario(origin) {
  if (!tools) { report.observations.memory = { skipped: 'Windows-only measurement' }; return; }
  const measure = async item => ({ label: item.label, ...treeSummary(await sampleTree(item)) });
  const first = await launchBrowser('memory-1');
  await newPage(first, `${origin}/memory-1`);
  await delay(2000);
  const one = await measure(first);
  const others = [];
  for (const n of [2, 3]) { const item = await launchBrowser(`memory-${n}`); await newPage(item, `${origin}/memory-${n}`); others.push(item); }
  await delay(2000);
  const three = [];
  for (const item of [first, ...others]) three.push(await measure(item));
  report.observations.memory = {
    oneInstance: one,
    threeInstances: { perInstance: three, workingSetBytes: three.reduce((s, r) => s + r.workingSetBytes, 0), privateBytes: three.reduce((s, r) => s + r.privateBytes, 0) },
    note: 'Each instance has one fixture page open, sampled 2 s after load'
  };
  for (const item of [...others, first]) await stopBrowser(item);
}

try {
  runRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'kaze-browser-spike-'));
  if (windows) {
    report.browserName = browserName;
    report.browserExecutable = executable();
    report.launchVariant = launchVariant;
    tools = new WinTools();
    monitor = startForegroundMonitor(focusEvents);
    for (let n = 0; n < 300 && !focusEvents.length; n++) await delay(50);
    assert.ok(focusEvents.length > 0, `native foreground observer did not report its initial state: ${monitor.stderrText}`);
    report.focusBaseline = focusEvents[0];
    // Keep learning this run's browser child PIDs while they live (Edge and Chrome spawn helpers over time).
    sampler = setInterval(() => { for (const item of browsers) sampleTree(item).catch(() => {}); }, 1000);
  }
  if (process.platform === 'darwin') {
    monitor = spawn(path.join(artifacts, 'focus-monitor'), [], { stdio: ['ignore', 'pipe', 'pipe'] });
    let buffer = '';
    monitor.stdout.on('data', chunk => {
      buffer += chunk;
      for (;;) { const end = buffer.indexOf('\n'); if (end < 0) break; focusEvents.push(JSON.parse(buffer.slice(0, end))); buffer = buffer.slice(end + 1); }
    });
    await delay(150);
    assert.ok(focusEvents.length > 0, 'native focus observer did not report its initial state');
  }
  fixture = createServer((request, response) => {
    if (request.url.startsWith('/network-failure')) { request.socket.destroy(); return; }
    if (request.url === '/favicon.ico') { response.writeHead(204); response.end(); return; }
    if (request.url.startsWith('/download-file')) {
      response.writeHead(200, { 'Content-Type': 'application/octet-stream', 'Content-Disposition': 'attachment; filename="kaze-spike.txt"', 'Cache-Control': 'no-store' });
      response.end('kaze agent browser spike download\n');
      return;
    }
    response.writeHead(200, { 'Content-Type': 'text/html', 'Cache-Control': 'no-store' });
    response.end(`<!doctype html><title>Kaze fixture ${request.url}</title><h1>Kaze browser fixture</h1><label>Message <input id="message"></label><button id="apply">Apply</button><p id="result" role="status">Waiting</p><script>document.querySelector('#apply').onclick=()=>document.querySelector('#result').textContent=document.querySelector('#message').value||'Applied';console.error('kaze-fixture-console-error');fetch('/network-failure').catch(()=>{});</script>`);
  });
  fixture.listen(0, '127.0.0.1'); await once(fixture, 'listening');
  const origin = `http://127.0.0.1:${fixture.address().port}`;
  report.fixtureOrigin = origin;
  for (const phase of phases) {
    console.log('PHASE', phase);
    if (phase === 'isolation') await scenario('isolation', () => isolation(origin));
    else if (phase === 'mcp') { for (const kind of ['playwright', 'devtools']) await scenario(kind, () => mcpProbe(kind, origin)); }
    else if (phase === 'popup') await scenario('popup', () => popupScenario(origin));
    else if (phase === 'dialogs') await scenario('dialogs', () => dialogScenario(origin));
    else if (phase === 'download') await scenario('download', () => downloadScenario(origin));
    else if (phase === 'memory') await scenario('memory', () => memoryScenario(origin));
  }
} catch (error) {
  report.error = error.stack;
  console.error(error);
  process.exitCode = 1;
} finally {
  if (sampler) clearInterval(sampler);
  for (const mcp of mcps) await mcp.close().catch(() => {});
  for (const relay of relays) await relay.close().catch(() => {});
  for (const browser of browsers) await stopBrowser(browser).catch(() => {});
  if (fixture) await new Promise(resolve => fixture.close(resolve));
  if (windows && tools) {
    // Cleanup check: anything from this run's private profile root still running after finally?
    await delay(2000);
    const leftovers = runRoot ? await tools.leftovers(runRoot).catch(error => [{ error: error.message }]) : [];
    const stillAlive = [];
    for (const [pid] of owned) { try { process.kill(pid, 0); stillAlive.push(pid); } catch {} }
    report.cleanup = { leftoverBrowserProcesses: leftovers, ownedPidsStillAlive: stillAlive, ownedPidCount: owned.size, note: 'ownedPidsStillAlive can include a PID already reused by an unrelated process; leftoverBrowserProcesses matches this run profile root in the command line' };
    if (leftovers.length && !leftovers[0].error) {
      report.error ??= 'browser processes from this run survived cleanup';
      process.exitCode = 1;
      await tools.leftovers(runRoot, true).catch(() => {}); // only processes whose command line names this run's profile root
    }
  }
  if (runRoot) {
    await fs.rm(runRoot, { recursive: true, force: true, maxRetries: 20, retryDelay: 250 }).catch(() => {});
    if (report.cleanup) report.cleanup.profileRootRemoved = !(await fs.stat(runRoot).then(() => true, () => false));
  }
  if (monitor) {
    report.focusMonitor = { baselineReceived: focusEvents.length > 0, runningUntilCleanup: monitor.exitCode === null };
    if (!report.focusMonitor.runningUntilCleanup) { report.error ??= 'native focus observer exited before cleanup'; process.exitCode = 1; }
    if (windows) await monitor.stop(); else { monitor.kill('SIGTERM'); await delay(100); }
  }
  if (tools) await tools.close();
  if (windows) {
    // Foreground changes go in focusEvents (as on macOS); "show" lines are kept only for this run's browser.
    const shows = focusEvents.filter(event => event.kind === 'show');
    report.browserWindowShows = shows.filter(event => ownedEvent(event, owned));
    report.focusEvents = focusEvents.filter(event => event.kind !== 'show');
    report.chromeActivationEvents = report.focusEvents.filter(event => event.kind === 'foreground' && ownedEvent(event, owned));
    report.scenarios = Object.fromEntries(Object.entries(scenarioSpans).map(([name, span]) => {
      const inSpan = event => event.at_ms >= span.startedAt && event.at_ms <= span.endedAt + 1000;
      return [name, { ...span, activationEvents: report.chromeActivationEvents.filter(inSpan).length, browserWindowShows: report.browserWindowShows.filter(inSpan).length }];
    }));
    report.windowSnapshots = windowSnapshots;
  } else {
    report.focusEvents = focusEvents;
    const ownedPids = new Set(report.browserInstances.map(browser => browser.pid));
    report.chromeActivationEvents = focusEvents.filter(event => ownedPids.has(event.pid));
  }
  if (report.chromeActivationEvents.length) { report.error ??= 'test browser became the foreground window'; process.exitCode = 1; }
  await fs.writeFile(path.join(artifacts, `${phases.join('-')}-results.json`), JSON.stringify(report, null, 2) + '\n');
  if (windows) {
    const suffix = browserName === 'edge' && launchVariant === 'node-default' && !askedPhases.length ? '' : `-${browserName}-${launchVariant}${askedPhases.length ? `-${askedPhases.join('-')}` : ''}`;
    const out = path.join(directory, 'results', `windows-${os.arch()}${suffix}.json`);
    await fs.writeFile(out, JSON.stringify(report, null, 2) + '\n');
    console.log('Wrote', out);
  }
  console.log('RESULT', JSON.stringify(report.observations, null, 2));
  console.log('Chrome activation events:', report.chromeActivationEvents.length);
  if (windows) console.log('Per scenario:', JSON.stringify(Object.fromEntries(Object.entries(report.scenarios).map(([k, v]) => [k, { ok: v.ok, activations: v.activationEvents, windowShows: v.browserWindowShows }]))));
}
