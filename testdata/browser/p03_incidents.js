'use strict';
// Actual control-script P03 fixture. This is a Node VM, not a rendered browser.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '..', '..', 'web', 'control', 'app.js'), 'utf8');

class FakeOption {
  constructor(label, value) { this.label = label; this.value = value; this.dataset = {}; this.disabled = false; }
}
function fakeNode(id) {
  return {
    id, value: '', textContent: '', title: '', className: '', hidden: false, disabled: false,
    checked: false, options: [], children: [], handlers: {}, dataset: {},
    addEventListener(type, fn) { this.handlers[type] = fn; },
    replaceChildren(...items) { this.options = items; this.children = []; },
    appendChild(item) { this.children.push(item); },
    add(item) { this.options.push(item); },
    get selectedOptions() { return this.options.length ? [this.options[0]] : []; }
  };
}
const nodes = new Map();
function node(id) { if (!nodes.has(id)) nodes.set(id, fakeNode(id)); return nodes.get(id); }

const snapshot = {
  mode: 'live', state_revision: 3, csrf_token: 'FIXTURE_CSRF',
  settings: { revision: 2, microphone: { name: 'Mic/Aux', uuid: 'mic' }, stream_track: 2, track_confirmed_at: '2026-11-01T16:00:00Z', visual: null },
  secrets: { mode: 'memory_only', available: true },
  monitoring_active: true,
  monitoring_session: { id: 'sess-fixture', active: true, started_at: '2026-11-01T16:00:00Z' },
  twitch: { status: 'connected' },
  obs: { status: 'connected', stale: false, observed_at: '2026-11-01T16:00:01Z', broadcasting: false },
  setup: { live_obs_resolution_ready: true },
  obs_discovery: { generation: 1, scene_collection: 'Main', captured_at: '2026-11-01T16:00:00Z', inputs: [], scenes: [] },
  incidents: [{
    id: 'inc-fixture', public_id: 'CM-ABC123', category: 'audio', stage: 'awaiting_approval',
    report_count: 2, distinct_reporters: 2, local_verification: 'not_attempted',
    feedback: { positive: 0, negative: 0 },
    findings: [{ code: 'MIC_MUTED', manual_guidance: 'Unmute manually.' }, { code: 'MIC_TRACK_MISSING' }]
  }]
};
let requests = [];
let stateListener = null;
const sandbox = {
  document: { getElementById: node, createElement: () => fakeNode('') },
  location: { hash: '', pathname: '/control/', search: '' },
  history: { replaceState() {} },
  URLSearchParams, Option: FakeOption,
  EventSource: class { addEventListener(type, fn) { if (type === 'state') stateListener = fn; } },
  fetch: async (url, options = {}) => {
    requests.push({ url, method: (options.method || 'GET'), body: options.body || '', csrf: (options.headers || {})['X-CSRF-Token'] });
    if (url === '/api/v1/state') return { ok: true, status: 200, json: async () => ({ data: snapshot }) };
    if (url === '/api/v1/settings') return { ok: true, status: 200, json: async () => ({ data: snapshot.settings }) };
    if (url === '/api/v1/monitoring/stop') return { ok: true, status: 200, json: async () => ({ data: {} }) };
    if (url === '/api/v1/monitoring/start') return { ok: true, status: 200, json: async () => ({ data: { session_id: 'sess-fixture' } }) };
    if (url === '/api/v1/incidents/check') return { ok: true, status: 202, json: async () => ({ data: { incident_id: 'inc-fixture', stage: 'investigating' } }) };
    if (url.endsWith('/dismiss')) return { ok: true, status: 200, json: async () => ({ data: { incident_id: 'inc-fixture', stage: 'dismissed' } }) };
    throw new Error('unexpected fetch ' + url);
  },
  console
};
vm.runInNewContext(script, sandbox, { filename: 'app.js' });

setImmediate(async () => {
  const problems = [];
  if (!/Monitoring active/.test(node('monitor-status').textContent)) problems.push('monitoring status not rendered');
  const list = node('incident-list');
  if (list.children.length !== 1) problems.push('incident card not rendered');
  const card = list.children[0];
  const buttonNodes = card.children.filter((child) => child.className === 'actions')[0];
  if (!buttonNodes || buttonNodes.children.length !== 2) problems.push('recheck/dismiss controls missing');
  await node('monitor-stop').handlers.click();
  await node('monitor-start').handlers.click();
  await node('manual-audio').handlers.click();
  if (buttonNodes && buttonNodes.children.length === 2) {
    await buttonNodes.children[0].handlers.click();
    await buttonNodes.children[1].handlers.click();
  }
  const paths = requests.map((request) => request.method + ' ' + request.url);
  const expected = [
    'POST /api/v1/monitoring/stop', 'POST /api/v1/monitoring/start',
    'POST /api/v1/incidents/check', 'POST /api/v1/incidents/check',
    'POST /api/v1/incidents/inc-fixture/dismiss'
  ];
  for (const want of expected) if (!paths.includes(want)) problems.push('missing ' + want);
  if (requests.some((request) => request.method !== 'GET' && request.csrf !== 'FIXTURE_CSRF')) problems.push('CSRF missing on mutation');
  stateListener({ data: JSON.stringify({ ...snapshot, state_revision: 4 }) });
  if (node('monitor-status').hidden) problems.push('SSE refresh hid app');
  console.log(JSON.stringify({ requests: paths, problems }, null, 2));
  if (problems.length) process.exitCode = 1;
});
