// 華龍算力账号窗口：只通过 window.gonaviPlugin.invoke 调用后端命令，
// 不直接访问网络、不读写本地存储。后端返回 {ok,data} / {ok,error} 信封。

const POLL_INTERVAL_MS = 2000;
const POLL_BUDGET_MS = 5 * 60 * 1000;

const STATUS_LABELS = {
  active: '正常',
  inactive: '未激活',
  disabled: '已停用',
  expired: '已过期',
  banned: '已封禁',
};

class CommandError extends Error {
  constructor(code, message) {
    super(message || code);
    this.name = 'CommandError';
    this.code = code;
  }
}

const state = {
  status: null,
  keys: [],
  revealed: new Set(),
  polling: false,
  timer: null,
};

const elements = {};

function cacheElements() {
  const ids = [
    'brand-sub', 'status', 'status-text', 'refresh-button', 'logout-button',
    'error', 'error-code', 'error-message', 'retry-button',
    'signed-out', 'signed-in', 'login-button', 'login-hint',
    'account-email', 'account-meta', 'account-chip',
    'balance-value', 'frozen-value', 'recharged-value', 'concurrency-value',
    'keys-meta', 'keys-error', 'keys-error-message', 'keys-retry', 'keys-empty',
    'keys-table', 'keys-body', 'footer-user', 'footer-updated',
  ];
  for (const id of ids) {
    elements[id] = document.getElementById(id);
  }
}

async function invoke(command, args = {}) {
  const bridge = globalThis.gonaviPlugin;
  if (!bridge || typeof bridge.invoke !== 'function') {
    throw new CommandError('bridge_unavailable', '宿主没有提供插件通道，请从 GoNavi 打开本窗口。');
  }
  let payload;
  try {
    payload = await bridge.invoke(command, args);
  } catch (error) {
    const code = error && typeof error.code === 'string' ? error.code : 'invocation_failed';
    throw new CommandError(code, `宿主拒绝了 ${command}（${code}）。`);
  }
  if (!payload || typeof payload !== 'object') {
    throw new CommandError('invalid_response', `${command} 返回了无法识别的内容。`);
  }
  if (payload.ok !== true) {
    const failure = payload.error && typeof payload.error === 'object' ? payload.error : {};
    const code = typeof failure.code === 'string' ? failure.code : 'internal';
    const message = typeof failure.message === 'string' && failure.message
      ? failure.message
      : `${command} 执行失败。`;
    throw new CommandError(code, message);
  }
  return payload.data;
}

function setStatus(kind, text) {
  elements.status.className = `status status-${kind}`;
  elements['status-text'].textContent = text;
}

function showError(error) {
  elements['error-code'].textContent = error && error.code ? error.code : 'error';
  elements['error-message'].textContent = error && error.message ? error.message : '未知错误。';
  elements.error.hidden = false;
}

function clearError() {
  elements.error.hidden = true;
  elements['error-code'].textContent = '';
  elements['error-message'].textContent = '';
}

function setBusy(busy) {
  elements['refresh-button'].disabled = busy;
  elements['login-button'].disabled = busy;
}

function formatAmount(value) {
  const number = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(number)) return '—';
  return number.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 4 });
}

function formatTime(value) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const pad = (part) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function statusLabel(status) {
  if (!status) return '—';
  return STATUS_LABELS[status] || status;
}

function maskKey(key) {
  const value = typeof key === 'string' ? key : '';
  if (value.length === 0) return '—';
  if (value.length <= 12) return '****';
  return `${value.slice(0, 7)}****${value.slice(-4)}`;
}

function quotaLabel(item) {
  const quota = Number(item.quota) || 0;
  const used = Number(item.quotaUsed) || 0;
  if (quota <= 0) return `不限 · 已用 ${formatAmount(used)}`;
  return `${formatAmount(quota)} · 已用 ${formatAmount(used)}`;
}

