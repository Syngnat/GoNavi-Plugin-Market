import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import {
  fetchPublisherPlugins,
  fetchPublisherSubmissions,
  marketErrorMessage,
  submissionStateLabel,
} from '../api/v2';
import type { MarketAccount, PublisherPlugin, SubmissionRecord } from '../api/types';

/** 未登录时的统一引导：保留当前路径作为登录回跳目标。 */
export function LoginGate({ title = '请先登录插件市场' }: { title?: string }) {
  const { error } = useAuth();
  const location = useLocation();
  const from = location.pathname + location.search + location.hash;

  return (
    <main className="flex-grow w-full max-w-xl mx-auto px-6 py-16">
      <section className="bg-card border border-hairline rounded-xl p-8 text-center space-y-4">
        <h1 className="text-2xl font-headline font-bold">{title}</h1>
        <p className="text-muted leading-relaxed">
          使用 GitHub 账号在网站登录，无需客户端配对码；浏览和安装公开插件无需登录。
        </p>
        {error && (
          <p role="alert" className="text-copper">
            {error}
          </p>
        )}
        <Link
          to={`/login?returnTo=${encodeURIComponent(from)}`}
          className="inline-flex px-5 py-2.5 rounded-lg bg-charcoal text-card font-medium hover:opacity-95 transition-all"
        >
          前往 GitHub 登录
        </Link>
      </section>
    </main>
  );
}

export function AccountHeader({
  account,
  publishAllowed,
}: {
  account: MarketAccount;
  publishAllowed: boolean;
}) {
  return (
    <header className="flex flex-wrap justify-between items-start gap-4">
      <div>
        <p className="text-sm text-copper">{account.login} 的作者工作台</p>
        <h1 className="text-3xl font-headline font-bold mt-2">我的插件</h1>
        <p className="text-sm text-muted mt-3">
          命名空间：<code className="font-mono">{account.publisherId || '未分配'}</code> · 角色：{account.role}
        </p>
      </div>
      {publishAllowed && (
        <Link
          to="/publish"
          className="px-5 py-2.5 rounded-lg bg-charcoal text-card font-semibold hover:opacity-95 transition-all"
        >
          发布新版本
        </Link>
      )}
    </header>
  );
}

export function SubmissionStatePill({ state }: { state: string }) {
  const tone = state === 'published' ? 'border-copper text-copper' : 'border-hairline text-muted';
  return (
    <span className={`text-xs rounded-full border bg-paper px-2 py-1 ${tone}`}>
      {submissionStateLabel(state)}
    </span>
  );
}

export function Pagination({
  loading,
  canPrev,
  canNext,
  onPrev,
  onNext,
}: {
  loading: boolean;
  canPrev: boolean;
  canNext: boolean;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <div className="flex justify-between text-sm">
      <button
        type="button"
        disabled={loading || !canPrev}
        onClick={onPrev}
        className="px-3 py-1.5 rounded-lg border border-hairline text-muted hover:text-ink disabled:opacity-40 transition-colors"
      >
        上一页
      </button>
      <button
        type="button"
        disabled={loading || !canNext}
        onClick={onNext}
        className="px-3 py-1.5 rounded-lg border border-hairline text-muted hover:text-ink disabled:opacity-40 transition-colors"
      >
        下一页
      </button>
    </div>
  );
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : marketErrorMessage(fallback);
}

