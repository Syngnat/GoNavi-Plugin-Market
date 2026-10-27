import { test } from 'node:test';
import { buildInstallDeepLink, githubSpecFromSourceUrl } from './deeplink.ts';
import type { PluginItem } from '../api/types';

// 运行：node --experimental-strip-types --test src/lib/deeplink.test.ts
// 覆盖安装源推导：服务端只返回 sourceUrl（https://github.com/owner/repo）时必须转成
// 客户端认得的 github:owner/repo，否则客户端会把插件 ID 当成 npm 包名。

function githubItem(overrides: Partial<PluginItem> = {}): PluginItem {
  return {
    id: 'gh-autumnnazi.log2sql',
    name: 'Log2SQL',
    description: '',
    author: 'AutumnNazi',
    publisherId: 'gh-autumnnazi',
    official: false,
    version: '1.0.0',
    platforms: ['windows'],
    updatedAt: '2026-10-27T00:00:00Z',
    sourceId: 'plugin-market-service',
    ...overrides,
  };
}

function paramsOf(item: PluginItem): URLSearchParams {
  const link = buildInstallDeepLink(item);
  return new URLSearchParams(link.slice(link.indexOf('?') + 1));
}

function expectEqual(actual: string | null, expected: string | null, label: string): void {
  if (actual !== expected) {
    throw new Error(`${label}：期望 ${JSON.stringify(expected)}，实际 ${JSON.stringify(actual)}`);
  }
}

test('sourceUrl 推导 github:owner/repo', () => {
  expectEqual(
    githubSpecFromSourceUrl('https://github.com/AutumnNazi/gonavi-log2sql'),
    'github:AutumnNazi/gonavi-log2sql',
    'sourceUrl 推导',
  );
  expectEqual(
    paramsOf(githubItem({ sourceUrl: 'https://github.com/AutumnNazi/gonavi-log2sql' })).get('spec'),
    'github:AutumnNazi/gonavi-log2sql',
    '深链包含推导出的 spec',
  );
});

test('忽略 .git 后缀与 releases/tree 尾巴', () => {
  expectEqual(
    githubSpecFromSourceUrl('https://github.com/AutumnNazi/gonavi-log2sql.git'),
    'github:AutumnNazi/gonavi-log2sql',
    '.git 后缀',
  );
  expectEqual(
    githubSpecFromSourceUrl('https://github.com/AutumnNazi/gonavi-log2sql/releases/tag/v1.0.0'),
    'github:AutumnNazi/gonavi-log2sql',
    'releases 尾巴',
  );
  expectEqual(
    githubSpecFromSourceUrl('https://github.com/AutumnNazi/gonavi-log2sql/tree/main/plugins'),
    'github:AutumnNazi/gonavi-log2sql',
    'tree 尾巴',
  );
});

test('显式 installSpec 优先于 sourceUrl', () => {
  expectEqual(
    paramsOf(
      githubItem({
        installSpec: 'npm:@gonavi/log2sql',
        sourceUrl: 'https://github.com/AutumnNazi/gonavi-log2sql',
      }),
    ).get('spec'),
    'npm:@gonavi/log2sql',
    'installSpec 优先',
  );
});

test('非 github.com / 非 https / 非法地址不生成 spec', () => {
  const invalid = [
    'https://gitlab.com/AutumnNazi/gonavi-log2sql',
    'https://www.github.com/AutumnNazi/gonavi-log2sql',
    'http://github.com/AutumnNazi/gonavi-log2sql',
    'https://github.com/AutumnNazi',
    'not-a-url',
    '',
  ];
  for (const sourceUrl of invalid) {
    expectEqual(githubSpecFromSourceUrl(sourceUrl), '', `非法来源 ${JSON.stringify(sourceUrl)}`);
    expectEqual(paramsOf(githubItem({ sourceUrl })).get('spec'), null, `深链不应带 spec：${sourceUrl}`);
  }
});

test('深链保留 id 与 version，且无来源时不传 spec', () => {
  const params = paramsOf(githubItem({ version: '2.1.0' }));
  expectEqual(params.get('id'), 'gh-autumnnazi.log2sql', 'id');
  expectEqual(params.get('version'), '2.1.0', 'version');
  expectEqual(params.get('spec'), null, '无 installSpec/sourceUrl 时不传 spec');
});
