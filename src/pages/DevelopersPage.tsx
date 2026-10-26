import { Link } from 'react-router-dom';

const DOC_LINKS = [
  { label: '作者完整指南', to: '/docs/author-guide' },
  { label: '作者 FAQ', to: '/docs/faq' },
  { label: 'SDK 指南', to: '/docs/sdk' },
  { label: '开发与打包', to: '/docs/development' },
  { label: '发布者 API', to: '/docs/market-api' },
  { label: '全部文档', to: '/docs' },
];

const CONTRIBUTIONS = [
  {
    icon: 'terminal',
    title: '命令',
    body: '带 JSON Schema 输入与读/写效果声明的可调用命令，是插件能力的基础单元。',
  },
  {
    icon: 'dashboard',
    title: '面板',
    body: '宿主渲染的声明式面板，或带独立窗口的完整 Web UI（如 Nginx 诊断助手）。',
  },
  {
    icon: 'psychology',
    title: 'AI 技能',
    body: '打包技能文档与参考资料，供 GoNavi Agent 按需加载；加载不等于授权执行。',
  },
  {
    icon: 'database',
    title: '数据源',
    body: '为工作台注册新的数据库类型，出现在连接管理里，与内置驱动同等使用。',
  },
  {
    icon: 'output',
    title: '导出器',
    body: '为查询结果集提供新的导出格式，出现在结果面板的导出菜单中。',
  },
  {
    icon: 'smart_toy',
    title: 'AI 供应商',
    body: '接入新的 LLM 服务端点，配置后即可在 AI 助手中选用。',
  },
];

const STEPS = [
  {
    title: '打包',
    body: 'ZIP/TGZ 根目录包含 gonavi.plugin.json；用示例打包器或你自己的脚本生成产物，先在本地用 GoNavi CLI 校验。',
  },
  {
    title: '选择仓库',
    body: '单插件仓库可直接填仓库根，市场读取 latest stable Release；一仓多插件必须填具体 Release/Tag，避免发布错插件。',
  },
  {
    title: '登录并预检',
    body: '在本网站用 GitHub 登录，进入「发布插件」粘贴 Release 链接。市场读取真实归档并展示 ID、版本、权限和 SHA-256。',
  },
  {
    title: '上架',
    body: '确认摘要后自助上架，无需等待人工审核。上传 ZIP/TGZ 仅作离线或 Release 不可用时的兜底。',
  },
];

const SECURITY_NOTES = [
  '插件以你的操作系统账户权限运行，进程隔离保护的是宿主响应性——安装前请审阅源码与权限清单。',
  '权限在 manifest 中显式声明，安装预览逐项展示；AI 技能加载与 Agent 工具调用均不隐含授权。',
  '市场采用自助上架 + 事后监管：每个版本做完整静态校验并记录审计，官方徽标单独认证，违规插件会被下架（yank）且已发布版本字节不可替换。',
];

