import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

const navLinkClass = (active: boolean) =>
  active
    ? 'text-copper border-b-2 border-copper pb-1 font-bold transition-all duration-200'
    : 'text-muted hover:text-ink transition-colors duration-200 pb-1 border-b-2 border-transparent';

const pillClass = (active: boolean) =>
  active
    ? 'px-4 py-2 rounded-lg border border-copper bg-copper text-white font-medium text-sm shadow-sm transition-all duration-200'
    : 'px-4 py-2 rounded-lg border border-copper text-copper font-medium text-sm hover:bg-copper hover:text-white transition-all duration-200 shadow-sm';

function AccountMenu() {
  const { loading, account, logout } = useAuth();
  const { pathname, search, hash } = useLocation();
  const [signingOut, setSigningOut] = useState(false);

  if (loading) {
    return (
      <span className="text-sm text-muted" role="status">
        检查账号…
      </span>
    );
  }

  if (!account) {
    return (
      <Link
        to={`/login?returnTo=${encodeURIComponent(pathname + search + hash)}`}
        className="text-muted hover:text-ink transition-colors duration-200 pb-1"
      >
        登录
      </Link>
    );
  }

  const signOut = async () => {
    setSigningOut(true);
    try {
      await logout();
    } catch {
      // 退出失败时 AuthContext 已给出中文错误，导航保留登录态。
    } finally {
      setSigningOut(false);
    }
  };

  return (
    <div className="flex items-center gap-3 text-sm">
      <Link to="/account" className="text-ink hover:text-copper transition-colors">
        {account.login} · 我的插件
      </Link>
      <button
        type="button"
        onClick={() => void signOut()}
        disabled={signingOut}
        className="text-muted hover:text-ink disabled:opacity-50 transition-colors"
      >
        {signingOut ? '退出中…' : '退出'}
      </button>
    </div>
  );
}

export function TopNav() {
  const { pathname, hash } = useLocation();
  const catalogActive = pathname.startsWith('/plugins') || (pathname === '/' && hash === '#catalog');

  return (
    <header className="bg-paper border-b border-hairline sticky top-0 z-50">
      <div className="flex flex-wrap gap-y-4 justify-between items-center w-full px-6 py-4 max-w-7xl mx-auto">
        <Link to="/" className="flex items-center space-x-3">
          <div className="w-9 h-9 rounded-lg bg-copper text-white flex items-center justify-center font-headline font-bold text-lg shadow-sm">
            GN
          </div>
          <div>
            <span className="text-xl font-headline font-bold text-ink">GoNavi 插件市场</span>
            <span className="text-xs text-muted block font-mono">plugin.gonavi.org</span>
          </div>
        </Link>
        <nav className="flex w-full flex-wrap items-center gap-x-5 gap-y-3 text-sm md:w-auto md:gap-x-8">
          <Link to="/" className={navLinkClass(pathname === '/' && !catalogActive)}>
            首页
          </Link>
          <Link to="/#catalog" className={navLinkClass(catalogActive)}>
            插件分类
          </Link>
          <Link to="/developers" className={navLinkClass(pathname.startsWith('/developers'))}>
            开发者文档
          </Link>
          <Link to="/community" className={navLinkClass(pathname.startsWith('/community'))}>
            社区
          </Link>
          <AccountMenu />
          <Link to="/publish" className={pillClass(pathname === '/publish')}>
            发布插件
          </Link>
        </nav>
      </div>
    </header>
  );
}
