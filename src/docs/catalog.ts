export type DocGroup = '开始使用' | '贡献类型' | '接口与契约' | '发布与运营';

export interface DocCatalogItem {
  slug: string;
  title: string;
  description: string;
  group: DocGroup;
  source: string;
  order: number;
  featured?: boolean;
}

export const DOC_GROUPS: DocGroup[] = ['开始使用', '贡献类型', '接口与契约', '发布与运营'];

export const DOC_CATALOG: DocCatalogItem[] = [
  {
    slug: 'author-guide',
    title: '第三方插件作者指南',
    description: '从选择插件形态、实现、打包检查到 GitHub Release 上架与更新的完整路径。',
    group: '开始使用',
    source: 'docs/plugins/author-guide.md',
    order: 1,
    featured: true,
  },
  {
    slug: 'faq',
    title: '插件作者 FAQ',
    description: '文档入口、ID 与版本、打包、发布、安装验收和常见错误的集中答疑。',
    group: '开始使用',
    source: 'docs/plugins/faq.md',
    order: 2,
    featured: true,
  },
  {
    slug: 'development',
    title: '开发、打包与检查',
    description: '构建 hello 示例、生成本地插件包，并使用 GoNavi CLI 检查与安装。',
    group: '开始使用',
    source: 'docs/plugins/development.md',
    order: 3,
  },
  {
    slug: 'troubleshooting',
    title: '常见故障排查',
    description: '检查、安装与运行阶段可精确复现的问题和处理方式。',
    group: '开始使用',
    source: 'docs/plugins/troubleshooting.md',
    order: 4,
  },
  {
    slug: 'data-sources',
    title: '数据源插件',
    description: '以插件方式接入 SQL 连接、查询、元数据与显式写入事务。',
    group: '贡献类型',
    source: 'docs/plugins/data-sources.md',
    order: 10,
  },
  {
    slug: 'ai-providers',
    title: 'AI Provider',
    description: '实现原生流式模型供应商，支持文本、推理、工具调用与图像输入。',
    group: '贡献类型',
    source: 'docs/plugins/ai-providers.md',
    order: 11,
  },
  {
    slug: 'skills',
    title: 'Packaged Skills',
    description: '声明不可变的技能文档，供用户预览或由 Agent 显式加载。',
    group: '贡献类型',
    source: 'docs/plugins/skills.md',
    order: 12,
  },
  {
    slug: 'exporters',
    title: '查询结果导出器',
    description: '对已经物化的查询结果做一次有界渲染并交给宿主写入文件。',
    group: '贡献类型',
    source: 'docs/plugins/exporters.md',
    order: 13,
  },
  {
    slug: 'pages',
    title: '插件页面与宿主能力',
    description: '声明式或沙箱 HTML 页面、标题栏菜单和按权限分派的宿主能力。',
    group: '贡献类型',
    source: 'docs/plugins/pages.md',
    order: 14,
  },
  {
    slug: 'ai-tools',
    title: 'Agent 工具',
    description: '将插件命令显式暴露给 Agent，并保持审批、摘要与代次约束。',
    group: '贡献类型',
    source: 'docs/plugins/ai-tools.md',
    order: 15,
  },
  {
    slug: 'sdk',
    title: 'Go / TypeScript SDK',
    description: '协议 transport、清单类型、校验函数、数据源分发器与 UI 接入点。',
    group: '接口与契约',
    source: 'docs/plugins/sdk.md',
    order: 20,
  },
  {
    slug: 'protocol',
    title: '插件 JSON-RPC 协议',
    description: 'stdio 帧、生命周期、取消、错误码以及包身份和摘要的线协议。',
    group: '接口与契约',
    source: 'docs/plugins/protocol.md',
    order: 21,
  },
  {
    slug: 'permissions',
    title: '权限参考',
    description: '宿主实际强制的权限、授予范围和不授予的能力边界。',
    group: '接口与契约',
    source: 'docs/plugins/permissions.md',
    order: 22,
  },
  {
    slug: 'data-source-contract',
    title: '数据源契约',
    description: '数据源贡献点、调用路径、结果 envelope 和兼容边界的精确定义。',
    group: '接口与契约',
    source: 'docs/plugins/data-source-contract.md',
    order: 23,
  },
  {
    slug: 'ai-provider-contract',
    title: 'AI Provider 契约',
    description: '模型声明、凭据投影、流式事件、工具调用和失败语义的完整契约。',
    group: '接口与契约',
    source: 'docs/plugins/ai-provider-contract.md',
    order: 24,
  },
  {
    slug: 'skills-contract',
    title: 'Skills 契约',
    description: 'Packaged Skill 的元数据、文档读取、生命周期与撤销边界。',
    group: '接口与契约',
    source: 'docs/plugins/skills-contract.md',
    order: 25,
  },
  {
    slug: 'exporter-contract',
    title: '导出器契约',
    description: '物化结果导出 v2 的 envelope、分块、原子写入和失败处理。',
    group: '接口与契约',
    source: 'docs/plugins/exporter-contract.md',
    order: 26,
  },
  {
    slug: 'host-log',
    title: 'host.log',
    description: '插件进程向宿主发送有界诊断消息的权限和协议。',
    group: '接口与契约',
    source: 'docs/plugins/host-log.md',
    order: 27,
  },
  {
    slug: 'host-describe',
    title: 'host.describe',
    description: '读取当前插件进程身份、宿主版本和 locale 的受限能力。',
    group: '接口与契约',
    source: 'docs/plugins/host-describe.md',
    order: 28,
  },
  {
    slug: 'lifecycle',
    title: '生命周期通知',
    description: '初始化完成后宿主向插件发送 running 通知的协议。',
    group: '接口与契约',
    source: 'docs/plugins/lifecycle.md',
    order: 29,
  },
  {
    slug: 'market-sources',
    title: '市场来源与聚合浏览',
    description: '官方与自定义市场来源、匿名浏览、分页缓存和来源隔离规则。',
    group: '发布与运营',
    source: 'docs/plugins/market-sources.md',
    order: 30,
  },
  {
    slug: 'market-deployment',
    title: '市场部署与运维',
    description: '生产站点、市场服务、PostgreSQL、工件存储与发布验收。',
    group: '发布与运营',
    source: 'docs/market-deployment.md',
    order: 31,
  },
  {
    slug: 'market-api',
    title: '发布者与管理员 API',
    description: 'GitHub 登录、Release 发布、自动更新来源、审核、下架与评分接口。',
    group: '发布与运营',
    source: 'docs/market-admin-api.md',
    order: 32,
  },
  {
    slug: 'implementation-contract',
    title: '插件生态实现契约',
    description: '宿主、SDK、市场服务与站点之间的共享 DTO、所有权和验收边界。',
    group: '发布与运营',
    source: 'docs/plugins/implementation-contract.md',
    order: 33,
  },
  {
    slug: 'market-auth',
    title: '插件市场认证方向',
    description: '桌面匿名只读、网站 GitHub 登录与运维 Bearer Token 的职责边界。',
    group: '发布与运营',
    source: 'docs/plugins/market-auth.md',
    order: 34,
  },
  {
    slug: 'v2-api',
    title: '市场 v2 账号与发布契约',
    description: '网站登录、发布预检和作者工作台的接口约定与实现边界。',
    group: '发布与运营',
    source: 'docs/v2-api.md',
    order: 35,
  },
];

