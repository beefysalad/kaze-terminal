// Windows-only helpers for probe.mjs: the passive foreground observer and the request/response tool process.
import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const directory = path.dirname(fileURLToPath(import.meta.url));
const powershell = process.env.KAZE_SPIKE_PWSH || 'pwsh.exe';
const psArgs = file => ['-NoLogo', '-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', path.join(directory, file)];

// Show commands passed through STARTUPINFO.wShowWindow by the "launch" tool.
export const SHOW = { shownoactivate: 4, showminnoactive: 7 };

function lines(stream, onLine) {
  let buffer = '';
  stream.setEncoding('utf8');
  stream.on('data', chunk => {
    buffer += chunk;
    if (buffer.length > 4000000) buffer = '';
    for (;;) {
      const end = buffer.indexOf('\n');
      if (end < 0) break;
      const line = buffer.slice(0, end).trim();
      buffer = buffer.slice(end + 1);
      if (line) onLine(line);
    }
  });
}

// Starts focus-monitor-win.ps1. Events are pushed into `events` as they arrive.
export function startForegroundMonitor(events) {
  const child = spawn(powershell, [...psArgs('focus-monitor-win.ps1'), '-ExitOnStdinClose'], { stdio: ['pipe', 'pipe', 'pipe'], windowsHide: true });
  child.stderrText = '';
  child.stderr.on('data', data => { child.stderrText = (child.stderrText + data).slice(-8000); });
  lines(child.stdout, line => { try { events.push(JSON.parse(line)); } catch {} });
  child.stop = async () => {
    child.stdin.end();
    for (let n = 0; n < 40 && child.exitCode === null; n++) await new Promise(r => setTimeout(r, 50));
    if (child.exitCode === null) child.kill();
  };
  return child;
}

export class WinTools {
  constructor() {
    this.next = 0;
    this.pending = new Map();
    this.process = spawn(powershell, psArgs('win-tools.ps1'), { stdio: ['pipe', 'pipe', 'pipe'], windowsHide: true });
    this.stderr = '';
    this.process.stderr.on('data', data => { this.stderr = (this.stderr + data).slice(-8000); });
    lines(this.process.stdout, line => {
      let message;
      try { message = JSON.parse(line); } catch { return; }
      const pending = this.pending.get(message.id);
      if (!pending) return;
      clearTimeout(pending.timer);
      this.pending.delete(message.id);
      if (message.ok) pending.resolve(message.result); else pending.reject(new Error(message.error));
    });
    this.process.on('exit', () => {
      for (const pending of this.pending.values()) { clearTimeout(pending.timer); pending.reject(new Error(`win-tools exited: ${this.stderr.slice(-1000)}`)); }
      this.pending.clear();
    });
  }
  call(cmd, params = {}, timeoutMs = 20000) {
    const id = ++this.next;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error(`win-tools timeout: ${cmd}`)); }, timeoutMs);
      this.pending.set(id, { resolve, reject, timer });
      this.process.stdin.write(JSON.stringify({ id, cmd, ...params }) + '\n');
    });
  }
  launch(exe, args, show) { return this.call('launch', { exe, args, show }); }
  launchWmi(cmdline, show) { return this.call('launchwmi', { cmdline, show }); }
  async tree(pid) {
    const rows = await this.call('tree', { pid });
    return Array.isArray(rows) ? rows : rows ? [rows] : [];
  }
  async windows(pids) {
    const rows = await this.call('windows', { pids: [...pids] });
    return Array.isArray(rows) ? rows : rows ? [rows] : [];
  }
  async leftovers(marker, kill = false) {
    const rows = await this.call('leftovers', { marker, kill }, 30000);
    return Array.isArray(rows) ? rows : rows ? [rows] : [];
  }
  async close() {
    this.process.stdin.end();
    for (let n = 0; n < 40 && this.process.exitCode === null; n++) await new Promise(r => setTimeout(r, 50));
    if (this.process.exitCode === null) this.process.kill();
  }
}

export function treeSummary(rows) {
  return {
    processCount: rows.length,
    workingSetBytes: rows.reduce((sum, row) => sum + Number(row.workingSetBytes || 0), 0),
    privateBytes: rows.reduce((sum, row) => sum + Number(row.privateBytes || 0), 0),
    note: 'Working-set sum counts shared pages once per process; private bytes are closer to the per-instance cost'
  };
}

// Is this event's process one of ours: a known browser PID, or a descendant of one (via the observer's
// ancestor chain)? `owned` maps pid -> { from, to } so a PID reused after our browser exited does not count.
export function ownedEvent(event, owned) {
  const live = pid => {
    const span = owned.get(pid);
    return !!span && event.at_ms >= span.from - 1000 && event.at_ms <= (span.to ?? Infinity) + 2000;
  };
  return live(event.pid) || (event.ancestors || []).some(live);
}
