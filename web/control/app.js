'use strict';

(() => {
  const $ = (id) => document.getElementById(id);
  const queryAll = (selector) => typeof document.querySelectorAll === 'function' ? Array.from(document.querySelectorAll(selector)) : [];
  const state = { csrf: '', revision: 0, currentRevision: 0, monitoringSessionId: '', incidents: [], connected: false, discovery: null, discoverySignature: '', mode: 'live', section: 'monitor', sectionChosen: false, closeOutcomes: {}, rehearsalPublicID: '' };
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
    const readiness = setup && setup.live_obs_resolution_ready ? 'Live resolution ready.' : 'Live resolution not ready.';
    setText('obs-readiness', state.mode === 'rehearsal' ? 'Simulated provider resolution ready.' : readiness);
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

  function renderMonitoring(privateState) {
    const active = Boolean(privateState.monitoring_active);
    const session = privateState.monitoring_session;
    state.monitoringSessionId = active && session ? session.id : '';
    state.proposals = Array.isArray(privateState.proposals) ? privateState.proposals : [];
    state.actionRecords = Array.isArray(privateState.actions) ? privateState.actions : [];
    state.incidents = Array.isArray(privateState.incidents) ? privateState.incidents : [];
    setText('monitor-status', active ? 'Monitoring active' + (session && session.id ? ' (session ' + session.id.slice(0, 8) + '…)' : '') : 'Monitoring is stopped.');
    const tw = privateState.twitch ? privateState.twitch.status : 'not connected';
    const obs = privateState.obs ? privateState.obs.status : 'not connected';
    const broadcast = privateState.obs && typeof privateState.obs.broadcasting === 'boolean' ? String(privateState.obs.broadcasting) : 'unknown';
    setText('monitor-context', (state.mode === 'rehearsal' ? 'Simulated providers' : 'Local providers') + ' · Twitch: ' + tw + ' · OBS: ' + obs + ' · broadcast: ' + broadcast + ' · monitoring: ' + (active ? 'active' : 'stopped'));
    renderIncidents(state.incidents);
  }

  function renderIncidents(incidents) {
    const list = $('incident-list');
    if (!list) {
      return;
    }
    const focusedID = document.activeElement && document.activeElement.id ? document.activeElement.id : '';
    const priorCards = Array.from(list.children || []);
    const cards = [];
    if (!incidents.length) {
      list.replaceChildren();
      list.textContent = 'No incidents in this session.';
      return;
    }
    for (const incident of incidents) {
      const signature = JSON.stringify({
        stage: incident.stage, feedback: incident.feedback, local_verification: incident.local_verification,
        closure_outcome: incident.closure_outcome, report_count: incident.report_count,
        findings: (incident.findings || []).map((finding) => finding.code),
        checks: (incident.checks || []).map((check) => check.unavailable_reason || check.status),
        streaming: incident.streaming,
      });
      const prior = priorCards.find((card) => card.dataset && card.dataset.incidentId === incident.id && card.dataset.signature === signature);
      if (prior) {
        cards.push(prior);
        continue;
      }
      const card = document.createElement('div');
      card.dataset = { incidentId: incident.id, signature };
      card.className = 'device-box';
      const title = document.createElement('p');
      title.className = 'connection-status';
      title.textContent = incident.public_id + ' · ' + incident.category + ' · ' + incident.stage;
      card.appendChild(title);
      const meta = document.createElement('p');
      meta.className = 'muted';
      const evidenceTime = incident.evidence_at ? new Date(incident.evidence_at).toISOString() : 'unknown';
      const revision = (incident.findings || []).length ? incident.findings[0].expectation_revision : 'unknown';
      const streaming = typeof incident.streaming === 'boolean' ? String(incident.streaming) : 'unknown';
      meta.textContent = incident.category + ' · origin: ' + incident.origin + ' · reports: ' + incident.report_count +
        ' · distinct: ' + incident.distinct_reporters + ' · evidence: ' + evidenceTime + ' · expectation revision: ' + revision +
        ' · local streaming: ' + streaming + ' · feedback: ' + incident.feedback.positive + ' positive / ' + incident.feedback.negative + ' negative';
      card.appendChild(meta);
      const evidence = document.createElement('p');
      evidence.className = 'muted';
      const findings = (incident.findings || []).map((finding) => finding.code).join(', ');
      const guidance = (incident.findings || []).map((finding) => finding.manual_guidance).filter(Boolean).join(' ');
      const limitations = (incident.checks || []).map((check) => check.unavailable_reason).filter(Boolean).join(' ');
      let evidenceText;
      if (findings) {
        evidenceText = 'Recorded diagnosis: ' + findings + (guidance ? '. ' + guidance : '');
      } else if (incident.stage === 'no_supported_finding') {
        evidenceText = 'No supported conflict was found in a completed sufficient check.';
      } else if (incident.stage === 'insufficient_information' || incident.stage === 'interrupted') {
        evidenceText = 'No completed diagnosis. ';
      } else {
        evidenceText = 'Diagnosis is not complete. ';
      }
      if (limitations) {
        evidenceText += ' Limitations: ' + limitations;
      }
      evidence.textContent = evidenceText + ' Local verification: ' + incident.local_verification + '. Track repair requires an exact private approval while monitoring.';
      card.appendChild(evidence);
      if (incident.manual_guidance) {
        const guidance = document.createElement('p');
        guidance.className = 'muted';
        guidance.textContent = incident.manual_guidance;
        card.appendChild(guidance);
      }
      const actions = document.createElement('div');
      actions.className = 'actions';
      const pendingAction = (state.actionRecords || []).find((entry) => entry.incident_id === incident.id && ['reserved', 'intent_recorded', 'write_sent', 'unknown'].includes(entry.status));
      const proposal = (state.proposals || []).find((entry) => entry.incident_id === incident.id && entry.status === 'proposed');
      if (proposal) {
        const detail = document.createElement('p'); detail.className = 'muted';
        detail.textContent = 'Add ' + proposal.input.name + ' to confirmed track ' + proposal.selected_track + '. Preserve assignments: ' + Object.entries(proposal.expected_tracks).map(([key, value]) => key + '=' + value).join(', ') + '. Expires ' + new Date(proposal.expires_at).toISOString() + '. Other findings remain. Another OBS client may change assignments between the read and write.';
        card.appendChild(detail);
        const approve = document.createElement('button'); approve.type = 'button'; approve.textContent = 'Approve track addition';
        approve.disabled = !state.monitoringSessionId || proposal.session_id !== state.monitoringSessionId || Date.now() >= Date.parse(proposal.expires_at) || Boolean(pendingAction);
        approve.addEventListener('click', async () => { approve.disabled = true; try { await postPrivate('/api/v1/proposals/' + encodeURIComponent(proposal.id) + '/approve', { expected_revision: proposal.config_revision }); await loadState(); } catch (error) { showNotice(error.message || 'Approval could not be accepted.', 'error'); await loadState(); } });
        actions.appendChild(approve);
      }
      const lastAction = (state.actionRecords || []).filter((entry) => entry.incident_id === incident.id).sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0];
      if (lastAction) {
        const result = document.createElement('p'); result.className = 'muted';
        result.textContent = lastAction.reconciled ? 'Current OBS configuration ' + (lastAction.status === 'verified' ? 'matches' : 'differs from') + ' the approved map. This does not establish who changed it or viewer recovery.' : lastAction.status === 'verified' ? 'Approved track addition verified in local OBS configuration. Remaining findings and viewer feedback are separate.' : lastAction.status === 'no_op' ? 'The track was already assigned. CueMender sent no change.' : 'Local action: ' + lastAction.status + ' (' + lastAction.detail_code + '). No automatic retry or rollback.'; card.appendChild(result);
      }
      const terminal = ['dismissed', 'closed', 'expired', 'invalidated'].includes(incident.stage);
      const currentSession = state.monitoringSessionId && incident.session_id === state.monitoringSessionId;
      if (incident.stage === 'closed') {
        const closure = document.createElement('p');
        closure.className = 'muted';
        closure.textContent = 'Persisted closure: ' + (incident.closure_outcome || 'unknown') + '. Further feedback, recheck and approval are rejected.';
        card.appendChild(closure);
      }
      if (!terminal && currentSession && !pendingAction) {
        const recheck = document.createElement('button');
        recheck.type = 'button';
        recheck.className = 'secondary';
        recheck.id = 'recheck-' + incident.id;
        recheck.textContent = 'Recheck';
        recheck.addEventListener('click', () => recheckIncident(incident.id));
        actions.appendChild(recheck);
      }
      if (!terminal && !pendingAction) {
        const dismiss = document.createElement('button');
        dismiss.type = 'button';
        dismiss.className = 'secondary';
        dismiss.id = 'dismiss-' + incident.id;
        dismiss.textContent = 'Dismiss';
        dismiss.addEventListener('click', () => dismissIncident(incident.id));
        actions.appendChild(dismiss);
        const closeLabel = document.createElement('label');
        closeLabel.textContent = 'Close outcome';
        closeLabel.htmlFor = 'close-' + incident.id;
        const closeSelect = document.createElement('select');
        closeSelect.id = 'close-' + incident.id;
        const blank = document.createElement('option'); blank.value = ''; blank.textContent = 'Choose outcome'; closeSelect.appendChild(blank);
        for (const [value, label] of [['viewer_confirmed', 'Viewer confirmed'], ['local_only', 'Local only'], ['unresolved', 'Unresolved']]) {
          const option = document.createElement('option'); option.value = value; option.textContent = label; closeSelect.appendChild(option);
        }
        closeSelect.value = state.closeOutcomes[incident.id] || '';
        closeSelect.addEventListener('change', () => { state.closeOutcomes[incident.id] = closeSelect.value; });
        const closeButton = document.createElement('button');
        closeButton.type = 'button';
        closeButton.className = 'secondary';
        closeButton.id = 'close-button-' + incident.id;
        closeButton.textContent = 'Close incident';
        closeButton.disabled = !closeSelect.value;
        closeSelect.addEventListener('change', () => { closeButton.disabled = !closeSelect.value; });
        closeButton.addEventListener('click', () => closeIncident(incident.id, closeSelect.value));
        actions.appendChild(closeLabel);
        actions.appendChild(closeSelect);
        actions.appendChild(closeButton);
      }
      card.appendChild(actions);
      cards.push(card);
    }
    list.replaceChildren();
    for (const card of cards) list.appendChild(card);
    if (focusedID && typeof document.getElementById === 'function') {
      const restored = document.getElementById(focusedID);
      if (restored && typeof restored.focus === 'function') restored.focus();
    }
  }

  async function startMonitoring() {
    try {
      await request('/api/v1/monitoring/start', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf },
        body: JSON.stringify({ expected_revision: state.currentRevision })
      });
      showNotice('Monitoring started.', 'ok');
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Monitoring could not start.', 'error');
    }
  }

  async function closeIncident(id, outcome) {
    try {
      await postPrivate('/api/v1/incidents/' + encodeURIComponent(id) + '/close', { outcome });
      showNotice('Incident closed with ' + outcome + '.', 'ok');
    } catch (error) {
      showNotice(error.message || 'Closure was not accepted. Reloading truthful state.', 'error');
    }
    await loadState();
  }

  async function stopMonitoring() {    try {
      await request('/api/v1/monitoring/stop', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf },
        body: '{}'
      });
      showNotice('Monitoring stopped.', 'ok');
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Monitoring could not stop.', 'error');
    }
  }

  async function manualCheck(category) {
    try {
      await request('/api/v1/incidents/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf },
        body: JSON.stringify({ category, expected_revision: state.currentRevision })
      });
      showNotice('Manual check scheduled.', 'ok');
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Manual check could not start.', 'error');
    }
  }

  async function recheckIncident(id) {
    try {
      const incident = (state.incidents || []).find((entry) => entry.id === id);
      await request('/api/v1/incidents/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf },
        body: JSON.stringify({ category: incident ? incident.category : 'audio', expected_revision: state.currentRevision, incident_id: id })
      });
      showNotice('Recheck scheduled.', 'ok');
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Recheck could not start.', 'error');
    }
  }

  async function dismissIncident(id) {
    try {
      await request('/api/v1/incidents/' + encodeURIComponent(id) + '/dismiss', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': state.csrf },
        body: '{}'
      });
      showNotice('Incident dismissed.', 'ok');
      await loadState();
    } catch (error) {
      showNotice(error.message || 'Incident could not be dismissed.', 'error');
    }
  }

  function applySection(section, chosen) {
    if (chosen) state.sectionChosen = true;
    state.section = section;
    for (const panel of Array.from(document.querySelectorAll ? document.querySelectorAll('#app > section.panel') : [])) {
      if (panel.id === 'rehearsal-panel') {
        panel.hidden = state.mode !== 'rehearsal' || section !== 'rehearsal';
        continue;
      }
      panel.hidden = section === 'setup' ? panel.dataset.section !== 'setup' : panel.dataset.section !== 'monitor';
    }
    for (const button of queryAll('#section-tabs button')) {
      if (button.classList) button.classList.toggle('active', button.dataset.section === section);
    }
  }

  function renderMode(mode) {
    state.mode = mode === 'rehearsal' ? 'rehearsal' : 'live';
    const badge = $('mode-badge');
    if (badge) {
      badge.textContent = state.mode === 'rehearsal' ? 'REHEARSAL' : 'LIVE';
      if (badge.classList) badge.classList.toggle('rehearsal', state.mode === 'rehearsal');
    }
    const tab = $('rehearsal-tab');
    if (tab) {
      tab.hidden = state.mode !== 'rehearsal';
    }
    const watermark = $('mode-watermark');
    if (watermark) {
      watermark.hidden = state.mode !== 'rehearsal';
    }
    if (state.mode !== 'rehearsal' && state.section === 'rehearsal') {
      state.section = 'monitor';
    }
    applySection(state.section);
  }

  function render(privateState) {
    showApp();
    if (typeof privateState.csrf_token === 'string' && privateState.csrf_token) {
      state.csrf = privateState.csrf_token;
    }
    state.currentRevision = privateState.settings ? privateState.settings.revision : 0;
    renderMonitoring(privateState);
    renderMode(privateState.mode || 'live');
    if (!state.sectionChosen) {
      const settings = privateState.settings;
      const unconfigured = !settings || (!settings.microphone && !settings.visual && !settings.stream_track);
      state.section = unconfigured ? 'setup' : 'monitor';
      applySection(state.section);
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
    if (state.mode === 'rehearsal') {
      setText('twitch', 'connected (simulated)');
      setText('obs', 'connected (simulated)');
      setText('twitch-status', 'Simulated connected — no real Twitch session.');
      setText('obs-status', 'Simulated connected — no real OBS socket.');
    }
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

  async function applyRehearsalScenario() {
    try {
      await postPrivate('/api/v1/rehearsal/scenario', { scenario: $('rehearsal-scenario').value });
      $('rehearsal-status').textContent = 'Scenario applied. Start monitoring, then simulate a report.';
      await loadState();
    } catch (error) {
      $('rehearsal-status').textContent = error.message || 'Scenario was not applied.';
    }
  }

  async function simulateRehearsalReport() {
    try {
      const scenario = $('rehearsal-scenario') ? $('rehearsal-scenario').value : 'track_missing';
      const category = scenario === 'wrong_scene' ? 'video' : 'audio';
      const incident = await postPrivate('/api/v1/rehearsal/report', { category });
      state.rehearsalPublicID = incident && incident.public_id ? incident.public_id : '';
      $('rehearsal-status').textContent = 'Simulated ' + category + ' viewer report accepted for ' + (state.rehearsalPublicID || 'the current incident') + '.';
      await loadState();
    } catch (error) {
      $('rehearsal-status').textContent = error.message || 'Simulated report was rejected.';
    }
  }

  function rehearsalFeedbackTarget() {
    const eligible = (state.incidents || []).filter((incident) =>
      incident.session_id === state.monitoringSessionId &&
      (incident.stage === 'awaiting_feedback' || incident.stage === 'unresolved'));
    const preferred = eligible.find((incident) => incident.public_id === state.rehearsalPublicID);
    if (preferred) return preferred;
    return eligible.sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at))[0] || null;
  }

  async function simulateRehearsalFeedback(vote) {
    const incident = rehearsalFeedbackTarget();
    if (!incident) {
      $('rehearsal-status').textContent = 'No current-session incident is eligible for feedback.';
      return;
    }
    try {
      const updated = await postPrivate('/api/v1/rehearsal/feedback', { public_id: incident.public_id, vote });
      state.rehearsalPublicID = incident.public_id;
      $('rehearsal-status').textContent = 'Simulated vote recorded for ' + incident.public_id + ': ' +
        updated.feedback.positive + ' positive / ' + updated.feedback.negative + ' negative.';
      await loadState();
    } catch (error) {
      $('rehearsal-status').textContent = error.message || 'Simulated feedback was rejected.';
      await loadState();
    }
  }

  $('refresh').addEventListener('click', loadState);
  $('settings-form').addEventListener('submit', save);
  $('monitor-start').addEventListener('click', startMonitoring);
  $('monitor-stop').addEventListener('click', stopMonitoring);
  $('manual-audio').addEventListener('click', () => manualCheck('audio'));
  $('manual-video').addEventListener('click', () => manualCheck('video'));
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

  for (const button of queryAll('#section-tabs button')) {
    button.addEventListener('click', () => applySection(button.dataset.section, true));
  }
  if ($('overlay-url')) {
    $('overlay-url').value = (typeof window !== 'undefined' && window.location ? window.location.origin : '') + '/overlay/';
    if ($('overlay-copy')) {
      $('overlay-copy').addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText($('overlay-url').value);
          $('overlay-copy-status').textContent = 'Copied';
        } catch {
          $('overlay-url').select();
          $('overlay-copy-status').textContent = 'Select and copy';
        }
      });
    }
  }
  if ($('rehearsal-apply')) {
    $('rehearsal-apply').addEventListener('click', applyRehearsalScenario);
    $('rehearsal-report').addEventListener('click', simulateRehearsalReport);
    $('rehearsal-feedback-fixed').addEventListener('click', () => simulateRehearsalFeedback('fixed'));
    $('rehearsal-feedback-broken').addEventListener('click', () => simulateRehearsalFeedback('stillbroken'));
  }

  (async () => {
    await bootstrapFromFragment();
    await loadState();
  })();
})();
