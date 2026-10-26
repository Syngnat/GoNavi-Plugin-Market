import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { canPublishWith, useAuth } from '../auth/AuthContext';
import { formatBytes } from '../lib/format';
import {
  MarketRequestError,
  categoryLabel,
  fetchPublisherSubmissions,
  inspectPackage,
  isGithubRepoUrl,
  isPackageFile,
  marketErrorMessage,
  submitPackage,
} from '../api/v2';
import type { MarketAccount, PackageInspection, SubmissionRecord } from '../api/types';
import { LoginGate } from './v2-shared';

type Phase = 'idle' | 'inspecting' | 'ready' | 'publishing' | 'unknown' | 'done';

interface PublishFlow {
  file?: File;
  preview?: PackageInspection;
  source: string;
  setSource: (value: string) => void;
  phase: Phase;
  error: string;
  result?: SubmissionRecord;
  accepted: boolean;
  setAccepted: (value: boolean) => void;
  chooseFile: (file?: File) => void;
  inspect: () => Promise<void>;
  publish: () => Promise<void>;
  reconcile: () => Promise<void>;
}

function errorText(err: unknown): string {
  return err instanceof Error && err.message ? err.message : '请求未完成，请稍后重试。';
}

function usePublishFlow(csrf: string): PublishFlow {
  const [file, setFile] = useState<File>();
  const [preview, setPreview] = useState<PackageInspection>();
  const [source, setSource] = useState('');
  const [phase, setPhase] = useState<Phase>('idle');
  const [error, setError] = useState('');
  const [result, setResult] = useState<SubmissionRecord>();
  const [accepted, setAccepted] = useState(false);
  const run = useRef(0);
  const inflight = useRef<AbortController>();

  useEffect(
    () => () => {
      run.current += 1;
      inflight.current?.abort();
    },
    [],
  );

  const busy = phase === 'inspecting' || phase === 'publishing';

  const chooseFile = (next?: File) => {
    if (busy) return;
    run.current += 1;
    inflight.current?.abort();
    setFile(next);
    setPreview(undefined);
    setResult(undefined);
    setAccepted(false);
    setError('');
    setPhase('idle');
    if (next && !isPackageFile(next)) {
      setError('请选择不超过 64 MiB 的 ZIP、TGZ 或 TAR.GZ 插件包。');
    }
  };

  const inspect = async () => {
    if (!file || !isPackageFile(file)) return;
    const id = ++run.current;
    inflight.current?.abort();
    const controller = new AbortController();
    inflight.current = controller;
    setPhase('inspecting');
    setError('');
    setAccepted(false);
    try {
      const next = await inspectPackage(file, csrf, controller.signal);
      if (id !== run.current) return;
      setPreview(next);
      setPhase('ready');
    } catch (err) {
      if (id !== run.current || controller.signal.aborted) return;
      setError(errorText(err));
      setPhase('idle');
    }
  };

  const publish = async () => {
    if (!file || !preview?.canPublish || phase !== 'ready' || !accepted) return;
    if (!isGithubRepoUrl(source)) {
      setError('仓库地址必须是无凭据、无参数的公开 GitHub 仓库 HTTPS 链接。');
      return;
    }
    const id = run.current;
    setPhase('publishing');
    setError('');
    try {
      const record = await submitPackage(file, preview, source, csrf);
      if (id !== run.current) return;
      setResult(record);
      setPhase('done');
    } catch (err) {
      if (id !== run.current) return;
      setError(errorText(err));
      // 4xx 说明请求被明确拒绝；网络中断/5xx 时结果未知，先核对再重试。
      setPhase(err instanceof MarketRequestError && err.status < 500 ? 'ready' : 'unknown');
    }
  };

  const reconcile = async () => {
    if (!preview) return;
    const id = run.current;
    setError('');
    try {
      const page = await fetchPublisherSubmissions(undefined, preview.pluginId);
      if (id !== run.current) return;
      const match = page.items.find(
        (item) => item.version === preview.version && item.sha256 === preview.sha256,
      );
      if (match?.state === 'published') {
        setResult(match);
        setPhase('done');
        return;
      }
      if (match?.state === 'rejected' || match?.state === 'yanked') {
        setError('该版本已被拒绝或下架，请查看版本记录，不可重试覆盖。');
        return;
      }
      setPhase('ready');
      setError('尚未确认上架，可以使用相同包再次提交；服务器会幂等恢复。');
    } catch (err) {
      if (id !== run.current) return;
      setError(errorText(err));
    }
  };

  return {
    file,
    preview,
    source,
    setSource,
    phase,
    error,
    result,
    accepted,
    setAccepted,
    chooseFile,
    inspect,
    publish,
    reconcile,
  };
}

