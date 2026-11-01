'use strict';

(() => {
  const $ = (id) => document.getElementById(id);
  const state = { csrf: '', revision: 0, connected: false, discovery: null, discoverySignature: '' };
  let events = null;

  class ApiError extends Error {
    constructor(status, body) {
      super((body && body.error && body.error.message) || 'Request failed.');
      this.status = status;
      this.body = body;
    }
  }

  async function request(path, options = {}) {
    const response = await fetch(path, {
      credentials: 'same-origin',
      ...options,
      headers: {
        'Accept': 'application/json',
        ...(options.headers || {})
      }
    });
    let body = null;
    try {
      body = await response.json();
    } catch (_) {
      body = null;
    }
    if (!response.ok) {
      throw new ApiError(response.status, body);
    }
    return body ? body.data : null;
  }

  function setText(id, text) {
    const node = $(id);
    if (node) {
      node.textContent = text;
    }
  }

  function showLocked(message) {
    $('app').hidden = true;
    $('locked').hidden = false;
    setText('locked-message', message);
  }

  function showApp() {
    $('locked').hidden = true;
    $('app').hidden = false;
  }

  function showNotice(message, kind) {
    const node = $('notice');
    node.textContent = message;
    node.className = 'notice' + (kind ? ' ' + kind : '');
  }

  function describeSecrets(secrets) {
    if (!secrets) {
      return 'unknown';
    }
    if (secrets.mode === 'os_keyring') {
      return 'OS keyring';
    }
    if (secrets.mode === 'memory_only') {
      return 'session only (memory)';
    }
    return 'needs attention';
  }

  function trackedLabel(settings) {
    if (settings.stream_track === null || settings.stream_track === undefined) {
      return 'No confirmation recorded.';
    }
    const when = settings.track_confirmed_at;
    return 'Confirmed track ' + settings.stream_track + (when ? ' at ' + when : '') + '.';
  }

  function renderTwitch(twitch) {
    const value = twitch || { status: 'not_connected', receipt: {} };
    setText('twitch', value.status || 'not_connected');
    setText('twitch-status', value.detail || value.status || 'not connected');
    const device = value.device;
    if (device) {
      $('twitch-device').hidden = false;
      setText('twitch-user-code', device.user_code || '');
      setText('twitch-verification-uri', device.verification_uri || '');
      setText('twitch-device-expiry', device.expires_at ? 'Code expires at ' + device.expires_at + '.' : '');
    } else {
      $('twitch-device').hidden = true;
      setText('twitch-user-code', '');
      setText('twitch-verification-uri', '');
      setText('twitch-device-expiry', '');
    }
    const receipt = value.receipt || {};
    if (receipt.last_at) {
      setText('twitch-receipt', 'Accepted commands: ' + (receipt.accepted || 0) + '. Last: ' +
        (receipt.last_command || 'unknown') + (receipt.last_category ? ' (' + receipt.last_category + ')' : '') +
        ' at ' + receipt.last_at + '.');
    } else {
      setText('twitch-receipt', 'No commands received.');
    }
  }

  function renderOBS(obs, startup, discovery, observation, setup) {
    const value = obs || { status: 'not_connected' };
    setText('obs', value.status || 'not_connected');
    setText('obs-status', value.detail || value.status || 'not connected');
    if (value.broadcasting === true) {
      setText('obs-broadcast', 'OBS reports the stream output active.');
    } else if (value.broadcasting === false) {
      setText('obs-broadcast', 'OBS reports the stream output inactive.');
    } else {
      setText('obs-broadcast', 'Broadcasting state unknown.');
    }
    setText('obs-stale', value.stale ? 'Observation data is stale.' : '');
    setText('obs-readiness', setup && setup.live_obs_resolution_ready ? 'Live resolution ready.' : 'Live resolution not ready.');
    setText('obs-observed', value.observed_at ? 'Observed at ' + value.observed_at : 'Observation time unknown.');
    if (startup && startup.obs_address && !$('obs-address').value) {
      $('obs-address').value = startup.obs_address;
    }
    if (discovery) {
      populateDiscovery(discovery);
    } else {
      state.discovery = null;
      state.discoverySignature = '';
      $('discovery-mic').replaceChildren(new Option('Select an input', ''));
      $('discovery-scene').replaceChildren(new Option('Select a scene', ''));
      $('discovery-item').replaceChildren(new Option('Select a scene item', ''));
      $('obs-discovery').hidden = true;
    }
    renderObservation(observation);
  }

  function renderObservation(observation) {
    const box = $('obs-observation');
    if (!observation) {
      box.hidden = true;
      return;
    }
    box.hidden = false;
    setText('obs-mute', 'Microphone mute: ' + (typeof observation.mute === 'boolean' ? String(observation.mute) : 'unknown'));
    const tracks = observation.tracks || {};
    const trackKeys = ['track1', 'track2', 'track3', 'track4', 'track5', 'track6'];
    const knownTracks = trackKeys.every((key) => typeof tracks[key] === 'boolean');
    setText('obs-tracks', 'Tracks: ' + (knownTracks ? trackKeys.map((key) => key.slice(5) + '=' + (tracks[key] ? 'on' : 'off')).join(' ') : 'unknown'));
    setText('obs-scene', 'Program scene: ' + (observation.program_scene || 'unknown'));
    setText('obs-visual', 'Visual item: ' + (typeof observation.visual_item_enabled === 'boolean' ? String(observation.visual_item_enabled) : 'unknown'));
    setText('obs-streaming', 'Streaming: ' + (typeof observation.streaming === 'boolean' ? String(observation.streaming) : 'unknown'));
    if (Array.isArray(observation.partial) && observation.partial.length) {
      const prior = $('obs-stale').textContent;
      setText('obs-stale', (prior ? prior + ' ' : '') + 'Partial reads: ' + observation.partial.join(', '));
    }
  }

  function populateDiscovery(discovery) {
    const signature = [
      String(discovery.generation || ''),
      String(discovery.scene_collection || ''),
      (discovery.inputs || []).map((entry) => entry.uuid + ':' + entry.name).join(','),
      (discovery.scenes || []).map((entry) => entry.uuid + ':' + entry.name).join(',')
    ].join('|');
    if (state.discoverySignature === signature) {
      state.discovery = discovery;
      return;
    }
    const selectedMic = $('discovery-mic').value;
    const selectedScene = $('discovery-scene').value;
    state.discoverySignature = signature;
    state.discovery = discovery;
    const mic = $('discovery-mic');
    const scene = $('discovery-scene');
    mic.replaceChildren(new Option('Select an input', ''));
    for (const input of (discovery.inputs || [])) {
      const option = new Option(input.name, input.name);
      option.dataset.uuid = input.uuid || '';
      option.dataset.kind = input.kind || '';
      option.dataset.context = discovery.scene_collection || '';
      mic.add(option);
    }
    scene.replaceChildren(new Option('Select a scene', ''));
    for (const item of (discovery.scenes || [])) {
      const option = new Option(item.name, item.name);
      option.dataset.uuid = item.uuid || '';
      option.dataset.context = discovery.scene_collection || '';
      scene.add(option);
    }
    restoreSelection(mic, selectedMic);
    restoreSelection(scene, selectedScene);
    $('discovery-item').replaceChildren(new Option('Select a scene item', ''));
    $('obs-discovery').hidden = false;
  }

  function restoreSelection(select, value) {
    if (!value) {
      return;
    }
    for (const option of select.options) {
      if (option.value === value) {
        select.value = value;
        return;
      }
    }
    select.value = '';
  }

  function populateSceneItems() {
    const selected = $('discovery-scene').value;
    const itemSelect = $('discovery-item');
    itemSelect.replaceChildren(new Option('Select a scene item', ''));
    if (!state.discovery || !selected) {
      return;
    }
    const scene = (state.discovery.scenes || []).find((entry) => entry.name === selected);
    if (!scene) {
      return;
    }
    for (const item of (scene.items || [])) {
      const label = item.source_name + (item.is_group ? ' (group)' : '') + (item.enabled ? '' : ' (hidden)');
      const option = new Option(label, String(item.id));
      option.dataset.source = item.source_name || '';
      option.dataset.uuid = item.source_uuid || '';
      option.dataset.context = state.discovery.scene_collection || '';
      option.disabled = item.is_group;
      itemSelect.add(option);
    }
  }

  function fillForm(settings) {
    $('mic-name').value = settings.microphone ? settings.microphone.name : '';
    $('mic-uuid').value = settings.microphone && settings.microphone.uuid ? settings.microphone.uuid : '';
    $('mic-context').value = settings.microphone && settings.microphone.context ? settings.microphone.context : '';
    $('track').value = settings.stream_track ? String(settings.stream_track) : '';
    $('track-confirmed').textContent = trackedLabel(settings);
    const visual = settings.visual;
    $('visual-enabled').checked = Boolean(visual);
    $('visual-fields').disabled = !visual;
    $('visual-scene').value = visual ? visual.scene : '';
    $('visual-scene-uuid').value = visual && visual.scene_uuid ? visual.scene_uuid : '';
    $('visual-item').value = visual ? visual.scene_item_id : '';
    $('visual-source').value = visual ? visual.source.name : '';
    $('visual-source-uuid').value = visual && visual.source.uuid ? visual.source.uuid : '';
    $('visual-context').value = visual && visual.source.context ? visual.source.context : '';
  }

  function render(privateState) {
    showApp();
    if (typeof privateState.csrf_token === 'string' && privateState.csrf_token) {
      state.csrf = privateState.csrf_token;
    }
    setText('mode', privateState.mode || 'live');
    setText('revision', String(privateState.settings ? privateState.settings.revision : 0));
    setText('state-revision', String(privateState.state_revision || 0));
    setText('secrets', describeSecrets(privateState.secrets));
    if (privateState.secrets && privateState.secrets.detail) {
      $('secrets').title = privateState.secrets.detail;
    }
    setText('twitch', privateState.twitch ? privateState.twitch.status : 'not connected');
    setText('obs', privateState.obs ? privateState.obs.status : 'not connected');
    renderTwitch(privateState.twitch);
    renderOBS(privateState.obs, privateState.startup, privateState.obs_discovery, privateState.obs_observation, privateState.setup);
    if (privateState.settings) {
      if (privateState.settings.revision !== state.revision) {
        state.revision = privateState.settings.revision;
        fillForm(privateState.settings);
      }
    }
  }

  function connectEvents() {
    if (events) {
      return;
    }
    events = new EventSource('/api/v1/events');
    events.addEventListener('state', (event) => {
      try {
        const privateState = JSON.parse(event.data);
        render(privateState);
        setText('notice', 'Live state updated.');
      } catch (_) {
        showNotice('A state update could not be read.', 'error');
      }
    });
    events.addEventListener('error', () => {
      setText('notice', 'Control stream reconnecting…');
    });
  }

  async function loadState() {
    try {
      const privateState = await request('/api/v1/state');
      state.connected = true;
      render(privateState);
      connectEvents();
    } catch (error) {
      if (error.status === 401) {
        state.csrf = '';
        showLocked('This page has no private session. Open the single-use link printed by CueMender.');
      } else {
        showLocked('Private state could not be loaded. Check the console and retry.');
      }
    }
  }

  async function postPrivate(path, body) {
    return request(path, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': state.csrf
      },
      body: JSON.stringify(body)
    });
  }

  async function connectTwitch() {
    const clientID = $('twitch-client-id').value.trim();
    if (!clientID) {
      showNotice('Enter the Twitch Public client ID first.', 'error');
      return;
    }
    try {
      showNotice('Starting Twitch authorization…');
      await postPrivate('/api/v1/twitch/connect', { client_id: clientID });
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Twitch authorization could not be started.', 'error');
    }
  }

  async function disconnectTwitch() {
    try {
      await postPrivate('/api/v1/twitch/disconnect', {});
      await loadState();
      showNotice('Twitch disconnected.');
    } catch (error) {
      showNotice(error.message || 'Twitch could not be disconnected.', 'error');
    }
  }

  async function connectOBS() {
    const address = $('obs-address').value.trim();
    const password = $('obs-password').value;
    if (!address) {
      showNotice('Enter the local OBS WebSocket address.', 'error');
      return;
    }
    const payload = { address };
    if (password !== '') {
      payload.password = password;
    } else if (!$('obs-reuse').checked) {
      payload.password = '';
    }
    try {
      showNotice('Connecting to OBS…');
      await postPrivate('/api/v1/obs/connect', payload);
      await loadState();
    } catch (error) {
      showNotice(error.message || 'OBS connection could not be started.', 'error');
    } finally {
      $('obs-password').value = '';
    }
  }

  async function disconnectOBS() {
    try {
      await postPrivate('/api/v1/obs/disconnect', {});
      await loadState();
      showNotice('OBS disconnected.');
    } catch (error) {
      showNotice(error.message || 'OBS could not be disconnected.', 'error');
    }
  }

  async function refreshDiscovery() {
    try {
      const discovery = await request('/api/v1/obs/discovery');
      state.discoverySignature = '';
      populateDiscovery(discovery);
      showNotice('OBS discovery refreshed.');
    } catch (error) {
      showNotice(error.message || 'OBS discovery is not available.', 'error');
    }
  }

  async function bootstrapFromFragment() {
    const raw = location.hash.startsWith('#') ? location.hash.slice(1) : '';
    const params = new URLSearchParams(raw);
    const token = params.get('bootstrap');
    if (!token) {
      return false;
    }
    history.replaceState(null, '', location.pathname + location.search);
    try {
      const data = await request('/api/v1/session/bootstrap', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token })
      });
      state.csrf = data.csrf_token;
      state.connected = true;
      return true;
    } catch (error) {
      showLocked('The launch link is invalid, expired or already used. Restart CueMender for a new link.');
      return false;
    }
  }

  function collectSettings() {
    const micName = $('mic-name').value.trim();
    const micUUID = $('mic-uuid').value.trim();
    const trackValue = $('track').value;
    const visualEnabled = $('visual-enabled').checked;
    const scene = $('visual-scene').value.trim();
    const sceneItem = $('visual-item').value.trim();
    const source = $('visual-source').value.trim();

    const payload = {
      expected_revision: state.revision,
      microphone: micName ? { name: micName } : null,
      stream_track: trackValue ? Number(trackValue) : null,
      visual: null
    };
    if (micName && micUUID) {
      payload.microphone.uuid = micUUID;
    }
    if (micName && $('mic-context').value.trim()) {
      payload.microphone.context = $('mic-context').value.trim();
    }
    if (visualEnabled) {
      if (!scene || !sceneItem || !source) {
        throw new Error('A visual expectation needs a scene, scene item ID and source name.');
      }
      payload.visual = {
        scene,
        scene_item_id: sceneItem,
        source: { name: source }
      };
      if ($('visual-scene-uuid').value.trim()) {
        payload.visual.scene_uuid = $('visual-scene-uuid').value.trim();
      }
      if ($('visual-source-uuid').value.trim()) {
        payload.visual.source.uuid = $('visual-source-uuid').value.trim();
      }
      if ($('visual-context').value.trim()) {
        payload.visual.source.context = $('visual-context').value.trim();
      }
    }
    return payload;
  }

  async function save(event) {
    event.preventDefault();
    const button = $('save');
    button.disabled = true;
    showNotice('Saving…');
    try {
      const payload = collectSettings();
      const settings = await request('/api/v1/settings', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': state.csrf
        },
        body: JSON.stringify(payload)
      });
      state.revision = settings.revision;
      fillForm(settings);
      setText('revision', String(settings.revision));
      showNotice('Saved revision ' + settings.revision + '.', 'ok');
    } catch (error) {
      if (error.status === 409) {
        showNotice('Settings changed elsewhere. Reloading the current revision…', 'error');
        await loadState();
      } else if (error.status === 401) {
        showLocked('The private session is no longer valid. Restart CueMender for a new link.');
      } else {
        showNotice(error.message || 'Settings could not be saved.', 'error');
      }
    } finally {
      button.disabled = false;
    }
  }

  $('refresh').addEventListener('click', loadState);
  $('settings-form').addEventListener('submit', save);
  $('visual-enabled').addEventListener('change', (event) => {
    $('visual-fields').disabled = !event.target.checked;
  });
  $('mic-name').addEventListener('input', () => { $('mic-uuid').value = ''; $('mic-context').value = ''; });
  $('visual-scene').addEventListener('input', () => { $('visual-scene-uuid').value = ''; });
  $('visual-item').addEventListener('input', () => { $('visual-context').value = ''; });
  $('visual-source').addEventListener('input', () => { $('visual-source-uuid').value = ''; $('visual-context').value = ''; });
  $('twitch-connect').addEventListener('click', connectTwitch);
  $('twitch-disconnect').addEventListener('click', disconnectTwitch);
  $('obs-connect').addEventListener('click', connectOBS);
  $('obs-disconnect').addEventListener('click', disconnectOBS);
  $('obs-refresh').addEventListener('click', refreshDiscovery);
  $('discovery-mic').addEventListener('change', (event) => {
    const option = event.target.selectedOptions[0];
    if (!option || !option.value) {
      return;
    }
    $('mic-name').value = option.value;
    $('mic-uuid').value = option.dataset.uuid || '';
    $('mic-context').value = option.dataset.context || '';
  });
  $('discovery-scene').addEventListener('change', () => {
    const option = $('discovery-scene').selectedOptions[0];
    $('visual-scene-uuid').value = option && option.dataset ? (option.dataset.uuid || '') : '';
    populateSceneItems();
  });
  $('discovery-item').addEventListener('change', (event) => {
    const option = event.target.selectedOptions[0];
    if (!option || !option.value) {
      return;
    }
    $('visual-enabled').checked = true;
    $('visual-fields').disabled = false;
    $('visual-scene').value = $('discovery-scene').value;
    $('visual-item').value = option.value;
    $('visual-source').value = option.dataset.source || '';
    $('visual-source-uuid').value = option.dataset.uuid || '';
    $('visual-context').value = option.dataset.context || '';
  });

  (async () => {
    await bootstrapFromFragment();
    await loadState();
  })();
})();
