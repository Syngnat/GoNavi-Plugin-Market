import { Link } from 'react-router-dom';

const GITHUB_BASE = 'https://github.com/Syngnat/GoNavi';
const PLUGIN_DOCS_URL = `${GITHUB_BASE}/tree/dev/docs/plugins`;
const SDK_URL = `${GITHUB_BASE}/blob/dev/docs/plugins/sdk.md`;
const DEV_GUIDE_URL = `${GITHUB_BASE}/blob/dev/docs/plugins/development.md`;
const PUBLISHER_API_URL = `${GITHUB_BASE}/blob/dev/deploy/plugin-market/admin-api.md`;

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
    title: '登录',
    body: '在本网站直接使用 GitHub 登录。首次登录自动分配独立发布者命名空间，以「我的插件」页面显示值为准，无需客户端配对或手工申请。',
  },
  {
    title: '发布',
    body: '在官网登录 GitHub，进入「发布插件」选择 ZIP/TGZ 包，先静态预检再明确确认上架。服务器不执行插件代码；客户端不承担发布。',
  },
  {
    title: '上架',
    body: '校验通过后立即进入市场目录，无需等待人工审核。官方徽标仍由市场方单独认证；违规插件会被下架。',
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
          登录后以「我的插件」显示的命名空间为准，示例 ID 仅示意；首次提交可登记公开
          可直接填写 GitHub 仓库或具体 Release/Tag；单插件仓库会读取 latest stable Release，monorepo 请指定 Tag。
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

      <section className="flex flex-col md:flex-row items-center justify-between gap-4 bg-paper border border-hairline rounded-xl px-6 py-5">
        <p className="text-sm text-muted">需要协议细节、权限清单或故障排查？完整文档在主仓库。</p>
        <div className="flex flex-wrap items-center gap-4 text-sm">
          <Link to="/community" className="text-copper hover:underline">
            社区讨论
          </Link>
          <a href={SDK_URL} target="_blank" rel="noreferrer" className="text-copper hover:underline">
            SDK 指南
          </a>
          <a href={DEV_GUIDE_URL} target="_blank" rel="noreferrer" className="text-copper hover:underline">
            开发与打包
          </a>
          <a href={PUBLISHER_API_URL} target="_blank" rel="noreferrer" className="text-copper hover:underline">
            发布者 API
          </a>
          <a href={PLUGIN_DOCS_URL} target="_blank" rel="noreferrer" className="text-copper hover:underline">
            全部文档
          </a>
        </div>
      </section>
    </main>
  );
}