function PackagePicker({ flow }: { flow: PublishFlow }) {
  const disabled = flow.phase === 'inspecting' || flow.phase === 'publishing';
  return (
    <section className="bg-card border border-hairline rounded-xl p-6 space-y-5">
      <label
        className="block rounded-xl border border-dashed border-hairline bg-paper p-8 text-center cursor-pointer hover:border-copper transition-colors"
        onDragOver={(event) => event.preventDefault()}
        onDrop={(event) => {
          event.preventDefault();
          if (disabled) return;
          flow.chooseFile(event.dataTransfer.files[0]);
        }}
      >
        <span className="block font-semibold mb-2">选择或拖入插件包</span>
        <span className="block text-sm text-muted mb-4">
          ZIP / TGZ / TAR.GZ，最大 64 MiB，根目录需包含 gonavi.plugin.json
        </span>
        <input
          aria-label="插件包"
          type="file"
          accept=".zip,.tgz,.tar.gz"
          disabled={disabled}
          onChange={(event) => {
            const picked = event.target.files?.[0];
            event.target.value = '';
            flow.chooseFile(picked);
          }}
          className="max-w-full text-sm"
        />
        {flow.file && (
          <span className="block mt-3 text-xs text-muted font-mono break-all">
            {flow.file.name} · {formatBytes(flow.file.size)}
          </span>
        )}
      </label>

      <label className="block text-sm font-medium">
        GitHub 源仓库（可选）
        <input
          aria-label="GitHub 源仓库"
          value={flow.source}
          disabled={disabled}
          onChange={(event) => flow.setSource(event.target.value)}
          placeholder="https://github.com/owner/repo"
          className="mt-2 w-full rounded-lg border border-hairline bg-paper p-3 font-mono text-sm focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper"
        />
      </label>
      <p className="text-xs text-muted leading-relaxed">
        首次登记后，发布含一个插件归档的稳定 GitHub Release 可自动同步；默认每 6 小时检查一次，手动上传仍可用。
      </p>

      <button
        type="button"
        disabled={!flow.file || disabled}
        onClick={() => void flow.inspect()}
        className="px-5 py-2.5 rounded-lg bg-charcoal text-card font-medium disabled:opacity-50 hover:opacity-95 transition-all"
      >
        {flow.phase === 'inspecting' ? '静态预检中…' : '预检安装包'}
      </button>
    </section>
  );
}

