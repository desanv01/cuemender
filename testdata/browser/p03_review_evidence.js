'use strict';
// Review-only actual-script evidence checks; this is not a rendered browser.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const fixture = fs.readFileSync(path.join(__dirname, 'p03_incidents.js'), 'utf8');
const prefix = fixture.split('setImmediate(async () => {')[0];
const checks = String.raw`
setImmediate(() => {
  const failures = [];
  const allText = (n) => [n.textContent, ...(n.children || []).map(allText)].join(' ');
  const before = allText(node('incident-list'));
  if (!before.includes('Unmute manually.')) failures.push('per-finding manual guidance is missing');
  const unknown = {...snapshot.incidents[0], stage:'insufficient_information', findings:[],
    checks:[{name:'obs_evidence',category:'audio',status:'unknown',explanation_code:'obs_unavailable',unavailable_reason:'Fresh OBS evidence is unavailable.'}]};
  stateListener({data:JSON.stringify({...snapshot,incidents:[unknown]})});
  const rendered = allText(node('incident-list'));
  if (rendered.includes('no supported conflict')) failures.push('unknown evidence is rendered as no supported conflict');
  if (!rendered.includes('Fresh OBS evidence is unavailable.')) failures.push('evidence limitation is omitted');
  console.log(JSON.stringify({failures},null,2));
  if(failures.length) process.exitCode=1;
});`;
vm.runInNewContext(prefix + checks, {require, __dirname, console, setImmediate, process, URLSearchParams}, {filename:'p03-review-evidence-harness.js'});
