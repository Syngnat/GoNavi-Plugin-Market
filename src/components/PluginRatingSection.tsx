import { useCallback, useEffect, useId, useMemo, useState, type FormEvent } from 'react';
import { useAuth } from '../auth/AuthContext';
import {
  deletePluginRating,
  fetchPluginRatings,
  marketErrorMessage,
  MarketRequestError,
  savePluginRating,
  webLoginUrl,
} from '../api/v2';
import type { PluginRating } from '../api/types';
import { relativeTime } from '../lib/format';

const PAGE_SIZE = 10;
const STAR_VALUES = [1, 2, 3, 4, 5] as const;

export interface PluginRatingSectionProps {
  pluginId: string;
  version: string;
  publisherId: string;
  ratingAverage?: number;
  ratingCount?: number;
  onChanged: () => void | Promise<void>;
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length;
}

function ratingErrorMessage(error: unknown): string {
  if (error instanceof MarketRequestError) {
    if (error.code === 'forbidden') {
      return '作者不能给自己的插件打分，或当前账号没有评分权限。';
    }
    if (error.code === 'authentication_required' || error.code === 'unauthorized') {
      return '请先登录插件市场再评分。';
    }
    if (error.code === 'invalid_request') {
      return '评分内容不合法：请选择 1–5 星，评价不超过 4000 字节。';
    }
    return marketErrorMessage(error.code);
  }
  return error instanceof Error && error.message
    ? error.message
    : '评分请求未完成，请稍后重试。';
}

