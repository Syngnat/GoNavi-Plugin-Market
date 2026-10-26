import { useEffect, useMemo, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Link, useParams } from 'react-router-dom';
import {
  DOC_BY_SLUG,
  DOC_CATALOG,
  DOC_GROUPS,
  resolveDocHref,
  slugifyHeading,
  type DocCatalogItem,
} from './catalog';

interface Heading {
  id: string;
  text: string;
  level: 2 | 3;
}

function textOf(children: ReactNode): string {
  if (typeof children === 'string' || typeof children === 'number') return String(children);
  if (Array.isArray(children)) return children.map(textOf).join('');
  if (children && typeof children === 'object' && 'props' in children) {
    return textOf((children as { props?: { children?: ReactNode } }).props?.children);
  }
  return '';
}

function headingsOf(markdown: string): Heading[] {
  const headings: Heading[] = [];
  let fenced = false;
  for (const line of markdown.split('\n')) {
    if (/^\s*```/.test(line)) {
      fenced = !fenced;
      continue;
    }
    if (fenced) continue;
    const match = /^(##|###)\s+(.+?)\s*#*\s*$/.exec(line);
    if (!match) continue;
    const text = match[2].replace(/`|\*\*|__|\[|\]\([^)]*\)/g, '');
    headings.push({ id: slugifyHeading(text), text, level: match[1] === '##' ? 2 : 3 });
  }
  return headings;
}

function isExternal(href: string): boolean {
  return /^(https?:|mailto:)/i.test(href);
}

function MarkdownLink({
  current,
  href,
  children,
}: {
  current: DocCatalogItem;
  href?: string;
  children?: ReactNode;
}) {
  if (!href) return <span>{children}</span>;
  const resolved = resolveDocHref(current, href);
  if (!resolved) return <span>{children}</span>;
  if (resolved.external || isExternal(resolved.href)) {
    return (
      <a href={resolved.href} target="_blank" rel="noreferrer">
        {children}
        <span className="material-symbols-outlined ml-1 align-[-2px] text-[13px]">open_in_new</span>
      </a>
    );
  }
  if (resolved.href.startsWith('/')) {
    return <Link to={resolved.href}>{children}</Link>;
  }
  return <a href={resolved.href}>{children}</a>;
}

