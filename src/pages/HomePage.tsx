import { useEffect, useMemo, useState } from 'react';
import { CategoryPills, type Category } from '../components/CategoryPills';
import { FeaturedCard } from '../components/FeaturedCard';
import { PluginRow } from '../components/PluginRow';
import { fetchCatalog } from '../api/client';
import type { PluginItem } from '../api/types';

export function HomePage() {
  const [category, setCategory] = useState<Category>('全部');
  const [q, setQ] = useState('');
  const [items, setItems] = useState<PluginItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(false);
    fetchCatalog({ category, q })
      .then((page) => {
        if (alive) setItems(page.items);
      })
      .catch(() => {
        if (alive) setError(true);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [category, q]);

  const featured = useMemo(() => items.find((entry) => entry.featured) ?? null, [items]);
  const rows = useMemo(
    () => (featured ? items.filter((entry) => entry.id !== featured.id) : items),
    [items, featured],
  );

  return (
    <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-10 space-y-16">
      <section className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-stretch">
        <div className="lg:col-span-7 bg-card p-8 rounded-xl border border-hairline shadow-sm flex flex-col justify-between gap-6">
          <div className="space-y-4">
            <div className="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-paper border border-hairline text-copper text-xs font-medium">
              <span className="material-symbols-outlined text-sm fill-icon">verified</span>
              <span>官方插件市场</span>
            </div>
            <h1 className="text-4xl font-headline font-bold text-ink tracking-tight">扩展 GoNavi 的工作台</h1>
            <p className="text-muted text-sm leading-relaxed max-w-xl">
              探索经过 SHA-256 安全核对的高性能插件。无缝集成数据源、AI 供应商、自定义面板与深度调试工具，全面提升您的开发效率。
            </p>
          </div>
          <div className="relative">
            <span className="absolute inset-y-0 left-0 flex items-center pl-4 text-muted pointer-events-none">
              <span className="material-symbols-outlined">search</span>
            </span>
            <input
              value={q}
              onChange={(event) => setQ(event.target.value)}
              className="w-full pl-11 pr-16 py-3.5 bg-paper border border-hairline rounded-lg text-ink placeholder-muted text-sm focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper transition-all"
              placeholder="搜索插件、数据源、技能..."
              type="text"
            />
            <div className="absolute inset-y-0 right-2 flex items-center">
              <kbd className="px-2 py-1 text-xs font-mono bg-card border border-hairline rounded text-muted">⌘K</kbd>
            </div>
          </div>
          <CategoryPills active={category} onChange={setCategory} />
        </div>
        {featured && <FeaturedCard item={featured} />}
      </section>

      <section id="catalog" className="space-y-6 scroll-mt-24">
        <div className="flex items-end justify-between gap-4">
          <div>
            <h2 className="text-2xl font-headline font-bold tracking-tight">精选插件列表</h2>
            <p className="text-muted text-sm mt-1">发现来自官方及社区的高效扩展模块</p>
          </div>
          <span className="text-sm text-muted shrink-0">共 {rows.length} 款插件</span>
        </div>
        {error ? (
          <div className="bg-card rounded-xl border border-hairline shadow-sm p-10 text-center text-sm text-muted">
            加载失败，请稍后重试。
          </div>
        ) : loading && rows.length === 0 ? (
          <div className="bg-card rounded-xl border border-hairline shadow-sm p-10 text-center text-sm text-muted">
            加载中…
          </div>
        ) : rows.length === 0 ? (
          <div className="bg-card rounded-xl border border-hairline shadow-sm p-10 text-center space-y-3">
            <p className="text-sm text-ink">首批插件正在整理上架。</p>
            <p className="text-xs text-muted">
              上架要求：插件包通过 GoNavi 的静态检查与内容摘要校验，且提供真实可验证的发布源。
            </p>
            <a
              href="https://github.com/Syngnat/GoNavi/blob/dev/docs/plugins/author-guide.md"
              target="_blank"
              rel="noreferrer"
              className="text-sm text-copper hover:underline inline-flex items-center gap-1"
            >
              想让自己的插件上架？看开发指南 <span className="material-symbols-outlined text-sm">open_in_new</span>
            </a>
          </div>
        ) : (
          <div className="bg-card rounded-xl border border-hairline shadow-sm divide-y divide-hairline">
            {rows.map((item) => (
              <PluginRow key={item.id} item={item} icon={item.icon ?? 'extension'} />
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
