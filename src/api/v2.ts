import type {
  ListPage,
  PackageInspection,
  PublisherPlugin,
  SourceUpdateResult,
  SubmissionRecord,
  WebSession,
} from './types';

// 与 client.ts 使用同一数据源口径：留空表示与站点同源；VITE_API_BASE 指向市场服务时走绝对地址。
// 账号接口依赖 HttpOnly 会话 Cookie，因此始终带 credentials: 'same-origin'。
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/+$/, '');

/** 会话失效事件：任何 401 都会触发，AuthContext 据此清空登录态。 */
export const SESSION_EXPIRED_EVENT = 'gonavi:market-session-expired';

export const MAX_PACKAGE_BYTES = 64 * 1024 * 1024;

export const PACKAGE_EXTENSIONS = ['.zip', '.tgz', '.tar.gz'];

export const SUBMISSION_STATE_LABELS: Record<string, string> = {
  draft: '等待提交',
  submitted: '已提交',
  validating: '校验中',
  pending_review: '等待发布',
  published: '已上架',
  rejected: '已拒绝',
  yanked: '已下架',
};

export const CATEGORY_LABELS: Record<string, string> = {
  '': '全部',
  datasource: '数据源',
  'ai-provider': 'AI 供应商',
  exporter: '导出器',
  skill: '技能',
  panel: '面板',
  command: '命令',
  ops: '运维',
  other: '其他',
};

const ERROR_MESSAGES: Record<string, string> = {
  authentication_required: '请先登录插件市场。',
  unauthorized: '登录已失效，请重新登录插件市场。',
  forbidden: '当前账号没有操作此插件的权限。',
  invalid_csrf: '登录状态已变化，请刷新账号后重试。',
  invalid_request: '请求内容不完整或格式不正确。',
  artifact_invalid: '插件包未通过静态校验，请检查 manifest、贡献声明和归档内容。',
  conflict: '此版本已被使用，不能覆盖已有内容；请查看版本记录。',
  version_locked: '该版本已拒绝或下架，不能重新发布。',
  inspection_changed: '提交的包与预检摘要不一致，请重新预检。',
  upload_busy: '当前上传较多，请稍后重试。',
  server_busy: '市场服务繁忙，请稍后重试。',
  rate_limited: '请求过于频繁，请稍后重试。',
  auth_unavailable: '市场登录暂不可用，请联系维护者。',
  not_found: '没有找到该资源，或你没有查看权限。',
  already_published: '此版本已经上架，无需重复发布。',
  version_conflict: '此版本已有不同内容，必须提升版本号。',
  source_update_failed: '插件已发布，但仓库登记未完成，请在我的插件中重试。',
  invalid_oauth_state: '登录链接已失效，请重新登录。',
  oauth_denied: 'GitHub 授权已取消。',
  identity_provider_failed: 'GitHub 身份验证暂未完成，请稍后重试。',
  unavailable: '市场服务暂不可用，请稍后重试。',
  invalid_response: '市场返回了无法解析的响应，请重试。',
};

/** 错误码 → 中文文案；未知错误码保留通用提示。 */
export function marketErrorMessage(code: string | null | undefined): string {
  if (!code) return '市场请求未完成，请稍后重试。';
  return ERROR_MESSAGES[code] ?? '市场请求未完成，请稍后重试。';
}

export class MarketRequestError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(code: string, status: number) {
    super(marketErrorMessage(code));
    this.name = 'MarketRequestError';
    this.code = code;
    this.status = status;
  }
}

/** 只允许同源路径作为登录回跳；登录页自身与 API 路径回退到 /account，避免死循环。 */
export function sanitizeReturnTo(value: string | null | undefined): string {
  const target = (value ?? '').trim();
  if (!target.startsWith('/') || target.startsWith('//') || target.includes('\\')) return '/account';
  const path = target.split(/[?#]/, 1)[0];
  if (path === '/login' || path.startsWith('/login/') || path.startsWith('/api/')) return '/account';
  return target;
}

function apiUrl(path: string): string {
  if (!path.startsWith('/')) throw new Error(`无效 API 路径：${path}`);
  return `${API_BASE}/api/v1${path}`;
}

interface RequestOptions extends RequestInit {
  csrf?: string;
}

function errorCodeOf(payload: unknown): string | null {
  if (typeof payload !== 'object' || payload === null || !('code' in payload)) return null;
  const code = (payload as { code?: unknown }).code;
  return typeof code === 'string' && code ? code : null;
}

/** 服务端未给出 code 时，按状态码回退到最接近的契约错误码，保证仍是中文文案。 */
function fallbackCode(status: number): string {
  if (status === 401) return 'authentication_required';
  if (status === 403) return 'forbidden';
  if (status === 404) return 'not_found';
  if (status === 409) return 'conflict';
  if (status === 429) return 'rate_limited';
  if (status >= 500) return 'server_busy';
  return 'unavailable';
}

async function requestJson<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { csrf, ...init } = options;
  const method = (init.method ?? 'GET').toUpperCase();
  const headers = new Headers(init.headers);
  if (method !== 'GET' && method !== 'HEAD') {
    // Cookie 认证的写请求必须带 CSRF；没有 token 说明会话未就绪，直接短路。
    if (!csrf) throw new MarketRequestError('authentication_required', 401);
    headers.set('X-GoNavi-CSRF', csrf);
  }

  const response = await fetch(apiUrl(path), {
    ...init,
    method,
    headers,
    credentials: 'same-origin',
    cache: 'no-store',
  });

  const text = await response.text();
  let payload: unknown = null;
  if (text) {
    try {
      payload = JSON.parse(text) as unknown;
    } catch {
      payload = null;
    }
  }

  if (response.status === 401) window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));

  if (!response.ok) {
    throw new MarketRequestError(errorCodeOf(payload) ?? fallbackCode(response.status), response.status);
  }
  if (payload === null) throw new MarketRequestError('invalid_response', response.status);
  return payload as T;
}