function documentTitle(markdown: string): string {
  const first = markdown.split('\n').find((line) => /^#\s+/.test(line));
  return first ? first.replace(/^#\s+/, '').trim() : '';
}

export function DocPage() {
  const { slug = '' } = useParams();
  const doc = DOC_BY_SLUG.get(slug);
  const [markdown, setMarkdown] = useState('');
  const [state, setState] = useState<'loading' | 'ready' | 'missing'>('loading');
  const [query, setQuery] = useState('');
  const [activeHeading, setActiveHeading] = useState('');
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    if (!doc) {
      setState('missing');
      return;
    }
    let alive = true;
    setState('loading');
    setMarkdown('');
    fetch(`/docs/${doc.slug}.md`)
      .then((response) => {
        if (!response.ok) throw new Error(`docs request failed: ${response.status}`);
        return response.text();
      })
      .then((text) => {
        if (!alive) return;
        setMarkdown(text);
        setState('ready');
      })
      .catch(() => {
        if (alive) setState('missing');
      });
    return () => {
      alive = false;
    };
  }, [doc]);

  useEffect(() => {
    if (state !== 'ready') return;
    const hash = window.location.hash.slice(1);
    const target = hash ? document.getElementById(decodeURIComponent(hash)) : null;
    requestAnimationFrame(() => {
      if (target) target.scrollIntoView({ block: 'start' });
      else window.scrollTo(0, 0);
    });
  }, [state, slug]);

  useEffect(() => {
    const onScroll = () => {
      const article = document.getElementById('doc-article');
      if (!article) return;
      const start = article.offsetTop;
      const length = Math.max(1, article.scrollHeight - window.innerHeight + 80);
      setProgress(Math.min(1, Math.max(0, (window.scrollY - start + 80) / length)));
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, [state]);

  const headings = useMemo(() => headingsOf(markdown), [markdown]);
  const filteredDocs = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return DOC_CATALOG;
    return DOC_CATALOG.filter((entry) =>
      [entry.title, entry.description, entry.slug].some((value) => value.toLowerCase().includes(q)),
    );
  }, [query]);

  useEffect(() => {
    if (headings.length === 0) return;
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (visible?.target.id) setActiveHeading(visible.target.id);
      },
      { rootMargin: '-100px 0px -70% 0px', threshold: [0, 1] },
    );
    headings.forEach((heading) => {
      const target = document.getElementById(heading.id);
      if (target) observer.observe(target);
    });
    return () => observer.disconnect();
  }, [headings, state]);

  const components: ComponentPropsWithoutRef<typeof ReactMarkdown>['components'] = {
    h1: ({ children }) => <h1 className="doc-h1">{children}</h1>,
    h2: ({ children }) => {
      const text = textOf(children);
      return (
        <h2 id={slugifyHeading(text)} className="doc-h2">
          {children}
          <a href={`#${slugifyHeading(text)}`} aria-label={`链接到 ${text}`}>
            #
          </a>
        </h2>
      );
    },
    h3: ({ children }) => {
      const text = textOf(children);
      return (
        <h3 id={slugifyHeading(text)} className="doc-h3">
          {children}
          <a href={`#${slugifyHeading(text)}`} aria-label={`链接到 ${text}`}>
            #
          </a>
        </h3>
      );
    },
    a: ({ href, children }) => (doc ? <MarkdownLink current={doc} href={href}>{children}</MarkdownLink> : <span>{children}</span>),
    table: ({ children }) => (
      <div className="doc-table-wrap">
        <table>{children}</table>
      </div>
    ),
    pre: ({ children }) => <pre className="doc-pre">{children}</pre>,
    code: ({ className, children }) => {
      const fenced = Boolean(className?.startsWith('language-'));
      return fenced ? <code className={className}>{children}</code> : <code className="doc-inline-code">{children}</code>;
    },
  };

  if (state === 'missing' || !doc) {
    return (
      <main className="flex-grow max-w-5xl w-full mx-auto px-6 py-20">
        <div className="bg-card border border-hairline rounded-xl p-10 text-center space-y-4">
          <span className="material-symbols-outlined text-4xl text-copper">menu_book</span>
          <h1 className="text-2xl font-headline font-bold">没有找到这篇文档</h1>
          <p className="text-sm text-muted">文档地址可能已更新，请从文档中心重新选择。</p>
          <Link to="/docs" className="inline-flex text-sm text-copper hover:underline">返回文档中心</Link>
        </div>
      </main>
    );
  }

  return (
    <main className="docs-page flex-grow w-full">
      <div className="docs-progress" style={{ transform: `scaleX(${progress})` }} />
      <div className="max-w-[1500px] mx-auto px-5 lg:px-8 py-8 lg:py-10">
        <div className="docs-shell">
          <aside className="docs-sidebar">
            <div className="docs-sidebar-head">
              <Link to="/docs" className="docs-brand">
                <span className="docs-brand-mark">GN</span>
                <span><strong>插件文档</strong><small>DOCS / V1</small></span>
              </Link>
              <label className="docs-search">
                <span className="material-symbols-outlined">search</span>
                <input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索文档"
                  aria-label="搜索文档"
                />
                <kbd>/</kbd>
              </label>
            </div>
            <nav aria-label="插件文档目录" className="docs-nav">
              {DOC_GROUPS.map((group) => {
                const entries = filteredDocs.filter((entry) => entry.group === group);
                if (entries.length === 0) return null;
                return (
                  <div className="docs-nav-group" key={group}>
                    <p>{group}</p>
                    {entries.map((entry) => (
                      <Link
                        key={entry.slug}
                        to={`/docs/${entry.slug}`}
                        className={entry.slug === doc.slug ? 'is-active' : ''}
                      >
                        <span>{entry.title}</span>
                        {entry.slug === doc.slug && <i />}
                      </Link>
                    ))}
                  </div>
                );
              })}
              {filteredDocs.length === 0 && <p className="docs-nav-empty">没有匹配的文档</p>}
            </nav>
            <div className="docs-sidebar-foot">
              <span className="docs-live-dot" /> 文档随市场版本发布
              <Link to="/developers">返回开发者概览 <span className="material-symbols-outlined">arrow_forward</span></Link>
            </div>
          </aside>

          <article className="docs-article" id="doc-article">
            <header className="docs-article-head">
              <div className="docs-breadcrumb">
                <Link to="/">插件市场</Link><span>/</span><Link to="/docs">文档</Link><span>/</span><span>{doc.group}</span>
              </div>
              <div className="docs-kicker">{doc.group} / {String(DOC_CATALOG.indexOf(doc) + 1).padStart(2, '0')}</div>
              <h1>{documentTitle(markdown) || doc.title}</h1>
              <p>{doc.description}</p>
              <div className="docs-meta">
                <span><i /> 站内规范来源</span>
                <span>Markdown</span>
                <a href={`https://github.com/Syngnat/GoNavi-Plugin-Market/blob/main/${doc.source}`} target="_blank" rel="noreferrer">
                  查看源码 <span className="material-symbols-outlined">open_in_new</span>
                </a>
              </div>
            </header>

            <div className="docs-content">
              {state === 'loading' ? (
                <div className="docs-loading"><span />加载文档中…</div>
              ) : (
                <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
                  {markdown}
                </ReactMarkdown>
              )}
            </div>

            <footer className="docs-article-foot">
              <div>
                <p>这篇文档有帮助吗？</p>
                <span>插件文档只在市场仓库维护，网站内容与部署版本同步。</span>
              </div>
              <div className="flex gap-2">
                <a href={`mailto:?subject=${encodeURIComponent(doc.title)}&body=${encodeURIComponent(window.location.href)}`}>分享</a>
                <a href={`https://github.com/Syngnat/GoNavi-Plugin-Market/blob/main/${doc.source}`} target="_blank" rel="noreferrer">纠错</a>
              </div>
            </footer>
          </article>

          <aside className="docs-toc" aria-label="本页目录">
            <p>ON THIS PAGE</p>
            {headings.length > 0 ? (
              <nav>
                {headings.map((heading) => (
                  <a
                    key={`${heading.id}-${heading.level}`}
                    href={`#${heading.id}`}
                    className={`${heading.level === 3 ? 'is-sub' : ''} ${activeHeading === heading.id ? 'is-active' : ''}`}
                    onClick={() => setActiveHeading(heading.id)}
                  >
                    {heading.text}
                  </a>
                ))}
              </nav>
            ) : (
              <span className="docs-toc-empty">短文档，无页内目录</span>
            )}
            <div className="docs-toc-help">
              <span className="material-symbols-outlined">lightbulb</span>
              <p>第一次开发？先看作者指南，再按贡献类型选择专项契约。</p>
              <Link to="/docs/author-guide">打开指南</Link>
            </div>
          </aside>
        </div>
      </div>
    </main>
  );
}

