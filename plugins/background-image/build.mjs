// Inlines the page UI into the manifest so the sandboxed page is self-contained.
// Usage: node plugins/background-image/build.mjs
import { readFileSync, writeFileSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const output = join(root, 'plugin-dist', 'background-image');

const manifest = JSON.parse(readFileSync(join(here, 'gonavi.plugin.json'), 'utf8'));
const template = readFileSync(join(here, 'ui', 'page.template.html'), 'utf8');
const styles = readFileSync(join(here, 'ui', 'styles.css'), 'utf8');
const app = readFileSync(join(here, 'ui', 'app.js'), 'utf8');
const locales = JSON.parse(readFileSync(join(here, 'ui', 'locales.json'), 'utf8'));

if (/<\/script/i.test(app) || /<\/script/i.test(styles)) {
  throw new Error('page sources must not contain a closing script tag');
}

const page = template
  .replace('/*__STYLES__*/', () => styles)
  .replace('/*__LOCALES__*/', () => 'window.__BG_LOCALES__ = ' + JSON.stringify(locales) + ';')
  .replace('/*__APP__*/', () => app);

if (new TextEncoder().encode(page).length > 256 * 1024) {
  throw new Error('page source exceeds the 256 KiB html-panel limit');
}

manifest.contributes.pages[0].html.source = page;

rmSync(output, { recursive: true, force: true });
mkdirSync(output, { recursive: true });
writeFileSync(join(output, 'gonavi.plugin.json'), JSON.stringify(manifest, null, 2) + '\n', 'utf8');
writeFileSync(join(output, 'README.md'), readFileSync(join(here, 'README.md'), 'utf8'), 'utf8');
writeFileSync(join(output, 'LICENSE'), readFileSync(join(here, 'LICENSE'), 'utf8'), 'utf8');
console.log('built', output, 'manifest bytes', new TextEncoder().encode(page).length);
