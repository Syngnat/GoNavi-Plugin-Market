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

// ---- 网站账号与发布闭环（v2）----
// 契约见 docs/v2-api.md；字段与 GoNavi market 服务的 /api/v1/auth/web 与 /api/v1/publisher 保持一致。

export interface MarketAccount {
  id: string;
  login: string;
  /** publisher | admin；保留字符串以兼容后续角色 */
  role: string;
  /** gh-<login> 发布者命名空间，包内 pluginId 必须以它加英文句点开头 */
  publisherId: string;
  official: boolean;
}

export interface WebSession {
  authenticated: boolean;
  account?: MarketAccount;
  csrfToken?: string;
}

/** 预检返回的 manifest 摘要：只依赖站内展示所需字段，其余原样透传。 */
export interface PluginManifestSummary {
  id?: string;
  name?: string;
  version?: string;
  description?: string;
  category?: string;
  minHostVersion?: string;
  platforms?: string[];
  permissions?: string[];
  [key: string]: unknown;
}

export interface PackageInspection {
  canPublish: boolean;
  reason: string;
  pluginId: string;
  version: string;
  name: string;
  description: string;
  category: string;
  minHostVersion?: string;
  platforms: string[];
  sha256: string;
  sizeBytes: number;
  manifest: PluginManifestSummary;
}

/** GitHub Release 直发预检：复用包身份字段，并附带具体 Release 来源。 */
export interface ReleaseInspection extends PackageInspection {
  sourceUrl: string;
  releaseUrl: string;
  releaseTag: string;
  assetName: string;
  prerelease: boolean;
}

export type SubmissionState =
  | 'draft'
  | 'submitted'
  | 'validating'
  | 'pending_review'
  | 'published'
  | 'rejected'
  | 'yanked';

export interface SubmissionRecord {
  id: string;
  pluginId: string;
  version: string;
  state: SubmissionState | string;
  reason: string;
  sha256: string;
  createdAt: string;
  updatedAt: string;
  warning?: string;
}

export interface PublisherPlugin {
  id: string;
  name: string;
  latestVersion: string;
  latestState: SubmissionState | string;
  publishedVersion: string;
  sourceUrl: string;
}

export interface ListPage<T> {
  items: T[];
  nextCursor?: string;
}

export interface SourceUpdateResult {
  sourceUrl: string;
}
