import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { canPublishWith, useAuth } from '../auth/AuthContext';
import {
  deletePublisherPlugin,
  fetchPublisherPlugins,
  isGithubRepoUrl,
  marketErrorMessage,
  submissionStateLabel,
  updatePluginSource,
} from '../api/v2';
import type { PublisherPlugin } from '../api/types';
import { LoginGate, SubmissionList } from './v2-shared';

function SourceForm({
  pluginId,
  initial,
  csrf,
  allowed,
  onSaved,
}: {
  pluginId: string;
  initial: string;
  csrf: string;
  allowed: boolean;
  onSaved: () => void;
}) {
  const [value, setValue] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [saved, setSaved] = useState(false);

  // 保存成功后父组件重新拉取插件，initial 会同步为服务端值；此时保留“已保存”提示。
  useEffect(() => {
    setValue(initial);
  }, [initial]);

  useEffect(() => {
    setSaving(false);
    setError('');
    setSaved(false);
  }, [pluginId]);

  const save = async () => {
    if (!csrf || !allowed || saving) return;
    if (!isGithubRepoUrl(value)) {
      setError('请输入公开 GitHub 仓库的 HTTPS 地址，不要包含凭据、查询或片段。');
      return;
    }
    setSaving(true);
    setError('');
    setSaved(false);
    try {
      await updatePluginSource(pluginId, value.trim(), csrf);
      setSaved(true);
      onSaved();
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : marketErrorMessage('source_update_failed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="bg-card border border-hairline rounded-xl p-6 space-y-4">
      <h2 className="text-lg font-headline font-bold">GitHub Release 自动更新来源</h2>
      <label className="block text-sm">
        公开仓库地址
        <input
          aria-label="自动更新仓库"
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
            setSaved(false);
          }}
          disabled={!allowed || saving}
          className="mt-2 w-full p-3 rounded-lg border border-hairline bg-paper font-mono text-sm focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper disabled:opacity-60"
          placeholder="https://github.com/owner/repo"
        />
      </label>
      <p className="text-xs text-muted leading-relaxed">
        登记后发布含一个插件归档的稳定 Release，市场默认每 6 小时检查并完整校验。留空保存可关闭该插件的自动同步；不会删除已发布版本。共享多插件仓库目前需注意仓库 latest Release 的选择限制，可继续使用手动上传。
      </p>
      {error && (
        <p role="alert" className="text-sm text-copper">
          {error}
        </p>
      )}
      {saved && (
        <p role="status" className="text-sm">
          来源配置已保存。
        </p>
      )}
      <button
        type="button"
        onClick={() => void save()}
        disabled={!allowed || saving}
        className="px-4 py-2 bg-charcoal text-card rounded-lg text-sm disabled:opacity-40 hover:opacity-95 transition-all"
      >
        {saving ? '保存中…' : '保存来源'}
      </button>
      {!allowed && (
        <p className="text-xs text-muted">当前账号仅可查看；修改来源需要有效的发布者权限。</p>
      )}
    </section>
  );
}

function DangerZone({
  pluginId,
  csrf,
  allowed,
}: {
  pluginId: string;
  csrf: string;
  allowed: boolean;
}) {
  const navigate = useNavigate();
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState('');

  const remove = async () => {
    if (!allowed || !csrf || deleting) return;
    const confirmed = window.confirm(
      `确定永久删除插件「${pluginId}」？\n\n` +
        '这会移除该插件及其全部已发布版本、发布记录与自动更新来源；' +
        '市场页面与安装入口立即失效，且删除后无法恢复。',
    );
    if (!confirmed) return;
    setDeleting(true);
    setError('');
    try {
      await deletePublisherPlugin(pluginId, csrf);
      navigate('/account');
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : marketErrorMessage('unavailable'));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <section className="border border-copper/40 bg-card rounded-xl p-6 space-y-3">
      <h2 className="text-lg font-headline font-bold text-copper">删除插件</h2>
      <p className="text-sm text-muted leading-relaxed">
        仅限插件所有者。删除会移除该插件及其全部已发布版本、发布记录与自动更新来源，已上架版本同时从市场下架，且无法恢复。若只想停止自动同步，请清空上方仓库地址。
      </p>
      {error && (
        <p role="alert" className="text-sm text-copper">
          {error}
        </p>
      )}
      <button
        type="button"
        disabled={!allowed || !csrf || deleting}
        onClick={() => void remove()}
        className="px-4 py-2 rounded-lg border border-copper text-copper text-sm font-medium hover:bg-copper hover:text-white disabled:opacity-40 transition-all"
      >
        {deleting ? '删除中…' : '删除插件'}
      </button>
      {!allowed && (
        <p className="text-xs text-muted">当前账号仅可查看；删除需要插件所有者权限。</p>
      )}
    </section>
  );
}

