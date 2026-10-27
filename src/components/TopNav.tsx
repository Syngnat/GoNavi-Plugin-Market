import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

const navLinkClass = (active: boolean) =>
  active
    ? 'text-copper border-b-2 border-copper border-t-2 border-t-transparent py-1 font-bold transition-all duration-200'
    : 'text-muted hover:text-ink transition-colors duration-200 py-1 border-b-2 border-transparent border-t-2 border-t-transparent';

const pillClass = (active: boolean) =>
  active
    ? 'px-4 py-2 rounded-lg bg-copper !text-white font-medium text-sm shadow-sm opacity-95 transition-all duration-200'
    : 'px-4 py-2 rounded-lg bg-copper !text-white font-medium text-sm shadow-sm hover:opacity-90 transition-all duration-200';

function AccountMenu() {
  const { loading, account, logout } = useAuth();
  const { pathname, search, hash } = useLocation();
  const [signingOut, setSigningOut] = useState(false);
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

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
        className={navLinkClass(pathname === '/login')}
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
    <div ref={menuRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-expanded={open}
        aria-haspopup="menu"
        className="flex items-center gap-1.5 py-1 text-ink hover:text-copper transition-colors cursor-pointer text-sm font-medium focus:outline-none"
      >
        <span>{account.login}</span>
        <svg
          className={`w-3.5 h-3.5 transition-transform duration-200 ${
            open ? 'rotate-180 text-copper' : 'text-muted'
          }`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div
          role="menu"
          className="account-menu-dropdown absolute right-0 mt-2 min-w-[7.5rem] rounded-lg border border-hairline bg-card shadow-xl py-1 text-sm z-50 whitespace-nowrap"
        >
          <Link
            to="/account"
            role="menuitem"
            onClick={() => setOpen(false)}
            className="block w-full px-4 py-2 text-ink hover:text-copper hover:bg-paper text-left transition-colors"
          >
            我的插件
          </Link>
          <Link
            to="/account/submissions"
            role="menuitem"
            onClick={() => setOpen(false)}
            className="block w-full px-4 py-2 text-ink hover:text-copper hover:bg-paper text-left transition-colors"
          >
            发布记录
          </Link>
          <div className="border-t border-hairline account-divider my-1" />
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false);
              void signOut();
            }}
            disabled={signingOut}
            className="block w-full px-4 py-2 text-muted hover:text-ink hover:bg-paper text-left transition-colors disabled:opacity-50"
          >
            {signingOut ? '退出中…' : '退出'}
          </button>
        </div>
      )}
    </div>
  );
}

export function TopNav() {
  const { pathname } = useLocation();
  const catalogActive = pathname === '/plugins' || pathname.startsWith('/plugins/');
  const docsActive = pathname.startsWith('/docs');

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
          <Link to="/" className={navLinkClass(pathname === '/')}>
            首页
          </Link>
          <Link to="/plugins" className={navLinkClass(catalogActive)}>
            插件分类
          </Link>
          <Link to="/docs" className={navLinkClass(docsActive || pathname.startsWith('/developers'))}>
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
