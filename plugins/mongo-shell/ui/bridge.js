/* Bridge helpers for the MongoShell window. The only host channel used here is
 * window.gonaviPlugin.invoke; no fetch, no storage, no window.go. */

export const DEFAULT_MAX_ROWS = 200;
export const MAX_ROWS_LIMIT = 2000;

export class PluginCommandError extends Error {
  constructor(code, message) {
    super(message);
    this.name = 'PluginCommandError';
    this.code = code;
  }
}

/** Invoke one declared backend command and unwrap the plugin result envelope. */
export async function invoke(command, args = {}) {
  const bridge = globalThis.gonaviPlugin;
  if (!bridge || typeof bridge.invoke !== 'function') {
    throw new PluginCommandError('bridge_unavailable', 'The GoNavi plugin bridge is not available in this window.');
  }
  let payload;
  try {
    payload = await bridge.invoke(command, args);
  } catch (error) {
    const code = error && typeof error.code === 'string' ? error.code : 'invocation_failed';
    throw new PluginCommandError(code, `The host rejected ${command} (${code}).`);
  }
  if (!payload || typeof payload !== 'object') {
    throw new PluginCommandError('invalid_response', `The plugin backend returned an invalid ${command} response.`);
  }
  if (payload.ok !== true) {
    const failure = payload.error && typeof payload.error === 'object' ? payload.error : {};
    const code = typeof failure.code === 'string' ? failure.code : 'internal';
    const message = typeof failure.message === 'string' && failure.message
      ? failure.message
      : `The ${command} command failed.`;
    throw new PluginCommandError(code, message);
  }
  return payload.data;
}

/** Wait for the host bootstrap and validate the connection-action context. */
export async function readLaunchContext() {
  const bridge = globalThis.gonaviPlugin;
  if (!bridge || !bridge.ready) {
    throw new PluginCommandError('bridge_unavailable', 'The GoNavi plugin bridge is not available in this window.');
  }
  let identity;
  try {
    identity = await bridge.ready;
  } catch (error) {
    const code = error && typeof error.code === 'string' ? error.code : 'bootstrap_failed';
    throw new PluginCommandError(code, `The host did not provide a window session (${code}).`);
  }
  const context = identity && typeof identity === 'object' ? identity.context : undefined;
  if (!context || typeof context !== 'object') {
    throw new PluginCommandError('invalid_context', 'This window was not launched from a MongoDB connection action.');
  }
  if (context.kind !== 'connectionAction') {
    throw new PluginCommandError('invalid_context', `Unsupported launch context: ${String(context.kind)}.`);
  }
  const connectionType = String(context.connectionType || '').toLowerCase();
  if (connectionType && connectionType !== 'mongodb') {
    throw new PluginCommandError('unsupported_connection', `MongoShell cannot open a ${connectionType} connection.`);
  }
  if (!context.config || typeof context.config !== 'object') {
    throw new PluginCommandError(
      'missing_credentials',
      'The launch context has no connection configuration. Enable host.connections.credentials and relaunch the window.',
    );
  }
  return {
    actionId: String(context.actionId || ''),
    connectionId: String(context.connectionId || ''),
    connectionName: String(context.connectionName || 'MongoDB'),
    connectionType: connectionType || 'mongodb',
    database: typeof context.database === 'string' ? context.database : '',
    collection: typeof context.collection === 'string' ? context.collection : '',
    environmentType: typeof context.environmentType === 'string' ? context.environmentType : '',
    readOnly: context.readOnly === true,
    config: context.config,
  };
}

export function buildConnectArguments(context) {
  return {
    config: context.config,
    readOnly: context.readOnly === true,
    database: context.database,
    collection: context.collection,
    connectionId: context.connectionId,
    connectionName: context.connectionName,
    environmentType: context.environmentType,
  };
}

/** Suggest a first script for the scope that opened the window. */
export function suggestScript(context) {
  if (context.collection) {
    return `db.getCollection(${JSON.stringify(context.collection)}).find({}, {}, 20)`;
  }
  return 'db.getCollectionNames()';
}

export function clampMaxRows(value) {
  const parsed = Number.parseInt(String(value ?? ''), 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return DEFAULT_MAX_ROWS;
  }
  return Math.min(parsed, MAX_ROWS_LIMIT);
}

/** Render any JSON-safe value for the output pane. */
export function toDisplay(value) {
  if (value === undefined) {
    return 'undefined';
  }
  if (typeof value === 'string') {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function formatCell(value) {
  if (value === null || value === undefined) {
    return 'null';
  }
  if (typeof value === 'string') {
    return value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}
