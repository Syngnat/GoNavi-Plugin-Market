import { Link } from 'react-router-dom';
import type { PluginItem } from '../api/types';
import { formatCount, relativeTime } from '../lib/format';
import { InstallButton } from './InstallButton';

export function PluginRow({ item, icon }: { item: PluginItem; icon: string }) {
  return (
    <div className="p-5 flex flex-col md:flex-row items-start md:items-center justify-between hover:bg-paper/50 transition-colors gap-4">
      <div className="flex items-start space-x-4 min-w-0">
        <div className="w-10 h-10 rounded-lg bg-paper border border-hairline flex items-center justify-center text-copper shrink-0 mt-0.5">
          <span className="material-symbols-outlined fill-icon">{icon}</span>
        </div>
        <div className="space-y-1 min-w-0">
          <div className="flex items-center space-x-2 flex-wrap">
            <Link
              to={`/plugins/${encodeURIComponent(item.id)}`}
              className="font-headline font-bold text-ink text-base hover:text-copper transition-colors"
            >
              {item.name}
            </Link>
            <span className="text-xs text-muted font-mono">{item.id}</span>
          </div>
          <p className="text-muted text-sm line-clamp-1">{item.description}</p>
          <div className="flex items-center space-x-3 text-xs text-muted pt-1">
            {item.ratingAverage !== undefined && (
              <>
                <span className="flex items-center space-x-0.5 text-copper">
                  <span className="material-symbols-outlined text-xs fill-icon">star</span>
                  <span>{item.ratingAverage.toFixed(1)}</span>
                </span>
                <span>•</span>
              </>
            )}
            {item.downloadCount !== undefined && (
              <>
                <span>下载 {formatCount(item.downloadCount)}</span>
                <span>•</span>
              </>
            )}
            <span className="font-mono">{item.version}</span>
          </div>
        </div>
      </div>
      <div className="flex items-center space-x-4 self-end md:self-center shrink-0">
        <span className="text-xs text-muted font-mono hidden lg:inline">{relativeTime(item.updatedAt)}更新</span>
        <InstallButton item={item} variant="row" />
      </div>
    </div>
  );
}
