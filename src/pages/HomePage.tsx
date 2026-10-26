import { useCallback, useEffect, useRef, useState } from 'react';
import { fetchCatalog } from '../api/client';
import type { PluginItem } from '../api/types';
import { PluginGalaxy, type GalaxyCategory } from '../components/PluginGalaxy';

export function HomePage() {
  const [category, setCategory] = useState<GalaxyCategory>('全部');
  const [query, setQuery] = useState('');
  const [items, setItems] = useState<PluginItem[]>([]);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState(false);
  const [stale, setStale] = useState<string | undefined>();
  const loadId = useRef(0);

  useEffect(() => {
    let alive = true;
    const current = ++loadId.current;
    setLoading(true);
    setError(false);
    setStale(undefined);
    const seen = new Set<string>();
    const allItems: PluginItem[] = [];
    let cursor: string | undefined;
    let nextPageCursor: string | undefined;
    let pageCount = 0;

    const loadAll = async () => {
      while (pageCount < 40) {
        const page = await fetchCatalog({ cursor });
        if (!alive || current !== loadId.current) return;
        for (const item of page.items) {
          if (seen.has(item.id)) continue;
          seen.add(item.id);
          allItems.push(item);
        }
        if (page.stale) {
          setStale(page.error || '目录服务暂不可用，显示本地缓存');
          break;
        }
        if (!page.nextCursor) {
          nextPageCursor = undefined;
          break;
        }
        nextPageCursor = page.nextCursor;
        cursor = page.nextCursor;
        pageCount += 1;
      }
      if (!alive || current !== loadId.current) return;
      setItems(allItems);
      setNextCursor(nextPageCursor);
    };

    loadAll()
      .catch(() => {
        if (!alive || current !== loadId.current) return;
        setItems([]);
        setNextCursor(undefined);
        setError(true);
      })
      .finally(() => {
        if (alive && current === loadId.current) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const loadMore = useCallback(() => {
    if (!nextCursor || loadingMore) return;
    const current = loadId.current;
    setLoadingMore(true);
    fetchCatalog({ cursor: nextCursor })
      .then((page) => {
        if (current !== loadId.current) return;
        setItems((previous) => {
          const seen = new Set(previous.map((item) => item.id));
          return [...previous, ...page.items.filter((item) => !seen.has(item.id))];
        });
        setNextCursor(page.nextCursor);
        if (page.stale) setStale(page.error || '目录服务暂不可用，显示本地缓存');
      })
      .catch(() => {
        if (current === loadId.current) setStale('加载更多插件失败，请稍后重试');
      })
      .finally(() => {
        if (current === loadId.current) setLoadingMore(false);
      });
  }, [loadingMore, nextCursor]);

  return (
    <main className="flex-grow w-full">
      <PluginGalaxy
        items={items}
        loading={loading}
        error={error}
        nextCursor={nextCursor}
        loadingMore={loadingMore}
        stale={stale}
        activeCategory={category}
        query={query}
        onCategoryChange={setCategory}
        onQueryChange={setQuery}
        onLoadMore={loadMore}
      />
    </main>
  );
}
