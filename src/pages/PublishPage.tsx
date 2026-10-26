import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { canPublishWith, useAuth } from '../auth/AuthContext';
import { formatBytes } from '../lib/format';
import {
  MarketRequestError,
  categoryLabel,
  fetchPublisherSubmissions,
  inspectPackage,
  inspectRelease,
  isGithubReleaseUrl,
  isPackageFile,
  marketErrorMessage,
  publishRelease,
  submitPackage,
} from '../api/v2';
import type {
  MarketAccount,
  PackageInspection,
  ReleaseInspection,
  SubmissionRecord,
} from '../api/types';
import { LoginGate } from './v2-shared';

type Phase =
  | 'idle'
  | 'inspecting'
  | 'ready'
  | 'publishing'
  | 'unknown'
  | 'done';

type PublishMethod = 'release' | 'upload';

/** 预检返回的 Release 摘要必须带 releaseUrl；否则它不是可直发的 Release 预检。 */
function isReleasePreview(preview: PackageInspection | ReleaseInspection): preview is ReleaseInspection {
  return 'releaseUrl' in preview;
}

function errorText(err: unknown): string {
  return err instanceof Error && err.message ? err.message : '请求未完成，请稍后重试。';
}

function Result({ result }: { result: SubmissionRecord }) {
  return (
    <section role="status" className="bg-card border border-copper/40 rounded-xl p-6 space-y-3">
      <h2 className="text-xl font-headline font-bold">插件已上架</h2>
      <p>
        {result.pluginId} · {result.version}
      </p>
      {result.warning && (
        <p className="text-sm text-copper">
          插件已发布，但自动更新来源登记未完成；请到账号页重试。
        </p>
      )}
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

function InspectionPreview({
  preview,
  source,
}: {
  preview: PackageInspection | ReleaseInspection;
  source?: ReactNode;
}) {
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
      {source}
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

function Confirm({
  preview,
  accepted,
  setAccepted,
  phase,
  onPublish,
}: {
  preview: PackageInspection | ReleaseInspection;
  accepted: boolean;
  setAccepted: (value: boolean) => void;
  phase: Phase;
  onPublish: () => void;
}) {
  const busy = phase === 'inspecting' || phase === 'publishing';
  return (
    <section className="bg-card border border-hairline rounded-xl p-6 space-y-4">
      <label className="flex items-start gap-3 text-sm">
        <input
          type="checkbox"
          checked={accepted}
          disabled={busy}
          onChange={(event) => setAccepted(event.target.checked)}
          className="mt-0.5"
        />
        <span>
          我已核对包内身份、版本、权限和实际摘要，确认有权发布；该版本上架后内容不可替换。
        </span>
      </label>
      <button
        type="button"
        disabled={!accepted || !preview.canPublish || phase !== 'ready' || busy}
        onClick={onPublish}
        className="px-6 py-3 rounded-lg bg-charcoal text-card font-semibold disabled:opacity-50 hover:opacity-95 transition-all"
      >
        {phase === 'publishing' ? '正在发布，请勿关闭页面…' : '确认发布到插件市场'}
      </button>
    </section>
  );
}

function ReleasePanel({
  allowed,
  phase,
  csrf,
  onPrepared,
  onInspectStart,
  onInspectError,
  onDirty,
}: {
  allowed: boolean;
  phase: Phase;
  csrf: string;
  onPrepared: (inspection: ReleaseInspection) => void;
  onInspectStart: () => void;
  onInspectError: () => void;
  onDirty: () => void;
}) {
  const [sourceUrl, setSourceUrl] = useState('');
  const [error, setError] = useState('');
  const request = useRef(0);
  const controller = useRef<AbortController>();
  const busy = phase === 'inspecting' || phase === 'publishing';

  useEffect(() => () => controller.current?.abort(), []);

  const inspect = async () => {
    if (!isGithubReleaseUrl(sourceUrl)) {
      setError('请输入公开 GitHub 仓库、Release 页面或 Release 资产链接。');
      return;
    }
    const run = ++request.current;
    controller.current?.abort();
    const next = new AbortController();
    controller.current = next;
    setError('');
    onInspectStart();
    try {
      const inspection = await inspectRelease(sourceUrl, csrf, next.signal);
      if (run === request.current) onPrepared(inspection);
    } catch (err) {
      if (run === request.current && !next.signal.aborted) {
        setError(errorText(err));
        onInspectError();
      }
    }
  };

  return (
    <section className="bg-card border border-copper/40 rounded-xl p-6 space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="text-lg font-headline font-bold">从 GitHub Release 发布</h2>
        <span className="rounded-full bg-copper/10 px-2 py-1 text-xs text-copper">推荐</span>
      </div>
      <label className="block text-sm font-medium">
        GitHub 仓库或 Release 链接
        <input
          aria-label="GitHub Release 链接"
          value={sourceUrl}
          onChange={(event) => {
            setSourceUrl(event.target.value);
            setError('');
            onDirty();
          }}
          disabled={!allowed || busy}
          placeholder="https://github.com/owner/repo/releases/tag/v1.0.0"
          className="mt-2 w-full rounded-lg border border-hairline bg-paper p-3 font-mono text-sm focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper"
        />
      </label>
      <p className="text-xs text-muted leading-relaxed">
        单插件仓库可以直接填仓库链接，市场读取 latest stable Release。官方
        <code className="mx-1">Syngnat/GoNavi-Plugin-Market</code>
        monorepo 必须填具体 Release Tag 或 Release 资产链接，避免误发其他插件。
      </p>
      {error && (
        <p role="alert" className="text-sm text-copper">
          {error}
        </p>
      )}
      <button
        type="button"
        disabled={!allowed || !sourceUrl.trim() || busy}
        onClick={() => void inspect()}
        className="px-5 py-2.5 rounded-lg bg-charcoal text-card font-medium disabled:opacity-50 hover:opacity-95 transition-all"
      >
        {phase === 'inspecting' ? '正在读取 Release 并预检…' : '读取并预检 Release'}
      </button>
      {!allowed && (
        <p className="text-xs text-muted">当前账号没有发布权限。</p>
      )}
    </section>
  );
}

function UploadPanel({
  allowed,
  phase,
  csrf,
  onPrepared,
  onInspectStart,
  onInspectError,
  onDirty,
}: {
  allowed: boolean;
  phase: Phase;
  csrf: string;
  onPrepared: (inspection: PackageInspection, file: File) => void;
  onInspectStart: () => void;
  onInspectError: () => void;
  onDirty: () => void;
}) {
  const [file, setFile] = useState<File>();
  const [error, setError] = useState('');
  const request = useRef(0);
  const controller = useRef<AbortController>();
  const busy = phase === 'inspecting' || phase === 'publishing';

  useEffect(() => () => controller.current?.abort(), []);

  const choose = (next?: File) => {
    request.current++;
    controller.current?.abort();
    setFile(next);
    onDirty();
    setError(next && !isPackageFile(next) ? '请选择不超过 64 MiB 的 ZIP、TGZ 或 TAR.GZ。' : '');
  };

  const inspect = async () => {
    if (!file || !isPackageFile(file)) return;
    const run = ++request.current;
    controller.current?.abort();
    const next = new AbortController();
    controller.current = next;
    setError('');
    onInspectStart();
    try {
      const inspection = await inspectPackage(file, csrf, next.signal);
      if (run === request.current) onPrepared(inspection, file);
    } catch (err) {
      if (run === request.current && !next.signal.aborted) {
        setError(errorText(err));
        onInspectError();
      }
    }
  };

  return (
    <section className="bg-card border border-hairline rounded-xl p-6 space-y-4">
      <h2 className="text-lg font-headline font-bold">手动上传插件包</h2>
      <p className="text-sm text-muted leading-relaxed">
        适用于离线包、私有仓库或 Release 暂时不可用的情况。上传入口不是主流程。
      </p>
      <label
        className="block rounded-xl border border-dashed border-hairline bg-paper p-6 text-center cursor-pointer hover:border-copper transition-colors"
        onDragOver={(event) => event.preventDefault()}
        onDrop={(event) => {
          event.preventDefault();
          if (!busy) choose(event.dataTransfer.files[0]);
        }}
      >
        <span className="block font-semibold mb-2">选择或拖入插件包</span>
        <span className="block text-sm text-muted mb-3">ZIP / TGZ / TAR.GZ，最大 64 MiB</span>
        <input
          aria-label="插件包"
          type="file"
          accept=".zip,.tgz,.tar.gz"
          disabled={!allowed || busy}
          onChange={(event) => {
            const picked = event.target.files?.[0];
            event.target.value = '';
            choose(picked);
          }}
          className="max-w-full text-sm"
        />
        {file && (
          <span className="block mt-3 text-xs text-muted font-mono break-all">
            {file.name} · {formatBytes(file.size)}
          </span>
        )}
      </label>
      {error && (
        <p role="alert" className="text-sm text-copper">
          {error}
        </p>
      )}
      <button
        type="button"
        disabled={!allowed || !file || busy}
        onClick={() => void inspect()}
        className="px-5 py-2.5 rounded-lg border border-hairline font-medium disabled:opacity-50 hover:border-copper transition-colors"
      >
        {phase === 'inspecting' ? '静态预检中…' : '预检上传包'}
      </button>
    </section>
  );
}

function PublishWorkspace({ account, csrf }: { account: MarketAccount; csrf: string }) {
  const allowed = canPublishWith(account);
  const [method, setMethod] = useState<PublishMethod>('release');
  const [phase, setPhase] = useState<Phase>('idle');
  const [preview, setPreview] = useState<PackageInspection | ReleaseInspection>();
  const [accepted, setAccepted] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<SubmissionRecord>();
  const [uploadFile, setUploadFile] = useState<File>();
  const [reloadHint, setReloadHint] = useState(0);

  const resetPreview = () => {
    setReloadHint((value) => value + 1);
    setPreview(undefined);
    setAccepted(false);
    setError('');
    setResult(undefined);
    setPhase('idle');
  };

  const chooseMethod = (next: PublishMethod) => {
    if (phase === 'publishing') return;
    setMethod(next);
    resetPreview();
  };

  const prepared = (inspection: PackageInspection | ReleaseInspection, file?: File) => {
    setPreview(inspection);
    setUploadFile(file);
    setAccepted(false);
    setError('');
    setPhase('ready');
  };

  const publish = async () => {
    if (!preview?.canPublish || phase !== 'ready' || !accepted) return;
    setPhase('publishing');
    setError('');
    try {
      let record: SubmissionRecord;
      if (method === 'release' && isReleasePreview(preview)) {
        record = await publishRelease(preview.releaseUrl, preview.sha256, csrf);
      } else {
        if (!uploadFile) {
          setError('插件包已失效，请重新选择并预检。');
          setPhase('idle');
          return;
        }
        record = await submitPackage(uploadFile, preview, '', csrf);
      }
      setResult(record);
      setPhase('done');
    } catch (err) {
      setError(errorText(err));
      setPhase(err instanceof MarketRequestError && err.status < 500 ? 'ready' : 'unknown');
    }
  };

  const reconcile = async () => {
    if (!preview) return;
    setError('');
    try {
      const page = await fetchPublisherSubmissions(undefined, preview.pluginId);
      const match = page.items.find((item) => item.version === preview.version && item.sha256 === preview.sha256);
      if (match?.state === 'published') {
        setResult(match);
        setPhase('done');
        return;
      }
      setPhase('ready');
      setError('尚未确认上架；可再次发布相同 Release，服务器会保持同一身份与摘要。');
    } catch (err) {
      setError(errorText(err));
    }
  };

  return (
    <main className="w-full max-w-4xl mx-auto px-6 py-10 space-y-6">
      <div>
        <p className="text-sm text-copper">作者工作台</p>
        <h1 className="text-3xl font-headline font-bold mt-2">发布插件</h1>
        <p className="text-muted mt-3 leading-relaxed">
          优先从 GitHub Release 读取已构建的插件包；上传包仅作兜底。
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
          <div className="flex gap-2" role="tablist" aria-label="发布方式">
            <button
              type="button"
              role="tab"
              aria-selected={method === 'release'}
              onClick={() => chooseMethod('release')}
              className={`rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                method === 'release' ? 'bg-charcoal text-card' : 'border border-hairline text-muted hover:text-ink'
              }`}
            >
              GitHub Release（推荐）
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={method === 'upload'}
              onClick={() => chooseMethod('upload')}
              className={`rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                method === 'upload' ? 'bg-charcoal text-card' : 'border border-hairline text-muted hover:text-ink'
              }`}
            >
              手动上传兜底
            </button>
          </div>

          {method === 'release' ? (
            <ReleasePanel
              key={`release-${reloadHint}`}
              allowed={allowed}
              phase={phase}
              csrf={csrf}
              onPrepared={(inspection) => prepared(inspection)}
              onInspectStart={() => {
                setError('');
                setPhase('inspecting');
              }}
              onInspectError={() => setPhase('idle')}
              onDirty={resetPreview}
            />
          ) : (
            <UploadPanel
              key={`upload-${reloadHint}`}
              allowed={allowed}
              phase={phase}
              csrf={csrf}
              onPrepared={prepared}
              onInspectStart={() => {
                setError('');
                setPhase('inspecting');
              }}
              onInspectError={() => setPhase('idle')}
              onDirty={resetPreview}
            />
          )}

          {error && (
            <div
              role="alert"
              className="border border-copper/40 bg-card rounded-xl p-4 space-y-2 text-sm"
            >
              <p className="text-copper">{error}</p>
              {phase === 'unknown' && (
                <>
                  <p>网络中断不代表发布失败，请先核对结果，避免重复提交。</p>
                  <button type="button" onClick={() => void reconcile()} className="text-copper underline">
                    确认发布结果
                  </button>
                </>
              )}
            </div>
          )}

          {preview && (
            <InspectionPreview
              preview={preview}
              source={
                method === 'release' && 'releaseUrl' in preview ? (
                  <div className="rounded-lg bg-paper p-3 text-sm space-y-1">
                    <p>
                      Release：
                      <a href={preview.releaseUrl} target="_blank" rel="noreferrer" className="text-copper hover:underline break-all">
                        {preview.releaseTag}
                      </a>
                      {preview.prerelease && <span className="ml-2 text-xs text-muted">预发布</span>}
                    </p>
                    <p className="font-mono text-xs text-muted break-all">{preview.assetName}</p>
                  </div>
                ) : undefined
              }
            />
          )}

          {preview?.canPublish && phase !== 'done' && (
            <Confirm
              preview={preview}
              accepted={accepted}
              setAccepted={setAccepted}
              phase={phase}
              onPublish={() => void publish()}
            />
          )}
          {result && <Result result={result} />}
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
