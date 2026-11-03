// Dry run of the Windows helpers without launching a browser: observer baseline, tool requests, clean exit.
import assert from 'node:assert/strict';
import { startForegroundMonitor, WinTools, treeSummary, ownedEvent } from './win.mjs';
const delay = ms => new Promise(r => setTimeout(r, ms));
const events = [];
const monitor = startForegroundMonitor(events);
const tools = new WinTools();
try {
  const seconds = Number(process.env.KAZE_SPIKE_DRYRUN_SECONDS || 4);
  for (let n = 0; n < 200 && !events.length; n++) await delay(50);
  assert.ok(events.length && events[0].kind === 'baseline', `no baseline: ${monitor.stderrText}`);
  console.log('baseline', JSON.stringify(events[0]));
  await delay(seconds * 1000);
  const tree = await tools.tree(process.pid);
  assert.equal(tree[0].pid, process.pid);
  console.log('tree(self)', JSON.stringify(treeSummary(tree)), tree.map(r => r.name).join(','));
  console.log('windows(self)', JSON.stringify(await tools.windows([process.pid])));
  const leftovers = await tools.leftovers('kaze-browser-spike-dry-run-no-match');
  assert.deepEqual(leftovers, []);
  console.log('leftovers(no match)', JSON.stringify(leftovers));
  const owned = new Map([[42, { from: 1000, to: 2000 }]]);
  assert.equal(ownedEvent({ pid: 42, at_ms: 1500 }, owned), true);
  assert.equal(ownedEvent({ pid: 7, ancestors: [42], at_ms: 1500 }, owned), true);
  assert.equal(ownedEvent({ pid: 42, at_ms: 9000 }, owned), false);
  console.log('events during dry run:', events.length, events.map(e => e.kind).join(','));
} finally {
  await tools.close();
  await monitor.stop();
  console.log('monitor exit', monitor.exitCode, 'tools exit', tools.process.exitCode);
}
