export const CATEGORIES = ['全部', '数据源', 'AI 供应商', '导出器', '技能', '面板', '命令', '运维'] as const;

export type Category = (typeof CATEGORIES)[number];

export function CategoryPills({ active, onChange }: { active: string; onChange: (next: Category) => void }) {
  return (
    <div className="flex flex-wrap gap-2 pt-2">
      {CATEGORIES.map((category) => {
        const selected = category === active;
        return (
          <button
            key={category}
            type="button"
            onClick={() => onChange(category)}
            className={
              selected
                ? 'px-4 py-1.5 rounded-full text-xs font-medium bg-copper text-white shadow-sm transition-all'
                : 'px-4 py-1.5 rounded-full text-xs font-medium bg-paper border border-hairline text-muted hover:text-ink hover:border-copper transition-all'
            }
          >
            {category}
          </button>
        );
      })}
    </div>
  );
}
