import { Link, useLocation } from 'react-router-dom';

const GITHUB_BASE = 'https://github.com/Syngnat/GoNavi';
const PLUGIN_DOCS_URL = `${GITHUB_BASE}/tree/dev/docs/plugins`;
const PLUGIN_DEV_GUIDE_URL = `${GITHUB_BASE}/blob/dev/docs/plugins/development.md`;
const ISSUES_URL = `${GITHUB_BASE}/issues`;

const navLinkClass = (active: boolean) =>
  active
    ? 'text-copper border-b-2 border-copper pb-1 font-bold transition-all duration-200'
    : 'text-muted hover:text-ink transition-colors duration-200 pb-1 border-b-2 border-transparent';

const externalLinkClass = 'text-muted hover:text-ink transition-colors duration-200 pb-1';

export function TopNav() {
  const { pathname } = useLocation();
  const catalogActive = pathname.startsWith('/plugins');

  return (
    <header className="bg-paper border-b border-hairline sticky top-0 z-50">
      <div className="flex justify-between items-center w-full px-6 py-4 max-w-7xl mx-auto">
        <Link to="/" className="flex items-center space-x-3">
          <div className="w-9 h-9 rounded-lg bg-copper text-white flex items-center justify-center font-headline font-bold text-lg shadow-sm">
            GN
          </div>
          <div>
            <span className="text-xl font-headline font-bold text-ink">GoNavi 插件市场</span>
            <span className="text-xs text-muted block font-mono">plugin.gonavi.org</span>
          </div>
        </Link>
        <nav className="hidden md:flex items-center space-x-8">
          <Link to="/" className={navLinkClass(!catalogActive)}>
            首页
          </Link>
          <Link to="/#catalog" className={navLinkClass(catalogActive)}>
            插件分类
          </Link>
          <a href={PLUGIN_DOCS_URL} target="_blank" rel="noreferrer" className={externalLinkClass}>
            开发者文档
          </a>
          <a href={ISSUES_URL} target="_blank" rel="noreferrer" className={externalLinkClass}>
            社区
          </a>
          <a
            href={PLUGIN_DEV_GUIDE_URL}
            target="_blank"
            rel="noreferrer"
            className="px-4 py-2 rounded-lg border border-copper text-copper font-medium text-sm hover:bg-copper hover:text-white transition-all duration-200 shadow-sm"
          >
            发布插件
          </a>
        </nav>
      </div>
    </header>
  );
}