function groupLabel(item) {
  const name = item.groupName || '默认分组';
  const rate = Number(item.rateMultiplier);
  const base = Number.isFinite(rate) && rate > 0 ? `${name} ×${rate}` : name;
  return item.platform ? `${base}（${item.platform}）` : base;
}

function cell(text, className) {
  const td = document.createElement('td');
  td.className = className;
  td.textContent = text;
  return td;
}

function chipCell(status) {
  const td = document.createElement('td');
  td.className = 'cell-status';
  const chip = document.createElement('span');
  chip.className = status === 'active' ? 'chip chip-ok' : 'chip chip-muted';
  chip.textContent = statusLabel(status);
  td.append(chip);
  return td;
}

function actionButton(label, className, onClick) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = className;
  button.textContent = label;
  button.addEventListener('click', onClick);
  return button;
}

async function copyKey(item, button) {
  try {
    await navigator.clipboard.writeText(item.key);
    const original = button.textContent;
    button.textContent = '已复制';
    globalThis.setTimeout(() => { button.textContent = original; }, 1200);
  } catch (error) {
    elements['keys-error-message'].textContent = '复制失败，请点击「显示」后手动选中。';
    elements['keys-error'].hidden = false;
  }
}

function buildKeyRow(item) {
  const row = document.createElement('tr');
  row.append(cell(item.name || '—', 'cell-name'));
  row.append(cell(groupLabel(item), 'cell-group'));
  row.append(chipCell(item.status));
  row.append(cell(quotaLabel(item), 'cell-quota'));
  row.append(cell(formatTime(item.lastUsedAt), 'cell-time'));

  const keyCell = document.createElement('td');
  keyCell.className = 'cell-key';
  const code = document.createElement('code');
  code.className = 'key-value';
  const revealed = state.revealed.has(item.id);
  code.textContent = revealed ? item.key : maskKey(item.key);
  keyCell.append(code);
  row.append(keyCell);

  const actions = document.createElement('td');
  actions.className = 'cell-actions';
  const group = document.createElement('div');
  group.className = 'row-actions';
  group.append(actionButton(revealed ? '隐藏' : '显示', 'button button-tiny', () => {
    if (state.revealed.has(item.id)) {
      state.revealed.delete(item.id);
    } else {
      state.revealed.add(item.id);
    }
    renderKeys();
  }));
  const copy = actionButton('复制', 'button button-tiny button-quiet', () => { void copyKey(item, copy); });
  group.append(copy);
  actions.append(group);
  row.append(actions);
  return row;
}

function renderKeys() {
  const items = Array.isArray(state.keys) ? state.keys : [];
  elements['keys-body'].replaceChildren();
  for (const item of items) {
    elements['keys-body'].append(buildKeyRow(item));
  }
  elements['keys-table'].hidden = items.length === 0;
  elements['keys-empty'].hidden = items.length !== 0;
  elements['keys-meta'].textContent = items.length === 0 ? '无 Key' : `共 ${items.length} 个`;
}

function renderSignedOut() {
  state.status = null;
  state.keys = [];
  state.revealed.clear();
  elements['signed-in'].hidden = true;
  elements['signed-out'].hidden = false;
  elements['logout-button'].hidden = true;
  elements['refresh-button'].hidden = true;
  elements['brand-sub'].textContent = '未登录';
  elements['footer-user'].textContent = '未登录';
  elements['footer-updated'].textContent = '—';
  setStatus('idle', '未登录');
}

