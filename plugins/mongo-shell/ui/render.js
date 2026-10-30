/* DOM rendering for the MongoShell window. All values are inserted with
 * textContent; nothing here interprets HTML. */

import { formatCell, toDisplay } from './bridge.js';
import { errorText, t } from './i18n.js';

export function createRenderer(elements) {
  // 空态是一段图形 + 标题 + 说明，只有说明这段会被状态文案替换。
  const setEmptyText = (text) => {
    if (elements.tableEmptyText) {
      elements.tableEmptyText.textContent = text;
      return;
    }
    if (elements.tableEmpty) elements.tableEmpty.textContent = text;
  };

  const setStatus = (state, text) => {
    elements.status.classList.remove('status-connecting', 'status-running', 'status-connected', 'status-disconnected', 'status-error');
    elements.status.classList.add(`status-${state}`);
    elements.statusText.textContent = text;
  };

  const setBusy = (busy) => {
    elements.runButton.disabled = busy || elements.runButton.dataset.connected !== 'true';
    elements.pingButton.disabled = busy || elements.pingButton.dataset.connected !== 'true';
    elements.connectButton.disabled = busy;
    elements.editor.readOnly = busy;
    elements.app.classList.toggle('is-busy', busy);
    // The run button keeps its spinner slot and keycap; only the label text swaps.
    if (elements.runLabel) elements.runLabel.textContent = busy ? t('status.running') : t('action.run');
    if (elements.runSpinner) elements.runSpinner.hidden = !busy;
    if (elements.staleStrip) elements.staleStrip.hidden = !busy;
  };

  const setConnected = (connected) => {
    elements.runButton.dataset.connected = String(connected);
    elements.pingButton.dataset.connected = String(connected);
    elements.runButton.disabled = !connected;
    elements.pingButton.disabled = !connected;
    elements.connectButton.textContent = connected ? t('action.disconnect') : t('action.connect');
    elements.databaseSelect.disabled = !connected;
    elements.collectionInput.disabled = !connected;
  };

  const showError = (error) => {
    elements.errorCode.textContent = error && error.code ? error.code : 'error';
    elements.errorMessage.textContent = errorText(error);
    elements.error.hidden = false;
  };

  const clearError = () => {
    elements.error.hidden = true;
    elements.errorCode.textContent = '';
    elements.errorMessage.textContent = '';
  };

  const setConnection = (connection) => {
    elements.connectionName.textContent = connection.connectionName || 'MongoDB';
    if (connection.environmentType) {
      elements.environment.textContent = connection.environmentType;
      elements.environment.hidden = false;
    } else {
      elements.environment.hidden = true;
    }
    elements.readonlyChip.hidden = !connection.readOnly;
    elements.footerHost.textContent = connection.host || '—';
    elements.footerReadonly.textContent = connection.readOnly ? t('footer.readonly') : t('footer.writes_enabled');
    elements.footerReadonly.classList.toggle('footer-danger', connection.readOnly);
    elements.editorScope.textContent = scopeLabel(connection);
  };

  const setDatabases = (databases, selected) => {
    const values = Array.isArray(databases) ? databases : [];
    elements.databaseSelect.textContent = '';
    if (values.length === 0) {
      elements.databaseSelect.appendChild(option('', '(no databases)'));
      return;
    }
    for (const database of values) {
      const name = database && typeof database.name === 'string' ? database.name : String(database);
      elements.databaseSelect.appendChild(option(name, name));
    }
    if (selected && !values.some((database) => (database && database.name) === selected)) {
      elements.databaseSelect.appendChild(option(selected, selected));
    }
    if (selected) {
      elements.databaseSelect.value = selected;
    }
    elements.footerDatabase.textContent = t('footer.database', { name: elements.databaseSelect.value || '—' });
  };

  const setCurrentDatabase = (name) => {
    elements.footerDatabase.textContent = t('footer.database', { name: name || '—' });
    if (name) {
      elements.databaseSelect.value = name;
    }
  };

  const setScope = (database, collection) => {
    elements.editorScope.textContent = collection
      ? `${database || '(default)'} / ${collection}`
      : (database || 'connection scope');
  };

  const setCollections = (collections) => {
    elements.collectionOptions.textContent = '';
    const names = Array.isArray(collections) ? collections : [];
    for (const name of names) {
      if (typeof name !== 'string') {
        continue;
      }
      const choice = document.createElement('option');
      choice.value = name;
      elements.collectionOptions.appendChild(choice);
    }
  };

  const renderTable = (table) => {
    elements.resultHead.textContent = '';
    elements.resultBody.textContent = '';
    const columns = table && Array.isArray(table.columns) ? table.columns : [];
    const rows = table && Array.isArray(table.rows) ? table.rows : [];
    if (columns.length === 0) {
      elements.resultTable.hidden = true;
      elements.tableEmpty.hidden = false;
      setEmptyText(table ? t('result.no_columns', { count: rows.length }) : t('result.empty'));
      return;
    }
    const headRow = document.createElement('tr');
    for (const column of columns) {
      const cell = document.createElement('th');
      cell.scope = 'col';
      cell.textContent = String(column);
      headRow.appendChild(cell);
    }
    elements.resultHead.appendChild(headRow);

    const body = document.createDocumentFragment();
    for (const row of rows) {
      const rowElement = document.createElement('tr');
      for (let index = 0; index < columns.length; index += 1) {
        const cell = document.createElement('td');
        const value = Array.isArray(row) ? row[index] : undefined;
        const text = formatCell(value);
        cell.textContent = text;
        if (text.length > 60) {
          cell.title = text;
          cell.classList.add('cell-long');
        }
        if (value === null || value === undefined) {
          cell.classList.add('cell-null');
        }
        rowElement.appendChild(cell);
      }
      body.appendChild(rowElement);
    }
    // 有列但零行：表头照常显示，空行提示放在表体里，保持列结构可读。
    if (rows.length === 0) {
      const emptyRow = document.createElement('tr');
      emptyRow.className = 'result-empty-row';
      const cell = document.createElement('td');
      cell.colSpan = columns.length;
      cell.textContent = t('result.matched_none');
      emptyRow.appendChild(cell);
      body.appendChild(emptyRow);
    }
    elements.resultBody.appendChild(body);
    elements.resultTable.hidden = false;
    elements.tableEmpty.hidden = true;
  };

  // 达到 Max rows 时说明为什么少了行，而不是让用户以为数据就这么多。
  const setTruncatedNotice = (table, rows, maxRows) => {
    if (!elements.truncatedStrip) return;
    const truncated = Boolean(table && table.truncated);
    elements.truncatedStrip.hidden = !truncated;
    if (truncated && elements.truncatedText) {
      elements.truncatedText.textContent = t('result.truncated', { rows, max: maxRows });
    }
  };

  const setGuidance = (visible) => {
    if (!elements.guidance) return;
    elements.guidance.hidden = !visible;
    elements.toolbar?.toggleAttribute('hidden', visible);
    elements.workspace?.toggleAttribute('hidden', visible);
  };

  const renderOutput = (data) => {
    const lines = Array.isArray(data.output) ? data.output.slice() : [];
    const tableRows = data.table && Array.isArray(data.table.rows) ? data.table.rows.length : 0;
    if (data.result !== undefined && tableRows === 0) {
      lines.push(`=> ${toDisplay(data.result)}`);
    }
    elements.output.textContent = lines.length > 0 ? lines.join('\n') : '(no output)';
    elements.outputMeta.textContent = lines.length > 0 ? `${lines.length} line(s)` : '';
  };

  const setResultMeta = (data) => {
    const table = data.table;
    const rows = table && Array.isArray(table.rows) ? table.rows.length : 0;
    const parts = [`${rows} row(s)`, `${data.elapsedMs} ms`, `max ${data.maxRows}`];
    if (table && table.truncated) {
      parts.push('truncated');
    }
    elements.resultMeta.textContent = parts.join(' · ');
    elements.resultMeta.classList.toggle('meta-warn', Boolean(table && table.truncated));
    setTruncatedNotice(table, rows, data.maxRows);
    elements.footerElapsed.textContent = `${data.elapsedMs} ms`;
  };

  const clearResults = () => {
    elements.output.textContent = '';
    elements.outputMeta.textContent = '';
    elements.resultMeta.textContent = 'No query yet';
    elements.resultMeta.classList.remove('meta-warn');
    if (elements.truncatedStrip) elements.truncatedStrip.hidden = true;
    elements.footerElapsed.textContent = '—';
    elements.resultHead.textContent = '';
    elements.resultBody.textContent = '';
    elements.resultTable.hidden = true;
    elements.tableEmpty.hidden = false;
    setEmptyText(t('result.empty'));
  };

  return {
    setStatus,
    setGuidance,
    setBusy,
    setConnected,
    showError,
    clearError,
    setConnection,
    setDatabases,
    setCurrentDatabase,
    setScope,
    setCollections,
    renderTable,
    renderOutput,
    setResultMeta,
    clearResults,
  };
}

function option(value, label) {
  const element = document.createElement('option');
  element.value = value;
  element.textContent = label;
  return element;
}

function scopeLabel(connection) {
  if (connection.collection) {
    return `${connection.database || '(default)'} / ${connection.collection}`;
  }
  if (connection.database) {
    return connection.database;
  }
  return 'connection scope';
}
