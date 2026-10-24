import { useState, type FormEvent } from 'react';
import { isDemoMode, verifyDeviceCode } from '../api/client';

export function DeviceLoginPage() {
  const [code, setCode] = useState('');
  const [state, setState] = useState<'idle' | 'submitting' | 'done' | 'error'>('idle');

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    const userCode = code.trim().toUpperCase();
    if (!userCode || state === 'submitting') return;
    setState('submitting');
    verifyDeviceCode(userCode)
      .then(() => setState('done'))
      .catch(() => setState('error'));
  };

  return (
    <main className="flex-grow flex items-center justify-center px-6 py-16">
      <div className="w-full max-w-md bg-card rounded-xl border border-hairline shadow-sm p-8 space-y-5">
        <div className="space-y-2 text-center">
          <div className="w-12 h-12 mx-auto rounded-lg bg-copper text-white flex items-center justify-center font-headline font-bold text-lg shadow-sm">
            GN
          </div>
          <h1 className="text-2xl font-headline font-bold tracking-tight pt-2">登录插件市场</h1>
          <p className="text-muted text-sm leading-relaxed">
            在 GoNavi 客户端的「插件市场 → 登录」弹窗中查看配对码，在此输入完成绑定。
          </p>
        </div>
        {isDemoMode && (
          <div className="text-xs text-muted bg-paper border border-hairline rounded-lg px-3 py-2 text-center">
            演示模式：市场服务尚未接入，输入不会产生真实绑定。
          </div>
        )}
        <form onSubmit={onSubmit} className="space-y-4">
          <input
            value={code}
            onChange={(event) => setCode(event.target.value.toUpperCase())}
            className="w-full px-4 py-3 bg-paper border border-hairline rounded-lg text-center font-mono text-lg tracking-[0.3em] placeholder-muted focus:outline-none focus:ring-1 focus:ring-copper focus:border-copper"
            placeholder="配对码"
            maxLength={12}
            aria-label="配对码"
          />
          <button
            type="submit"
            disabled={state === 'submitting' || !code.trim()}
            className="w-full py-3 rounded-lg bg-copper text-white font-medium text-sm shadow-sm hover:opacity-95 disabled:opacity-50 transition-all"
          >
            {state === 'submitting' ? '提交中…' : '连接账户'}
          </button>
        </form>
        {state === 'done' && (
          <p className="text-sm text-center text-ink">已提交，回到 GoNavi 客户端确认即可完成登录。</p>
        )}
        {state === 'error' && (
          <p className="text-sm text-center text-muted">提交失败，请核对配对码后重试。</p>
        )}
      </div>
    </main>
  );
}
