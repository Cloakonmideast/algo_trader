import { filterToParams } from "./filters";
import type { Signal, SignalFilter } from "./types";

// Mirrors EXPORT_CAP and MAX_PAGE_SIZE in lib/store/types.ts (client code can't import the store).
const EXPORT_CAP = 100_000;
const PAGE_SIZE = 1_000;

interface Page {
  rows: Signal[];
  total: number;
  error?: string;
}

async function getPage(filter: SignalFilter, page: number, fetchFn: typeof fetch): Promise<Page> {
  const params = filterToParams(filter);
  params.set("page", String(page));
  params.set("pageSize", String(PAGE_SIZE));
  const res = await fetchFn(`/api/signals?${params}`, { cache: "no-store" });
  const body = (await res.json().catch(() => ({}))) as Page;
  if (!res.ok) throw new Error(body.error ?? `Export failed (HTTP ${res.status}).`);
  return body;
}

/** Fetches every row matching the filter, one page at a time (Vercel caps each response at ~4.5 MB). */
export async function fetchAllSignals(
  filter: SignalFilter,
  fetchFn: typeof fetch = fetch,
  onProgress?: (done: number, total: number) => void,
): Promise<Signal[]> {
  const first = await getPage(filter, 1, fetchFn);
  if (first.total > EXPORT_CAP) {
    throw new Error(
      `${first.total.toLocaleString("en-US")} signals match — narrow the range to ${EXPORT_CAP.toLocaleString("en-US")} or fewer.`,
    );
  }
  const rows = [...first.rows];
  onProgress?.(rows.length, first.total);
  for (let page = 2; rows.length < first.total; page++) {
    const next = await getPage(filter, page, fetchFn);
    if (next.rows.length === 0) break;
    rows.push(...next.rows);
    onProgress?.(rows.length, first.total);
  }
  return rows;
}
