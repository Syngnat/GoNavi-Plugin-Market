import type { PluginDetail, PluginItem } from '../api/types';

// 上架演示数据，字段与 /api/v1 契约一致；服务端就绪后由真实目录替换。
const ago = (hours: number) => new Date(Date.now() - hours * 3600_000).toISOString();

const sha = (prefix: string) => (prefix + 'a83f1c27e94b05d6f2a7c318e04b95d6137fa2c8be50416d973e2b1c8da04f5'.slice(prefix.length)).slice(0, 64);

export const FEATURED_ID = 'com.gonavi.sql-insights';

const item = (partial: Omit<PluginItem, 'sourceId' | 'platforms' | 'author' | 'publisherId' | 'official'> & Partial<Pick<PluginItem, 'platforms' | 'author' | 'publisherId' | 'official'>>): PluginItem => ({
  sourceId: 'seed',
  platforms: ['windows', 'macos', 'linux'],
  author: 'GoNavi',
  publisherId: 'gonavi',
  official: true,
  ...partial,
});

export const seedCatalog: PluginItem[] = [
  item({
    id: 'com.gonavi.sql-insights',
    name: 'SQL 深度诊断',
    description: '实时捕获并分析慢查询，提供基于执行计划的智能索引优化建议，内置多维性能指标监控。支持 MySQL、PostgreSQL 与 OceanBase。',
    version: 'v1.4.2',
    category: '数据源',
    minHostVersion: '1.1.0',
    ratingAverage: 4.8,
    ratingCount: 328,
    downloadCount: 45200,
    updatedAt: ago(48),
    artifact: { url: '', sha256: sha('7f3a9c'), sizeBytes: 5_033_164, platform: 'all', version: 'v1.4.2' },
  }),
  item({
    id: 'com.gonavi.redis-inspector',
    name: 'Redis 键空间巡检',
    description: '深度扫描大 Key、热点 Key 与内存碎片率，提供一键清理与内存优化策略。',
    version: 'v2.1.0',
    category: '数据源',
    minHostVersion: '1.0.0',
    ratingAverage: 4.9,
    ratingCount: 121,
    downloadCount: 28400,
    updatedAt: ago(3),
    artifact: { url: '', sha256: sha('b21e08'), sizeBytes: 3_145_728, platform: 'all', version: 'v2.1.0' },
  }),
  item({
    id: 'com.gonavi.openrouter-bridge',
    name: 'OpenRouter 供应商桥接',
    description: '无缝接入百款大语言模型，统一管理 API 额度、Fallback 策略与延迟统计。',
    version: 'v1.8.5',
    category: 'AI 供应商',
    minHostVersion: '1.0.0',
    ratingAverage: 4.7,
    ratingCount: 98,
    downloadCount: 61900,
    updatedAt: ago(26),
    artifact: { url: '', sha256: sha('c94d72'), sizeBytes: 2_097_152, platform: 'all', version: 'v1.8.5' },
  }),
  item({
    id: 'com.gonavi.sql-audit',
    name: 'SQL 审计合规助手',
    description: '自动拦截高危 DDL/DML 操作，强制执行企业数据库变更规范与审批流。',
    version: 'v3.0.1',
    category: '命令',
    minHostVersion: '1.1.0',
    ratingAverage: 4.9,
    ratingCount: 87,
    downloadCount: 19300,
    updatedAt: ago(72),
    artifact: { url: '', sha256: sha('d05b6a'), sizeBytes: 4_194_304, platform: 'all', version: 'v3.0.1' },
  }),
  item({
    id: 'com.gonavi.excel-stream',
    name: 'Excel 流式导出器',
    description: '专为千万级大数据量设计的高性能流式 Excel 导出插件，零 OOM 崩溃风险。',
    version: 'v1.2.0',
    category: '导出器',
    minHostVersion: '1.0.0',
    ratingAverage: 4.6,
    ratingCount: 210,
    downloadCount: 34700,
    updatedAt: ago(120),
    artifact: { url: '', sha256: sha('e7f1b3'), sizeBytes: 6_291_456, platform: 'all', version: 'v1.2.0' },
  }),
  item({
    id: 'com.gonavi.masking-skill',
    name: '数据脱敏技能包',
    description: '基于正则与 NLP 的自动化敏感信息（手机号、身份证、密码）实时脱敏掩码组件。',
    version: 'v2.0.4',
    category: '技能',
    minHostVersion: '1.0.0',
    ratingAverage: 4.8,
    ratingCount: 176,
    downloadCount: 22100,
    updatedAt: ago(168),
    artifact: { url: '', sha256: sha('f3c2a9'), sizeBytes: 1_835_008, platform: 'all', version: 'v2.0.4' },
  }),
  item({
    id: 'com.gonavi.clickhouse-dash',
    name: 'ClickHouse 实时面板',
    description: '面向 OLAP 列式数据库的高频流式可视化看板，毫秒级响应海量日志检索。',
    version: 'v1.1.2',
    category: '面板',
    minHostVersion: '1.0.0',
    ratingAverage: 4.9,
    ratingCount: 64,
    downloadCount: 15800,
    updatedAt: ago(170),
    artifact: { url: '', sha256: sha('0a6e4d'), sizeBytes: 2_621_440, platform: 'all', version: 'v1.1.2' },
  }),
];

export const seedIcons: Record<string, string> = {
  'com.gonavi.sql-insights': 'monitoring',
  'com.gonavi.redis-inspector': 'storage',
  'com.gonavi.openrouter-bridge': 'hub',
  'com.gonavi.sql-audit': 'shield',
  'com.gonavi.excel-stream': 'table_chart',
  'com.gonavi.masking-skill': 'visibility_off',
  'com.gonavi.clickhouse-dash': 'bolt',
};

const sqlInsightsDetail: PluginDetail = {
  ...seedCatalog[0],
  license: 'Apache-2.0',
  features: [
    '把选中的 SQL 展开成执行计划，标出全表扫描、回表和未使用的索引。',
    '对照慢查询日志，给出一条可复制的索引建议，而不是一段笼统的提示。',
    '在独立面板里看扫描行数、耗时和锁等待，不把结果写回你的库。',
  ],
  permissions: [
    { name: '读取执行计划', key: 'query.explain', scope: '只读', granted: true },
    { name: '读取慢查询日志', key: 'query.slowlog', scope: '只读', granted: true },
    { name: '打开独立面板', key: 'window.open', scope: '界面', granted: true },
    { name: '写入或修改数据', key: '', scope: '未授予', granted: false },
  ],
  ratingDistribution: [78, 14, 5, 2, 1],
  reviews: [
    { author: '林夏', stars: 5, body: '把一条 12 秒的报表 SQL 指到了漏掉的联合索引，建议可以直接复制。' },
    { author: '周衡', stars: 5, body: 'OceanBase 的执行计划也能展开，权限清单里没有写库，放心装。' },
  ],
};

export function seedPluginDetail(id: string): PluginDetail | null {
  if (id === FEATURED_ID) return sqlInsightsDetail;
  const found = seedCatalog.find((entry) => entry.id === id);
  if (!found) return null;
  return { ...found, license: 'Apache-2.0', features: [], permissions: [], reviews: [] };
}
