import type { Order } from "@/lib/table-sort";

/** A table header cell that sorts by its column when clicked. */
export function SortHeader({
  label,
  active,
  order,
  onClick,
  className = "",
}: {
  label: string;
  active: boolean;
  order: Order;
  onClick: () => void;
  className?: string;
}) {
  return (
    <th className={`px-3 py-2 ${className}`} aria-sort={active ? (order === "asc" ? "ascending" : "descending") : undefined}>
      <button type="button" onClick={onClick} className="uppercase tracking-wide hover:underline">
        {label}
        {active ? (order === "asc" ? " ↑" : " ↓") : ""}
      </button>
    </th>
  );
}
