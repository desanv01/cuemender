'use strict';

(() => {
  const $ = (id) => document.getElementById(id);
  const state = { csrf: '', revision: 0, connected: false };
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

  function fillForm(settings) {
    $('mic-name').value = settings.microphone ? settings.microphone.name : '';
    $('mic-uuid').value = settings.microphone && settings.microphone.uuid ? settings.microphone.uuid : '';
    $('track').value = settings.stream_track ? String(settings.stream_track) : '';
    $('track-confirmed').textContent = trackedLabel(settings);
    const visual = settings.visual;
    $('visual-enabled').checked = Boolean(visual);
    $('visual-fields').disabled = !visual;
    $('visual-scene').value = visual ? visual.scene : '';
    $('visual-item').value = visual ? visual.scene_item_id : '';
    $('visual-source').value = visual ? visual.source.name : '';
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
        showLocked('This page has no private session. Open the single-use link printed by LiveCheck.');
      } else {
        showLocked('Private state could not be loaded. Check the console and retry.');
      }
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
      showLocked('The launch link is invalid, expired or already used. Restart LiveCheck for a new link.');
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
    if (visualEnabled) {
      if (!scene || !sceneItem || !source) {
        throw new Error('A visual expectation needs a scene, scene item ID and source name.');
      }
      payload.visual = {
        scene,
        scene_item_id: sceneItem,
        source: { name: source }
      };
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
        showLocked('The private session is no longer valid. Restart LiveCheck for a new link.');
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

  (async () => {
    await bootstrapFromFragment();
    await loadState();
  })();
})();
