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
    renderer.setStatus('error', 'Unavailable');
  }
}

async function connect() {
  if (!state.context || state.busy) {
    return false;
  }
  setBusy(true);
  renderer.clearError();
  renderer.setStatus('connecting', 'Connecting…');
  try {
    const data = await invoke('connect', buildConnectArguments(state.context));
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
    renderer.setStatus('connected', data.readOnly ? 'Connected · read-only' : 'Connected');
    await loadCollections();
    return true;
  } catch (error) {
    setConnected(false);
    renderer.showError(error);
    renderer.setStatus('error', 'Connection failed');
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
    await invoke('disconnect', {});
  } catch (error) {
    renderer.showError(error);
  } finally {
    setBusy(false);
    setConnected(false);
    renderer.setStatus('disconnected', 'Disconnected');
    renderer.setCollections([]);
    renderer.setDatabases([], '');
    renderer.setCurrentDatabase('');
    renderer.setScope('', '');
    state.database = '';
  }
}

async function ping() {
  if (!state.connected || state.busy) {
    return;
  }
  try {
    const data = await invoke('ping', {});
    renderer.setStatus('connected', `Connected · ${data.latencyMs} ms`);
  } catch (error) {
    renderer.showError(error);
    renderer.setStatus('error', 'Ping failed');
  }
}

async function run() {
  if (!state.connected || state.busy) {
    return;
  }
  const script = elements.editor.value;
  if (!script.trim()) {
    renderer.showError(new PluginCommandError('empty_script', 'Write a script before running it.'));
    elements.editor.focus();
    return;
  }
  setBusy(true);
  renderer.clearError();
  renderer.setStatus('running', 'Running…');
  elements.outputMeta.textContent = '';
  try {
    const data = await invoke('run', {
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
    renderer.setStatus('connected', data.readOnly ? 'Connected · read-only' : 'Connected');
  } catch (error) {
    renderer.showError(error);
    renderer.setStatus('error', 'Run failed');
  } finally {
    setBusy(false);
  }
}

async function loadCollections() {
  if (!state.connected) {
    return;
  }
  try {
    const data = await invoke('run', {
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

wireEvents();
void start();