function renderAccount(status) {
  state.status = status;
  elements['signed-out'].hidden = true;
  elements['signed-in'].hidden = false;
  elements['logout-button'].hidden = false;
  elements['refresh-button'].hidden = false;
  elements['account-email'].textContent = status.email || '—';
  elements['account-meta'].textContent = `用户 ID ${status.userId ?? '—'} · 角色 ${status.role || '—'}`;
  elements['account-chip'].textContent = statusLabel(status.status);
  elements['account-chip'].className = status.status === 'active' ? 'chip chip-ok' : 'chip chip-muted';
  elements['balance-value'].textContent = formatAmount(status.balance);
  elements['frozen-value'].textContent = formatAmount(status.frozenBalance);
  elements['recharged-value'].textContent = formatAmount(status.totalRecharged);
  elements['concurrency-value'].textContent = Number.isFinite(Number(status.concurrency))
    ? String(status.concurrency)
    : '—';
  elements['brand-sub'].textContent = status.email || '已登录';
  elements['footer-user'].textContent = status.email || '—';
  elements['footer-updated'].textContent = `最近活跃 ${formatTime(status.lastActiveAt)}`;
  setStatus('ok', '已登录');
}

async function loadKeys() {
  elements['keys-error'].hidden = true;
  try {
    const data = await invoke('account.keys', {});
    state.keys = Array.isArray(data && data.items) ? data.items : [];
    renderKeys();
  } catch (error) {
    state.keys = [];
    renderKeys();
    elements['keys-error-message'].textContent = error.message || '读取 API Key 失败。';
    elements['keys-error'].hidden = false;
  }
}

async function loadAccount() {
  clearError();
  setBusy(true);
  setStatus('busy', '正在读取账号…');
  try {
    const status = await invoke('account.status', {});
    if (!status || status.state !== 'ready') {
      renderSignedOut();
      return;
    }
    renderAccount(status);
    await loadKeys();
  } catch (error) {
    showError(error);
    setStatus('error', '读取失败');
    if (!state.status) {
      elements['signed-in'].hidden = true;
      elements['signed-out'].hidden = true;
    }
  } finally {
    setBusy(false);
  }
}

function stopPolling() {
  state.polling = false;
  if (state.timer) {
    globalThis.clearTimeout(state.timer);
    state.timer = null;
  }
  elements['login-hint'].hidden = true;
  elements['login-button'].disabled = false;
}

function startPolling() {
  if (state.polling) return;
  state.polling = true;
  const deadline = Date.now() + POLL_BUDGET_MS;
  const tick = async () => {
    if (!state.polling) return;
    if (Date.now() > deadline) {
      stopPolling();
      showError(new CommandError('login_timeout', '登录等待超时，请重新点击「打开浏览器登录」。'));
      setStatus('error', '登录超时');
      return;
    }
    try {
      const data = await invoke('account.poll', {});
      if (data && data.state === 'ready') {
        stopPolling();
        await loadAccount();
        return;
      }
    } catch (error) {
      stopPolling();
      showError(error);
      setStatus('error', '读取登录状态失败');
      return;
    }
    state.timer = globalThis.setTimeout(() => { void tick(); }, POLL_INTERVAL_MS);
  };
  state.timer = globalThis.setTimeout(() => { void tick(); }, POLL_INTERVAL_MS);
}

async function startLogin() {
  clearError();
  setBusy(true);
  setStatus('busy', '正在打开浏览器…');
  try {
    await invoke('account.login', {});
    elements['login-hint'].hidden = false;
    setStatus('busy', '等待登录…');
    setBusy(false);
    startPolling();
  } catch (error) {
    setBusy(false);
    showError(error);
    setStatus('error', '打开浏览器失败');
  }
}

async function signOut() {
  clearError();
  stopPolling();
  setBusy(true);
  try {
    await invoke('account.logout', {});
    renderSignedOut();
  } catch (error) {
    showError(error);
  } finally {
    setBusy(false);
  }
}

function bindEvents() {
  elements['login-button'].addEventListener('click', () => { void startLogin(); });
  elements['logout-button'].addEventListener('click', () => { void signOut(); });
  elements['refresh-button'].addEventListener('click', () => { void loadAccount(); });
  elements['retry-button'].addEventListener('click', () => { void loadAccount(); });
  elements['keys-retry'].addEventListener('click', () => { void loadKeys(); });
}

function main() {
  cacheElements();
  bindEvents();
  renderSignedOut();
  setStatus('busy', '连接中…');
  void loadAccount();
}

main();
