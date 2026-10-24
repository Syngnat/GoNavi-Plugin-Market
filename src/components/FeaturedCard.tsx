import { Link } from 'react-router-dom';
import type { PluginItem } from '../api/types';
import { formatCount, relativeTime } from '../lib/format';
import { InstallButton } from './InstallButton';

export function FeaturedCard({ item }: { item: PluginItem }) {
  return (
    <div className="lg:col-span-5 bg-card p-8 rounded-xl border border-hairline shadow-sm flex flex-col justify-between relative overflow-hidden">
      <div className="absolute top-0 right-0 w-32 h-32 bg-copper/5 rounded-bl-full pointer-events-none" />
      <div className="space-y-4">
        <div className="flex justify-between items-start">
          <div className="w-12 h-12 rounded-lg bg-paper border border-hairline flex items-center justify-center text-copper shadow-sm">
            <span className="material-symbols-outlined text-2xl fill-icon">monitoring</span>
          </div>
          <div className="flex items-center space-x-2">
            {item.official && (
              <span className="px-2.5 py-0.5 rounded text-xs font-medium bg-paper border border-hairline text-copper flex items-center space-x-1">
                <span className="material-symbols-outlined text-sm fill-icon">verified</span>
                <span>官方</span>
              </span>
            )}
            <span className="text-xs text-muted font-mono">{item.version}</span>
          </div>
        </div>
        <p className="text-xs text-muted font-mono">{item.id}</p>
        <h2 className="text-2xl font-headline font-bold tracking-tight">{item.name}</h2>
        <p className="text-muted text-sm leading-relaxed">{item.description}</p>
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted pt-1">
          <span className="flex items-center space-x-0.5 text-copper">
            <span className="material-symbols-outlined text-sm fill-icon">star</span>
            <span className="font-medium">{item.ratingAverage.toFixed(1)}</span>
            <span>({item.ratingCount})</span>
          </span>
          <span>•</span>
          <span>
            下载量 <span className="text-ink font-medium">{formatCount(item.downloadCount)}</span>
          </span>
          <span>•</span>
          <span>
            更新于 <span className="text-ink font-medium">{relativeTime(item.updatedAt)}</span>
          </span>
        </div>
      </div>
      <div className="space-y-3 pt-6">
        <InstallButton pluginId={item.id} variant="block" label="安装插件" />
        <div className="text-center">
          <Link
            to={`/plugins/${encodeURIComponent(item.id)}`}
            className="text-xs text-muted hover:text-copper transition-colors inline-flex items-center gap-1"
          >
            查看详情 <span className="material-symbols-outlined text-sm">arrow_forward</span>
          </Link>
        </div>
      </div>
    </div>
  );
}