/** 当前账号拥有的插件列表（分页）。 */
export function OwnedPluginList({ refreshKey = 0 }: { refreshKey?: number }) {
  const { account } = useAuth();
  const accountId = account?.id ?? '';
  const [items, setItems] = useState<PublisherPlugin[]>([]);
  const [cursor, setCursor] = useState('');
  const [stack, setStack] = useState<string[]>([]);
  const [next, setNext] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const run = useRef(0);

  useEffect(() => {
    if (!accountId) return;
    const id = ++run.current;
    setLoading(true);
    setError('');
    fetchPublisherPlugins(cursor || undefined)
      .then((page) => {
        if (id !== run.current) return;
        setItems(page.items);
        setNext(page.nextCursor ?? '');
      })
      .catch((err: unknown) => {
        if (id !== run.current) return;
        setError(errorText(err, 'unavailable'));
      })
      .finally(() => {
        if (id === run.current) setLoading(false);
      });
    return () => {
      run.current++;
    };
  }, [accountId, cursor, refreshKey, reload]);

  return (
    <section className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="font-headline font-bold text-xl">所属插件</h2>
        <button
          type="button"
          onClick={() => setReload((value) => value + 1)}
          disabled={loading}
          className="text-sm text-copper hover:underline disabled:opacity-40"
        >
          刷新
        </button>
      </div>

      {error && (
        <p role="alert" className="text-sm text-copper">
          {error}
        </p>
      )}

      {loading ? (
        <p role="status" className="text-sm text-muted">
          读取插件…
        </p>
      ) : items.length ? (
        <div className="space-y-3">
          {items.map((plugin) => (
            <article key={plugin.id} className="bg-card border border-hairline rounded-xl p-6 space-y-3">
              <div className="flex flex-wrap justify-between gap-3">
                <div>
                  <h3 className="font-bold text-lg">{plugin.name}</h3>
                  <p className="text-xs font-mono text-muted break-all">{plugin.id}</p>
                </div>
                <span className="text-sm text-muted">
                  {submissionStateLabel(plugin.latestState)} · {plugin.latestVersion || '—'}
                </span>
              </div>
              <p className="text-sm text-muted">市场公开版本：{plugin.publishedVersion || '尚未上架'}</p>
              <div className="flex flex-wrap gap-4 text-sm">
                <Link
                  to={`/account/plugins/${encodeURIComponent(plugin.id)}`}
                  className="text-copper hover:underline"
                >
                  版本与自动更新来源
                </Link>
                {plugin.publishedVersion && (
                  <Link to={`/plugins/${encodeURIComponent(plugin.id)}`} className="text-copper hover:underline">
                    查看市场页面
                  </Link>
                )}
                {plugin.sourceUrl && (
                  <a
                    href={plugin.sourceUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="text-muted hover:text-ink transition-colors"
                  >
                    源仓库 ↗
                  </a>
                )}
              </div>
            </article>
          ))}
        </div>
      ) : (
        <div className="bg-card border border-hairline rounded-xl p-8 text-sm text-muted">
          还没有所属插件，可先在发布页上传包做静态预检；预检不会创建上架条目。
        </div>
      )}

      <Pagination
        loading={loading}
        canPrev={stack.length > 0}
        canNext={!!next}
        onPrev={() => {
          setStack((prev) => prev.slice(0, -1));
          setCursor(stack[stack.length - 2] ?? '');
        }}
        onNext={() => {
          setStack((prev) => [...prev, cursor]);
          setCursor(next);
        }}
      />
    </section>
  );
}

/** 发布记录列表；传 pluginId 时只看该插件，否则是当前账号的全部记录。 */
export function SubmissionList({
  title,
  pluginId,
  refreshKey = 0,
}: {
  title: string;
  pluginId?: string;
  refreshKey?: number;
}) {
  const { account } = useAuth();
  const accountId = account?.id ?? '';
  const [items, setItems] = useState<SubmissionRecord[]>([]);
  const [cursor, setCursor] = useState('');
  const [stack, setStack] = useState<string[]>([]);
  const [next, setNext] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const run = useRef(0);

  useEffect(() => {
    if (!accountId) return;
    const id = ++run.current;
    setLoading(true);
    setError('');
    fetchPublisherSubmissions(cursor || undefined, pluginId)
      .then((page) => {
        if (id !== run.current) return;
        setItems(page.items);
        setNext(page.nextCursor ?? '');
      })
      .catch((err: unknown) => {
        if (id !== run.current) return;
        setError(errorText(err, 'unavailable'));
      })
      .finally(() => {
        if (id === run.current) setLoading(false);
      });
    return () => {
      run.current++;
    };
  }, [accountId, pluginId, cursor, refreshKey, reload]);

  useEffect(() => {
    // 切换插件时重置分页游标，避免沿用上一个插件的下一页地址。
    setCursor('');
    setStack([]);
  }, [pluginId]);

  return (
    <section className="bg-card border border-hairline rounded-xl overflow-hidden">
      <div className="p-5 flex justify-between items-center gap-3">
        <h2 className="text-lg font-headline font-bold">{title}</h2>
        <button
          type="button"
          disabled={loading}
          onClick={() => setReload((value) => value + 1)}
          className="text-sm text-copper hover:underline disabled:opacity-40"
        >
          刷新记录
        </button>
      </div>

      {error && (
        <p role="alert" className="mx-5 mb-4 text-sm text-copper">
          {error}
        </p>
      )}

      {loading ? (
        <p role="status" className="px-5 pb-5 text-sm text-muted">
          读取版本记录…
        </p>
      ) : items.length ? (
        <ul className="divide-y divide-hairline">
          {items.map((item) => (
            <li key={item.id} className="p-5 space-y-2">
              <div className="flex flex-wrap justify-between gap-2">
                <span className="font-mono text-sm break-all">
                  {item.pluginId} · {item.version}
                </span>
                <SubmissionStatePill state={item.state} />
              </div>
              <p className="text-xs text-muted">{new Date(item.updatedAt).toLocaleString()}</p>
              {item.reason && <p className="text-sm break-words">原因：{item.reason}</p>}
              {item.sha256 && (
                <p className="font-mono text-xs text-muted break-all">SHA-256：{item.sha256}</p>
              )}
            </li>
          ))}
        </ul>
      ) : (
        <p className="px-5 pb-5 text-sm text-muted">还没有发布记录。</p>
      )}

      <div className="p-5 border-t border-hairline">
        <Pagination
          loading={loading}
          canPrev={stack.length > 0}
          canNext={!!next}
          onPrev={() => {
            setStack((prev) => prev.slice(0, -1));
            setCursor(stack[stack.length - 2] ?? '');
          }}
          onNext={() => {
            setStack((prev) => [...prev, cursor]);
            setCursor(next);
          }}
        />
      </div>
    </section>
  );
}
