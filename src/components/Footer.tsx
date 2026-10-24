const links = ['关于我们', '使用条款', '隐私政策', '联系支持'];

export function Footer() {
  return (
    <footer className="bg-charcoal mt-16">
      <div className="flex flex-col md:flex-row justify-between items-center w-full px-8 py-6 max-w-7xl mx-auto space-y-4 md:space-y-0">
        <div className="text-sm text-[#f6f1ea]/80">© 2026 GoNavi 插件市场. 保留所有权利.</div>
        <div className="flex items-center space-x-6 text-sm">
          {links.map((label) => (
            <a key={label} href="#" className="text-[#8a7e74] hover:text-[#c4622d] transition-colors">
              {label}
            </a>
          ))}
        </div>
      </div>
    </footer>
  );
}
