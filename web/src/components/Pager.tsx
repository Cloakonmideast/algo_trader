export function Pager({ page, pages, href }: { page: number; pages: number; href: (page: number) => string }) {
  if (pages <= 1) return null;
  const btn = "rounded-md border border-slate-300 px-3 py-1.5 text-sm dark:border-slate-700";
  return (
    <div className="flex items-center justify-center gap-3">
      {page > 1 ? (
        <a href={href(page - 1)} className={btn}>
          ← Newer
        </a>
      ) : (
        <span className={`${btn} opacity-40`}>← Newer</span>
      )}
      <span className="text-sm tabular-nums text-slate-500">
        Page {page} of {pages}
      </span>
      {page < pages ? (
        <a href={href(page + 1)} className={btn}>
          Older →
        </a>
      ) : (
        <span className={`${btn} opacity-40`}>Older →</span>
      )}
    </div>
  );
}
