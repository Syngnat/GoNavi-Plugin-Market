import type { CatalogPage, LoginStatus, PluginDetail, PluginItem } from './types';

// 默认数据源是随站点发布的 catalog.json（真实上架目录，无运营数据）；
// 配置 VITE_API_BASE 后切换到市场服务的 /api/v1。
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/+$/, '');
export const isMarketServiceConnected = API_BASE !== '';

export type CatalogSort = 'recent' | 'rating' | 'downloads';

export interface CatalogQuery {
  category?: string;
  sort?: CatalogSort;
  q?: string;
  cursor?: string;
}

async function fetchLocalCatalog(): Promise<PluginItem[]> {
  const resp = await fetch('catalog.json');
  if (!resp.ok) throw new Error(`catalog.json request failed: ${resp.status}`);
  const page = (await resp.json()) as CatalogPage;
  return page.items;
}

export async function fetchCatalog(query: CatalogQuery): Promise<CatalogPage> {
  if (isMarketServiceConnected) {
    const params = new URLSearchParams();
    if (query.category && query.category !== '全部') params.set('category', query.category);
    if (query.q) params.set('q', query.q);
    if (query.cursor) params.set('cursor', query.cursor);
    params.set('sort', query.sort ?? 'recent');
    const resp = await fetch(`${API_BASE}/api/v1/plugins?${params.toString()}`);
    if (!resp.ok) throw new Error(`catalog request failed: ${resp.status}`);
    return (await resp.json()) as CatalogPage;
  }

  const items = await fetchLocalCatalog();
  const q = (query.q ?? '').trim().toLowerCase();
  const filtered = items.filter((entry) => {
    if (query.category && query.category !== '全部' && entry.category !== query.category) return false;
    if (!q) return true;
    return [entry.name, entry.description, entry.id].some((field) => field.toLowerCase().includes(q));
  });
  const sort = query.sort ?? 'recent';
  const sorted = [...filtered].sort((a, b) => {
    if (sort === 'rating') {
      return (b.ratingAverage ?? -1) - (a.ratingAverage ?? -1) || (b.ratingCount ?? 0) - (a.ratingCount ?? 0);
    }
    if (sort === 'downloads') return (b.downloadCount ?? 0) - (a.downloadCount ?? 0);
    return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
  });
  return { items: sorted, stale: false, sourceId: 'plugin-market-site', nextCursor: undefined };
}

export async function fetchPluginDetail(id: string): Promise<PluginDetail> {
  if (isMarketServiceConnected) {
    const resp = await fetch(`${API_BASE}/api/v1/plugins/${encodeURIComponent(id)}`);
    if (resp.status === 404) throw new NotFoundError(id);
    if (!resp.ok) throw new Error(`plugin request failed: ${resp.status}`);
    return (await resp.json()) as PluginDetail;
  }
  const items = await fetchLocalCatalog();
  const found = items.find((entry) => entry.id === id);
  if (!found) throw new NotFoundError(id);
  return found;
}

// 设备码登录：客户端展示 userCode，用户在本页输入完成绑定。
export async function verifyDeviceCode(userCode: string): Promise<LoginStatus> {
  if (!isMarketServiceConnected) {
    return Promise.resolve({ status: 'pending_approval' });
  }
  const resp = await fetch(`${API_BASE}/api/v1/auth/device/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ userCode }),
  });
  if (!resp.ok) throw new Error(`device verify failed: ${resp.status}`);
  return (await resp.json()) as LoginStatus;
}

export class NotFoundError extends Error {
  constructor(id: string) {
    super(`plugin not found: ${id}`);
    this.name = 'NotFoundError';
  }
}

export type { CatalogPage, LoginStatus, PluginDetail, PluginItem };
