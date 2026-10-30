/* MongoShell window entry. The launch context is read from
 * window.gonaviPlugin.ready; every backend call goes through gonaviPlugin.invoke
 * via the bridge module. The host UI SDK stylesheet and import map are declared
 * in index.html. */

import {
  PluginCommandError,
  buildConnectArguments,
  clampMaxRows,
  invoke,
  readLaunchContext,
  suggestScript,
} from './bridge.js';
import { createRenderer } from './render.js';
import { applyStaticText, applyHostTheme, t, useHostLocale } from './i18n.js';

const elements = {
  app: document.querySelector('.app'),
  connectionName: document.getElementById('connection-name'),
  environment: document.getElementById('environment'),
  readonlyChip: document.getElementById('readonly-chip'),
  status: document.getElementById('status'),
  statusText: document.getElementById('status-text'),
  pingButton: document.getElementById('ping-button'),
  connectButton: document.getElementById('connect-button'),
  error: document.getElementById('error'),
  errorCode: document.getElementById('error-code'),
  errorMessage: document.getElementById('error-message'),
  errorDismiss: document.getElementById('error-dismiss'),
  databaseSelect: document.getElementById('database-select'),
  collectionInput: document.getElementById('collection-input'),
  collectionOptions: document.getElementById('collection-options'),
  maxRows: document.getElementById('max-rows'),
  clearButton: document.getElementById('clear-button'),
  runButton: document.getElementById('run-button'),
  editorScope: document.getElementById('editor-scope'),
  editor: document.getElementById('editor'),
  resultMeta: document.getElementById('result-meta'),
  tableWrap: document.getElementById('table-wrap'),
  tableEmpty: document.getElementById('table-empty'),
  tableEmptyText: document.querySelector('#table-empty [data-i18n="result.empty"]'),
  runLabel: document.querySelector('#run-button .run-label'),
  runSpinner: document.querySelector('#run-button .spinner'),
  staleStrip: document.getElementById('result-stale'),
  truncatedStrip: document.getElementById('result-truncated'),
  truncatedText: document.getElementById('result-truncated-text'),
  guidance: document.getElementById('guidance'),
  toolbar: document.getElementById('toolbar'),
  workspace: document.getElementById('workspace'),
  outputToggle: document.getElementById('output-toggle'),
  resultTable: document.getElementById('result-table'),
  resultHead: document.getElementById('result-head'),
  resultBody: document.getElementById('result-body'),
  output: document.getElementById('output'),
  outputMeta: document.getElementById('output-meta'),
  footerHost: document.getElementById('footer-host'),
  footerDatabase: document.getElementById('footer-database'),
  footerReadonly: document.getElementById('footer-readonly'),
  footerElapsed: document.getElementById('footer-elapsed'),
};

const state = {
  context: null,
  sessionId: '',
  disposed: false,
  connected: false,
  busy: false,
  database: '',
  collection: '',
  maxRows: clampMaxRows(elements.maxRows.value),
  lastSuggestedScript: '',
};

const renderer = createRenderer(elements);

function setBusy(busy) {
  state.busy = busy;
  renderer.setBusy(busy);
}

function setConnected(connected) {
  state.connected = connected;
  renderer.setConnected(connected);
}

async function start() {
  renderer.setConnected(false);
  elements.maxRows.value = String(state.maxRows);
  try {
    const context = await readLaunchContext();
    if (state.disposed) return;
    state.context = context;
    state.database = context.database;
    state.collection = context.collection;
    state.maxRows = clampMaxRows(state.maxRows);
    elements.collectionInput.value = context.collection;
    renderer.setConnection({ ...context, host: '' });
    const script = suggestScript(context);
    elements.editor.value = script;
    state.lastSuggestedScript = script;
    await connect();
  } catch (error) {
    setConnected(false);
    renderer.showError(error);
    renderer.setStatus('error', t('status.unavailable'));
    // 没有连接上下文时不再摆一个用不了的工具栏，直接给出入口说明。
    renderer.setGuidance(['invalid_context', 'missing_credentials', 'bridge_unavailable'].includes(error && error.code));
  }
}

async function connect() {
  if (!state.context || state.busy || state.disposed) {
    return false;
  }
  setBusy(true);
  renderer.clearError();
  renderer.setStatus('connecting', t('status.connecting'));
  try {
    const data = await invoke('connect', buildConnectArguments(state.context));
    if (!data || typeof data.sessionId !== 'string' || !data.sessionId) {
      throw new PluginCommandError('invalid_response', 'The backend did not return a sessionId.');
    }
    if (state.disposed) {
      await releaseSession(data.sessionId);
      return false;
    }
    state.sessionId = data.sessionId;
    state.database = data.database || state.database;
    state.readOnly = data.readOnly === true;
    setConnected(true);
    renderer.setConnection({
      connectionName: data.connectionName || state.context.connectionName,
      environmentType: data.environmentType || state.context.environmentType,
      host: data.host,
      readOnly: data.readOnly === true,
      database: state.database,
      collection: state.collection,
    });
    renderer.setDatabases(data.databases, state.database);
    renderer.setCurrentDatabase(state.database);
    renderer.setScope(state.database, state.collection);
    renderer.setStatus('connected', data.readOnly ? t('status.connected_readonly') : t('status.connected'));
    await loadCollections();
    return true;
  } catch (error) {
    setConnected(false);
    renderer.showError(error);
    renderer.setStatus('error', t('status.connection_failed'));
    return false;
  } finally {
    setBusy(false);
  }
}