export function AccountPluginPage() {
  const { id = '' } = useParams();
  const { loading, account, csrf } = useAuth();
  const accountId = account?.id ?? '';
  const [plugin, setPlugin] = useState<PublisherPlugin>();
  const [state, setState] = useState<'loading' | 'ready' | 'missing' | 'error'>('loading');
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);

  useEffect(() => {
    if (!accountId) return;
    let alive = true;
    setState('loading');
    setError('');
    fetchPublisherPlugins(undefined, id)
      .then((page) => {
        if (!alive) return;
        const found = page.items[0];
        setPlugin(found);
        setState(found ? 'ready' : 'missing');
      })
      .catch((err: unknown) => {
        if (!alive) return;
        setError(err instanceof Error && err.message ? err.message : marketErrorMessage('unavailable'));
        setState('error');
      });
    return () => {
      alive = false;
    };
  }, [accountId, id, reload]);

  if (loading) {
    return (
      <main className="p-8 max-w-5xl w-full mx-auto" role="status">
        <p className="text-sm text-muted">检查账号…</p>
      </main>
    );
  }
  if (!account) return <LoginGate title="登录后管理插件" />;

  const allowed = canPublishWith(account);

  return (
    <main className="max-w-5xl w-full mx-auto px-6 py-10 space-y-6">
      <Link to="/account" className="text-sm text-copper hover:underline">
        返回我的插件
      </Link>

      {state === 'loading' && (
        <p role="status" className="text-sm text-muted">
          读取所属插件…
        </p>
      )}

      {state === 'error' && (
        <p role="alert" className="text-sm text-copper">
          {error}
          <button type="button" onClick={() => setReload((value) => value + 1)} className="ml-3 underline">
            重试
          </button>
        </p>
      )}

      {state === 'missing' && (
        <div className="bg-card border border-hairline rounded-xl p-8 space-y-3">
          <p className="text-sm text-muted">未找到所属插件，或此插件不属于当前账号。</p>
          <Link to="/publish" className="text-sm text-copper hover:underline">
            前往发布页
          </Link>
        </div>
      )}

      {state === 'ready' && plugin && (
        <>
          <header className="space-y-3">
            <h1 className="text-3xl font-headline font-bold">{plugin.name}</h1>
            <p className="font-mono text-sm text-muted break-all">{plugin.id}</p>
            <p className="text-sm text-muted">
              市场公开版本：{plugin.publishedVersion || '尚未上架'} · 最近提交：
              {submissionStateLabel(plugin.latestState)} {plugin.latestVersion || '—'}
            </p>
            <p className="text-sm">
              单个版本上架后内容不可替换；如需整体下架，可在本页底部删除插件（会移除该插件的全部版本与发布记录）。更新请提升包内版本号后再次提交，或在绑定的仓库发布新 Release。
            </p>
            {allowed && (
              <Link
                to="/publish"
                className="inline-flex px-4 py-2 rounded-lg bg-charcoal text-card text-sm font-medium hover:opacity-95 transition-all"
              >
                发布新版本
              </Link>
            )}
          </header>

          <SourceForm
            pluginId={plugin.id}
            initial={plugin.sourceUrl ?? ''}
            csrf={csrf}
            allowed={allowed}
            onSaved={() => setReload((value) => value + 1)}
          />

          <SubmissionList title="版本与发布记录" pluginId={plugin.id} refreshKey={reload} />

          <DangerZone pluginId={plugin.id} csrf={csrf} allowed={allowed} />
        </>
      )}
    </main>
  );
}