export function DocsIndexPage() {
  const [query, setQuery] = useState('');
  const filtered = DOC_CATALOG.filter((doc) => {
    const q = query.trim().toLowerCase();
    return !q || [doc.title, doc.description, doc.group].some((value) => value.toLowerCase().includes(q));
  });

  return (
    <main className="docs-index flex-grow w-full">
      <section className="docs-index-hero">
        <div className="docs-grid-bg" />
        <div className="relative max-w-7xl mx-auto px-6 py-16 lg:py-24">
          <div className="docs-index-kicker"><i /> GONAVI / PLUGIN DOCUMENTATION</div>
          <div className="grid lg:grid-cols-[1fr_390px] gap-10 items-end">
            <div>
              <h1>写一个插件，<br /><span>从这里走到上架。</span></h1>
              <p>
                面向第三方作者的完整文档。先理解插件边界，再选择贡献类型、构建归档、发布 GitHub Release，
                最后在市场完成安装验收。
              </p>
              <div className="docs-index-actions">
                <Link to="/docs/author-guide" className="docs-primary">从作者指南开始 <span className="material-symbols-outlined">arrow_forward</span></Link>
                <Link to="/publish" className="docs-secondary">发布插件</Link>
              </div>
            </div>
            <div className="docs-index-stats">
              <div><strong>{String(DOC_CATALOG.length).padStart(2, '0')}</strong><span>篇站内文档</span></div>
              <div><strong>01</strong><span>唯一规范来源</span></div>
              <div><strong>V1</strong><span>插件协议版本</span></div>
            </div>
          </div>
        </div>
      </section>

      <section className="max-w-7xl mx-auto px-6 py-12">
        <div className="docs-index-toolbar">
          <div>
            <p className="docs-section-label">DOCUMENT INDEX / 文档目录</p>
            <h2>选择你现在要解决的问题</h2>
          </div>
          <label className="docs-index-search">
            <span className="material-symbols-outlined">search</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索标题或主题…" aria-label="搜索插件文档" />
          </label>
        </div>

        <div className="docs-featured">
          {filtered.filter((doc) => doc.featured).map((doc, index) => (
            <Link key={doc.slug} to={`/docs/${doc.slug}`} className="docs-feature-card">
              <span className="docs-feature-number">0{index + 1}</span>
              <span className="docs-feature-icon material-symbols-outlined">{index === 0 ? 'route' : 'help'}</span>
              <h3>{doc.title}</h3>
              <p>{doc.description}</p>
              <span className="docs-feature-link">打开文档 <span className="material-symbols-outlined">east</span></span>
            </Link>
          ))}
        </div>

        <div className="docs-groups">
          {DOC_GROUPS.map((group) => {
            const entries = filtered.filter((doc) => doc.group === group && !doc.featured);
            if (entries.length === 0) return null;
            return (
              <div className="docs-group" key={group}>
                <div className="docs-group-title">
                  <span>{group}</span>
                  <i />
                  <small>{String(entries.length).padStart(2, '0')}</small>
                </div>
                <div className="docs-group-links">
                  {entries.map((doc) => (
                    <Link key={doc.slug} to={`/docs/${doc.slug}`}>
                      <span className="docs-row-index">{String(DOC_CATALOG.indexOf(doc) + 1).padStart(2, '0')}</span>
                      <span className="docs-row-copy"><strong>{doc.title}</strong><small>{doc.description}</small></span>
                      <span className="material-symbols-outlined">north_east</span>
                    </Link>
                  ))}
                </div>
              </div>
            );
          })}
          {filtered.length === 0 && <div className="docs-no-results">没有匹配“{query}”的文档。</div>}
        </div>
      </section>

      <section className="max-w-7xl mx-auto px-6 pb-16">
        <div className="docs-index-callout">
          <div>
            <span className="docs-section-label">ONE SOURCE OF TRUTH</span>
            <h2>文档在这里，代码仍在 GoNavi。</h2>
            <p>插件文档、市场 API 与上架规范统一在市场仓库维护；SDK、宿主运行时代码与示例仍在 GoNavi 主仓。文中涉及源码的链接会明确跳转到对应仓库。</p>
          </div>
          <Link to="/docs/development" className="docs-secondary">查看开发与打包 <span className="material-symbols-outlined">arrow_forward</span></Link>
        </div>
      </section>
    </main>
  );
}
