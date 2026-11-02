'use strict';
// Actual control-script P04 fixture. This is a Node VM, not a rendered browser.
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
    session_id: 'sess-fixture', origin: 'viewer', evidence_at: '2026-11-01T16:00:01Z',
    report_count: 2, distinct_reporters: 2, local_verification: 'not_attempted',
    feedback: { positive: 0, negative: 0 },
    findings: [{ code: 'MIC_MUTED', manual_guidance: 'Unmute manually.' }, { code: 'MIC_TRACK_MISSING' }]
  }]
};
snapshot.proposals = [{ id: 'proposal-fixture', incident_id: 'inc-fixture', session_id: 'sess-fixture', mode: 'live', input: { name: '<private mic>', uuid: 'mic', context: 'Collection' }, selected_track: 2, expected_tracks: { track1: true, track2: false, track3: false, track4: true, track5: false, track6: true }, config_revision: 2, expires_at: new Date(Date.now() + 60000).toISOString(), status: 'proposed' }];
snapshot.actions = [];
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
    if (url.endsWith('/approve')) return { ok: true, status: 202, json: async () => ({ data: { id: 'action-fixture', status: 'reserved' } }) };
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
  function allNodes(root) { return [root, ...root.children.flatMap(allNodes)]; }
  const controls = () => allNodes(node('incident-list'));
  let approve = controls().find((entry) => entry.textContent === 'Approve track addition');
  if (!approve || approve.disabled) problems.push('eligible approval missing or disabled');
  const text = controls().map((entry) => entry.textContent).join(' ');
  if (!text.includes('<private mic>') || !text.includes('track1=true') || !text.includes('Other findings remain')) problems.push('exact target/preservation/remaining findings copy missing');
  if (approve) await approve.handlers.click();
  const post = requests.find((entry) => entry.url.endsWith('/approve'));
  if (!post || post.csrf !== 'FIXTURE_CSRF' || post.body !== JSON.stringify({ expected_revision: 2 })) problems.push('approval did not send only revision with CSRF');
  snapshot.proposals[0].expires_at = new Date(Date.now() - 1).toISOString(); snapshot.state_revision++;
  stateListener({ data: JSON.stringify(snapshot) });
  approve = controls().find((entry) => entry.textContent === 'Approve track addition');
  if (!approve || !approve.disabled) problems.push('expired approval not disabled');
  snapshot.actions = [{ id: 'action-fixture', incident_id: 'inc-fixture', status: 'unknown', detail_code: 'write_outcome_unknown', created_at: new Date().toISOString() }]; snapshot.proposals[0].status = 'reserved'; snapshot.incidents[0].stage = 'action_unknown'; snapshot.state_revision++;
  stateListener({ data: JSON.stringify(snapshot) });
  if (controls().some((entry) => ['Recheck', 'Dismiss', 'Approve track addition'].includes(entry.textContent))) problems.push('pending unknown action can be discarded or replayed');
  snapshot.actions[0].status = 'verified'; snapshot.actions[0].reconciled = true; snapshot.incidents[0].stage = 'awaiting_feedback'; snapshot.incidents[0].local_verification = 'verified'; snapshot.state_revision++;
  stateListener({ data: JSON.stringify(snapshot) });
  if (!controls().some((entry) => entry.textContent.includes('does not establish who changed it or viewer recovery'))) problems.push('reconciled outcome falsely attributes recovery');
  console.log(JSON.stringify({ fixture: 'P04 actual control script', problems }, null, 2));
  if (problems.length) process.exitCode = 1;
});
