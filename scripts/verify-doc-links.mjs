import { existsSync, readFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';

const root = resolve(new URL('..', import.meta.url).pathname);
const catalogSource = readFileSync(resolve(root, 'src/docs/catalog.ts'), 'utf8');
const sources = [...catalogSource.matchAll(/source:\s*'([^']+)'/g)].map((match) => match[1]);
const DOC_SOURCES = new Set(
  sources.map((source) => source.replace(/^docs\//, '')),
);
if (sources.length < 20) {
  throw new Error(`documentation catalog has only ${sources.length} entries; scanner probably stopped working`);
}

for (const source of sources) {
  if (!existsSync(resolve(root, source))) {
    throw new Error(`documentation catalog points to a missing source: ${source}`);
  }
}

const linkPattern = /\]\(<([^>]+)>\)|\]\(([^)\s]+)\)/g;
let scanned = 0;
let internal = 0;
let external = 0;
for (const source of sources) {
  const path = resolve(root, source);
  const body = readFileSync(path, 'utf8');
  for (const match of body.matchAll(linkPattern)) {
    const href = match[1] ?? match[2] ?? '';
    if (!href || href.startsWith('#') || href.startsWith('/docs/') || /^(https?:|mailto:)/i.test(href)) {
      external += 1;
      continue;
    }
    scanned += 1;
    const [target] = href.split('#');
    if (!target) continue;
    const destination = resolve(dirname(path), target.replace(/^<|>$/g, ''));
    if (existsSync(destination)) {
      internal += 1;
      continue;
    }
    // Source-code links intentionally point at the GoNavi repository. A docs-relative
    // Markdown link, however, must resolve inside this repository.
    if (
      /\.(go|ts|tsx|js|json|ya?ml|yml|toml|sh)$/.test(target) ||
      !DOC_SOURCES.has(target.replace(/^(\.\.\/)+/, '')) &&
      (target.includes('/examples/') || target.includes('/.github/') || target.includes('/internal/') || target.includes('/sdk/'))
    ) {
      continue;
    }
    throw new Error(`${relative(root, path)} links to a missing local document: ${href}`);
  }
}
if (scanned < 100) {
  throw new Error(`scanned only ${scanned} relative links; the link scanner probably stopped working`);
}
console.log(`documentation links: ${sources.length} sources, ${internal} local links, ${external} external or anchors`);
