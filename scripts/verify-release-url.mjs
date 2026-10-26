import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('../src/api/v2.ts', import.meta.url), 'utf8');
const start = source.indexOf('export function isGithubReleaseUrl');
if (start < 0) throw new Error('isGithubReleaseUrl is missing');
const end = source.indexOf('\n}', start);
if (end < 0) throw new Error('isGithubReleaseUrl has no end');
const body = source
  .slice(start, end + 2)
  .replace('export function isGithubReleaseUrl', 'function isGithubReleaseUrl')
  .replace('(value: string): boolean', '(value)')
  .replace(/url: URL;/g, 'url;')
  .replace(/file is File/g, 'file');
const check = new Function(body + '\nreturn isGithubReleaseUrl;')();

const accepted = [
  'https://github.com/Syngnat/GoNavi-Plugin-Market',
  'https://github.com/Syngnat/GoNavi-Plugin-Market/releases/tag/nginx-inspector-v1.0.0',
  'https://github.com/Syngnat/GoNavi-Plugin-Market/releases/download/nginx-inspector-v1.0.0/plugin.zip',
];
const rejected = [
  'http://github.com/acme/tool',
  'https://gitlab.com/acme/tool',
  'https://github.com/acme/tool/tree/main/plugins/example',
  'https://github.com/acme/tool/issues',
  'https://user@github.com/acme/tool',
  'https://github.com:443/acme/tool',
  'https://github.com/acme/tool?ref=main',
  'https://github.com/acme/tool#readme',
];
for (const value of accepted) {
  if (!check(value)) throw new Error(`expected accepted URL: ${value}`);
}
for (const value of rejected) {
  if (check(value)) throw new Error(`expected rejected URL: ${value}`);
}
console.log(`release URL contract: ${accepted.length} accepted, ${rejected.length} rejected`);
