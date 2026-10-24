import type { CatalogPage, LoginStatus, PluginDetail, PluginItem } from './types';
import { FEATURED_ID, seedCatalog, seedIcons, seedPluginDetail } from '../data/seed';

// VITE_API_BASE 未配置时进入演示模式：全部数据来自种子目录，不发网络请求。
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/+$/, '');
export const isDemoMode = API_BASE === '';

export type CatalogSort = 'recent' | 'rating' | 'downloads';

export interface CatalogQuery {
  category?: string;
  sort?: CatalogSort;
  q?: string;
  cursor?: string;
}

export async function fetchCatalog(query: CatalogQuery): Promise<CatalogPage> {
  if (isDemoMode) {
    return Promise.resolve(demoCatalogPage(query));
  }
  const params = new URLSearchParams();
  if (query.category && query.category !== '全部') params.set('category', query.category);
  if (query.q) params.set('q', query.q);
  if (query.cursor) params.set('cursor', query.cursor);
  params.set('sort', query.sort ?? 'recent');
  const resp = await fetch(`${API_BASE}/api/v1/plugins?${params.toString()}`);
  if (!resp.ok) throw new Error(`catalog request failed: ${resp.status}`);
  return (await resp.json()) as CatalogPage;
}

export async function fetchPluginDetail(id: string): Promise<PluginDetail> {
  if (isDemoMode) {
    const detail = seedPluginDetail(id);
    if (!detail) throw new NotFoundError(id);
    return Promise.resolve(detail);
  }
  const resp = await fetch(`${API_BASE}/api/v1/plugins/${encodeURIComponent(id)}`);
  if (resp.status === 404) throw new NotFoundError(id);
  if (!resp.ok) throw new Error(`plugin request failed: ${resp.status}`);
  return (await resp.json()) as PluginDetail;
}

// 设备码登录：客户端展示 userCode，用户在本页输入完成绑定。
export async function verifyDeviceCode(userCode: string): Promise<LoginStatus> {
  if (isDemoMode) {
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

function demoCatalogPage(query: CatalogQuery): CatalogPage {
  const q = (query.q ?? '').trim().toLowerCase();
  let items = seedCatalog.filter((entry) => {
    if (query.category && query.category !== '全部' && entry.category !== query.category) return false;
    if (!q) return true;
    return [entry.name, entry.description, entry.id].some((field) => field.toLowerCase().includes(q));
  });
  const sort = query.sort ?? 'recent';
  items = [...items].sort((a, b) => {
    if (sort === 'rating') return b.ratingAverage - a.ratingAverage || b.ratingCount - a.ratingCount;
    if (sort === 'downloads') return b.downloadCount - a.downloadCount;
    return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
  });
  return { items, stale: false, sourceId: 'seed', nextCursor: undefined };
}

export { FEATURED_ID, seedCatalog, seedIcons, seedPluginDetail };
export type { CatalogPage, LoginStatus, PluginDetail, PluginItem };
