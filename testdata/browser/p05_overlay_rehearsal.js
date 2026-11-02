'use strict';
// Actual overlay-script P05 fixture; Node VM is not rendered-browser evidence.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const script = fs.readFileSync(path.join(__dirname, '..', '..', 'web', 'overlay', 'app.js'), 'utf8');
function fakeNode(id) {
  return { id, textContent: '', hidden: false, children: [], replaceChildren(...items) { this.children = items; }, appendChild(item) { this.children.push(item); } };
}
const nodes = new Map();
const node = (id) => { if (!nodes.has(id)) nodes.set(id, fakeNode(id)); return nodes.get(id); };
let stateListener = null;
let errorListener = null;
const sandbox = {
  document: { getElementById: node, createElement: () => fakeNode('') },
  EventSource: class { addEventListener(type, fn) { if (type === 'state') stateListener = fn; if (type === 'error') errorListener = fn; } },
  console,
};
vm.runInNewContext(script, sandbox, { filename: 'overlay-p05.js' });
const failures = [];
const rehearsal = {
  mode: 'rehearsal', demo_label: 'REHEARSAL — simulated data', service_status: 'ready',
  incidents: [{ public_id: 'CM-DEMO', category: 'audio', stage: 'awaiting_approval', feedback_summary: '1 positive / 0 negative', closure_outcome: '' }],
};
stateListener({ data: JSON.stringify(rehearsal) });
const text = (n) => [n.textContent, ...(n.children || []).map(text)].join(' ');
if (node('demo').hidden || !text(node('demo')).includes('simulated data')) failures.push('rehearsal watermark missing');
if (!text(node('incidents')).includes('CM-DEMO') || !text(node('incidents')).includes('audio')) failures.push('public code/category missing');
if (text(node('incidents')).includes('Rehearsal Mic') || text(node('incidents')).includes('rehearsal-mic')) failures.push('private evidence leaked');
if (!text(node('incidents')).includes('Awaiting private approval')) failures.push('fixed stage copy missing');
if (node('panel').hidden) failures.push('active rehearsal panel hidden');
stateListener({ data: JSON.stringify({ mode: 'live', service_status: 'idle', incidents: [] }) });
if (!node('panel').hidden) failures.push('idle live overlay not hidden');
if (!node('demo').hidden) failures.push('rehearsal watermark persisted into live mode');
errorListener();
if (node('panel').hidden) failures.push('connection error left idle overlay hidden');
if (!text(node('status')).includes('Not connected')) failures.push('connection error did not show disconnected status');
console.log(JSON.stringify({ failures }, null, 2));
if (failures.length) process.exitCode = 1;
