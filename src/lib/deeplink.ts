import type { PluginItem } from '../api/types';

/**
 * 从市场公开条目里的 GitHub 仓库地址推导客户端安装源。
 *
 * 客户端按 `github:owner/repo[@tag]` 解析，不能直接喂 `https://github.com/...`
 * （会被当成 npm 包名）。因此这里只取前两段路径：`/releases`、`/tree/...`、
 * `/blob/...` 等尾巴一律忽略，`.git` 后缀去掉；非 github.com 或非 https 返回空串。
 */
export function githubSpecFromSourceUrl(sourceUrl: string | undefined): string {
  const trimmed = sourceUrl?.trim();
  if (!trimmed) return '';
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return '';
  }
  if (url.protocol !== 'https:' || url.hostname !== 'github.com') return '';
  const segments = url.pathname.split('/').filter(Boolean);
  if (segments.length < 2) return '';
  const owner = segments[0];
  const repo = segments[1].replace(/\.git$/i, '');
  if (!owner || !repo) return '';
  return `github:${owner}/${repo}`;
}

/** 安装源：显式 installSpec 优先，其次由公开 sourceUrl 推导；都没有就不传 spec。 */
export function resolveInstallSpec(item: PluginItem): string {
  const explicit = item.installSpec?.trim();
  if (explicit) return explicit;
  return githubSpecFromSourceUrl(item.sourceUrl);
}

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
  const spec = resolveInstallSpec(item);
  if (spec) params.set('spec', spec);
  params.set('id', item.id);
  if (item.version) params.set('version', item.version);
  return `gonavi://plugin/install?${params.toString()}`;
}
