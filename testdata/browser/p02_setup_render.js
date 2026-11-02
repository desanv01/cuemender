'use strict';

// Actual control-script rendering fixture for P02 private state. This is a
// Node VM with DOM/fetch/EventSource stubs, not a rendered-browser test.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const script = fs.readFileSync(path.join(__dirname, '..', '..', 'web', 'control', 'app.js'), 'utf8');
const nodes = new Map();
let failure = null;

class FakeOption {
  constructor(label, value) {
    this.label = label;
    this.value = value;
    this.dataset = {};
    this.disabled = false;
  }
}

function node(id) {
  if (!nodes.has(id)) {
    nodes.set(id, {
      value: '', textContent: '', title: '', className: '', hidden: false,
      disabled: false, checked: false, options: [], handlers: {},
      addEventListener(type, fn) { this.handlers[type] = fn; },
      replaceChildren(...items) { this.options = items; },
      add(item) { this.options.push(item); },
      get selectedOptions() {
        const match = this.options.find((option) => option.value === this.value);
        return match ? [match] : (this.options.length ? [this.options[0]] : []);
      }
    });
  }
  return nodes.get(id);
}

let stateListener = null;

const snapshot = {
  mode: 'live',
  state_revision: 4,
  csrf_token: 'FIXTURE_CSRF',
  settings: {
    revision: 1,
    microphone: { name: 'Mic/Aux', uuid: 'mic-uuid', context: 'Main Collection' },
    stream_track: 2,
    visual: {
      scene: 'Main', scene_uuid: 'scene-uuid', scene_item_id: '7',
      source: { name: 'Camera', uuid: 'camera-uuid', context: 'Main Collection' }
    }
  },
  secrets: { mode: 'memory_only', available: true },
  twitch: {
    status: 'authorizing',
    detail: 'Complete the device authorization on Twitch.',
    device: {
      user_code: 'ABCD-1234',
      verification_uri: 'https://www.twitch.tv/activate',
      expires_at: '2026-11-01T12:10:00Z'
    },
    receipt: { accepted: 2, last_command: 'issue_audio', last_category: 'audio', last_at: '2026-11-01T12:00:00Z' }
  },
  obs: {
    status: 'connected', detail: 'OBS is connected.', broadcasting: true,
    stale: false, generation: 3, observed_at: '2026-11-01T12:00:00Z'
  },
  startup: { obs_address: 'ws://127.0.0.1:4455' },
  obs_discovery: {
    generation: 3,
    scene_collection: 'Main Collection',
    captured_at: '2026-11-01T12:00:00Z',
    inputs: [{ name: 'Mic/Aux', uuid: 'mic-uuid', kind: 'wasapi_input_capture' }],
    scenes: [{ name: 'Main', uuid: 'scene-uuid', items: [{ id: 7, source_name: 'Camera', source_uuid: 'camera-uuid', enabled: true, is_group: false }] }]
  },
  obs_observation: {
    generation: 3,
    config_revision: 1,
    observed_at: '2026-11-01T12:00:01Z',
    mute: false,
    tracks: { track1: false, track2: true, track3: false, track4: false, track5: false, track6: false },
    program_scene: 'Main',
    visual_item_enabled: true,
    streaming: true
  }
};

let receiveOnlyRequest = null;
let postedSettings = null;
let disconnected = false;
let reloadSavedIdentity = false;
let reloadSaveRevision = null;
const sandbox = {
  document: { getElementById: node },
  location: { hash: '', pathname: '/control/', search: '' },
  history: { replaceState() {} },
  URLSearchParams,
  Option: FakeOption,
  EventSource: class {
    addEventListener(type, fn) { if (type === 'state') stateListener = fn; }
  },
  fetch: async (url, options = {}) => {
    if (url === '/api/v1/state') {
      return { ok: true, status: 200, json: async () => ({ data: snapshot }) };
    }
    if (url === '/api/v1/settings') {
      postedSettings = JSON.parse(options.body);
      if (options.headers['X-CSRF-Token'] !== snapshot.csrf_token ||
          postedSettings.expected_revision !== snapshot.settings.revision) {
        throw new Error('save did not retain the session CSRF or loaded settings revision');
      }
      const { expected_revision, ...saved } = postedSettings;
      snapshot.settings = { ...saved, revision: expected_revision + 1 };
      return { ok: true, status: 200, json: async () => ({ data: snapshot.settings }) };
    }
    if (url === '/api/v1/twitch/receive-only/connect') {
      receiveOnlyRequest = JSON.parse(options.body);
      if (options.headers['X-CSRF-Token'] !== snapshot.csrf_token) throw new Error('receive-only request lost CSRF');
      return { ok: true, status: 202, json: async () => ({ data: { status: 'connecting' } }) };
    }
    if (url === '/api/v1/obs/disconnect') {
      disconnected = true;
      return { ok: true, status: 200, json: async () => ({ data: { status: 'not_connected' } }) };
    }
    throw new Error('unexpected fetch ' + url);
  },
  console
};

vm.runInNewContext(script, sandbox, { filename: path.join(__dirname, '..', '..', 'web', 'control', 'app.js') });