export function PluginRatingSection({
  pluginId,
  version,
  publisherId,
  ratingAverage,
  ratingCount,
  onChanged,
}: PluginRatingSectionProps) {
  const { loading: authLoading, account, csrf } = useAuth();
  const [ratings, setRatings] = useState<PluginRating[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [stars, setStars] = useState(5);
  const [body, setBody] = useState('');
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState('');
  const [writeError, setWriteError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [page, setPage] = useState(1);
  const starsLabelId = useId();
  const bodyId = useId();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRatings(await fetchPluginRatings(pluginId));
    } catch (error) {
      setLoadError(ratingErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [pluginId]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setStars(5);
    setBody('');
    setFeedback('');
    setWriteError('');
    setConfirmDelete(false);
    setPage(1);
  }, [pluginId, account?.id]);

  const mine = useMemo(
    () => (account ? ratings.find((rating) => rating.userId === account.id) : undefined),
    [account, ratings],
  );

  useEffect(() => {
    if (!mine) return;
    setStars(mine.stars);
    setBody(mine.body);
  }, [mine?.userId, mine?.updatedAt, mine?.stars, mine?.body]);

  const totalPages = Math.max(1, Math.ceil(ratings.length / PAGE_SIZE));
  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  const visibleRatings = ratings.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
  const distribution = useMemo(
    () =>
      [...STAR_VALUES].reverse().map((star) => {
        const count = ratings.filter((rating) => rating.stars === star).length;
        return { star, count, percent: ratings.length ? Math.round((count * 100) / ratings.length) : 0 };
      }),
    [ratings],
  );

  const isOwner = Boolean(account?.publisherId && account.publisherId === publisherId);
  const canRate = Boolean(account && csrf && !isOwner);
  const average = ratingAverage ?? 0;
  const count = ratingCount ?? ratings.length;
  const bodyBytes = byteLength(body);
  const loginHref = webLoginUrl(
    typeof window === 'undefined' ? '/' : `${window.location.pathname}${window.location.search}`,
  );

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canRate || !csrf) {
      setWriteError('请先登录插件市场再评分。');
      return;
    }
    if (stars < 1 || stars > 5) {
      setWriteError('请选择 1–5 星。');
      return;
    }
    if (bodyBytes > 4000) {
      setWriteError('评价不能超过 4000 字节。');
      return;
    }
    setBusy(true);
    setWriteError('');
    setFeedback('');
    setConfirmDelete(false);
    try {
      await savePluginRating(pluginId, { stars, body: body.trim(), version }, csrf);
      setFeedback(mine ? '评价已更新。' : '评价已发布。');
      await load();
      await onChanged();
    } catch (error) {
      setWriteError(ratingErrorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!account || !csrf) {
      setWriteError('请先登录插件市场再操作。');
      return;
    }
    if (!confirmDelete) {
      setConfirmDelete(true);
      return;
    }
    setBusy(true);
    setWriteError('');
    setFeedback('');
    try {
      await deletePluginRating(pluginId, csrf);
      setFeedback('评价已删除。');
      setStars(5);
      setBody('');
      setConfirmDelete(false);
      await load();
      await onChanged();
    } catch (error) {
      setWriteError(ratingErrorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section
      className="bg-card rounded-xl border border-hairline shadow-sm p-8"
      aria-labelledby="plugin-ratings-title"
    >
      <div className="flex items-end justify-between gap-4 mb-5">
        <div>
          <h2 id="plugin-ratings-title" className="font-headline font-bold text-lg">
            评价
          </h2>
          <p className="text-xs text-muted mt-1">登录后可打分，作者不能给自己的插件打分。</p>
        </div>
        <div className="text-right shrink-0">
          <div className="text-2xl font-headline font-bold leading-none">
            {count > 0 ? average.toFixed(1) : '—'}
          </div>
          <div className="text-xs text-muted mt-1">{count} 条评价</div>
        </div>
      </div>

      {count > 0 && ratings.length > 0 && (
        <div className="space-y-2 mb-6" aria-label="评分分布">
          {distribution.map(({ star, count: starCount, percent }) => (
            <div key={star} className="flex items-center gap-3 text-xs text-muted">
              <span className="w-4 text-right">{star}</span>
              <div className="flex-1 h-1.5 rounded-full bg-paper overflow-hidden">
                <div className="h-full bg-copper" style={{ width: `${percent}%` }} />
              </div>
              <span className="w-8 text-right">{starCount}</span>
            </div>
          ))}
        </div>
      )}

      {authLoading ? (
        <div className="rounded-lg border border-dashed border-hairline bg-paper/60 p-4 text-sm text-muted">
          正在检查登录状态…
        </div>
      ) : !account ? (
        <div className="rounded-lg border border-dashed border-hairline bg-paper/60 p-4 flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted">登录后可以给这个插件打分。</p>
          <a
            href={loginHref}
            className="inline-flex items-center gap-1.5 rounded-lg bg-copper px-3 py-2 text-sm font-medium text-white hover:opacity-90"
          >
            <span className="material-symbols-outlined text-base">login</span>
            GitHub 登录
          </a>
        </div>
      ) : isOwner ? (
        <div className="rounded-lg border border-dashed border-hairline bg-paper/60 p-4 text-sm text-muted">
          你是这个插件的发布者，不能给自己的插件打分。
        </div>
      ) : (
        <form onSubmit={submit} className="rounded-lg border border-hairline bg-paper/60 p-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span id={starsLabelId} className="text-sm font-medium">
              你的评分
            </span>
            <div role="group" aria-labelledby={starsLabelId} className="flex items-center gap-1">
              {STAR_VALUES.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-label={`${value} 星`}
                  aria-pressed={stars === value}
                  disabled={busy}
                  onClick={() => setStars(value)}
                  className={`text-2xl leading-none transition-colors ${
                    value <= stars ? 'text-copper' : 'text-hairline hover:text-copper/60'
                  } disabled:opacity-60`}
                >
                  ★
                </button>
              ))}
              <span className="ml-1 text-xs text-muted">{stars} 星</span>
            </div>
          </div>

          <label htmlFor={bodyId} className="mt-4 block text-sm font-medium">
            评价（可选）
          </label>
          <textarea
            id={bodyId}
            value={body}
            onChange={(event) => setBody(event.target.value)}
            rows={3}
            maxLength={4000}
            disabled={busy}
            placeholder="说说你的使用体验、适用场景或遇到的问题…"
            className="mt-2 w-full resize-y rounded-lg border border-hairline bg-card px-3 py-2 text-sm leading-relaxed outline-none focus:border-copper disabled:opacity-60"
          />
          <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
            <span className={`text-xs ${bodyBytes > 4000 ? 'text-red-600' : 'text-muted'}`}>
              {bodyBytes}/4000 字节
            </span>
            <div className="flex flex-wrap items-center gap-2">
              {mine &&
                (confirmDelete ? (
                  <>
                    <button
                      type="button"
                      onClick={remove}
                      disabled={busy}
                      className="rounded-lg border border-red-300 px-3 py-2 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-60"
                    >
                      确认删除
                    </button>
                    <button
                      type="button"
                      onClick={() => setConfirmDelete(false)}
                      disabled={busy}
                      className="rounded-lg border border-hairline px-3 py-2 text-sm text-muted hover:text-ink disabled:opacity-60"
                    >
                      取消
                    </button>
                  </>
                ) : (
                  <button
                    type="button"
                    onClick={remove}
                    disabled={busy}
                    className="rounded-lg border border-hairline px-3 py-2 text-sm text-muted hover:border-red-300 hover:text-red-700 disabled:opacity-60"
                  >
                    删除评价
                  </button>
                ))}
              <button
                type="submit"
                disabled={busy || bodyBytes > 4000 || stars < 1}
                className="rounded-lg bg-copper px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy ? '提交中…' : mine ? '更新评价' : '发布评价'}
              </button>
            </div>
          </div>
          {writeError && (
            <p role="alert" className="mt-3 text-sm text-red-700">
              {writeError}
            </p>
          )}
          {feedback && (
            <p role="status" className="mt-3 text-sm text-green-700">
              {feedback}
            </p>
          )}
        </form>
      )}

      <div className="mt-7 border-t border-hairline pt-5">
        {loading ? (
          <p className="text-sm text-muted">正在加载评价…</p>
        ) : loadError ? (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
            <span>{loadError}</span>
            <button type="button" onClick={() => void load()} className="underline">
              重试
            </button>
          </div>
        ) : ratings.length === 0 ? (
          <p className="text-sm text-muted">还没有评价，成为第一个打分的人。</p>
        ) : (
          <>
            <div className="space-y-4">
              {visibleRatings.map((rating) => (
                <article
                  key={`${rating.userId}-${rating.updatedAt}`}
                  className="border-b border-hairline pb-4 last:border-0 last:pb-0"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                    <span className="font-medium break-all">
                      {account?.id === rating.userId ? '你' : `用户 ${rating.userId}`}
                    </span>
                    <span className="text-copper text-xs tracking-wide" aria-label={`${rating.stars} 星`}>
                      {'★'.repeat(rating.stars)}
                      <span className="text-muted/30">{'☆'.repeat(5 - rating.stars)}</span>
                    </span>
                  </div>
                  <div className="mt-1 text-xs text-muted">
                    {rating.version || '未标注版本'} · {relativeTime(rating.updatedAt)}
                  </div>
                  {rating.body && (
                    <p className="mt-2 text-sm text-muted whitespace-pre-wrap break-words">{rating.body}</p>
                  )}
                </article>
              ))}
            </div>
            <div className="mt-5 flex items-center justify-between gap-3 text-xs text-muted">
              <span>
                共 {ratings.length} 条可见评价
                {count > ratings.length ? `，显示最近 ${ratings.length} 条` : ''}
              </span>
              {totalPages > 1 && (
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setPage((current) => Math.max(1, current - 1))}
                    disabled={page <= 1}
                    className="rounded border border-hairline px-2 py-1 disabled:opacity-40"
                  >
                    上一页
                  </button>
                  <span>
                    {page}/{totalPages}
                  </span>
                  <button
                    type="button"
                    onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                    disabled={page >= totalPages}
                    className="rounded border border-hairline px-2 py-1 disabled:opacity-40"
                  >
                    下一页
                  </button>
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </section>
  );
}
