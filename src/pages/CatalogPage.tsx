import { useEffect, useMemo, useRef, useState } from 'react';
import { fetchCatalog } from '../api/client';
import type { PluginItem } from '../api/types';
import { PluginRow } from '../components/PluginRow';

const CATEGORIES = ['全部', '数据源', 'AI 供应商', '导出器', '技能', '面板', '命令', '运维', '其他'] as const;
type Category = (typeof CATEGORIES)[number];
type SectionCategory = Exclude<Category, '全部'>;

const SERVER_CATEGORY_LABELS: Record<string, SectionCategory> = {
  datasource: '数据源',
  'ai-provider': 'AI 供应商',
  exporter: '导出器',
  skill: '技能',
  panel: '面板',
  command: '命令',
  ops: '运维',
  other: '其他',
  数据源: '数据源',
  'AI 供应商': 'AI 供应商',
  导出器: '导出器',
  技能: '技能',
  面板: '面板',
  命令: '命令',
  运维: '运维',
  其他: '其他',
};

const SECTION_CATEGORIES = CATEGORIES.slice(1) as SectionCategory[];

function categoryOf(item: PluginItem): SectionCategory {
  return SERVER_CATEGORY_LABELS[(item.category ?? '').trim()] ?? '其他';
}

export function CatalogPage() {
  const [items, setItems] = useState<PluginItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [category, setCategory] = useState<Category>('全部');
  const [query, setQuery] = useState('');
  const loadId = useRef(0);

  useEffect(() => {
    let alive = true;
    const current = ++loadId.current;
    setLoading(true);
    setError(false);

    const loadAll = async () => {
      const seen = new Set<string>();
      const allItems: PluginItem[] = [];
      let cursor: string | undefined;
      for (let pageCount = 0; pageCount < 40; pageCount += 1) {
        const page = await fetchCatalog({ cursor });
        if (!alive || current !== loadId.current) return;
        for (const item of page.items) {
          if (seen.has(item.id)) continue;
          seen.add(item.id);
          allItems.push(item);
        }
        if (page.stale || !page.nextCursor) break;
        cursor = page.nextCursor;
      }
      if (!alive || current !== loadId.current) return;
      setItems(allItems);
    };

    loadAll()
      .catch(() => {
        if (!alive || current !== loadId.current) return;
        setItems([]);
        setError(true);
      })
      .finally(() => {
        if (alive && current === loadId.current) setLoading(false);
      });

    return () => {
      alive = false;
    };
  }, []);

  const counts = useMemo(() => {
    const result = new Map<Category, number>([['全部', items.length]]);
    for (const section of SECTION_CATEGORIES) result.set(section, 0);
    for (const item of items) {
      const key = categoryOf(item);
      result.set(key, (result.get(key) ?? 0) + 1);
    }
    return result;
  }, [items]);

  const sections = useMemo(() => {
    const q = query.trim().toLowerCase();
    const groups = new Map<SectionCategory, PluginItem[]>();
    for (const section of SECTION_CATEGORIES) groups.set(section, []);
    for (const item of items) {
      if (category !== '全部' && categoryOf(item) !== category) continue;
      if (q && ![item.name, item.description, item.id].some((field) => field.toLowerCase().includes(q))) continue;
      groups.get(categoryOf(item))?.push(item);
    }
    return SECTION_CATEGORIES
      .map((section) => ({ category: section, items: groups.get(section) ?? [] }))
      .filter((section) => section.items.length > 0);
  }, [category, items, query]);

  const visibleCount = sections.reduce((total, section) => total + section.items.length, 0);

  return (
    <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-10 space-y-8">
      <section className="bg-card border border-hairline rounded-xl p-8 space-y-5 shadow-sm">
        <div className="space-y-2">
          <p className="text-xs font-mono tracking-[0.2em] text-copper">CATALOG / 插件分类</p>
          <h1 className="text-3xl md:text-4xl font-headline font-bold tracking-tight">按分类浏览插件</h1>
          <p className="text-muted text-sm leading-relaxed max-w-2xl">
            官方与社区插件按能力分区展示：数据源、AI 供应商、导出器、技能、面板、命令与运维。
          </p>
        </div>

        <div className="flex flex-col lg:flex-row lg:items-center gap-3">
          <label className="relative flex-1">
            <span className="absolute inset-y-0 left-3 flex items-center text-muted pointer-events-none">
              <span className="material-symbols-outlined text-lg">search</span>
            </span>
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="w-full pl-10 pr-4 py-2.5 bg-paper border border-hairline rounded-lg text-sm text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper"
              placeholder="搜索插件名称、ID 或描述…"
              aria-label="搜索插件"
            />
          </label>
          <span className="text-xs text-muted font-mono shrink-0">
            当前 {visibleCount} / {items.length} 款
          </span>
        </div>

        <div className="flex flex-wrap gap-2">
          {CATEGORIES.map((entry) => {
            const selected = entry === category;
            const count = counts.get(entry) ?? 0;
            return (
              <button
                key={entry}
                type="button"
                onClick={() => setCategory(entry)}
                className={
                  selected
                    ? 'px-4 py-1.5 rounded-full text-xs font-medium bg-copper text-white shadow-sm transition-all'
                    : 'px-4 py-1.5 rounded-full text-xs font-medium bg-paper border border-hairline text-muted hover:text-ink hover:border-copper transition-all'
                }
              >
                {entry} <span className="ml-1 font-mono opacity-70">{count}</span>
              </button>
            );
          })}
        </div>
      </section>

      {loading ? (
        <p role="status" className="bg-card border border-hairline rounded-xl p-10 text-center text-sm text-muted">
          正在读取插件目录…
        </p>
      ) : error ? (
        <p role="alert" className="bg-card border border-hairline rounded-xl p-10 text-center text-sm text-muted">
          目录暂不可用，请稍后重试。
        </p>
      ) : sections.length === 0 ? (
        <p className="bg-card border border-hairline rounded-xl p-10 text-center text-sm text-muted">
          当前筛选没有匹配的插件。
        </p>
      ) : (
        sections.map((section) => (
          <section key={section.category} className="space-y-3" aria-labelledby={`catalog-${section.category}`}>
            <header className="flex items-end justify-between gap-4">
              <div>
                <h2 id={`catalog-${section.category}`} className="text-xl font-headline font-bold tracking-tight">
                  {section.category}
                </h2>
                <p className="text-xs text-muted mt-1">{section.items.length} 款插件</p>
              </div>
              <span className="material-symbols-outlined text-copper/70">north_east</span>
            </header>
            <div className="bg-card border border-hairline rounded-xl shadow-sm divide-y divide-hairline overflow-hidden">
              {section.items.map((item) => (
                <PluginRow key={item.id} item={item} icon={item.icon ?? 'extension'} />
              ))}
            </div>
          </section>
        ))
      )}
    </main>
  );
}
