'use strict';

(() => {
  const statusNode = document.getElementById('status');
  const detailNode = document.getElementById('detail');
  const demoNode = document.getElementById('demo');

  function describe(state) {
    if (state.mode === 'rehearsal') {
      return { status: 'Rehearsal', detail: 'Simulated data only.' };
    }
    const count = Array.isArray(state.incidents) ? state.incidents.length : 0;
    if (state.service_status === 'checking') {
      return { status: 'Checking', detail: count ? count + ' active report' + (count === 1 ? '' : 's') : 'No active reports.' };
    }
    if (state.service_status === 'ready') {
      return { status: 'Ready', detail: count ? count + ' active report' + (count === 1 ? '' : 's') : 'No active reports.' };
    }
    if (state.service_status === 'idle') {
      return { status: 'Idle', detail: 'No active reports.' };
    }
    return { status: 'Not connected', detail: 'Waiting for LiveCheck.' };
  }

  function render(state) {
    const view = describe(state);
    statusNode.textContent = view.status;
    detailNode.textContent = view.detail;
    if (state.mode === 'rehearsal' && state.demo_label) {
      demoNode.textContent = state.demo_label;
      demoNode.hidden = false;
    } else {
      demoNode.hidden = true;
    }
  }

  const events = new EventSource('/public/v1/events');
  events.addEventListener('state', (event) => {
    try {
      render(JSON.parse(event.data));
    } catch (_) {
      statusNode.textContent = 'Not connected';
      detailNode.textContent = 'Waiting for LiveCheck.';
    }
  });
  events.addEventListener('error', () => {
    statusNode.textContent = 'Not connected';
    detailNode.textContent = 'Waiting for LiveCheck.';
  });
})();