function listQuery(cursor?: string, pluginId?: string): string {
  const params = new URLSearchParams({ limit: '24' });
  if (cursor) params.set('cursor', cursor);
  if (pluginId) params.set('pluginId', pluginId);
  return `?${params.toString()}`;
}

// ---- 网站会话 ----

export function fetchWebSession(signal?: AbortSignal): Promise<WebSession> {
  return requestJson('/auth/web/session', { signal });
}

/** 登录地址是导航目标而非 fetch：服务端 303 到 GitHub OAuth。 */
export function webLoginUrl(returnTo: string): string {
  const params = new URLSearchParams({ returnTo: sanitizeReturnTo(returnTo) });
  return `${apiUrl('/auth/web/start')}?${params.toString()}`;
}

export async function logoutWebSession(csrf: string): Promise<void> {
  await requestJson<{ revoked: boolean }>('/auth/web/logout', { method: 'POST', csrf });
}

// ---- 发布者接口 ----

export function inspectPackage(file: File, csrf: string, signal?: AbortSignal): Promise<PackageInspection> {
  const body = new FormData();
  body.set('package', file);
  return requestJson('/publisher/inspect', { method: 'POST', body, csrf, signal });
}

export interface SubmissionIdentity {
  pluginId: string;
  version: string;
  sha256: string;
}

export function submitPackage(
  file: File,
  identity: SubmissionIdentity,
  source: string,
  csrf: string,
): Promise<SubmissionRecord> {
  const body = new FormData();
  body.set('package', file);
  body.set('pluginId', identity.pluginId);
  body.set('version', identity.version);
  body.set('expectedDigest', identity.sha256);
  const trimmedSource = source.trim();
  if (trimmedSource) body.set('source', trimmedSource);
  return requestJson('/submissions', { method: 'POST', body, csrf });
}

export function fetchPublisherPlugins(
  cursor?: string,
  pluginId?: string,
): Promise<ListPage<PublisherPlugin>> {
  return requestJson(`/publisher/plugins${listQuery(cursor, pluginId)}`);
}

export function fetchPublisherSubmissions(
  cursor?: string,
  pluginId?: string,
): Promise<ListPage<SubmissionRecord>> {
  return requestJson(`/publisher/submissions${listQuery(cursor, pluginId)}`);
}

export function updatePluginSource(
  pluginId: string,
  sourceUrl: string,
  csrf: string,
): Promise<SourceUpdateResult> {
  return requestJson(`/publisher/plugins/${encodeURIComponent(pluginId)}/source`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sourceUrl }),
    csrf,
  });
}

// ---- 展示辅助 ----

export function submissionStateLabel(state: string): string {
  return SUBMISSION_STATE_LABELS[state] ?? state;
}

export function categoryLabel(category: string | undefined): string {
  if (!category) return '其他';
  return CATEGORY_LABELS[category] ?? '其他';
}

/** 与服务端静态校验一致：ZIP/TGZ/TAR.GZ 且 ≤64 MiB。 */
export function isPackageFile(file: File | undefined): file is File {
  if (!file) return false;
  if (!(file.size > 0) || file.size > MAX_PACKAGE_BYTES) return false;
  const name = file.name.toLowerCase();
  return PACKAGE_EXTENSIONS.some((ext) => name.endsWith(ext));
}

/** 自动更新来源只接受公开 GitHub 仓库 HTTPS 链接（无凭据、端口、查询与片段）。 */
export function isGithubRepoUrl(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) return true;
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return false;
  }
  return (
    url.protocol === 'https:' &&
    url.hostname === 'github.com' &&
    !url.port &&
    !url.username &&
    !url.password &&
    !url.search &&
    !url.hash &&
    url.pathname.split('/').filter(Boolean).length === 2
  );
}