setImmediate(async () => {
  const micSelect = node('discovery-mic');
  micSelect.value = 'Mic/Aux';
  await micSelect.handlers.change({ target: micSelect });
  const updated = JSON.parse(JSON.stringify(snapshot));
  updated.obs_discovery.captured_at = '2026-11-01T12:00:02Z';
  stateListener({ data: JSON.stringify(updated) });
  if (micSelect.value !== 'Mic/Aux') {
    failure = 'timestamp-only discovery update lost the selected microphone';
  }
  await node('settings-form').handlers.submit({ preventDefault() {} });

  // Reload in a fresh VM/DOM using the data actually posted by the first VM.
  // No bootstrap fragment is present; the private state supplies CSRF again.
  const firstSaved = JSON.parse(JSON.stringify(snapshot.settings));
  nodes.clear();
  stateListener = null;
  vm.runInNewContext(script, { ...sandbox }, { filename: 'control-reload.js' });
  await new Promise((resolve) => setImmediate(resolve));
  reloadSavedIdentity = node('mic-name').value === firstSaved.microphone.name &&
    node('mic-uuid').value === firstSaved.microphone.uuid &&
    node('mic-context').value === firstSaved.microphone.context &&
    node('track').value === String(firstSaved.stream_track) &&
    node('visual-scene').value === firstSaved.visual.scene &&
    node('visual-scene-uuid').value === firstSaved.visual.scene_uuid &&
    node('visual-item').value === firstSaved.visual.scene_item_id &&
    node('visual-source').value === firstSaved.visual.source.name &&
    node('visual-source-uuid').value === firstSaved.visual.source.uuid &&
    node('visual-context').value === firstSaved.visual.source.context;
  if (!reloadSavedIdentity) failure = failure || 'fresh VM did not reload saved identities and track';
  await node('settings-form').handlers.submit({ preventDefault() {} });
  reloadSaveRevision = postedSettings.expected_revision;
  if (reloadSaveRevision !== firstSaved.revision || snapshot.settings.revision !== firstSaved.revision + 1) {
    failure = failure || 'second save after reload lost the saved settings revision';
  }
  node('mic-name').value = 'Manually renamed microphone';
  node('mic-name').handlers.input();
  if (node('mic-uuid').value !== '' || node('mic-context').value !== '') {
    failure = failure || 'manual rename retained a mismatched hidden identity';
  }
  await node('obs-disconnect').handlers.click();
  if (!disconnected) {
    failure = failure || 'disconnect handler did not issue the private request';
  }
  if (node('twitch-user-code').textContent !== 'ABCD-1234') {
    failure = failure || 'device code not rendered';
  } else if (node('twitch-device').hidden !== false) {
    failure = failure || 'device instruction box stayed hidden';
  } else if (node('obs-address').value !== 'ws://127.0.0.1:4455') {
    failure = failure || 'OBS endpoint not rendered';
  } else if (!/output active/.test(node('obs-broadcast').textContent)) {
    failure = failure || 'OBS broadcasting status not rendered';
  } else if (node('discovery-mic').options.length !== 2) {
    failure = failure || 'microphone discovery select was not populated';
  } else if (node('discovery-scene').options.length !== 2) {
    failure = failure || 'scene discovery select was not populated';
  } else if (node('obs-discovery').hidden !== false) {
    failure = failure || 'discovery controls stayed hidden';
  } else if (node('visual-source-uuid').value !== 'camera-uuid' || node('visual-context').value !== 'Main Collection' ||
      node('visual-scene-uuid').value !== 'scene-uuid') {
    failure = failure || 'visual identity was not preserved on load';
  } else if (!/2=on/.test(node('obs-tracks').textContent) || !/Main/.test(node('obs-scene').textContent)) {
    failure = failure || 'live OBS observation was not rendered';
  } else if (!postedSettings || !postedSettings.visual || postedSettings.visual.source.uuid !== 'camera-uuid' ||
      postedSettings.visual.source.context !== 'Main Collection' || postedSettings.microphone.context !== 'Main Collection') {
    failure = failure || 'visual or microphone identity was discarded on save';
  }
  snapshot.twitch = { status: 'not_connected', provider: 'anonymous_irc', receive_only: true, login: 'testchannel', receipt: {} };
  snapshot.state_revision++;
  stateListener({data: JSON.stringify(snapshot)});
  if (!node('twitch-client-id').hidden || node('receive-only-setup').hidden || !node('mode-badge').textContent.includes('RECEIVE-ONLY')) failure = failure || 'anonymous capabilities mislabeled';
  await node('twitch-connect').handlers.click();
  if (receiveOnlyRequest) failure = failure || 'unchecked channel confirmation dispatched';
  node('receive-only-confirm').checked = true;
  await node('twitch-connect').handlers.click();
  if (!receiveOnlyRequest || receiveOnlyRequest.channel !== 'testchannel' || receiveOnlyRequest.confirmed !== true) failure = failure || 'confirmed channel did not dispatch';
  console.log(JSON.stringify({
    deviceCode: node('twitch-user-code').textContent,
    deviceVisible: node('twitch-device').hidden === false,
    obsBroadcasting: node('obs-broadcast').textContent,
    micOptions: node('discovery-mic').options.length,
    sceneOptions: node('discovery-scene').options.length,
    savedIdentity: Boolean(postedSettings && postedSettings.visual && postedSettings.visual.source.uuid),
    reloadSavedIdentity,
    reloadSaveRevision,
    failure
  }, null, 2));
  if (failure) process.exitCode = 1;
});
