// Osprey Studio: a small browser UI over the Osprey HTTP API.
//
// No framework and no build step. All server data goes into the page through
// textContent or element properties, never innerHTML, because rule names,
// reasons, and IDs are free text.

const $ = (sel, root = document) => root.querySelector(sel);

// API base: the studio is served at <base>studio/, so strip that suffix. This
// keeps the studio working behind a reverse proxy that adds a path prefix.
const API_BASE = location.pathname.replace(/studio\/.*$/, '');

const OUTCOME_ORDER = { '.fail': 0, '.err': 1, '.review': 2, '.pass': 3 };

const store = {
  get(area, key, fallback) {
    try {
      const raw = area.getItem('osprey-studio.' + key);
      return raw === null ? fallback : JSON.parse(raw);
    } catch {
      return fallback;
    }
  },
  set(area, key, value) {
    try {
      area.setItem('osprey-studio.' + key, JSON.stringify(value));
    } catch {
      // Storage can be full or blocked; the studio still works without it.
    }
  },
};

const state = {
  // The tenant is not a secret, so it persists. The admin token lives only in
  // this tab's session storage and is gone when the tab closes.
  tenant: store.get(localStorage, 'tenant', 'studio'),
  token: store.get(sessionStorage, 'token', ''),
  health: null,
  rules: [],
  typologies: [],
  variables: [],
  // Osprey lists only enabled rules and typologies. The studio keeps a copy
  // of the ones disabled from this browser so they can be enabled again.
  disabledRules: store.get(localStorage, 'disabledRules', {}),
  disabledTypologies: store.get(localStorage, 'disabledTypologies', {}),
  history: store.get(sessionStorage, 'history', []),
  selectedRule: null, // id of the rule in the editor, or null for a new rule
  selectedTypology: null,
};

// ---------------------------------------------------------------------------
// DOM and API helpers

