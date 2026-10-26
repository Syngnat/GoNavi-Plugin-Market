import { readFileSync } from 'node:fs';
import { extname, resolve, sep } from 'node:path';
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';

const docsRoot = resolve(__dirname, 'docs');
const docFiles = new Map<string, string>([
  ['author-guide', 'plugins/author-guide.md'],
  ['faq', 'plugins/faq.md'],
  ['development', 'plugins/development.md'],
  ['troubleshooting', 'plugins/troubleshooting.md'],
  ['data-sources', 'plugins/data-sources.md'],
  ['ai-providers', 'plugins/ai-providers.md'],
  ['skills', 'plugins/skills.md'],
  ['exporters', 'plugins/exporters.md'],
  ['pages', 'plugins/pages.md'],
  ['ai-tools', 'plugins/ai-tools.md'],
  ['sdk', 'plugins/sdk.md'],
  ['protocol', 'plugins/protocol.md'],
  ['permissions', 'plugins/permissions.md'],
  ['data-source-contract', 'plugins/data-source-contract.md'],
  ['ai-provider-contract', 'plugins/ai-provider-contract.md'],
  ['skills-contract', 'plugins/skills-contract.md'],
  ['exporter-contract', 'plugins/exporter-contract.md'],
  ['host-log', 'plugins/host-log.md'],
  ['host-describe', 'plugins/host-describe.md'],
  ['lifecycle', 'plugins/lifecycle.md'],
  ['market-sources', 'plugins/market-sources.md'],
  ['market-api', 'market-admin-api.md'],
  ['market-deployment', 'market-deployment.md'],
  ['implementation-contract', 'plugins/implementation-contract.md'],
  ['market-auth', 'plugins/market-auth.md'],
  ['v2-api', 'v2-api.md'],
]);

function serveMarkdown(
  source: string,
  method: string,
  response: { statusCode: number; setHeader: (name: string, value: string) => void; end: (body?: string) => void },
) {
  const target = resolve(docsRoot, source);
  if (!target.startsWith(`${docsRoot}${sep}`) || extname(target) !== '.md') {
    response.statusCode = 404;
    response.end('Not Found');
    return;
  }
  try {
    response.statusCode = 200;
    response.setHeader('Content-Type', 'text/markdown; charset=utf-8');
    response.setHeader('Cache-Control', 'no-cache');
    response.end(method === 'HEAD' ? undefined : readFileSync(target));
  } catch {
    response.statusCode = 404;
    response.end('Not Found');
  }
}

function markdownDocsPlugin(): Plugin {
  const handleDocsRequest = (
    request: { url?: string; method?: string },
    response: { statusCode: number; setHeader: (name: string, value: string) => void; end: (body?: string) => void },
    next: () => void,
  ) => {
    const pathname = (request.url ?? '').split('?', 1)[0];
    if (!pathname.startsWith('/docs-source/') && !/^\/docs\/[^/]+\.md$/.test(pathname)) {
      next();
      return;
    }
    if ((request.method ?? 'GET') !== 'GET' && request.method !== 'HEAD') {
      response.statusCode = 405;
      response.end('Method Not Allowed');
      return;
    }
    if (pathname.startsWith('/docs/')) {
      const match = /^\/docs\/([^/]+)\.md$/.exec(pathname);
      const source = match ? docFiles.get(decodeURIComponent(match[1])) : undefined;
      if (!source) {
        response.statusCode = 404;
        response.end('Not Found');
        return;
      }
      serveMarkdown(source, request.method ?? 'GET', response);
      return;
    }
    let relativePath: string;
    try {
      relativePath = decodeURIComponent(pathname.slice('/docs-source/'.length));
    } catch {
      response.statusCode = 400;
      response.end('Bad Request');
      return;
    }
    serveMarkdown(relativePath, request.method ?? 'GET', response);
  };

  return {
    name: 'gonavi-markdown-docs',
    configureServer(server) {
      server.middlewares.use(handleDocsRequest);
    },
    configurePreviewServer(server) {
      server.middlewares.use(handleDocsRequest);
    },
    generateBundle() {
      for (const [slug, source] of docFiles) {
        this.emitFile({
          type: 'asset',
          fileName: `docs/${slug}.md`,
          source: readFileSync(resolve(docsRoot, source)),
        });
      }
    },
  };
}

export default defineConfig({
  plugins: [react(), markdownDocsPlugin()],
});
