import { Link } from 'react-router-dom';
import { canPublishWith, useAuth } from '../auth/AuthContext';
import { AccountHeader, LoginGate, OwnedPluginList, SubmissionList } from './v2-shared';

type AccountView = 'overview' | 'plugins' | 'submissions';

const TABS: { view: AccountView; label: string; to: string }[] = [
  { view: 'overview', label: '账号总览', to: '/account' },
  { view: 'plugins', label: '我的插件', to: '/account/plugins' },
  { view: 'submissions', label: '发布记录', to: '/account/submissions' },
];

function AccountTabs({ active }: { active: AccountView }) {
  return (
    <nav className="flex flex-wrap gap-2 border-b border-hairline pb-3">
      {TABS.map((tab) => (
        <Link
          key={tab.view}
          to={tab.to}
          className={
            tab.view === active
              ? 'px-4 py-1.5 rounded-full text-xs font-medium bg-copper text-white shadow-sm transition-all'
              : 'px-4 py-1.5 rounded-full text-xs font-medium bg-paper border border-hairline text-muted hover:text-ink hover:border-copper transition-all'
          }
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  );
}

export function AccountPage({ view = 'overview' }: { view?: AccountView }) {
  const { loading, account } = useAuth();

  if (loading) {
    return (
      <main className="p-8 max-w-5xl mx-auto w-full" role="status">
        <p className="text-sm text-muted">检查账号…</p>
      </main>
    );
  }

  if (!account) return <LoginGate title="登录后查看我的插件" />;

  const publishAllowed = canPublishWith(account);

  return (
    <main className="max-w-5xl mx-auto w-full px-6 py-10 space-y-7">
      <AccountHeader account={account} publishAllowed={publishAllowed} />
      <AccountTabs active={view} />

      {!publishAllowed && (
        <p className="border border-hairline bg-card rounded-lg p-4 text-sm text-muted">
          当前账号仅可查看记录；发布或修改来源需要有效的发布者权限。
        </p>
      )}

      {(view === 'overview' || view === 'plugins') && <OwnedPluginList />}

      {(view === 'overview' || view === 'submissions') && <SubmissionList title="我的发布记录" />}
    </main>
  );
}
