const GITHUB_BASE = 'https://github.com/Syngnat/GoNavi';
const PLUGIN_DOCS_URL = `${GITHUB_BASE}/tree/dev/docs/plugins`;
const COMMUNITY_URL = `${GITHUB_BASE}/discussions/1582`;

export function Footer() {
  return (
    <footer className="bg-charcoal mt-16">
      <div className="flex flex-col md:flex-row justify-between items-center w-full px-8 py-6 max-w-7xl mx-auto space-y-4 md:space-y-0">
        <div className="text-sm text-[#f6f1ea]/80">© 2026 GoNavi 插件市场. 保留所有权利.</div>
        <div className="flex items-center space-x-6 text-sm">
          <a
            href={PLUGIN_DOCS_URL}
            target="_blank"
            rel="noreferrer"
            className="text-[#8a7e74] hover:text-[#c4622d] transition-colors"
          >
            插件文档
          </a>
          <a
            href={COMMUNITY_URL}
            target="_blank"
            rel="noreferrer"
            className="text-[#8a7e74] hover:text-[#c4622d] transition-colors"
          >
            社区讨论
          </a>
          <a
            href="https://github.com/Syngnat/GoNavi-Plugin-Market"
            target="_blank"
            rel="noreferrer"
            className="text-[#8a7e74] hover:text-[#c4622d] transition-colors"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  );
}
