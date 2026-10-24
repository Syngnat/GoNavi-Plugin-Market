import { useEffect, useRef, useState } from 'react';

interface InstallButtonProps {
  pluginId: string;
  variant: 'row' | 'block';
  label?: string;
}

// 站点侧安装 = 复制插件 ID，用户在 GoNavi 客户端粘贴完成安装；
// 市场服务就绪后可替换为唤起客户端协议或直接下发 artifact。
export function InstallButton({ pluginId, variant, label }: InstallButtonProps) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number>();

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const onClick = async () => {
    try {
      await navigator.clipboard.writeText(pluginId);
    } catch {
      // 剪贴板不可用（非安全上下文等）时仅提示用户手动复制
    }
    setCopied(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setCopied(false), 2000);
  };

  const base =
    variant === 'block'
      ? 'w-full py-3 rounded-lg bg-copper text-white font-medium text-sm shadow-sm hover:opacity-95 transition-all inline-flex items-center justify-center gap-1.5'
      : 'px-4 py-2 bg-paper border border-hairline text-ink font-medium text-xs rounded-lg hover:border-copper hover:text-copper transition-all';

  return (
    <button type="button" onClick={onClick} className={base}>
      {copied ? (
        '已复制 ID'
      ) : (
        <>
          {variant === 'block' && <span className="material-symbols-outlined text-lg">download</span>}
          {label ?? '安装'}
        </>
      )}
    </button>
  );
}
