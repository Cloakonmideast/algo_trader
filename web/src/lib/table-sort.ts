export type Order = "asc" | "desc";
export interface SortState<K extends string> {
  key: K;
  order: Order;
}

/** A click on a column header: the same column flips its order, a new one starts in `firstOrder(key)`. */
export function nextSort<K extends string>(cur: SortState<K>, key: K, firstOrder: (key: K) => Order): SortState<K> {
  if (cur.key === key) return { key, order: cur.order === "asc" ? "desc" : "asc" };
  return { key, order: firstOrder(key) };
}
