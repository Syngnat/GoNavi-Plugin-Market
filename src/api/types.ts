// 与 GoNavi 主仓库 internal/pluginmarket 的 JSON 契约保持同字段兼容；
// 后端就绪前，展示层扩展字段由种子数据提供。
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
  ratingAverage: number;
  ratingCount: number;
  downloadCount: number;
  updatedAt: string;
  artifact?: Artifact | null;
  sourceId: string;
}

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

export interface PluginDetail extends PluginItem {
  license?: string;
  features?: string[];
  permissions?: PermissionEntry[];
  /** 5→1 星的百分比，长度 5 */
  ratingDistribution?: number[];
  reviews?: PluginReview[];
}
