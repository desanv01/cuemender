'use strict';

(() => {
  const statusNode = document.getElementById('status');
  const detailNode = document.getElementById('detail');
  const demoNode = document.getElementById('demo');
  const panelNode = document.getElementById('panel');
  const incidentsNode = document.getElementById('incidents');

  function stageCopy(incident) {
    switch (incident.stage) {
      case 'checking':
      case 'investigating':
      case 'reported': return 'Checking a viewer report';
      case 'finding_available': return incident.category === 'video' ? 'Visual item needs manual attention' : 'Microphone needs manual attention';
      case 'awaiting_approval': return 'Awaiting private approval';
      case 'applying': return 'Applying approved local change';
      case 'action_unknown': return 'Action outcome unknown; read-only reconciliation';
      case 'awaiting_feedback':
        return incident.local_verification === 'verified' ? 'Local configuration verified; manual findings remain' : 'Verification needs viewer feedback';
      case 'no_supported_finding': return 'No supported conflict found';
      case 'insufficient_information':
      case 'interrupted': return 'Evidence is incomplete';
      case 'closed':
        if (incident.closure_outcome === 'local_only') return 'Local configuration verified';
        if (incident.closure_outcome === 'viewer_confirmed') return 'Viewer confirmed';
        return 'Unresolved';
      default: return 'Unknown / unresolved';
    }
  }

  function describe(state) {
    const count = Array.isArray(state.incidents) ? state.incidents.length : 0;
    if (state.mode === 'rehearsal') {
      return { status: 'Rehearsal', detail: 'Simulated data only.' };
    }
    if (state.service_status === 'checking') return { status: 'Checking', detail: count ? 'Viewer report in progress.' : 'No active reports.' };
    if (state.service_status === 'ready') return { status: 'Watching', detail: count ? 'Viewer reports are being handled.' : 'No active reports.' };
    if (state.service_status === 'idle') return { status: 'Idle', detail: 'No active reports.' };
    return { status: 'Not connected', detail: 'Waiting for CueMender.' };
  }

  function render(state) {
    const view = describe(state);
    const count = Array.isArray(state.incidents) ? state.incidents.length : 0;
    panelNode.hidden = count === 0 && state.mode !== 'rehearsal' && state.service_status !== 'disconnected';
    statusNode.textContent = view.status;
    detailNode.textContent = view.detail;
    incidentsNode.replaceChildren();
    for (const incident of (state.incidents || []).slice(0, 1)) {
      const line = document.createElement('p');
      line.className = 'incident';
      line.textContent = incident.public_id + ' · ' + incident.category + ' · ' + stageCopy(incident);
      incidentsNode.appendChild(line);
      const feedback = document.createElement('p');
      feedback.className = 'detail';
      feedback.textContent = 'Viewer feedback: ' + incident.feedback_summary;
      incidentsNode.appendChild(feedback);
    }
    demoNode.hidden = !(state.mode === 'rehearsal' && state.demo_label);
    if (!demoNode.hidden) demoNode.textContent = state.demo_label;
  }

  const events = new EventSource('/public/v1/events');
  events.addEventListener('state', (event) => {
    try {
      render(JSON.parse(event.data));
    } catch (_) {
      statusNode.textContent = 'Not connected';
      detailNode.textContent = 'Waiting for CueMender.';
    }
  });
  events.addEventListener('error', () => {
    panelNode.hidden = false;
    statusNode.textContent = 'Not connected';
    detailNode.textContent = 'CueMender connection is unavailable.';
    incidentsNode.replaceChildren();
  });
})();
