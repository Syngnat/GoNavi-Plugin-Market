import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { InstallButton } from '../components/InstallButton';
import { fetchPluginDetail, NotFoundError, seedIcons } from '../api/client';
import type { PluginDetail } from '../api/types';
import { formatBytes, formatCount, relativeTime } from '../lib/format';

const PLATFORM_LABELS: Record<string, string> = {
  windows: 'Windows',
  macos: 'macOS',
  linux: 'Linux',
};

const FEATURE_ICONS = ['account_tree', 'speed', 'monitoring'];

export function PluginDetailPage() {
  const { id = '' } = useParams();
  const [detail, setDetail] = useState<PluginDetail | null>(null);
  const [state, setState] = useState<'loading' | 'ready' | 'notfound' | 'error'>('loading');

  useEffect(() => {
    let alive = true;
    setState('loading');
    fetchPluginDetail(id)
      .then((value) => {
        if (alive) {
          setDetail(value);
          setState('ready');
        }
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setState(err instanceof NotFoundError ? 'notfound' : 'error');
      });
    return () => {
      alive = false;
    };
  }, [id]);

  if (state === 'loading') {
    return (
      <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-16">
        <div className="bg-card rounded-xl border border-hairline shadow-sm p-10 text-center text-sm text-muted">
          加载中…
        </div>
      </main>
    );
  }

  if (state !== 'ready' || !detail) {
    return (
      <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-16">
        <div className="bg-card rounded-xl border border-hairline shadow-sm p-10 text-center space-y-3">
          <p className="text-sm text-muted">
            {state === 'notfound' ? '没有找到这个插件，可能已下架。' : '加载失败，请稍后重试。'}
          </p>
          <Link to="/" className="text-sm text-copper hover:underline">
            返回插件目录
          </Link>
        </div>
      </main>
    );
  }

  const icon = seedIcons[detail.id] ?? 'extension';

  return (
    <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-8">
      <div className="text-sm text-muted mb-6 flex items-center gap-1">
        <Link to="/" className="hover:text-copper transition-colors">
          首页
        </Link>
        <span className="material-symbols-outlined text-base">chevron_right</span>
        <Link to="/#catalog" className="hover:text-copper transition-colors">
          插件分类
        </Link>
        <span className="material-symbols-outlined text-base">chevron_right</span>
        <span className="text-ink">{detail.name}</span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        <div className="lg:col-span-7 space-y-6">
          <section className="bg-card rounded-xl border border-hairline shadow-sm p-8">
            <div className="flex items-start gap-4">
              <div className="w-14 h-14 rounded-lg bg-paper border border-hairline flex items-center justify-center text-copper shrink-0">
                <span className="material-symbols-outlined text-3xl fill-icon">{icon}</span>
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2 mb-1">
                  {detail.official && (
                    <span className="px-2.5 py-0.5 rounded text-xs font-medium bg-paper border border-hairline text-copper inline-flex items-center gap-1">
                      <span className="material-symbols-outlined text-sm fill-icon">verified</span>官方
                    </span>
                  )}
                  <span className="text-xs text-muted font-mono">{detail.version}</span>
                </div>
                <p className="text-xs text-muted font-mono mb-2">{detail.id}</p>
                <h1 className="text-3xl font-headline font-bold tracking-tight">{detail.name}</h1>
              </div>
            </div>
            <p className="text-muted text-sm leading-relaxed mt-5">{detail.description}</p>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm mt-5">
              <span className="inline-flex items-center gap-1 text-copper font-medium">
                <span className="material-symbols-outlined text-base fill-icon">star</span>
                {detail.ratingAverage.toFixed(1)} <span className="text-muted font-normal">({detail.ratingCount})</span>
              </span>
              <span className="text-muted">·</span>
              <span className="text-muted">
                下载量 <span className="text-ink font-medium">{formatCount(detail.downloadCount)}</span>
              </span>
              <span className="text-muted">·</span>
              <span className="text-muted">
                更新于 <span className="text-ink font-medium">{relativeTime(detail.updatedAt)}</span>
              </span>
              <span className="text-muted">·</span>
              <span className="text-muted">
                作者 <span className="text-ink font-medium">{detail.author}</span>
              </span>
            </div>
          </section>

          {detail.features && detail.features.length > 0 && (
            <section className="bg-card rounded-xl border border-hairline shadow-sm p-8">
              <h2 className="font-headline font-bold text-lg mb-4">它会做什么</h2>
              <ul className="space-y-3 text-sm text-ink/90">
                {detail.features.map((feature, index) => (
                  <li key={feature} className="flex gap-3">
                    <span className="material-symbols-outlined text-copper text-xl">
                      {FEATURE_ICONS[index % FEATURE_ICONS.length]}
                    </span>
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          <section className="bg-card rounded-xl border border-hairline shadow-sm overflow-hidden">
            <div className="px-8 py-5 border-b border-hairline flex items-center justify-between">
              <h2 className="font-headline font-bold text-lg">权限清单</h2>
              <span className="text-xs text-muted">安装前可逐项核对</span>
            </div>
            <div className="divide-y divide-hairline text-sm">
              {(detail.permissions ?? []).map((permission) => (
                <div key={permission.key || permission.name} className="px-8 py-4 flex items-center justify-between gap-4">
                  <div>
                    <div className="font-medium">{permission.name}</div>
                    {permission.key && (
                      <div className="text-xs text-muted font-mono mt-0.5">{permission.key}</div>
                    )}
                  </div>
                  <span
                    className={
                      permission.granted
                        ? 'text-xs px-2 py-1 rounded-full bg-paper border border-hairline text-muted'
                        : 'text-xs px-2 py-1 rounded-full bg-paper border border-hairline text-copper'
                    }
                  >
                    {permission.scope}
                  </span>
                </div>
              ))}
              {(!detail.permissions || detail.permissions.length === 0) && (
                <div className="px-8 py-4 text-muted">此插件未声明额外权限。</div>
              )}
            </div>
          </section>

          {detail.reviews && detail.reviews.length > 0 && (
            <section className="bg-card rounded-xl border border-hairline shadow-sm p-8">
              <div className="flex items-end justify-between mb-5">
                <h2 className="font-headline font-bold text-lg">评价</h2>
                <div className="text-right">
                  <div className="text-2xl font-headline font-bold leading-none">
                    {detail.ratingAverage.toFixed(1)}
                  </div>
                  <div className="text-xs text-muted mt-1">{detail.ratingCount} 条评价</div>
                </div>
              </div>
              {detail.ratingDistribution && detail.ratingDistribution.length === 5 && (
                <div className="space-y-2 mb-6">
                  {detail.ratingDistribution.map((percent, index) => (
                    <div key={index} className="flex items-center gap-3 text-xs text-muted">
                      <span className="w-4 text-right">{5 - index}</span>
                      <div className="flex-1 h-1.5 rounded-full bg-paper overflow-hidden">
                        <div className="h-full bg-copper" style={{ width: `${percent}%` }} />
                      </div>
                    </div>
                  ))}
                </div>
              )}
              <div className="space-y-4 border-t border-hairline pt-5">
                {detail.reviews.map((review) => (
                  <div key={review.author}>
                    <div className="flex items-center justify-between text-sm">
                      <span className="font-medium">{review.author}</span>
                      <span className="text-copper text-xs tracking-wide">{'★'.repeat(review.stars)}</span>
                    </div>
                    <p className="text-sm text-muted mt-1">{review.body}</p>
                  </div>
                ))}
              </div>
            </section>
          )}
        </div>

        <aside className="lg:col-span-5 lg:sticky lg:top-24 space-y-6">
          <section className="bg-card rounded-xl border border-hairline shadow-sm p-8 relative overflow-hidden">
            <div className="absolute top-0 right-0 w-32 h-32 bg-copper/5 rounded-bl-full pointer-events-none" />
            <div className="flex items-center justify-between text-sm mb-5">
              <span className="text-muted">当前版本</span>
              <span className="font-mono font-medium">{detail.version}</span>
            </div>
            <InstallButton item={detail} variant="block" />
            <dl className="mt-6 space-y-3 text-sm border-t border-hairline pt-5">
              <div className="flex justify-between gap-4">
                <dt className="text-muted">最低宿主版本</dt>
                <dd className="font-mono">{detail.minHostVersion ?? '—'}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted">平台</dt>
                <dd>{detail.platforms.map((platform) => PLATFORM_LABELS[platform] ?? platform).join(' · ')}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted">包体积</dt>
                <dd className="font-mono">{formatBytes(detail.artifact?.sizeBytes ?? 0)}</dd>
              </div>
              <div className="flex justify-between gap-4">
                <dt className="text-muted">协议</dt>
                <dd>{detail.license ?? '—'}</dd>
              </div>
            </dl>
          </section>

          {detail.artifact?.sha256 && (
            <section className="bg-card rounded-xl border border-hairline shadow-sm p-6">
              <h2 className="font-headline font-bold text-sm mb-3">安装前核对</h2>
              <p className="text-[11px] uppercase tracking-wider text-muted mb-1">SHA-256</p>
              <p className="font-mono text-xs leading-relaxed break-all text-ink">{detail.artifact.sha256}</p>
              <p className="text-xs text-muted mt-3">摘要与市场上架记录不一致时，GoNavi 会拒绝安装。</p>
            </section>
          )}
        </aside>
      </div>
    </main>
  );
}
