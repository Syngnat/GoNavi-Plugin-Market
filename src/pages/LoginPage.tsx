import { Link, useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { marketErrorMessage, sanitizeReturnTo, webLoginUrl } from '../api/v2';

export function LoginPage() {
  const { loading, account, error, refresh } = useAuth();
  const [params] = useSearchParams();
  const target = sanitizeReturnTo(params.get('returnTo'));
  const errorCode = params.get('error');
  const loginHref = webLoginUrl(target);
  const message = errorCode ? marketErrorMessage(errorCode) : error;

  return (
    <main className="flex-grow flex items-center justify-center px-6 py-16">
      <section className="w-full max-w-lg bg-card border border-hairline rounded-xl shadow-sm p-8 space-y-5">
        <div className="w-12 h-12 rounded-xl bg-copper text-white font-headline font-bold text-xl flex items-center justify-center shadow-sm">
          GN
        </div>
        <h1 className="text-2xl font-headline font-bold tracking-tight">登录 GoNavi 插件市场</h1>
        <p className="text-muted leading-relaxed">
          在网站直接使用 GitHub 登录，管理插件、上传发布和评分，无需打开客户端或输入配对码。
        </p>

        {message && (
          <p role="alert" className="rounded-lg border border-hairline p-3 text-sm text-copper">
            {message}
          </p>
        )}

        {account ? (
          <div className="space-y-3">
            <p className="text-sm">
              已登录为 <strong>{account.login}</strong>
            </p>
            <Link
              to={target}
              className="block text-center rounded-lg bg-charcoal text-card px-4 py-3 font-semibold hover:opacity-95 transition-all"
            >
              继续前往目标页面
            </Link>
            <Link to="/account" className="block text-center text-sm text-copper hover:underline">
              前往作者工作台
            </Link>
          </div>
        ) : loading ? (
          <button
            type="button"
            disabled
            className="w-full rounded-lg bg-charcoal text-card px-4 py-3 font-semibold opacity-60"
          >
            检查登录服务…
          </button>
        ) : (
          <a
            href={loginHref}
            className="block text-center rounded-lg bg-charcoal text-card px-4 py-3 font-semibold hover:opacity-95 transition-all"
          >
            使用 GitHub 登录
          </a>
        )}

        {!loading && error && (
          <button
            type="button"
            onClick={() => void refresh()}
            className="w-full px-4 py-2 border border-hairline rounded-lg text-sm text-muted hover:text-ink transition-colors"
          >
            重试登录服务
          </button>
        )}

        <p className="text-xs text-muted leading-relaxed">
          仅验证账号身份，不请求仓库权限；市场不保存 GitHub 密码。登录凭据使用 HttpOnly Cookie，不写入浏览器本地存储。
        </p>
        <Link to="/" className="text-sm text-copper hover:underline inline-block">
          返回插件目录
        </Link>
      </section>
    </main>
  );
}