function PreviewCard({ preview }: { preview: PackageInspection }) {
  const permissions = Array.isArray(preview.manifest?.permissions)
    ? preview.manifest.permissions.filter((item): item is string => typeof item === 'string')
    : [];

  return (
    <section className="rounded-xl border border-hairline bg-card p-6 space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 className="text-xl font-headline font-bold">{preview.name}</h2>
          <p className="text-sm text-muted font-mono break-all">
            {preview.pluginId} · {preview.version}
          </p>
        </div>
        <span className="rounded-full border border-hairline px-3 py-1 text-xs">
          {categoryLabel(preview.category)}
        </span>
      </div>

      <p className="text-sm text-muted leading-relaxed">{preview.description}</p>

      <dl className="grid sm:grid-cols-2 gap-3 text-sm">
        <div>
          <dt className="text-muted">安装包大小</dt>
          <dd>{formatBytes(preview.sizeBytes)}</dd>
        </div>
        <div>
          <dt className="text-muted">最低 GoNavi</dt>
          <dd>{preview.minHostVersion || '未额外限制'}</dd>
        </div>
        <div className="sm:col-span-2">
          <dt className="text-muted">支持平台</dt>
          <dd>{preview.platforms?.join(' · ') || '未声明'}</dd>
        </div>
        <div className="sm:col-span-2">
          <dt className="text-muted">实际包 SHA-256</dt>
          <dd className="font-mono text-xs break-all">{preview.sha256}</dd>
        </div>
      </dl>

      <div>
        <h3 className="font-headline font-semibold text-sm mb-2">包内权限声明</h3>
        {permissions.length ? (
          <ul className="flex flex-wrap gap-2">
            {permissions.map((permission) => (
              <li
                key={permission}
                className="px-2 py-1 rounded bg-paper border border-hairline text-xs font-mono"
              >
                {permission}
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted">未声明额外权限。</p>
        )}
      </div>

      {!preview.canPublish && (
        <p role="alert" className="rounded-lg border border-copper/40 bg-paper p-3 text-sm">
          {preview.reason
            ? `当前不可发布：${marketErrorMessage(preview.reason)}`
            : '当前账号不可发布此插件，请核对包内 ID 前缀与发布者权限。'}
        </p>
      )}

      <p className="text-xs text-muted leading-relaxed">
        预检不会运行代码、预约版本或上架；只有确认发布才会提交。摘要是完整性证明，不代表插件代码可信。
      </p>
    </section>
  );
}

function ConfirmPublish({ flow }: { flow: PublishFlow }) {
  const busy = flow.phase === 'inspecting' || flow.phase === 'publishing';
  return (
    <section className="bg-card border border-hairline rounded-xl p-6 space-y-4">
      <label className="flex items-start gap-3 text-sm">
        <input
          type="checkbox"
          checked={flow.accepted}
          disabled={busy}
          onChange={(event) => flow.setAccepted(event.target.checked)}
          className="mt-0.5"
        />
        <span>
          我已核对包内身份、版本、权限和实际摘要，确认有权发布；该版本上架后内容不可替换。
        </span>
      </label>
      <button
        type="button"
        disabled={!flow.accepted || flow.phase !== 'ready'}
        onClick={() => void flow.publish()}
        className="px-6 py-3 rounded-lg bg-charcoal text-card font-semibold disabled:opacity-50 hover:opacity-95 transition-all"
      >
        {flow.phase === 'publishing' ? '正在发布，请勿关闭页面…' : '确认发布到插件市场'}
      </button>
    </section>
  );
}

function PublishResult({ result }: { result: SubmissionRecord }) {
  return (
    <section role="status" className="bg-card border border-copper/40 rounded-xl p-6 space-y-3">
      <h2 className="text-xl font-headline font-bold">插件已上架</h2>
      <p>
        {result.pluginId} · {result.version}
      </p>
      {result.warning && <p className="text-sm text-copper">{result.warning}</p>}
      <div className="flex flex-wrap gap-4 text-sm">
        <Link to={`/plugins/${encodeURIComponent(result.pluginId)}`} className="text-copper hover:underline">
          查看市场详情
        </Link>
        <Link
          to={`/account/plugins/${encodeURIComponent(result.pluginId)}`}
          className="text-copper hover:underline"
        >
          管理版本与来源
        </Link>
      </div>
    </section>
  );
}

function PublishWorkspace({ account, csrf }: { account: MarketAccount; csrf: string }) {
  const flow = usePublishFlow(csrf);
  const allowed = canPublishWith(account);

  return (
    <main className="w-full max-w-4xl mx-auto px-6 py-10 space-y-6">
      <div>
        <p className="text-sm text-copper">作者工作台</p>
        <h1 className="text-3xl font-headline font-bold mt-2">发布插件</h1>
        <p className="text-muted mt-3 leading-relaxed">
          先上传包做完整静态预检，再确认上架。发布后的版本内容不可覆盖。
        </p>
      </div>

      <section className="bg-card border border-hairline rounded-xl p-6">
        <div className="flex justify-between flex-wrap gap-3">
          <p>
            当前账号：<strong>{account.login}</strong>
          </p>
          <Link to="/account" className="text-copper hover:underline text-sm">
            我的插件与版本记录
          </Link>
        </div>
        <p className="text-sm mt-2">
          您的命名空间：<code className="font-mono">{account.publisherId || '尚未分配'}</code>
          ，包内 ID 必须以它加英文句点开头。
        </p>
      </section>

      {allowed ? (
        <>
          <PackagePicker flow={flow} />

          {flow.error && (
            <div
              role="alert"
              className="border border-copper/40 bg-card rounded-xl p-4 space-y-2 text-sm"
            >
              <p className="text-copper">{flow.error}</p>
              {flow.phase === 'unknown' && (
                <>
                  <p>网络中断不代表发布失败，请先核对结果，避免换包覆盖。</p>
                  <button
                    type="button"
                    onClick={() => void flow.reconcile()}
                    className="text-copper underline"
                  >
                    确认发布结果
                  </button>
                </>
              )}
            </div>
          )}

          {flow.preview && <PreviewCard preview={flow.preview} />}
          {flow.preview?.canPublish && flow.phase !== 'done' && <ConfirmPublish flow={flow} />}
          {flow.result && <PublishResult result={flow.result} />}
        </>
      ) : (
        <p role="alert" className="border border-copper/40 bg-card rounded-xl p-4 text-sm">
          当前账号没有发布权限，请联系市场维护者；不要使用其他作者的命名空间。
        </p>
      )}
    </main>
  );
}

export function PublishPage() {
  const { loading, account, csrf } = useAuth();

  if (loading) {
    return (
      <main className="w-full max-w-4xl mx-auto px-6 py-10" role="status">
        <p className="text-sm text-muted">检查账号…</p>
      </main>
    );
  }
  if (!account) return <LoginGate title="登录后发布插件" />;

  return <PublishWorkspace key={account.id} account={account} csrf={csrf} />;
}
