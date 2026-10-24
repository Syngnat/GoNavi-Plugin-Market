import type { PluginItem } from '../api/types';

/**
 * 构建 gonavi:// 深链，把安装请求交给本机 GoNavi 客户端。
 *
 * 契约（与客户端 internal/deeplink 的解析一致）：
 *   gonavi://plugin/install?spec=<github:owner/repo 或 npm 包名>&id=<插件ID>&version=<版本>
 * - spec：可选。客户端会预填安装面板并自动执行检查（仍需用户确认摘要后才安装）。
 * - id：市场插件 ID。市场服务接入前客户端只预填输入框，由用户手动触发检查。
 *
 * 链接只把用户送到客户端的安装面板，真正的下载、静态检查与摘要确认都发生在客户端。
 */
export function buildInstallDeepLink(item: PluginItem): string {
  const params = new URLSearchParams();
  if (item.installSpec && item.installSpec.trim()) params.set('spec', item.installSpec.trim());
  params.set('id', item.id);
  if (item.version) params.set('version', item.version);
  return `gonavi://plugin/install?${params.toString()}`;
}
