import { Link } from 'react-router-dom';

const DISCUSSIONS_URL = 'https://github.com/Syngnat/GoNavi/discussions/1582';
const MARKET_REPO_URL = 'https://github.com/Syngnat/GoNavi-Plugin-Market';

const WAYS = [
  {
    icon: 'forum',
    title: '提问与答疑',
    body: '插件开发、安装、配置遇到问题，或想评估一个想法是否适合做成插件，都可以发起讨论。',
    action: '去发起讨论',
  },
  {
    icon: 'rocket_launch',
    title: '分享你的插件',
    body: '完成了打包与本地校验？在这里介绍你的插件、征集反馈，再走市场上架流程。',
    action: '分享插件',
  },
  {
    icon: 'flag',
    title: '报告问题',
    body: '发现市场条目描述与实际行为不符、权限声明可疑，或想举报违规插件，请附证据发帖。',
    action: '报告问题',
  },
];

export function CommunityPage() {
  return (
    <main className="flex-grow max-w-7xl w-full mx-auto px-6 py-10 space-y-12">
      <section className="bg-card p-8 rounded-xl border border-hairline shadow-sm space-y-4">
        <div className="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-paper border border-hairline text-copper text-xs font-medium">
          <span className="material-symbols-outlined text-sm fill-icon">groups</span>
          <span>社区</span>
        </div>
        <h1 className="text-3xl font-headline font-bold text-ink">GoNavi 插件生态社区</h1>
        <p className="text-muted leading-relaxed max-w-3xl">
          社区讨论托管在 GitHub Discussions，围绕插件生态展开：开发答疑、成果分享、问题上报。
          市场内的评分与举报功能需要先登录；更正式的提案和插件上架申请也请在这里发起。
        </p>
        <a
          href={DISCUSSIONS_URL}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center space-x-2 px-5 py-2.5 rounded-lg bg-copper text-white font-medium text-sm shadow-sm hover:opacity-95 transition-all"
        >
          <span>进入 GitHub Discussions</span>
          <span className="material-symbols-outlined text-base fill-icon">open_in_new</span>
        </a>
      </section>

      <section className="space-y-6">
        <h2 className="text-xl font-headline font-bold text-ink">参与方式</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
          {WAYS.map((way) => (
            <div key={way.title} className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-3 flex flex-col">
              <span className="material-symbols-outlined text-copper fill-icon">{way.icon}</span>
              <h3 className="font-headline font-bold text-ink">{way.title}</h3>
              <p className="text-sm text-muted leading-relaxed flex-grow">{way.body}</p>
              <a
                href={DISCUSSIONS_URL}
                target="_blank"
                rel="noreferrer"
                className="text-copper text-sm hover:underline"
              >
                {way.action} →
              </a>
            </div>
          ))}
        </div>
      </section>

      <section className="grid grid-cols-1 md:grid-cols-2 gap-5">
        <div className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-3">
          <h3 className="font-headline font-bold text-ink">想给 GoNavi 写插件？</h3>
          <p className="text-sm text-muted leading-relaxed">
            从贡献点类型、SDK 快速开始到市场发布流程，都在开发者文档里。
          </p>
          <Link to="/developers" className="text-copper text-sm hover:underline">
            查看开发者文档 →
          </Link>
        </div>
        <div className="bg-card p-6 rounded-xl border border-hairline shadow-sm space-y-3">
          <h3 className="font-headline font-bold text-ink">官方插件与市场源码</h3>
          <p className="text-sm text-muted leading-relaxed">
            官方插件源码、上架目录与本站都在 GoNavi-Plugin-Market 仓库，欢迎提 Issue 与 PR。
          </p>
          <a href={MARKET_REPO_URL} target="_blank" rel="noreferrer" className="text-copper text-sm hover:underline">
            访问市场仓库 →
          </a>
        </div>
      </section>
    </main>
  );
}
