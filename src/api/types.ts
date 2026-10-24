// 与 GoNavi 主仓库 internal/pluginmarket 的 JSON 契约保持同字段兼容。
// 评分/下载量等运营数据由市场服务（/api/v1）提供；静态目录阶段缺省，UI 不渲染。
// installSpec / contentDigest / features / permissions 是站点目录（catalog.json）的扩展字段。
export interface Artifact {
  url: string;
  sha256: string;
  sizeBytes: number;
  platform: string;
  version: string;
}

export interface PluginItem {
  id: string;
  name: string;
  description: string;
  author: string;
  publisherId: string;
  official: boolean;
  version: string;
  category?: string;
  minHostVersion?: string;
  platforms: string[];
  ratingAverage?: number;
  ratingCount?: number;
  downloadCount?: number;
  updatedAt: string;
  artifact?: Artifact | null;
  sourceId: string;
  /** 站点目录扩展字段 */
  icon?: string;
  featured?: boolean;
  installSpec?: string;
  repoUrl?: string;
  contentDigest?: string;
  license?: string;
  features?: string[];
  permissions?: PermissionEntry[];
  ratingDistribution?: number[];
  reviews?: PluginReview[];
}

export type PluginDetail = PluginItem;

export interface CatalogPage {
  items: PluginItem[];
  nextCursor?: string;
  stale: boolean;
  error?: string;
  cachedAt?: string;
  sourceId: string;
}

export interface DeviceAuthorization {
  verificationUri: string;
  userCode: string;
  deviceCode: string;
  intervalSeconds: number;
  expiresIn: number;
}

export interface LoginStatus {
  status: string;
}

export interface PermissionEntry {
  name: string;
  key: string;
  scope: string;
  granted: boolean;
}

export interface PluginReview {
  author: string;
  stars: number;
  body: string;
}
