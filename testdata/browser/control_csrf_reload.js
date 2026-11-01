'use strict';

// Browser-logic regression fixture for the actual control script. This is not
// a rendered-browser test: it runs web/control/app.js in a small Node VM with
// DOM/fetch/EventSource stubs, matching the review's reproduction method.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const scriptPath = path.join(__dirname, '..', '..', 'web', 'control', 'app.js');
const script = fs.readFileSync(scriptPath, 'utf8');

function makeNode() {
  return {
    value: '',
    hidden: false,
    disabled: false,
    checked: false,
    textContent: '',
    title: '',
    className: '',
    handler: null,
    replaceChildren() {},
    add() {},
    addEventListener(type, fn) {
      if (type === 'submit') {
        this.handler = fn;
      }
    },
    async submit(event) {
      if (!this.handler) {
        throw new Error('submit handler was not registered');
      }
      return this.handler(event);
    }
  };
}

function stateSnapshot(revision, csrfToken) {
  return {
    mode: 'live',
    state_revision: revision + 1,
    csrf_token: csrfToken,
    settings: {
      revision,
      microphone: { name: 'Browser fixture mic' },
      stream_track: 1,
      track_confirmed_at: '2026-11-01T06:00:00Z',
      visual: null
    },
    secrets: { mode: 'memory_only', available: true },
    twitch: { status: 'not_connected' },
    obs: { status: 'not_connected' }
  };
}

function flush() {
  return new Promise((resolve) => setImmediate(resolve));
}

async function runScenario(name, { hash, snapshot, fetchedState, expectedRevision }) {
  const nodes = new Map();
  const getNode = (id) => {
    if (!nodes.has(id)) {
      nodes.set(id, makeNode());
    }
    return nodes.get(id);
  };
  const failures = [];
  let postedCSRF = null;
  let saveStatus = null;
  let stateFetches = 0;
  let bootstrapFetches = 0;
  let hashCleared = false;
  let currentHash = hash;

  getNode('mic-name').value = 'Browser fixture mic';
  getNode('track').value = '1';
  getNode('visual-enabled').checked = false;

  const response = (status, data) => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () => data
  });

  const sandbox = {
    document: { getElementById: getNode },
    location: {
      get hash() { return currentHash; },
      pathname: '/control/',
      search: ''
    },
    history: {
      replaceState(_state, _title, url) {
        currentHash = '';
        hashCleared = url === '/control/';
      }
    },
    URLSearchParams,
    Option: class {
      constructor(label, value) {
        this.label = label;
        this.value = value;
        this.dataset = {};
        this.disabled = false;
      }
    },
    EventSource: class {
      addEventListener() {}
    },
    fetch: async (url, options = {}) => {
      if (url === '/api/v1/session/bootstrap') {
        bootstrapFetches++;
        return response(200, { data: { csrf_token: snapshot.csrf_token } });
      }
      if (url === '/api/v1/state') {
        stateFetches++;
        return fetchedState
          ? response(401, { error: { message: 'A private session is required.' } })
          : response(200, { data: snapshot });
      }
      if (url === '/api/v1/settings') {
        postedCSRF = options.headers['X-CSRF-Token'];
        saveStatus = postedCSRF === snapshot.csrf_token ? 200 : 403;
        return response(saveStatus, {
          data: {
            ...snapshot.settings,
            revision: snapshot.settings.revision + 1
          }
        });
      }
      throw new Error('unexpected fetch ' + url);
    },
    console
  };

  vm.runInNewContext(script, sandbox, { filename: scriptPath });
  await flush();
  await flush();

  if (fetchedState) {
    // Locked reload path: no save should be posted.
    return {
      name,
      failures: stateFetches === 1 && bootstrapFetches === 0 ? [] : ['unexpected locked-state calls'],
      locked: getNode('locked').hidden === false
    };
  }

  if (hash) {
    if (!hashCleared) {
      failures.push('bootstrap fragment was not cleared');
    }
    if (bootstrapFetches !== 1) {
      failures.push('bootstrap was not called exactly once');
    }
  } else if (bootstrapFetches !== 0) {
    failures.push('reload unexpectedly attempted bootstrap');
  }

  if (stateFetches !== 1) {
    failures.push('private state was not loaded exactly once');
  }

  await getNode('settings-form').submit({ preventDefault() {} });
  await flush();

  if (postedCSRF !== snapshot.csrf_token) {
    failures.push('save did not use the state CSRF token');
  }
  if (saveStatus !== 200) {
    failures.push('save was rejected');
  }
  const notice = getNode('notice').textContent;
  if (!/Saved revision/.test(notice)) {
    failures.push('save success was not rendered');
  }
  if (getNode('app').hidden !== false) {
    failures.push('authenticated app was not shown');
  }
  if (expectedRevision !== undefined && snapshot.settings.revision !== expectedRevision) {
    failures.push('fixture revision mismatch');
  }

  return { name, failures, postedCSRF, saveStatus, notice, hashCleared };
}

(async () => {
  const bootstrap = await runScenario('bootstrap-then-save', {
    hash: '#bootstrap=DUMMY-ONE-TIME-TOKEN',
    snapshot: stateSnapshot(0, 'FIXTURE_SESSION_CSRF'),
    expectedRevision: 0
  });
  const reload = await runScenario('reload-with-cookie-then-save', {
    hash: '',
    snapshot: stateSnapshot(1, 'FIXTURE_SESSION_CSRF'),
    expectedRevision: 1
  });
  const locked = await runScenario('locked-reload', {
    hash: '',
    snapshot: stateSnapshot(1, 'FIXTURE_SESSION_CSRF'),
    fetchedState: true
  });

  const failures = [...bootstrap.failures, ...reload.failures, ...locked.failures];
  console.log(JSON.stringify({
    bootstrapCSRF: bootstrap.postedCSRF === 'FIXTURE_SESSION_CSRF',
    reloadCSRF: reload.postedCSRF === 'FIXTURE_SESSION_CSRF',
    reloadSaveStatus: reload.saveStatus,
    lockedReload: locked.locked === true,
    failures
  }, null, 2));
  if (failures.length) {
    process.exitCode = 1;
  }
})();
