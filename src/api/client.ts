import type { CatalogPage, PluginDetail, PluginItem } from './types';

// 站点与市场服务同源；默认直接使用 /api/v1，只有服务不可用时才回退到随站点发布的
// catalog.json。VITE_API_BASE 仍可指向另一个市场来源（开发/预览）。
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/+$/, '');

export type CatalogSort = 'recent' | 'rating' | 'downloads';

export interface CatalogQuery {
  category?: string;
  sort?: CatalogSort;
  q?: string;
  cursor?: string;
}

// 站点分类是中文展示名，服务端 /api/v1 使用稳定的英文 category。
const SERVER_CATEGORY: Record<string, string> = {
  数据源: 'datasource',
  'AI 供应商': 'ai-provider',
  导出器: 'exporter',
  技能: 'skill',
  面板: 'panel',
  命令: 'command',
  运维: 'ops',
  其他: 'other',
};

function remoteCatalogParams(query: CatalogQuery): URLSearchParams {
  const params = new URLSearchParams();
  if (query.category && query.category !== '全部') {
    params.set('category', SERVER_CATEGORY[query.category] ?? query.category);
  }
  if (query.q) params.set('q', query.q);
  if (query.cursor) params.set('cursor', query.cursor);
  // 服务端只接受 updated/rating/downloads；站点的 "recent" 对应 updated。
  params.set('sort', query.sort === 'rating' || query.sort === 'downloads' ? query.sort : 'updated');
  params.set('limit', '48');
  return params;
}

async function fetchRemoteCatalog(query: CatalogQuery): Promise<CatalogPage> {
  const resp = await fetch(`${API_BASE}/api/v1/plugins?${remoteCatalogParams(query).toString()}`);
  if (!resp.ok) throw new Error(`catalog request failed: ${resp.status}`);
  const page = (await resp.json()) as CatalogPage;
  return { ...page, stale: false, sourceId: page.sourceId || 'plugin-market-service' };
}

async function fetchLocalCatalog(): Promise<PluginItem[]> {
  const resp = await fetch('catalog.json');
  if (!resp.ok) throw new Error(`catalog.json request failed: ${resp.status}`);
  const page = (await resp.json()) as CatalogPage;
  return page.items;
}

function localCatalogPage(query: CatalogQuery, items: PluginItem[]): CatalogPage {
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

export async function fetchCatalog(query: CatalogQuery): Promise<CatalogPage> {
  try {
    return await fetchRemoteCatalog(query);
  } catch (remoteError) {
    // 服务不可用/接口异常时保留静态目录兜底，避免首页直接空白。
    try {
      const page = localCatalogPage(query, await fetchLocalCatalog());
      return { ...page, stale: true, error: '目录服务暂不可用，显示本地缓存' };
    } catch {
      throw remoteError;
    }
  }
}

export async function fetchPluginDetail(id: string): Promise<PluginDetail> {
  try {
    const resp = await fetch(`${API_BASE}/api/v1/plugins/${encodeURIComponent(id)}`);
    if (resp.status === 404) throw new NotFoundError(id);
    if (!resp.ok) throw new Error(`plugin request failed: ${resp.status}`);
    return (await resp.json()) as PluginDetail;
  } catch (remoteError) {
    if (remoteError instanceof NotFoundError) throw remoteError;
    const found = (await fetchLocalCatalog()).find((entry) => entry.id === id);
    if (!found) throw new NotFoundError(id);
    return found;
  }
}

export class NotFoundError extends Error {
  constructor(id: string) {
    super(`plugin not found: ${id}`);
    this.name = 'NotFoundError';
  }
}

export type { CatalogPage, PluginDetail, PluginItem };
