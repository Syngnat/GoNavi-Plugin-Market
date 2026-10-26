import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { SESSION_EXPIRED_EVENT, fetchWebSession, logoutWebSession, marketErrorMessage } from '../api/v2';
import type { MarketAccount } from '../api/types';

interface SessionState {
  account: MarketAccount | null;
  csrf: string;
}

export interface AuthContextValue {
  loading: boolean;
  account: MarketAccount | null;
  csrf: string;
  error: string;
  /** 重新读取会话；quiet 用于窗口聚焦时的静默刷新。 */
  refresh: (quiet?: boolean) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/** 发布/改来源需要 publisher 或 admin 角色，并且已分配命名空间。 */
export function canPublishWith(account: MarketAccount | null): boolean {
  if (!account) return false;
  return (account.role === 'publisher' || account.role === 'admin') && !!account.publisherId;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState>({ account: null, csrf: '' });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const attempt = useRef(0);

  const refresh = useCallback(async (quiet = false) => {
    const run = ++attempt.current;
    if (!quiet) setLoading(true);
    setError('');
    try {
      const next = await fetchWebSession();
      if (run !== attempt.current) return;
      setSession({
        account: next.authenticated ? (next.account ?? null) : null,
        csrf: next.authenticated ? (next.csrfToken ?? '') : '',
      });
    } catch (err) {
      if (run !== attempt.current) return;
      setError(err instanceof Error && err.message ? err.message : marketErrorMessage('auth_unavailable'));
    } finally {
      if (run === attempt.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();

    const onExpired = () => {
      attempt.current++;
      setSession({ account: null, csrf: '' });
      setLoading(false);
      setError('登录已过期，请重新登录。');
    };

    // 服务端会话 Cookie 24h 过期；切回页面时静默复核，避免展示过期登录态。
    let lastCheck = Date.now();
    const onFocus = () => {
      if (document.visibilityState !== 'visible') return;
      if (Date.now() - lastCheck < 15_000) return;
      lastCheck = Date.now();
      void refresh(true);
    };

    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    window.addEventListener('focus', onFocus);
    return () => {
      attempt.current++;
      window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
      window.removeEventListener('focus', onFocus);
    };
  }, [refresh]);

  const logout = useCallback(async () => {
    if (!session.csrf) {
      setSession({ account: null, csrf: '' });
      return;
    }
    setError('');
    try {
      await logoutWebSession(session.csrf);
      setSession({ account: null, csrf: '' });
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : marketErrorMessage('unavailable'));
      throw err;
    }
  }, [session.csrf]);

  const value = useMemo<AuthContextValue>(
    () => ({
      loading,
      account: session.account,
      csrf: session.csrf,
      error,
      refresh,
      logout,
    }),
    [loading, session, error, refresh, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error('AuthProvider is required');
  return context;
}