export const DOC_BY_SLUG = new Map(DOC_CATALOG.map((doc) => [doc.slug, doc]));

/** Every rendered title is also a stable in-page anchor for the sidebar. */
export function slugifyHeading(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[`*_~[\]()]/g, '')
    .replace(/[^\p{L}\p{N}]+/gu, '-')
    .replace(/^-+|-+$/g, '');
}

/** Internal docs keep the same route vocabulary while source links stay portable. */
export function resolveDocHref(
  current: DocCatalogItem,
  href: string,
): { href: string; external: boolean } | null {
  if (href.startsWith('#')) return { href, external: false };
  if (href === '/docs' || href.startsWith('/docs/')) return { href, external: false };
  if (/^(https?:|mailto:)/i.test(href)) return { href, external: true };

  const [rawTarget, ...hashParts] = href.split('#');
  const rawHash = hashParts.join('#');
  if (!rawTarget) return null;
  const cleanTarget = rawTarget.replace(/^<|>$/g, '');
  const target = cleanTarget
    .replace(/^\.\//, '')
    .replace(/^\.\.\/\.\.\/\.\.\//, '')
    .replace(/^\.\.\/\.\.\//, '')
    .replace(/^\.\.\//, '');
  const hash = rawHash ? `#${rawHash}` : '';

  const bySource = DOC_CATALOG.find((doc) => {
    const source = doc.source.replace(/^docs\//, '');
    return source === target || `plugins/${source}` === target || `docs/${source}` === rawTarget;
  });
  if (bySource) return { href: `/docs/${bySource.slug}${hash}`, external: false };

  if (target.endsWith('.md')) {
    const source = target.replace(/^docs\/plugins\//, 'plugins/');
    const byLocalSource = DOC_CATALOG.find((doc) => doc.source === source || doc.source.endsWith(`/${source}`));
    if (byLocalSource) return { href: `/docs/${byLocalSource.slug}${hash}`, external: false };
  }

  const repoPath = cleanTarget.replace(/^\.\//, '');
  if (
    repoPath.startsWith('../../') ||
    /^([^/]+\/)*[^/]+\.(go|ts|tsx|js|json|ya?ml|yml|toml|sh)$/.test(repoPath)
  ) {
    const normalized = repoPath.replace(/^(\.\.\/)+/, '');
    return {
      href: `https://github.com/Syngnat/GoNavi/blob/dev/${normalized}`,
      external: true,
    };
  }
  return { href: current.source, external: false };
}