export function DevelopersPage() {
  return (
    <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-10 space-y-14">
      <section className="bg-card p-8 rounded-xl border border-hairline shadow-sm space-y-4">
        <div className="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-paper border border-hairline text-copper text-xs font-medium">
          <span className="material-symbols-outlined text-sm fill-icon">code</span>
          <span>开发者文档</span>
        </div>
        <h1 className="text-3xl font-headline font-bold text-ink">为 GoNavi 开发插件</h1>
        <p className="text-muted leading-relaxed max-w-3xl">
          插件是进程隔离的第三方代码，通过标准输入输出与宿主通信：Go 后端编译为原生可执行文件，
          在 manifest 中声明贡献点与权限。安装前用户会看到完整的权限预览，市场审核只做静态校验、
          从不执行插件代码。
        </p>
      </section>

      <section className="space-y-6">
        <h2 className="text-xl font-headline font-bold text-ink">插件能贡献什么</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {CONTRIBUTIONS.map((item) => (
            <div key={item.title} className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-3">
              <span className="material-symbols-outlined text-copper fill-icon">{item.icon}</span>
              <h3 className="font-headline font-bold text-ink">{item.title}</h3>
              <p className="text-sm text-muted leading-relaxed">{item.body}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-6">
        <h2 className="text-xl font-headline font-bold text-ink">快速开始</h2>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <div className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-4">
            <h3 className="font-headline font-bold text-ink">1. 用 SDK 写第一个插件</h3>
            <p className="text-sm text-muted leading-relaxed">
              Go SDK 与示例在主仓库 <code className="font-mono text-copper">sdk/go</code> 与{' '}
              <code className="font-mono text-copper">examples/plugins</code>，从可取消的 hello
              命令开始，逐步添加面板与 AI 能力。
            </p>
            <pre className="bg-paper border border-hairline rounded-lg px-4 py-3 text-xs font-mono text-ink overflow-x-auto">
              {`import plugin "github.com/Syngnat/GoNavi/sdk/go"

server := plugin.NewServer(plugin.Options{
    Handlers: map[string]plugin.Handler{
        "hello": hello, // func(ctx, args) (any, error)
    },
})
server.ServeStdio(ctx)`}
            </pre>
          </div>
          <div className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-4">
            <h3 className="font-headline font-bold text-ink">2. 打包并本地校验</h3>
            <p className="text-sm text-muted leading-relaxed">
              示例打包器编译原生二进制并生成最终 manifest；用 GoNavi CLI
              静态校验打包产物（永远是产物目录/压缩包，不是源码树）。
            </p>
            <pre className="bg-paper border border-hairline rounded-lg px-4 py-3 text-xs font-mono text-ink overflow-x-auto">
              {`# 打包 hello 示例（在主仓库根目录）
go -C examples/plugins run ./package -output $package

# 静态校验，不执行任何插件代码
gonavi plugin --root $root check $package
gonavi plugin --root $root inspect $package`}
            </pre>
          </div>
        </div>
      </section>

      <section className="space-y-6">
        <h2 className="text-xl font-headline font-bold text-ink">发布到市场</h2>
        <p className="text-sm text-muted leading-relaxed max-w-3xl">
          登录后以「我的插件」显示的命名空间为准，示例 ID 仅示意。优先填写 GitHub 仓库或具体 Release/Tag；
          单插件仓库会读取 latest stable Release，monorepo 必须指定 Tag。上传 ZIP/TGZ 仅作兜底。
        </p>
        <Link
          to="/publish"
          className="inline-flex px-4 py-2 rounded-lg bg-charcoal text-card text-sm font-medium hover:opacity-95 transition-all"
        >
          前往站内发布
        </Link>
        <ol className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-5">
          {STEPS.map((step, index) => (
            <li key={step.title} className="bg-card p-5 rounded-xl border border-hairline shadow-sm space-y-2">
              <div className="w-7 h-7 rounded-full bg-copper text-white flex items-center justify-center font-bold text-sm">
                {index + 1}
              </div>
              <h3 className="font-headline font-bold text-ink text-sm">{step.title}</h3>
              <p className="text-xs text-muted leading-relaxed">{step.body}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="bg-card p-8 rounded-xl border border-hairline shadow-sm space-y-4">
        <h2 className="text-xl font-headline font-bold text-ink">安全模型</h2>
        <ul className="text-sm text-muted leading-relaxed space-y-2 list-disc list-inside max-w-4xl">
          {SECURITY_NOTES.map((note) => (
            <li key={note}>{note}</li>
          ))}
        </ul>
      </section>

      <section className="bg-charcoal rounded-xl px-7 py-8 text-card shadow-sm relative overflow-hidden">
        <div className="absolute top-0 right-0 w-56 h-56 border border-copper/25 rounded-full -translate-y-1/3 translate-x-1/3 pointer-events-none" />
        <div className="relative flex flex-col lg:flex-row lg:items-end justify-between gap-7">
          <div className="space-y-3 max-w-xl">
            <p className="font-mono text-[10px] tracking-[0.14em] text-copper uppercase">One source of truth</p>
            <h2 className="text-2xl font-headline font-semibold tracking-tight text-card">插件文档就在这里</h2>
            <p className="text-sm text-[#d8cabb] leading-relaxed">
              作者指南、FAQ、SDK、协议、权限和发布契约统一在市场站内维护。涉及源码的链接会明确跳转到
              GoNavi 主仓，不再把整篇文档带到 GitHub。
            </p>
          </div>
          <Link
            to="/docs"
            className="inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-lg bg-copper text-white text-sm font-medium hover:bg-[#d37442] transition-colors shrink-0"
          >
            打开文档中心
            <span className="material-symbols-outlined text-base">arrow_forward</span>
          </Link>
        </div>
        <div className="relative flex flex-wrap gap-x-6 gap-y-2 mt-7 pt-5 border-t border-white/10 text-sm">
          {DOC_LINKS.map((link) => (
            <Link key={link.to} to={link.to} className="text-[#c9b7a5] hover:text-copper transition-colors">
              {link.label}
            </Link>
          ))}
          <Link to="/community" className="text-[#c9b7a5] hover:text-copper transition-colors">
            社区讨论
          </Link>
        </div>
      </section>
    </main>
  );
}