async function disconnect() {
  if (state.busy) {
    return;
  }
  setBusy(true);
  try {
    const sessionId = state.sessionId;
    state.sessionId = '';
    if (sessionId) await invoke('disconnect', { sessionId });
  } catch (error) {
    renderer.showError(error);
  } finally {
    setBusy(false);
    setConnected(false);
    renderer.setStatus('disconnected', t('status.disconnected'));
    renderer.setCollections([]);
    renderer.setDatabases([], '');
    renderer.setCurrentDatabase('');
    renderer.setScope('', '');
    state.database = '';
  }
}

async function ping() {
  if (!state.connected || !state.sessionId || state.disposed || state.busy) {
    return;
  }
  try {
    const data = await invoke('ping', { sessionId: state.sessionId });
    renderer.setStatus('connected', t('status.connected_latency', { ms: data.latencyMs }));
  } catch (error) {
    renderer.showError(error);
    renderer.setStatus('error', t('status.ping_failed'));
  }
}

async function run() {
  if (!state.connected || !state.sessionId || state.disposed || state.busy) {
    return;
  }
  const script = elements.editor.value;
  if (!script.trim()) {
    renderer.showError(new PluginCommandError('empty_script', t('error.empty_script')));
    elements.editor.focus();
    return;
  }
  setBusy(true);
  renderer.clearError();
  renderer.setStatus('running', t('status.running'));
  elements.outputMeta.textContent = '';
  try {
    const data = await invoke('run', {
      sessionId: state.sessionId,
      script,
      database: state.database || undefined,
      collection: state.collection || undefined,
      maxRows: state.maxRows,
    });
    state.database = data.database || state.database;
    renderer.setCurrentDatabase(state.database);
    renderer.setScope(state.database, state.collection);
    renderer.renderTable(data.table);
    renderer.renderOutput(data);
    renderer.setResultMeta(data);
    renderer.setStatus('connected', data.readOnly ? t('status.connected_readonly') : t('status.connected'));
  } catch (error) {
    renderer.showError(error);
    renderer.setStatus('error', t('status.run_failed'));
  } finally {
    setBusy(false);
  }
}

async function loadCollections() {
  if (!state.connected || !state.sessionId || state.disposed) {
    return;
  }
  try {
    const data = await invoke('run', {
      sessionId: state.sessionId,
      script: 'db.getCollectionNames()',
      database: state.database || undefined,
      maxRows: 1000,
    });
    const names = Array.isArray(data.result)
      ? data.result
      : (data.table && Array.isArray(data.table.rows) ? data.table.rows.map((row) => row[0]) : []);
    renderer.setCollections(names.filter((name) => typeof name === 'string'));
    if (state.collection) {
      elements.collectionInput.value = state.collection;
    }
  } catch {
    // The collection list is a convenience; a missing list must not hide a
    // successful connection. The user can still run getCollectionNames().
    renderer.setCollections([]);
  }
}

function syncSuggestedScript() {
  if (elements.editor.value === state.lastSuggestedScript) {
    const script = suggestScript({
      collection: state.collection,
      database: state.database,
    });
    elements.editor.value = script;
    state.lastSuggestedScript = script;
  }
}

function wireEvents() {
  if (elements.outputToggle) {
    elements.outputToggle.addEventListener('click', () => {
      const collapsed = elements.app.classList.toggle('output-collapsed');
      elements.outputToggle.setAttribute('aria-expanded', String(!collapsed));
    });
  }
  elements.runButton.addEventListener('click', () => { void run(); });
  elements.clearButton.addEventListener('click', () => renderer.clearResults());
  elements.pingButton.addEventListener('click', () => { void ping(); });
  elements.connectButton.addEventListener('click', () => {
    if (state.connected) {
      void disconnect();
    } else {
      void connect();
    }
  });
  elements.errorDismiss.addEventListener('click', () => renderer.clearError());

  elements.databaseSelect.addEventListener('change', () => {
    state.database = elements.databaseSelect.value;
    renderer.setCurrentDatabase(state.database);
    renderer.setScope(state.database, state.collection);
    syncSuggestedScript();
    void loadCollections();
  });

  elements.collectionInput.addEventListener('change', () => {
    state.collection = elements.collectionInput.value.trim();
    renderer.setScope(state.database, state.collection);
    syncSuggestedScript();
  });

  elements.maxRows.addEventListener('change', () => {
    state.maxRows = clampMaxRows(elements.maxRows.value);
    elements.maxRows.value = String(state.maxRows);
  });

  elements.editor.addEventListener('keydown', (event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      void run();
      return;
    }
    if (event.key === 'Tab') {
      event.preventDefault();
      insertAtCursor('  ');
    }
  });
}

function insertAtCursor(text) {
  const editor = elements.editor;
  const start = editor.selectionStart;
  const end = editor.selectionEnd;
  editor.value = `${editor.value.slice(0, start)}${text}${editor.value.slice(end)}`;
  editor.selectionStart = start + text.length;
  editor.selectionEnd = start + text.length;
}

async function releaseSession(sessionId) {
  try {
    await invoke('disconnect', { sessionId });
  } catch {
    // Unload is best-effort; the backend also releases clients on process close.
  }
}

function dispose() {
  state.disposed = true;
  const sessionId = state.sessionId;
  state.sessionId = '';
  setConnected(false);
  if (sessionId) void releaseSession(sessionId);
}

window.addEventListener('pagehide', dispose);
window.addEventListener('beforeunload', dispose);
useHostLocale();
applyStaticText();
applyHostTheme();
wireEvents();
void start();
