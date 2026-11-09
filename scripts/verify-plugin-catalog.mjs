import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const root = new URL('..', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1');
const catalog = JSON.parse(readFileSync(join(root, 'public', 'catalog.json'), 'utf8'));
const pluginsDir = join(root, 'plugins');
const failures = [];
const notes = [];

function semver(value) {
  const m = /^(\d+)\.(\d+)\.(\d+)/.exec(String(value || ''));
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}
function gte(a, b) {
  const x = semver(a);
  const y = semver(b);
  if (!x || !y) return false;
  for (let i = 0; i < 3; i += 1) {
    if (x[i] !== y[i]) return x[i] > y[i];
  }
  return true;
}

const manifests = new Map();
for (const name of readdirSync(pluginsDir)) {
  const file = join(pluginsDir, name, 'gonavi.plugin.json');
  if (!existsSync(file)) continue;
  try {
    manifests.set(name, JSON.parse(readFileSync(file, 'utf8')));
  } catch (err) {
    failures.push(`plugins/${name}/gonavi.plugin.json 无法解析: ${err.message}`);
  }
}

for (const item of catalog.items || []) {
  const candidates = [...manifests.entries()].filter(([, m]) => m.id === item.id);
  if (!candidates.length) continue;
  const [dir, manifest] = candidates[0];
  if (manifest.version !== item.version) {
    failures.push(`${item.id}: catalog ${item.version} != plugins/${dir} ${manifest.version}`);
  }
  const tag = `${dir}-v${item.version}`;
  if (!String(item.installSpec || '').endsWith('@' + tag)) {
    failures.push(`${item.id}: installSpec 未固定到 @${tag}，实际 ${item.installSpec}`);
  }
  if (item.artifact && item.artifact.version !== item.version) {
    failures.push(`${item.id}: artifact.version ${item.artifact.version} != ${item.version}`);
  }
  if (item.artifact) {
    const url = String(item.artifact.url || '');
    if (!url.includes(`/releases/download/${tag}/`)) {
      failures.push(`${item.id}: artifact.url 未指向 Release ${tag}: ${url}`);
    }
    if (!url.endsWith(`-${item.version}.zip`)) {
      failures.push(`${item.id}: artifact.url 文件名未以 -${item.version}.zip 结尾（gh release 用本地 basename 上传，改名会导致 404）: ${url}`);
    }
    if (!/^[0-9a-f]{64}$/.test(String(item.artifact.sha256 || ''))) {
      failures.push(`${item.id}: artifact.sha256 不是 64 位小写十六进制`);
    }
    if (!(Number(item.artifact.sizeBytes) > 0)) {
      failures.push(`${item.id}: artifact.sizeBytes 不是正数`);
    }
  }
  if (manifest.window && !(manifest.permissions || []).includes('ui.html-panel')) {
    failures.push(`${item.id}: 声明了 window 但 manifest permissions 缺少 ui.html-panel`);
  }
  if (item.minHostVersion !== manifest.minHostVersion) {
    notes.push(`${item.id}: catalog minHostVersion=${item.minHostVersion}，manifest=${manifest.minHostVersion}`);
  }
  if (manifest.minHostVersion && item.minHostVersion && !gte(manifest.minHostVersion, item.minHostVersion)) {
    failures.push(`${item.id}: manifest minHostVersion ${manifest.minHostVersion} 低于 catalog ${item.minHostVersion}`);
  }
}

if (failures.length) {
  console.error('插件目录校验失败:');
  for (const line of failures) console.error(' - ' + line);
  process.exit(1);
}
for (const line of notes) console.warn('提示: ' + line);
console.log(`plugin catalog check: ${manifests.size} manifest(s), ${(catalog.items || []).length} item(s), ok`);
