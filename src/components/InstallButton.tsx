import { useEffect, useRef, useState } from 'react';
import type { PluginItem } from '../api/types';
import { buildInstallDeepLink } from '../lib/deeplink';

type Phase = 'idle' | 'launching' | 'launched' | 'fallback';

interface InstallButtonProps {
  item: PluginItem;
  variant: 'row' | 'block';
}

/**
 * 安装 = 唤起本机 GoNavi 客户端（gonavi://plugin/install）。
 * 2 秒内窗口没有失焦（浏览器没有弹出协议处理框）就判定未安装客户端，
 * 回退为复制插件 ID，用户在 GoNavi 的安装面板粘贴即可。
 */
export function InstallButton({ item, variant }: InstallButtonProps) {
  const [phase, setPhase] = useState<Phase>('idle');
  const [copied, setCopied] = useState(false);
  const timer = useRef<number>();

  useEffect(() => () => window.clearTimeout(timer.current), []);

  useEffect(() => {
    // 浏览器弹出“打开 GoNavi？”时会夺走焦点：视为唤起成功，撤回退计时。
    const markLaunched = () => {
      setPhase((current) => {
        if (current !== 'launching') return current;
        window.clearTimeout(timer.current);
        timer.current = window.setTimeout(() => setPhase('idle'), 4000);
        return 'launched';
      });
    };
    window.addEventListener('blur', markLaunched);
    document.addEventListener('visibilitychange', markLaunched);
    return () => {
      window.removeEventListener('blur', markLaunched);
      document.removeEventListener('visibilitychange', markLaunched);
    };
  }, []);

  const launch = () => {
    if (phase === 'launching') return;
    setCopied(false);
    setPhase('launching');
    window.location.href = buildInstallDeepLink(item);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      setPhase((current) => (current === 'launching' ? 'fallback' : current));
    }, 2000);
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(item.id);
    } catch {
      // 剪贴板不可用（非安全上下文等）时仅提示用户手动复制
    }
    setCopied(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      setCopied(false);
      setPhase('idle');
    }, 2000);
  };

  if (variant === 'block') {
    const hint =
      phase === 'fallback'
        ? '未检测到 GoNavi 客户端，复制插件 ID 后在 GoNavi 的「安装插件」面板粘贴。'
        : phase === 'launched'
          ? '已唤起 GoNavi，请在客户端确认安装。'
          : '将安装到本机 GoNavi，安装前仍需确认摘要';
    return (
      <div className="space-y-3">
        <button
          type="button"
          onClick={phase === 'fallback' ? copy : launch}
          className={
            phase === 'fallback'
              ? 'w-full py-3 rounded-lg bg-paper border border-hairline text-ink font-medium text-sm hover:border-copper hover:text-copper transition-all inline-flex items-center justify-center gap-1.5'
              : 'w-full py-3 rounded-lg bg-copper text-white font-medium text-sm shadow-sm hover:opacity-95 transition-all inline-flex items-center justify-center gap-1.5'
          }
        >
          {phase === 'launching' && <span className="material-symbols-outlined text-lg">rocket_launch</span>}
          {phase === 'fallback' && (copied ? '已复制插件 ID' : '复制插件 ID')}
          {phase === 'idle' && (
            <>
              <span className="material-symbols-outlined text-lg">download</span>
              安装插件
            </>
          )}
          {phase === 'launched' && '已在 GoNavi 中打开'}
        </button>
        <p className="text-center text-xs text-muted">{hint}</p>
      </div>
    );
  }

  const rowLabel =
    phase === 'launching'
      ? '唤起中…'
      : phase === 'launched'
        ? '已唤起'
        : phase === 'fallback'
          ? copied
            ? '已复制'
            : '复制 ID'
          : '安装';

  return (
    <button
      type="button"
      onClick={phase === 'fallback' ? copy : launch}
      disabled={phase === 'launching'}
      className="px-4 py-2 bg-paper border border-hairline text-ink font-medium text-xs rounded-lg hover:border-copper hover:text-copper disabled:opacity-60 transition-all"
    >
      {rowLabel}
    </button>
  );
}