function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'text') el.textContent = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k in el && typeof v !== 'string') el[k] = v;
    else el.setAttribute(k, v === true ? '' : String(v));
  }
  for (const c of children.flat()) {
    if (c === null || c === undefined || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return el;
}

function toast(message, bad = false) {
  const el = h('div', { class: bad ? 'bad' : '', text: message });
  $('#toast').append(el);
  setTimeout(() => el.remove(), bad ? 8000 : 3500);
}

function explain(status, data) {
  const msg = (data && data.error) || `HTTP ${status}`;
  if (status === 401) return `${msg}. Enter the admin token (OSPREY_ADMIN_TOKEN) at the top of the page.`;
  if (status === 503 && /admin token is not configured/.test(msg)) return `${msg}. Start Osprey with OSPREY_ADMIN_TOKEN set to save changes.`;
  return msg;
}

async function api(method, path, body, { admin = false } = {}) {
  const headers = { 'X-Tenant-ID': state.tenant || 'studio' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (admin && state.token) headers['Authorization'] = 'Bearer ' + state.token;

  const res = await fetch(API_BASE + path.replace(/^\//, ''), {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { error: text.trim() };
  }
  if (!res.ok) {
    const err = new Error(explain(res.status, data));
    err.status = res.status;
    throw err;
  }
  return data;
}

function fmtScore(n) {
  return typeof n === 'number' ? n.toFixed(3).replace(/\.?0+$/, '') || '0' : '';
}

function sortById(list) {
  return [...list].sort((a, b) => a.id.localeCompare(b.id));
}

function download(filename, data) {
  const blob = new Blob([JSON.stringify(data, null, 2) + '\n'], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = h('a', { href: url, download: filename });
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Opens the file picker and resolves with the parsed JSON of the chosen file.
function pickJSON() {
  return new Promise((resolve, reject) => {
    const input = $('#file-input');
    input.value = '';
    input.onchange = async () => {
      const file = input.files[0];
      if (!file) return;
      try {
        resolve(JSON.parse(await file.text()));
      } catch (e) {
        reject(new Error(`${file.name} is not valid JSON: ${e.message}`));
      }
    };
    input.click();
  });
}

// ---------------------------------------------------------------------------
// Connection and tabs

async function loadHealth() {
  const pill = $('#server-status');
  try {
    const res = await fetch(API_BASE + 'health');
    state.health = await res.json();
    pill.textContent = `${state.health.status} · ${state.health.mode} mode · ${state.health.version}`;
    pill.className = 'pill ' + (state.health.status === 'healthy' ? 'ok' : 'bad');
  } catch {
    state.health = null;
    pill.textContent = 'Osprey not reachable';
    pill.className = 'pill bad';
  }
  const banner = $('#typology-banner');
  if (state.health && state.health.mode !== 'compliance') {
    banner.textContent = 'This server runs in detection mode, so evaluations do not use typologies. Start Osprey with OSPREY_MODE=compliance to use them. Typologies are global: a change applies to every tenant.';
  }
}

function selectTab(name) {
  for (const btn of document.querySelectorAll('.tabs button')) {
    btn.setAttribute('aria-selected', String(btn.dataset.tab === name));
  }
  for (const sec of document.querySelectorAll('.tab')) {
    sec.hidden = sec.id !== 'tab-' + name;
  }
  store.set(sessionStorage, 'tab', name);
}

// ---------------------------------------------------------------------------
// Test tab

const PRESETS = [
  {
    name: 'Normal transfer',
    tx: { type: 'TRANSFER', debtor: { id: 'user-a', accountId: 'acct-a' }, creditor: { id: 'merchant-b', accountId: 'acct-b' }, amount: { value: 125, currency: 'USD' } },
  },
  {
    name: 'Same-party transfer',
    tx: { type: 'TRANSFER', debtor: { id: 'user-alert', accountId: 'acct-alert-a' }, creditor: { id: 'user-alert', accountId: 'acct-alert-b' }, amount: { value: 125, currency: 'USD' } },
  },
  {
    name: 'Large transfer',
    tx: { type: 'TRANSFER', debtor: { id: 'user-c', accountId: 'acct-c' }, creditor: { id: 'payee-d', accountId: 'acct-d' }, amount: { value: 15000, currency: 'USD' } },
  },
  {
    name: 'Account emptied, new payee',
    tx: {
      type: 'TRANSFER',
      debtor: { id: 'user-e', accountId: 'acct-e' },
      creditor: { id: 'payee-f', accountId: 'acct-f' },
      amount: { value: 4900, currency: 'GBP' },
      metadata: { old_balance: 5000, new_balance: 100, new_payee: true, account_age_days: 3 },
      enrichment: { cop_result: 'no_match' },
    },
  },
];

function fillTx(tx) {
  const f = $('#tx-form').elements;
  f.type.value = tx.type || '';
  f.amount.value = tx.amount ? tx.amount.value : '';
  f.currency.value = tx.amount ? tx.amount.currency : '';
  f.debtorId.value = tx.debtor ? tx.debtor.id : '';
  f.debtorAccountId.value = tx.debtor ? tx.debtor.accountId || '' : '';
  f.creditorId.value = tx.creditor ? tx.creditor.id : '';
  f.creditorAccountId.value = tx.creditor ? tx.creditor.accountId || '' : '';
  f.id.value = tx.id || '';
  f.timestamp.value = tx.timestamp || '';
  f.metadata.value = tx.metadata ? JSON.stringify(tx.metadata, null, 2) : '';
  f.enrichment.value = tx.enrichment ? JSON.stringify(tx.enrichment, null, 2) : '';
}

function parseJSONField(field, label) {
  const raw = field.value.trim();
  if (!raw) return undefined;
  let value;
  try {
    value = JSON.parse(raw);
  } catch (e) {
    throw new Error(`${label} is not valid JSON: ${e.message}`);
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error(`${label} must be a JSON object, for example {"country": "GB"}`);
  }
  return value;
}

function readTx() {
  const f = $('#tx-form').elements;
  const tx = {
    type: f.type.value.trim(),
    debtor: { id: f.debtorId.value.trim(), accountId: f.debtorAccountId.value.trim() },
    creditor: { id: f.creditorId.value.trim(), accountId: f.creditorAccountId.value.trim() },
    amount: { value: Number(f.amount.value), currency: f.currency.value.trim() },
  };
  if (f.id.value.trim()) tx.id = f.id.value.trim();
  if (f.timestamp.value.trim()) tx.timestamp = f.timestamp.value.trim();
  const metadata = parseJSONField(f.metadata, 'Metadata');
  const enrichment = parseJSONField(f.enrichment, 'Enrichment');
  if (metadata) tx.metadata = metadata;
  if (enrichment) tx.enrichment = enrichment;
  return tx;
}

async function evaluate(event) {
  event.preventDefault();
  let base;
  try {
    base = readTx();
  } catch (e) {
    toast(e.message, true);
    return;
  }
  const count = Math.min(Math.max(parseInt($('#repeat').value, 10) || 1, 1), 50);
  const button = $('#tx-form button[type=submit]');
  button.disabled = true;
  try {
    let last;
    for (let i = 1; i <= count; i++) {
      const tx = structuredClone(base);
      // A fixed transaction ID can be stored only once, so number the repeats.
      if (tx.id && count > 1) tx.id = `${base.id}-${i}`;
      const resp = await api('POST', '/evaluate', tx);
      last = { resp, tx };
      addHistory(resp, tx);
    }
    await showEvaluation(last.resp.evaluationId, last.resp, last.tx);
    if (count > 1) toast(`Sent ${count} transactions. The result is for the last one.`);
  } catch (e) {
    toast(e.message, true);
  } finally {
    button.disabled = false;
  }
}

function addHistory(resp, tx) {
  state.history.unshift({
    at: new Date().toISOString(),
    evaluationId: resp.evaluationId,
    txId: resp.txId,
    debtor: tx.debtor.id,
    creditor: tx.creditor.id,
    amount: `${tx.amount.value} ${tx.amount.currency.toUpperCase()}`,
    status: resp.status,
    score: resp.score,
  });
  state.history = state.history.slice(0, 200);
  store.set(sessionStorage, 'history', state.history);
  renderHistory();
}

function renderHistory() {
  const body = $('#history tbody');
  body.replaceChildren(
    ...state.history.map((e) =>
      h('tr', { class: 'click', title: 'Open this evaluation', onclick: () => showEvaluation(e.evaluationId) },
        h('td', { class: 'num', text: new Date(e.at).toLocaleTimeString() }),
        h('td', { class: 'mono', text: e.txId }),
        h('td', { text: `${e.debtor} → ${e.creditor}` }),
        h('td', { class: 'num', text: e.amount }),
        h('td', {}, h('span', { class: 'badge ' + e.status, text: e.status })),
        h('td', { class: 'num', text: fmtScore(e.score) }),
      ),
    ),
  );
  if (!state.history.length) {
    body.append(h('tr', {}, h('td', { colspan: 6, class: 'hint', text: 'No evaluations yet.' })));
  }
}

// Loads the stored evaluation (rule results and typology results) and renders
// it. The POST /evaluate response has only the summary, so this needs the
// repository. If the read fails, the summary is shown.
async function showEvaluation(id, summary, tx) {
  const box = $('#result');
  let evaluation = null;
  let readError = null;
  try {
    evaluation = await api('GET', '/evaluations/' + encodeURIComponent(id));
  } catch (e) {
    readError = e;
    if (!summary) {
      toast(e.message, true);
      return;
    }
  }
  box.replaceChildren(...renderEvaluation(evaluation, summary, tx, readError));
}

function decisionWhy(ev) {
  const results = ev.ruleResults || [];
  const failed = results.filter((r) => r.subRuleRef === '.fail' || r.subRuleRef === '.err');
  const typologies = ev.typologyResults || [];
  const detection = typologies.find((t) => t.typologyId === 'detection-summary');
  const failedText = failed.map((r) => `${r.ruleId} (${r.subRuleRef})`).join(', ');

  if (ev.status === 'ALRT') {
    if (failed.length) return `Alert because ${failedText}. Any .fail or .err outcome gives an alert, whatever the score.`;
    if (detection) return `Alert because the weighted score ${fmtScore(ev.score)} is at or above the threshold ${fmtScore(detection.threshold)}.`;
    const hit = typologies.filter((t) => t.triggered).map((t) => t.typologyName || t.typologyId);
    if (hit.length) return `Alert because these typologies triggered: ${hit.join(', ')}.`;
    return 'Alert.';
  }
  if (!results.length) return 'No alert. No enabled rules were evaluated. Add rules on the Rules tab.';
  if (detection) return `No alert. No rule gave .fail, and the weighted score is below the threshold ${fmtScore(detection.threshold)}.`;
  return 'No alert. No typology reached its threshold, and no rule gave .fail.';
}

function outcomeBadge(ref) {
  const cls = (ref || '').replace('.', '');
  return h('span', { class: 'badge ' + cls, text: ref || '?' });
}

function ruleTable(results) {
  const sorted = [...results].sort(
    (a, b) => (OUTCOME_ORDER[a.subRuleRef] ?? 9) - (OUTCOME_ORDER[b.subRuleRef] ?? 9) || a.ruleId.localeCompare(b.ruleId),
  );
  return h('table', { class: 'table' },
    h('thead', {}, h('tr', {}, ...['Rule', 'Outcome', 'Score', 'Weight', 'Reason'].map((t) => h('th', { text: t })))),
    h('tbody', {},
      sorted.map((r) =>
        h('tr', {},
          h('td', { class: 'mono', text: r.ruleId }),
          h('td', {}, outcomeBadge(r.subRuleRef)),
          h('td', { class: 'num', text: fmtScore(r.score) }),
          h('td', { class: 'num', text: fmtScore(r.weight) }),
          h('td', { text: r.reason }),
        ),
      ),
    ),
  );
}

function meter(score, threshold) {
  const pct = (v) => `${Math.max(0, Math.min(1, v)) * 100}%`;
  // Set through the style object: the studio CSP blocks style attributes.
  const bar = h('span');
  bar.style.width = pct(score);
  const mark = h('i');
  mark.style.left = pct(threshold);
  return h('div', { class: 'meter', title: `score ${fmtScore(score)}, threshold ${fmtScore(threshold)}` }, bar, mark);
}

function typologyTable(typologies) {
  return h('table', { class: 'table' },
    h('thead', {}, h('tr', {}, ...['Typology', 'Score', 'Threshold', '', 'Triggered'].map((t) => h('th', { text: t })))),
    h('tbody', {},
      typologies.map((t) =>
        h('tr', {},
          h('td', {}, h('div', { text: t.typologyName || t.typologyId }), h('div', { class: 'hint mono', text: t.typologyId })),
          h('td', { class: 'num', text: fmtScore(t.score) }),
          h('td', { class: 'num', text: fmtScore(t.threshold) }),
          h('td', {}, meter(t.score, t.threshold)),
          h('td', {}, h('span', { class: 'badge ' + (t.triggered ? 'ALRT' : 'NALT'), text: t.triggered ? 'yes' : 'no' })),
        ),
      ),
    ),
  );
}

function renderEvaluation(ev, summary, tx, readError) {
  const status = ev ? ev.status : summary.status;
  const score = ev ? ev.score : summary.score;
  const out = [
    h('div', { class: 'decision' },
      h('div', { class: 'status ' + status, text: status }),
      h('div', { class: 'score num' }, fmtScore(score), h('small', { text: 'score' })),
    ),
  ];
  const ids = [
    `evaluation ${ev ? ev.id : summary.evaluationId}`,
    `transaction ${ev ? ev.txId : summary.txId}`,
  ];
  if (ev && ev.timestamp) ids.push(new Date(ev.timestamp).toLocaleString());
  out.push(h('div', { class: 'ids', text: ids.join(' · ') }));

  if (!ev) {
    out.push(h('p', { class: 'why', text: (summary.reasons || []).join('; ') || 'No reasons.' }));
    out.push(h('p', { class: 'hint', text: `Rule details are not available: ${readError.message}` }));
  } else {
    out.push(h('p', { class: 'why', text: decisionWhy(ev) }));
    const typologies = (ev.typologyResults || []).filter((t) => t.typologyId !== 'detection-summary');
    if (typologies.length) {
      out.push(h('h3', { text: 'Typologies' }), typologyTable(typologies));
    }
    out.push(h('h3', { text: `Rules (${(ev.ruleResults || []).length})` }));
    if ((ev.ruleResults || []).length) out.push(ruleTable(ev.ruleResults));
    else out.push(h('p', { class: 'hint', text: 'No rules ran.' }));
  }

  const raw = { evaluation: ev || summary };
  if (tx) raw.request = tx;
  out.push(h('details', {}, h('summary', { text: 'Raw JSON' }), h('pre', { text: JSON.stringify(raw, null, 2) })));
  return out;
}

// ---------------------------------------------------------------------------
// Rules tab

async function loadRules() {
  try {
    const data = await api('GET', '/rules');
    state.rules = sortById(data.rules || []);
  } catch (e) {
    state.rules = [];
    toast('Could not load rules: ' + e.message, true);
  }
  // A rule that is loaded again is no longer disabled.
  for (const r of state.rules) delete state.disabledRules[r.id];
  store.set(localStorage, 'disabledRules', state.disabledRules);
  $('#rule-ids').replaceChildren(...state.rules.map((r) => h('option', { value: r.id })));
  renderRuleList();
}

function listButton(item, active, onclick, muted) {
  return h('li', { class: muted ? 'muted-item' : '' },
    h('button', { type: 'button', class: active ? 'active' : '', onclick },
      h('span', { text: item.name || item.id }),
      h('span', { class: 'sub mono', text: item.id }),
    ),
  );
}

function renderRuleList() {
  const filter = $('#rule-filter').value.trim().toLowerCase();
  const match = (r) => !filter || [r.id, r.name, r.expression].some((s) => (s || '').toLowerCase().includes(filter));
  const list = $('#rule-list');
  const rules = state.rules.filter(match);
  list.replaceChildren(...rules.map((r) => listButton(r, r.id === state.selectedRule, () => editRule(r))));
  if (!rules.length) list.append(h('li', { class: 'hint', text: state.rules.length ? 'No match.' : 'No rules loaded.' }));

  const disabled = sortById(Object.values(state.disabledRules)).filter(match);
  $('#rule-disabled').replaceChildren(
    ...(disabled.length
      ? [
          h('div', { class: 'section-label', text: 'Disabled from this browser' }),
          h('ul', { class: 'items' }, disabled.map((r) => listButton(r, r.id === state.selectedRule, () => editRule(r), true))),
        ]
      : []),
  );
}

function newRule() {
  editRule(null);
}

function editRule(rule) {
  state.selectedRule = rule ? rule.id : null;
  const f = $('#rule-form').elements;
  const r = rule || {
    id: '',
    name: '',
    description: '',
    expression: 'amount > 10000.0',
    weight: 0.5,
    enabled: true,
    bands: boolBands('Amount above 10,000', 'Amount within limit'),
  };
  f.id.value = r.id;
  f.id.readOnly = Boolean(rule);
  f.name.value = r.name;
  f.description.value = r.description || '';
  f.expression.value = r.expression;
  f.weight.value = r.weight;
  f.enabled.checked = Boolean(r.enabled);
  $('#rule-title').textContent = rule ? `Edit ${rule.id}` : 'New rule';
  $('#rule-delete').disabled = !rule;
  renderBands(r.bands || []);
  renderRuleList();
  validateExpression();
}

function boolBands(failReason, passReason) {
  return [
    { lowerLimit: 1, subRuleRef: '.fail', reason: failReason || 'Condition matched' },
    { lowerLimit: 0, upperLimit: 1, subRuleRef: '.pass', reason: passReason || 'Condition not matched' },
  ];
}

function renderBands(bands) {
  $('#band-rows').replaceChildren(...bands.map(bandRow));
}

function bandRow(band = {}) {
  const num = (v) => (v === undefined || v === null ? '' : String(v));
  const select = h('select', { name: 'band-ref' },
    ['.pass', '.review', '.fail'].map((ref) => h('option', { value: ref, text: ref, selected: band.subRuleRef === ref })),
  );
  const row = h('tr', {},
    h('td', {}, h('input', { name: 'band-lower', type: 'number', step: 'any', value: num(band.lowerLimit), placeholder: '0' })),
    h('td', {}, h('input', { name: 'band-upper', type: 'number', step: 'any', value: num(band.upperLimit), placeholder: 'none' })),
    h('td', {}, select),
    h('td', {}, h('input', { name: 'band-reason', value: band.reason || '', required: true })),
    h('td', {}, h('button', { type: 'button', title: 'Remove band', text: '×', onclick: () => row.remove() })),
  );
  return row;
}

function readBands() {
  return [...$('#band-rows').children].map((row) => {
    const lower = $('[name=band-lower]', row).value.trim();
    const upper = $('[name=band-upper]', row).value.trim();
    const band = { subRuleRef: $('[name=band-ref]', row).value, reason: $('[name=band-reason]', row).value.trim() };
    if (lower !== '') band.lowerLimit = Number(lower);
    if (upper !== '') band.upperLimit = Number(upper);
    return band;
  });
}

let validateTimer = null;
let validateSeq = 0;

function validateExpression() {
  clearTimeout(validateTimer);
  const line = $('#expr-status');
  // Never show a result for an older version of the expression.
  validateSeq++;
  line.textContent = 'Checking…';
  line.className = 'status-line';
  validateTimer = setTimeout(async () => {
    const expression = $('#rule-form').elements.expression.value.trim();
    if (!expression) {
      line.textContent = '';
      line.className = 'status-line';
      return;
    }
    const seq = ++validateSeq;
    try {
      const res = await api('POST', '/rules/validate', { expression });
      if (seq !== validateSeq) return; // a newer check is running
      line.textContent = res.valid ? '✓ Expression compiles' : '✗ ' + friendlyCELError(res.error);
      line.className = 'status-line ' + (res.valid ? 'ok' : 'bad');
    } catch (e) {
      if (seq !== validateSeq) return;
      line.textContent = 'Could not check the expression: ' + e.message;
      line.className = 'status-line bad';
    }
  }, 300);
}

function friendlyCELError(message) {
  let msg = message.replace(/^failed to compile rule expression: /, '').replace(/^rule expression: /, '');
  // The most common mistake: CEL does not mix int and double in a comparison.
  if (/\(double, int\)|\(int, double\)/.test(msg)) {
    msg += '\nTip: amount is a double. Write 10000.0, not 10000.';
  }
  return msg;
}

function insertVariable(text) {
  const area = $('#rule-form').elements.expression;
  const start = area.selectionStart ?? area.value.length;
  const end = area.selectionEnd ?? area.value.length;
  area.setRangeText(text, start, end, 'end');
  area.focus();
  validateExpression();
}

async function loadVariables() {
  try {
    const data = await api('GET', '/rules/variables');
    state.variables = data.variables || [];
  } catch {
    state.variables = [];
  }
  const items = [];
  let group = null;
  for (const v of state.variables) {
    if (v.group !== group) {
      group = v.group;
      items.push(h('li', { class: 'group', text: group }));
    }
    const insert = v.access ? v.access.replace(/<[^>]+>/, 'key') : v.name;
    items.push(
      h('li', {},
        h('div', {},
          h('button', { type: 'button', class: 'link', text: v.access || v.name, onclick: () => insertVariable(insert) }),
          h('span', { class: 'type', text: v.type }),
        ),
        h('div', { text: v.description || '' }),
        v.note ? h('div', { text: v.note }) : null,
      ),
    );
  }
  $('#var-list').replaceChildren(...items);
}

function readRuleForm() {
  const f = $('#rule-form').elements;
  return {
    id: f.id.value.trim(),
    name: f.name.value.trim(),
    description: f.description.value.trim(),
    expression: f.expression.value.trim(),
    weight: Number(f.weight.value),
    enabled: f.enabled.checked,
    bands: readBands(),
  };
}

// Creates the rule, or updates it if it already exists on the server.
async function putRule(rule, exists) {
  if (exists) return api('PUT', '/rules/' + encodeURIComponent(rule.id), rule, { admin: true });
  try {
    return await api('POST', '/rules', rule, { admin: true });
  } catch (e) {
    // POST is an upsert on some versions and a conflict on others. Retry as PUT.
    if (e.status === 409) return api('PUT', '/rules/' + encodeURIComponent(rule.id), rule, { admin: true });
    throw e;
  }
}

function ruleExists(id) {
  return state.rules.some((r) => r.id === id) || id in state.disabledRules;
}

async function saveRule(event) {
  event.preventDefault();
  const rule = readRuleForm();
  try {
    await putRule(rule, ruleExists(rule.id));
  } catch (e) {
    toast(e.message, true);
    return;
  }
  if (rule.enabled) {
    delete state.disabledRules[rule.id];
    toast(`Saved ${rule.id}. The engine uses it now.`);
  } else {
    state.disabledRules[rule.id] = rule;
    toast(`Saved ${rule.id} as disabled. Osprey does not list disabled rules, so the studio keeps a copy in this browser.`);
  }
  store.set(localStorage, 'disabledRules', state.disabledRules);
  state.selectedRule = rule.id;
  await loadRules();
  editRule(state.rules.find((r) => r.id === rule.id) || state.disabledRules[rule.id]);
}

async function deleteRule() {
  const id = state.selectedRule;
  if (!id || !confirm(`Delete rule ${id}? This applies to every tenant.`)) return;
  try {
    await api('DELETE', '/rules/' + encodeURIComponent(id), undefined, { admin: true });
  } catch (e) {
    // A disabled rule is not loaded; if the server does not know it, drop the local copy.
    if (!(e.status === 404 && id in state.disabledRules)) {
      toast(e.message, true);
      return;
    }
  }
  delete state.disabledRules[id];
  store.set(localStorage, 'disabledRules', state.disabledRules);
  toast(`Deleted ${id}.`);
  await loadRules();
  newRule();
}

// Import accepts one item, an array of items, or an object that holds the
// array under key (the format of GET /rules and GET /typologies).
function listFrom(data, key) {
  if (Array.isArray(data)) return data;
  if (data && Array.isArray(data[key])) return data[key];
  if (data && typeof data === 'object' && data.id) return [data];
  return null;
}

function exportable(rule) {
  const { id, name, description, expression, bands, weight, enabled } = rule;
  return { id, name, description, expression, bands, weight, enabled };
}

function exportRules() {
  download('osprey-rules.json', state.rules.map(exportable));
}

async function importRules() {
  let data;
  try {
    data = await pickJSON();
  } catch (e) {
    if (e) toast(e.message, true);
    return;
  }
  const rules = listFrom(data, 'rules');
  if (!rules) {
    toast('Expected one rule, a JSON array of rules, or an object with a "rules" array.', true);
    return;
  }
  const failed = [];
  for (const r of rules) {
    try {
      await putRule(exportable({ enabled: true, ...r }), ruleExists(r.id));
    } catch (e) {
      failed.push(`${r.id || '(no id)'}: ${e.message}`);
    }
  }
  await loadRules();
  const ok = rules.length - failed.length;
  toast(`Imported ${ok} of ${rules.length} rules.` + (failed.length ? '\n' + failed.join('\n') : ''), failed.length > 0);
}

// ---------------------------------------------------------------------------
// Typologies tab

async function loadTypologies() {
  try {
    const data = await api('GET', '/typologies');
    state.typologies = sortById(data.typologies || []);
  } catch (e) {
    state.typologies = [];
    if (e.status !== 503) toast('Could not load typologies: ' + e.message, true);
  }
  // Health depends on the loaded typologies in compliance mode.
  loadHealth();
  for (const t of state.typologies) delete state.disabledTypologies[t.id];
  store.set(localStorage, 'disabledTypologies', state.disabledTypologies);
  renderTypologyList();
}

function renderTypologyList() {
  const filter = $('#typology-filter').value.trim().toLowerCase();
  const match = (t) => !filter || [t.id, t.name].some((s) => (s || '').toLowerCase().includes(filter));
  const list = $('#typology-list');
  const items = state.typologies.filter(match);
  list.replaceChildren(...items.map((t) => listButton(t, t.id === state.selectedTypology, () => editTypology(t))));
  if (!items.length) list.append(h('li', { class: 'hint', text: state.typologies.length ? 'No match.' : 'No typologies loaded.' }));

  const disabled = sortById(Object.values(state.disabledTypologies)).filter(match);
  $('#typology-disabled').replaceChildren(
    ...(disabled.length
      ? [
          h('div', { class: 'section-label', text: 'Disabled from this browser' }),
          h('ul', { class: 'items' }, disabled.map((t) => listButton(t, t.id === state.selectedTypology, () => editTypology(t), true))),
        ]
      : []),
  );
}

function editTypology(typology) {
  state.selectedTypology = typology ? typology.id : null;
  const f = $('#typology-form').elements;
  const t = typology || { id: '', name: '', description: '', alertThreshold: 0.5, enabled: true, rules: [] };
  f.id.value = t.id;
  f.id.readOnly = Boolean(typology);
  f.name.value = t.name;
  f.description.value = t.description || '';
  f.alertThreshold.value = t.alertThreshold;
  f.enabled.checked = Boolean(t.enabled);
  $('#typology-title').textContent = typology ? `Edit ${typology.id}` : 'New typology';
  $('#typology-delete').disabled = !typology;
  $('#trule-rows').replaceChildren(...(t.rules || []).map(typologyRuleRow));
  if (!typology) $('#trule-rows').append(typologyRuleRow());
  renderTypologyList();
}

function typologyRuleRow(tr = {}) {
  const row = h('tr', {},
    h('td', {}, h('input', { name: 'trule-id', list: 'rule-ids', value: tr.ruleId || '', placeholder: 'rule ID', required: true, spellcheck: 'false' })),
    h('td', {}, h('input', { name: 'trule-weight', type: 'number', min: '0', max: '1', step: '0.05', value: tr.weight ?? 0.5, required: true })),
    h('td', {}, h('button', { type: 'button', title: 'Remove rule', text: '×', onclick: () => row.remove() })),
  );
  return row;
}

function readTypologyForm() {
  const f = $('#typology-form').elements;
  return {
    id: f.id.value.trim(),
    name: f.name.value.trim(),
    description: f.description.value.trim(),
    alertThreshold: Number(f.alertThreshold.value),
    enabled: f.enabled.checked,
    rules: [...$('#trule-rows').children].map((row) => ({
      ruleId: $('[name=trule-id]', row).value.trim(),
      weight: Number($('[name=trule-weight]', row).value),
    })),
  };
}

function typologyExists(id) {
  return state.typologies.some((t) => t.id === id) || id in state.disabledTypologies;
}

async function putTypology(t, exists) {
  if (exists) return api('PUT', '/typologies/' + encodeURIComponent(t.id), t, { admin: true });
  try {
    return await api('POST', '/typologies', t, { admin: true });
  } catch (e) {
    if (e.status === 409) return api('PUT', '/typologies/' + encodeURIComponent(t.id), t, { admin: true });
    throw e;
  }
}

async function saveTypology(event) {
  event.preventDefault();
  const t = readTypologyForm();
  try {
    await putTypology(t, typologyExists(t.id));
  } catch (e) {
    toast(e.message, true);
    return;
  }
  if (t.enabled) {
    delete state.disabledTypologies[t.id];
    toast(`Saved ${t.id}.`);
  } else {
    state.disabledTypologies[t.id] = t;
    toast(`Saved ${t.id} as disabled. Osprey does not list disabled typologies, so the studio keeps a copy in this browser.`);
  }
  store.set(localStorage, 'disabledTypologies', state.disabledTypologies);
  await loadTypologies();
  editTypology(state.typologies.find((x) => x.id === t.id) || state.disabledTypologies[t.id]);
}

async function deleteTypology() {
  const id = state.selectedTypology;
  if (!id || !confirm(`Delete typology ${id}? This applies to every tenant.`)) return;
  try {
    await api('DELETE', '/typologies/' + encodeURIComponent(id), undefined, { admin: true });
  } catch (e) {
    if (!(e.status === 404 && id in state.disabledTypologies)) {
      toast(e.message, true);
      return;
    }
  }
  delete state.disabledTypologies[id];
  store.set(localStorage, 'disabledTypologies', state.disabledTypologies);
  toast(`Deleted ${id}.`);
  await loadTypologies();
  editTypology(null);
}

function exportableTypology(t) {
  const { id, name, description, rules, alertThreshold, enabled } = t;
  return { id, name, description, rules, alertThreshold, enabled };
}

async function importTypologies() {
  let data;
  try {
    data = await pickJSON();
  } catch (e) {
    toast(e.message, true);
    return;
  }
  const list = listFrom(data, 'typologies');
  if (!list) {
    toast('Expected one typology, a JSON array of typologies, or an object with a "typologies" array.', true);
    return;
  }
  const failed = [];
  for (const t of list) {
    try {
      await putTypology(exportableTypology({ enabled: true, ...t }), typologyExists(t.id));
    } catch (e) {
      failed.push(`${t.id || '(no id)'}: ${e.message}`);
    }
  }
  await loadTypologies();
  const ok = list.length - failed.length;
  toast(`Imported ${ok} of ${list.length} typologies.` + (failed.length ? '\n' + failed.join('\n') : ''), failed.length > 0);
}

// ---------------------------------------------------------------------------
// Start

function init() {
  const tenant = $('#tenant');
  const token = $('#token');
  tenant.value = state.tenant;
  token.value = state.token;
  tenant.addEventListener('change', () => {
    state.tenant = tenant.value.trim() || 'studio';
    tenant.value = state.tenant;
    store.set(localStorage, 'tenant', state.tenant);
  });
  token.addEventListener('input', () => {
    state.token = token.value.trim();
    store.set(sessionStorage, 'token', state.token);
  });

  for (const btn of document.querySelectorAll('.tabs button')) {
    btn.addEventListener('click', () => selectTab(btn.dataset.tab));
  }
  selectTab(store.get(sessionStorage, 'tab', 'test'));

  // Test tab
  const preset = $('#preset');
  preset.replaceChildren(...PRESETS.map((p, i) => h('option', { value: i, text: p.name })));
  preset.addEventListener('change', () => fillTx(PRESETS[preset.value].tx));
  fillTx(PRESETS[0].tx);
  $('#tx-form').addEventListener('submit', evaluate);
  $('#lookup').addEventListener('submit', (e) => {
    e.preventDefault();
    const id = $('#lookup-id').value.trim();
    if (id) showEvaluation(id);
  });
  renderHistory();

  // Rules tab
  $('#rule-filter').addEventListener('input', renderRuleList);
  $('#rule-new').addEventListener('click', newRule);
  $('#rule-form').addEventListener('submit', saveRule);
  $('#rule-form').elements.expression.addEventListener('input', validateExpression);
  $('#rule-delete').addEventListener('click', deleteRule);
  $('#band-add').addEventListener('click', () => $('#band-rows').append(bandRow({ subRuleRef: '.review' })));
  $('#band-bool').addEventListener('click', () => renderBands(boolBands()));
  $('#rule-export').addEventListener('click', exportRules);
  $('#rule-import').addEventListener('click', importRules);

  // Typologies tab
  $('#typology-filter').addEventListener('input', renderTypologyList);
  $('#typology-new').addEventListener('click', () => editTypology(null));
  $('#typology-form').addEventListener('submit', saveTypology);
  $('#typology-delete').addEventListener('click', deleteTypology);
  $('#trule-add').addEventListener('click', () => $('#trule-rows').append(typologyRuleRow()));
  $('#typology-export').addEventListener('click', () => download('osprey-typologies.json', state.typologies.map(exportableTypology)));
  $('#typology-import').addEventListener('click', importTypologies);

  loadVariables();
  loadRules().then(() => newRule());
  loadTypologies().then(() => editTypology(null));
}

init();
